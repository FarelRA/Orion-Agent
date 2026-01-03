// Package media provides automatic media downloading and file management.
//
// MediaService downloads media files from WhatsApp messages and saves them to
// the local filesystem. It uses a worker pool for concurrent downloads with
// retry logic and duplicate detection.

package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/store"
	"orion-agent/internal/infra/config"
)

const (
	backoffFactor = 2.0
)

// MediaService handles automatic media downloading.
//
// It processes incoming messages and profile pictures, downloading media to
// local filesystem and tracking downloads in the media cache database.
type MediaService struct {
	client     *whatsmeow.Client
	config     *config.MediaConfig
	storePath  string
	mediaCache *store.MediaCacheStore
	log        waLog.Logger

	// Download queue - buffered channel for pending downloads
	queue    chan downloadJob
	wg       sync.WaitGroup
	stopOnce sync.Once
	stopCh   chan struct{}
}

// downloadJob represents a queued download task.
type downloadJob struct {
	// For message media
	MessageID string
	ChatJID   types.JID
	MediaType string
	Filename  string // Original filename (for documents)

	// Media download info (for WhatsApp CDN downloads)
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    int64
	Mimetype      string

	// For profile pictures (HTTP download)
	IsProfilePic bool
	JID          types.JID
	PicID        string
	PicURL       string
}

// NewMediaService creates a new MediaService.
//
// Parameters:
//   - client: WhatsApp client for downloading encrypted media
//   - cfg: Media configuration from config.json
//   - storePath: Base path for storing downloaded files
//   - mediaCache: Store for tracking downloaded files
//   - log: Logger instance
func NewMediaService(
	client *whatsmeow.Client,
	cfg *config.MediaConfig,
	storePath string,
	mediaCache *store.MediaCacheStore,
	log waLog.Logger,
) *MediaService {
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 3
	}

	return &MediaService{
		client:     client,
		config:     cfg,
		storePath:  storePath,
		mediaCache: mediaCache,
		log:        log.Sub("MediaService"),
		queue:      make(chan downloadJob, 100),
		stopCh:     make(chan struct{}),
	}
}

// SetClient updates the whatsmeow client.
func (s *MediaService) SetClient(client *whatsmeow.Client) {
	s.client = client
}

// Start starts the download workers.
//
// worker_count determines how many concurrent downloads can happen.
// Higher values = faster bulk downloads but more memory/bandwidth usage.
// Default is 3 workers.
func (s *MediaService) Start() {
	if !s.config.AutoDownload {
		s.log.Infof("Media auto-download is disabled")
		return
	}

	workerCount := s.config.WorkerCount
	if workerCount <= 0 {
		workerCount = 3
	}

	s.log.Infof("Starting %d download workers", workerCount)
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
}

// Stop stops the download workers gracefully.
func (s *MediaService) Stop() {
	s.stopOnce.Do(func() {
		s.log.Infof("Stopping media service...")
		close(s.stopCh)
		s.wg.Wait()
		s.log.Infof("Media service stopped")
	})
}

// QueueMessageMedia queues a message's media for download.
//
// Checks:
// - Auto-download enabled
// - Media exists (has direct path)
// - Type enabled in config
// - File size within limit
// - Not already downloaded (checks cache + filesystem)
// - View-once policy
func (s *MediaService) QueueMessageMedia(msg *store.Message) {
	if !s.config.AutoDownload || msg == nil {
		return
	}

	// Skip if no media
	if msg.MediaDirectPath == "" {
		return
	}

	// Skip view-once if not enabled in types
	if msg.IsViewOnce && !s.isTypeEnabled("view_once") {
		return
	}

	// Check file size limit
	if s.config.MaxFileSizeMB > 0 && msg.FileLength > int64(s.config.MaxFileSizeMB)*1024*1024 {
		s.log.Debugf("Skipping media %s: size %d exceeds limit", msg.ID, msg.FileLength)
		return
	}

	// Check if type is enabled
	mediaType := getMediaTypeFromMessageType(msg.MessageType)
	if !s.isTypeEnabled(mediaType) {
		return
	}

	// Check if already downloaded (database check)
	if s.isAlreadyDownloaded(msg.ID, msg.ChatJID) {
		s.log.Debugf("Media already downloaded: %s", msg.ID)
		return
	}

	select {
	case s.queue <- downloadJob{
		MessageID:     msg.ID,
		ChatJID:       msg.ChatJID,
		MediaType:     mediaType,
		Filename:      msg.DisplayName, // Original filename for documents
		DirectPath:    msg.MediaDirectPath,
		MediaKey:      msg.MediaKey,
		FileSHA256:    msg.FileSHA256,
		FileEncSHA256: msg.FileEncSHA256,
		FileLength:    msg.FileLength,
		Mimetype:      msg.Mimetype,
	}:
	default:
		s.log.Warnf("Download queue full, dropping media %s", msg.ID)
	}
}

// QueueProfilePicture queues a profile picture for download.
func (s *MediaService) QueueProfilePicture(jid types.JID, picID, picURL string) {
	if !s.config.AutoDownload || picURL == "" {
		return
	}

	if !s.isTypeEnabled("profile_picture") {
		return
	}

	// Check if already downloaded (by checking file existence)
	filePath := s.buildProfilePicPath(jid, picID)
	if _, err := os.Stat(filePath); err == nil {
		s.log.Debugf("Profile pic already exists: %s", filePath)
		return
	}

	select {
	case s.queue <- downloadJob{
		IsProfilePic: true,
		JID:          jid,
		PicID:        picID,
		PicURL:       picURL,
	}:
	default:
		s.log.Warnf("Download queue full, dropping profile pic for %s", jid)
	}
}

// isAlreadyDownloaded checks if media has already been downloaded.
func (s *MediaService) isAlreadyDownloaded(messageID string, chatJID types.JID) bool {
	if s.mediaCache == nil {
		return false
	}

	// Check database
	cached, err := s.mediaCache.Get(messageID, chatJID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if cached == nil {
		return false
	}

	// Verify file still exists
	if _, err := os.Stat(cached.LocalPath); err != nil {
		// File missing, delete cache entry and allow re-download
		s.mediaCache.Delete(messageID, chatJID)
		return false
	}

	return true
}

// buildProfilePicPath builds the file path for a profile picture.
func (s *MediaService) buildProfilePicPath(jid types.JID, picID string) string {
	jidDir := sanitizeJID(jid.String())
	filename := fmt.Sprintf("%s.jpg", picID)

	return filepath.Join(
		s.storePath,
		"media",
		jidDir,
		"profile",
		filename,
	)
}

// worker processes download jobs.
func (s *MediaService) worker(id int) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case job := <-s.queue:
			if job.IsProfilePic {
				s.downloadProfilePicWithRetry(job)
			} else {
				s.downloadMediaWithRetry(job)
			}
		}
	}
}

// downloadMediaWithRetry downloads media with exponential backoff retry.
func (s *MediaService) downloadMediaWithRetry(job downloadJob) {
	err := s.retryWithBackoff(func() error {
		return s.downloadMedia(job)
	})
	if err != nil {
		max := s.config.RetryMaxAttempts
		if max <= 0 {
			max = 3
		}
		s.log.Errorf("Failed to download media %s after %d retries: %v", job.MessageID, max, err)
	}
}

// downloadProfilePicWithRetry downloads profile pic with retry.
func (s *MediaService) downloadProfilePicWithRetry(job downloadJob) {
	err := s.retryWithBackoff(func() error {
		return s.downloadProfilePic(job)
	})
	if err != nil {
		max := s.config.RetryMaxAttempts
		if max <= 0 {
			max = 3
		}
		s.log.Errorf("Failed to download profile pic for %s after %d retries: %v", job.JID, max, err)
	}
}

// retryWithBackoff executes fn with exponential backoff.
func (s *MediaService) retryWithBackoff(fn func() error) error {
	var err error

	maxRetries := s.config.RetryMaxAttempts
	if maxRetries <= 0 {
		maxRetries = 3
	}

	initialWait := time.Duration(s.config.RetryInitialBackoffMs) * time.Millisecond
	if initialWait <= 0 {
		initialWait = 500 * time.Millisecond
	}

	maxWait := time.Duration(s.config.RetryMaxBackoffMs) * time.Millisecond
	if maxWait <= 0 {
		maxWait = 30 * time.Second
	}

	wait := initialWait

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		if attempt == maxRetries {
			break
		}

		s.log.Debugf("Download failed (attempt %d/%d): %v, retrying in %v", attempt, maxRetries, err, wait)

		select {
		case <-s.stopCh:
			return errors.New("service stopped")
		case <-time.After(wait):
		}

		wait = time.Duration(float64(wait) * backoffFactor)
		if wait > maxWait {
			wait = maxWait
		}
	}

	return err
}

// downloadMedia downloads encrypted message media.
func (s *MediaService) downloadMedia(job downloadJob) error {
	if s.client == nil {
		return errors.New("client not initialized")
	}

	// Build file path: {store}/media/{chatjid}/{messageid}/{type}/{filename}
	chatDir := sanitizeJID(job.ChatJID.String())
	filename := buildFilename(job)

	filePath := filepath.Join(
		s.storePath,
		"media",
		chatDir,
		job.MessageID,
		job.MediaType,
		filename,
	)

	// Create directory
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Double-check if already downloaded (race condition prevention)
	if _, err := os.Stat(filePath); err == nil {
		s.log.Debugf("Media already exists: %s", filePath)
		return nil
	}

	// Download using whatsmeow
	mediaType := whatsmeowMediaType(job.MediaType)
	data, err := s.client.DownloadMediaWithPath(
		context.Background(),
		job.DirectPath,
		job.FileEncSHA256,
		job.FileSHA256,
		job.MediaKey,
		int(job.FileLength),
		mediaType,
		"",
	)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	s.log.Infof("Downloaded media: %s (%d bytes)", filePath, len(data))

	// Update media cache
	if s.mediaCache != nil {
		if err := s.mediaCache.Put(&store.MediaCache{
			MessageID: job.MessageID,
			ChatJID:   job.ChatJID,
			MediaType: job.MediaType,
			LocalPath: filePath,
			FileSize:  int64(len(data)),
		}); err != nil {
			s.log.Warnf("Failed to update media cache for %s: %v", job.MessageID, err)
		}
	}

	return nil
}

// downloadProfilePic downloads a profile picture via HTTP.
func (s *MediaService) downloadProfilePic(job downloadJob) error {
	filePath := s.buildProfilePicPath(job.JID, job.PicID)

	// Create directory
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Double-check if already downloaded
	if _, err := os.Stat(filePath); err == nil {
		s.log.Debugf("Profile pic already exists: %s", filePath)
		return nil
	}

	// Download via HTTP with timeout
	timeout := time.Duration(s.config.DownloadTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", job.PicURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}

	// Read body with size limit
	limit := int64(s.config.MaxFileSizeMB) * 1024 * 1024
	if limit <= 0 {
		limit = 100 * 1024 * 1024 // 100MB default
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	s.log.Infof("Downloaded profile pic: %s (%d bytes)", filePath, len(data))
	return nil
}

// isTypeEnabled checks if a media type is enabled for download.
func (s *MediaService) isTypeEnabled(mediaType string) bool {
	if len(s.config.Types) == 0 {
		return true // All types enabled if none specified
	}
	for _, t := range s.config.Types {
		if t == mediaType {
			return true
		}
	}
	return false
}

// getMediaTypeFromMessageType converts message type to media type.
// Covers all downloadable media types from WhatsApp.
func getMediaTypeFromMessageType(msgType string) string {
	switch msgType {
	case "image":
		return "image"
	case "video", "ptv": // PTV = Push-to-Talk Video (video notes)
		return "video"
	case "audio", "ptt": // PTT = Push-to-Talk (voice messages)
		return "audio"
	case "document":
		return "document"
	case "sticker":
		return "sticker"
	default:
		return ""
	}
}

// whatsmeowMediaType converts our media type to whatsmeow media type.
func whatsmeowMediaType(mediaType string) whatsmeow.MediaType {
	switch mediaType {
	case "image":
		return whatsmeow.MediaImage
	case "video":
		return whatsmeow.MediaVideo
	case "audio":
		return whatsmeow.MediaAudio
	case "document":
		return whatsmeow.MediaDocument
	case "sticker":
		return whatsmeow.MediaImage // Stickers use image type
	default:
		return whatsmeow.MediaDocument
	}
}

// buildFilename creates a filename for the download.
// Uses original filename if available, otherwise generates from mimetype.
func buildFilename(job downloadJob) string {
	// Use original filename if available (documents, etc.)
	if job.Filename != "" {
		// Sanitize: remove path separators
		name := filepath.Base(job.Filename)
		// Ensure it has an extension
		if filepath.Ext(name) == "" {
			ext := getExtension(job.Mimetype)
			name += ext
		}
		return sanitizeFilename(name)
	}

	// Generate filename from mimetype
	ext := getExtension(job.Mimetype)
	return fmt.Sprintf("media%s", ext)
}

// sanitizeFilename removes unsafe characters from filename.
func sanitizeFilename(name string) string {
	// Replace problematic characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}

// sanitizeJID creates a filesystem-safe version of a JID.
func sanitizeJID(jid string) string {
	// Replace @ and : with safe characters
	s := strings.ReplaceAll(jid, "@", "_at_")
	s = strings.ReplaceAll(s, ":", "_")
	return s
}

// getExtension gets file extension from mimetype.
func getExtension(mimetype string) string {
	// Map common MIME types to extensions
	mimeMap := map[string]string{
		"image/jpeg":             ".jpg",
		"image/png":              ".png",
		"image/gif":              ".gif",
		"image/webp":             ".webp",
		"video/mp4":              ".mp4",
		"video/3gpp":             ".3gp",
		"video/quicktime":        ".mov",
		"audio/ogg":              ".ogg",
		"audio/ogg; codecs=opus": ".ogg",
		"audio/mpeg":             ".mp3",
		"audio/mp4":              ".m4a",
		"audio/aac":              ".aac",
		"application/pdf":        ".pdf",
		"application/zip":        ".zip",
		"application/msword":     ".doc",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
		"application/vnd.ms-excel": ".xls",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
		"application/vnd.ms-powerpoint":                                             ".ppt",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
		"text/plain": ".txt",
		"text/csv":   ".csv",
	}

	// Exact match
	if ext, ok := mimeMap[mimetype]; ok {
		return ext
	}

	// Prefix match
	for prefix, ext := range map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"video/mp4":  ".mp4",
		"video/":     ".mp4",
		"audio/ogg":  ".ogg",
		"audio/mpeg": ".mp3",
		"audio/":     ".m4a",
	} {
		if strings.HasPrefix(mimetype, prefix) {
			return ext
		}
	}

	return ""
}

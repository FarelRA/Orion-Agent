package send

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"  // GIF decoder
	_ "image/jpeg" // JPEG decoder
	_ "image/png"  // PNG decoder
	"orion-agent/internal/utils"

	"bytes"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// ImageContent represents an image message.
type ImageContent struct {
	Data          []byte
	MimeType      string
	Caption       string
	ViewOnce      bool
	Width         uint32
	Height        uint32
	ThumbnailJPEG []byte
	ContextInfo   *ContextInfo

	uploaded *whatsmeow.UploadResponse
}

// Image creates an image message.
func Image(data []byte, mimeType string) *ImageContent {
	img := &ImageContent{Data: data, MimeType: mimeType}
	img.detectDimensions()
	return img
}

// ImageWithCaption creates an image message with caption.
func ImageWithCaption(data []byte, mimeType, caption string) *ImageContent {
	img := &ImageContent{Data: data, MimeType: mimeType, Caption: caption}
	img.detectDimensions()
	return img
}

// detectDimensions attempts to detect image dimensions.
func (i *ImageContent) detectDimensions() {
	if len(i.Data) == 0 {
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(i.Data))
	if err == nil {
		i.Width = uint32(cfg.Width)
		i.Height = uint32(cfg.Height)
	}
}

// WithDimensions sets image dimensions manually.
func (i *ImageContent) WithDimensions(width, height uint32) *ImageContent {
	i.Width = width
	i.Height = height
	return i
}

// WithThumbnail sets the JPEG thumbnail.
func (i *ImageContent) WithThumbnail(jpeg []byte) *ImageContent {
	i.ThumbnailJPEG = jpeg
	return i
}

// AsViewOnce marks the image as view-once.
func (i *ImageContent) AsViewOnce() *ImageContent {
	i.ViewOnce = true
	return i
}

// WithContext adds context info.
func (i *ImageContent) WithContext(ctx *ContextInfo) *ImageContent {
	i.ContextInfo = ctx
	return i
}

// Upload implements MediaUploader.
func (i *ImageContent) Upload(ctx context.Context, client *whatsmeow.Client) error {
	if i.uploaded != nil {
		return nil
	}
	resp, err := client.Upload(ctx, i.Data, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	i.uploaded = &resp
	return nil
}

// IsUploaded implements MediaUploader.
func (i *ImageContent) IsUploaded() bool {
	return i.uploaded != nil
}

// ToMessage implements Content.
func (i *ImageContent) ToMessage() (*waE2E.Message, error) {
	if i.uploaded == nil {
		return nil, fmt.Errorf("image not uploaded")
	}

	img := &waE2E.ImageMessage{
		URL:           proto.String(i.uploaded.URL),
		DirectPath:    proto.String(i.uploaded.DirectPath),
		MediaKey:      i.uploaded.MediaKey,
		Mimetype:      proto.String(i.MimeType),
		FileEncSHA256: i.uploaded.FileEncSHA256,
		FileSHA256:    i.uploaded.FileSHA256,
		FileLength:    proto.Uint64(i.uploaded.FileLength),
	}

	if i.Width > 0 {
		img.Width = proto.Uint32(i.Width)
	}
	if i.Height > 0 {
		img.Height = proto.Uint32(i.Height)
	}
	if len(i.ThumbnailJPEG) > 0 {
		img.JPEGThumbnail = i.ThumbnailJPEG
	}
	if i.Caption != "" {
		img.Caption = proto.String(i.Caption)
	}
	if i.ViewOnce {
		img.ViewOnce = proto.Bool(true)
	}
	if i.ContextInfo != nil {
		img.ContextInfo = i.ContextInfo.Build()
	}

	return &waE2E.Message{ImageMessage: img}, nil
}

// MediaType implements Content.
func (i *ImageContent) MediaType() utils.Type {
	return utils.TypeImage
}

// VideoContent represents a video message.
type VideoContent struct {
	Data            []byte
	MimeType        string
	Caption         string
	DurationSeconds uint32
	Width           uint32
	Height          uint32
	ThumbnailJPEG   []byte
	GifPlayback     bool
	ViewOnce        bool
	IsPTV           bool
	ContextInfo     *ContextInfo

	uploaded *whatsmeow.UploadResponse
}

// Video creates a video message.
func Video(data []byte, mimeType string, duration uint32) *VideoContent {
	return &VideoContent{Data: data, MimeType: mimeType, DurationSeconds: duration}
}

// VideoWithCaption creates a video message with caption.
func VideoWithCaption(data []byte, mimeType, caption string, duration uint32) *VideoContent {
	return &VideoContent{Data: data, MimeType: mimeType, Caption: caption, DurationSeconds: duration}
}

// WithDimensions sets video dimensions.
func (v *VideoContent) WithDimensions(width, height uint32) *VideoContent {
	v.Width = width
	v.Height = height
	return v
}

// WithThumbnail sets the JPEG thumbnail.
func (v *VideoContent) WithThumbnail(jpeg []byte) *VideoContent {
	v.ThumbnailJPEG = jpeg
	return v
}

// AsGIF marks the video as GIF playback.
func (v *VideoContent) AsGIF() *VideoContent {
	v.GifPlayback = true
	return v
}

// AsViewOnce marks the video as view-once.
func (v *VideoContent) AsViewOnce() *VideoContent {
	v.ViewOnce = true
	return v
}

// AsPTV marks the video as push-to-talk video (video note).
func (v *VideoContent) AsPTV() *VideoContent {
	v.IsPTV = true
	return v
}

// WithContext adds context info.
func (v *VideoContent) WithContext(ctx *ContextInfo) *VideoContent {
	v.ContextInfo = ctx
	return v
}

// Upload implements MediaUploader.
func (v *VideoContent) Upload(ctx context.Context, client *whatsmeow.Client) error {
	if v.uploaded != nil {
		return nil
	}
	resp, err := client.Upload(ctx, v.Data, whatsmeow.MediaVideo)
	if err != nil {
		return err
	}
	v.uploaded = &resp
	return nil
}

// IsUploaded implements MediaUploader.
func (v *VideoContent) IsUploaded() bool {
	return v.uploaded != nil
}

// ToMessage implements Content.
func (v *VideoContent) ToMessage() (*waE2E.Message, error) {
	if v.uploaded == nil {
		return nil, fmt.Errorf("video not uploaded")
	}

	vid := &waE2E.VideoMessage{
		URL:           proto.String(v.uploaded.URL),
		DirectPath:    proto.String(v.uploaded.DirectPath),
		MediaKey:      v.uploaded.MediaKey,
		Mimetype:      proto.String(v.MimeType),
		FileEncSHA256: v.uploaded.FileEncSHA256,
		FileSHA256:    v.uploaded.FileSHA256,
		FileLength:    proto.Uint64(v.uploaded.FileLength),
		Seconds:       proto.Uint32(v.DurationSeconds),
	}

	if v.Width > 0 {
		vid.Width = proto.Uint32(v.Width)
	}
	if v.Height > 0 {
		vid.Height = proto.Uint32(v.Height)
	}
	if len(v.ThumbnailJPEG) > 0 {
		vid.JPEGThumbnail = v.ThumbnailJPEG
	}
	if v.Caption != "" {
		vid.Caption = proto.String(v.Caption)
	}
	if v.GifPlayback {
		vid.GifPlayback = proto.Bool(true)
	}
	if v.ViewOnce {
		vid.ViewOnce = proto.Bool(true)
	}
	if v.ContextInfo != nil {
		vid.ContextInfo = v.ContextInfo.Build()
	}

	msg := &waE2E.Message{VideoMessage: vid}

	// PTV uses a different field
	if v.IsPTV {
		msg.VideoMessage = nil
		msg.PtvMessage = vid
	}

	return msg, nil
}

// MediaType implements Content.
func (v *VideoContent) MediaType() utils.Type {
	return utils.TypeVideo
}

// AudioContent represents an audio message.
type AudioContent struct {
	Data            []byte
	MimeType        string
	DurationSeconds uint32
	IsPTT           bool
	Waveform        []byte
	ContextInfo     *ContextInfo

	uploaded *whatsmeow.UploadResponse
}

// Audio creates an audio message.
func Audio(data []byte, mimeType string, duration uint32) *AudioContent {
	return &AudioContent{Data: data, MimeType: mimeType, DurationSeconds: duration}
}

// VoiceNote creates a voice note (PTT audio).
func VoiceNote(data []byte, duration uint32) *AudioContent {
	return &AudioContent{
		Data:            data,
		MimeType:        "audio/ogg; codecs=opus",
		DurationSeconds: duration,
		IsPTT:           true,
	}
}

// AsPTT marks the audio as a voice note.
func (a *AudioContent) AsPTT() *AudioContent {
	a.IsPTT = true
	return a
}

// WithWaveform adds waveform visualization data.
func (a *AudioContent) WithWaveform(waveform []byte) *AudioContent {
	a.Waveform = waveform
	return a
}

// WithContext adds context info.
func (a *AudioContent) WithContext(ctx *ContextInfo) *AudioContent {
	a.ContextInfo = ctx
	return a
}

// Upload implements MediaUploader.
func (a *AudioContent) Upload(ctx context.Context, client *whatsmeow.Client) error {
	if a.uploaded != nil {
		return nil
	}
	resp, err := client.Upload(ctx, a.Data, whatsmeow.MediaAudio)
	if err != nil {
		return err
	}
	a.uploaded = &resp
	return nil
}

// IsUploaded implements MediaUploader.
func (a *AudioContent) IsUploaded() bool {
	return a.uploaded != nil
}

// ToMessage implements Content.
func (a *AudioContent) ToMessage() (*waE2E.Message, error) {
	if a.uploaded == nil {
		return nil, fmt.Errorf("audio not uploaded")
	}

	aud := &waE2E.AudioMessage{
		URL:           proto.String(a.uploaded.URL),
		DirectPath:    proto.String(a.uploaded.DirectPath),
		MediaKey:      a.uploaded.MediaKey,
		Mimetype:      proto.String(a.MimeType),
		FileEncSHA256: a.uploaded.FileEncSHA256,
		FileSHA256:    a.uploaded.FileSHA256,
		FileLength:    proto.Uint64(a.uploaded.FileLength),
		Seconds:       proto.Uint32(a.DurationSeconds),
		PTT:           proto.Bool(a.IsPTT),
	}

	if len(a.Waveform) > 0 {
		aud.Waveform = a.Waveform
	}
	if a.ContextInfo != nil {
		aud.ContextInfo = a.ContextInfo.Build()
	}

	return &waE2E.Message{AudioMessage: aud}, nil
}

// MediaType implements Content.
func (a *AudioContent) MediaType() utils.Type {
	return utils.TypeAudio
}

// DocumentContent represents a document message.
type DocumentContent struct {
	Data          []byte
	MimeType      string
	Filename      string
	Title         string
	Caption       string
	PageCount     uint32
	ThumbnailJPEG []byte
	ContextInfo   *ContextInfo

	uploaded *whatsmeow.UploadResponse
}

// Document creates a document message.
func Document(data []byte, mimeType, filename string) *DocumentContent {
	return &DocumentContent{Data: data, MimeType: mimeType, Filename: filename}
}

// DocumentWithCaption creates a document message with caption.
func DocumentWithCaption(data []byte, mimeType, filename, caption string) *DocumentContent {
	return &DocumentContent{Data: data, MimeType: mimeType, Filename: filename, Caption: caption}
}

// WithTitle sets the document title.
func (d *DocumentContent) WithTitle(title string) *DocumentContent {
	d.Title = title
	return d
}

// WithPageCount sets the page count (for PDFs).
func (d *DocumentContent) WithPageCount(count uint32) *DocumentContent {
	d.PageCount = count
	return d
}

// WithThumbnail sets the JPEG thumbnail.
func (d *DocumentContent) WithThumbnail(jpeg []byte) *DocumentContent {
	d.ThumbnailJPEG = jpeg
	return d
}

// WithContext adds context info.
func (d *DocumentContent) WithContext(ctx *ContextInfo) *DocumentContent {
	d.ContextInfo = ctx
	return d
}

// Upload implements MediaUploader.
func (d *DocumentContent) Upload(ctx context.Context, client *whatsmeow.Client) error {
	if d.uploaded != nil {
		return nil
	}
	resp, err := client.Upload(ctx, d.Data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	d.uploaded = &resp
	return nil
}

// IsUploaded implements MediaUploader.
func (d *DocumentContent) IsUploaded() bool {
	return d.uploaded != nil
}

// ToMessage implements Content.
func (d *DocumentContent) ToMessage() (*waE2E.Message, error) {
	if d.uploaded == nil {
		return nil, fmt.Errorf("document not uploaded")
	}

	doc := &waE2E.DocumentMessage{
		URL:           proto.String(d.uploaded.URL),
		DirectPath:    proto.String(d.uploaded.DirectPath),
		MediaKey:      d.uploaded.MediaKey,
		Mimetype:      proto.String(d.MimeType),
		FileEncSHA256: d.uploaded.FileEncSHA256,
		FileSHA256:    d.uploaded.FileSHA256,
		FileLength:    proto.Uint64(d.uploaded.FileLength),
		FileName:      proto.String(d.Filename),
	}

	if d.Title != "" {
		doc.Title = proto.String(d.Title)
	}
	if d.PageCount > 0 {
		doc.PageCount = proto.Uint32(d.PageCount)
	}
	if len(d.ThumbnailJPEG) > 0 {
		doc.JPEGThumbnail = d.ThumbnailJPEG
	}
	if d.Caption != "" {
		doc.Caption = proto.String(d.Caption)
	}
	if d.ContextInfo != nil {
		doc.ContextInfo = d.ContextInfo.Build()
	}

	return &waE2E.Message{DocumentMessage: doc}, nil
}

// MediaType implements Content.
func (d *DocumentContent) MediaType() utils.Type {
	return utils.TypeDocument
}

// StickerContent represents a sticker message.
type StickerContent struct {
	Data        []byte
	MimeType    string
	Width       uint32
	Height      uint32
	IsAnimated  bool
	IsLottie    bool
	ContextInfo *ContextInfo

	uploaded *whatsmeow.UploadResponse
}

// Sticker creates a sticker message.
func Sticker(data []byte) *StickerContent {
	return &StickerContent{Data: data, MimeType: "image/webp", Width: 512, Height: 512}
}

// AnimatedSticker creates an animated sticker message.
func AnimatedSticker(data []byte) *StickerContent {
	return &StickerContent{Data: data, MimeType: "image/webp", Width: 512, Height: 512, IsAnimated: true}
}

// LottieSticker creates a Lottie sticker message.
func LottieSticker(data []byte) *StickerContent {
	return &StickerContent{Data: data, MimeType: "application/x-lottiesticker", IsLottie: true}
}

// WithDimensions sets sticker dimensions.
func (s *StickerContent) WithDimensions(width, height uint32) *StickerContent {
	s.Width = width
	s.Height = height
	return s
}

// WithContext adds context info.
func (s *StickerContent) WithContext(ctx *ContextInfo) *StickerContent {
	s.ContextInfo = ctx
	return s
}

// Upload implements MediaUploader.
func (s *StickerContent) Upload(ctx context.Context, client *whatsmeow.Client) error {
	if s.uploaded != nil {
		return nil
	}
	resp, err := client.Upload(ctx, s.Data, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	s.uploaded = &resp
	return nil
}

// IsUploaded implements MediaUploader.
func (s *StickerContent) IsUploaded() bool {
	return s.uploaded != nil
}

// ToMessage implements Content.
func (s *StickerContent) ToMessage() (*waE2E.Message, error) {
	if s.uploaded == nil {
		return nil, fmt.Errorf("sticker not uploaded")
	}

	sticker := &waE2E.StickerMessage{
		URL:           proto.String(s.uploaded.URL),
		DirectPath:    proto.String(s.uploaded.DirectPath),
		MediaKey:      s.uploaded.MediaKey,
		Mimetype:      proto.String(s.MimeType),
		FileEncSHA256: s.uploaded.FileEncSHA256,
		FileSHA256:    s.uploaded.FileSHA256,
		FileLength:    proto.Uint64(s.uploaded.FileLength),
		Width:         proto.Uint32(s.Width),
		Height:        proto.Uint32(s.Height),
		IsAnimated:    proto.Bool(s.IsAnimated),
		IsLottie:      proto.Bool(s.IsLottie),
	}

	if s.ContextInfo != nil {
		sticker.ContextInfo = s.ContextInfo.Build()
	}

	return &waE2E.Message{StickerMessage: sticker}, nil
}

// MediaType implements Content.
func (s *StickerContent) MediaType() utils.Type {
	return utils.TypeSticker
}

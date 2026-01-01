package media

import (
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
)

// Type represents a media type.
type Type string

const (
	TypeImage    Type = "image"
	TypeVideo    Type = "video"
	TypeAudio    Type = "audio"
	TypeDocument Type = "document"
	TypeSticker  Type = "sticker"
	TypeUnknown  Type = "unknown"
)

// WhatsmeowType maps our Type to whatsmeow.MediaType.
func (t Type) WhatsmeowType() whatsmeow.MediaType {
	switch t {
	case TypeImage:
		return whatsmeow.MediaImage
	case TypeVideo:
		return whatsmeow.MediaVideo
	case TypeAudio:
		return whatsmeow.MediaAudio
	case TypeDocument:
		return whatsmeow.MediaDocument
	case TypeSticker:
		return whatsmeow.MediaImage // Stickers use image type
	default:
		return whatsmeow.MediaDocument
	}
}

// FromMimeType detects media type from MIME type.
func FromMimeType(mime string) Type {
	mime = strings.ToLower(mime)

	switch {
	case strings.HasPrefix(mime, "image/webp"):
		return TypeSticker
	case strings.HasPrefix(mime, "image/"):
		return TypeImage
	case strings.HasPrefix(mime, "video/"):
		return TypeVideo
	case strings.HasPrefix(mime, "audio/"):
		return TypeAudio
	default:
		return TypeDocument
	}
}

// FromExtension detects media type from file extension.
func FromExtension(filename string) Type {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		return TypeImage
	case ".webp":
		return TypeSticker
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".3gp":
		return TypeVideo
	case ".mp3", ".ogg", ".wav", ".m4a", ".aac", ".opus", ".flac":
		return TypeAudio
	default:
		return TypeDocument
	}
}

// IsMedia returns true if the type is a media type (not document).
func IsMedia(t Type) bool {
	return t == TypeImage || t == TypeVideo || t == TypeAudio || t == TypeSticker
}

// SupportedImageExts returns supported image extensions.
func SupportedImageExts() []string {
	return []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
}

// SupportedVideoExts returns supported video extensions.
func SupportedVideoExts() []string {
	return []string{".mp4", ".mov", ".avi", ".mkv", ".3gp"}
}

// SupportedAudioExts returns supported audio extensions.
func SupportedAudioExts() []string {
	return []string{".mp3", ".ogg", ".wav", ".m4a", ".aac", ".opus"}
}

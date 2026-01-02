package send

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"orion-agent/internal/utils/media"
)

// LocationContent represents a location message.
type LocationContent struct {
	Latitude    float64
	Longitude   float64
	Name        string
	Address     string
	URL         string
	Comment     string
	ContextInfo *ContextInfo
}

// Location creates a location message.
func Location(lat, lng float64, name, address string) *LocationContent {
	return &LocationContent{
		Latitude:  lat,
		Longitude: lng,
		Name:      name,
		Address:   address,
	}
}

// LocationWithURL creates a location message with a maps URL.
func LocationWithURL(lat, lng float64, name, address, url string) *LocationContent {
	return &LocationContent{
		Latitude:  lat,
		Longitude: lng,
		Name:      name,
		Address:   address,
		URL:       url,
	}
}

// WithComment adds a comment to the location.
func (l *LocationContent) WithComment(comment string) *LocationContent {
	l.Comment = comment
	return l
}

// WithContext adds context info.
func (l *LocationContent) WithContext(ctx *ContextInfo) *LocationContent {
	l.ContextInfo = ctx
	return l
}

// ToMessage implements Content.
func (l *LocationContent) ToMessage() (*waE2E.Message, error) {
	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(l.Latitude),
		DegreesLongitude: proto.Float64(l.Longitude),
	}

	if l.Name != "" {
		loc.Name = proto.String(l.Name)
	}
	if l.Address != "" {
		loc.Address = proto.String(l.Address)
	}
	if l.URL != "" {
		loc.URL = proto.String(l.URL)
	}
	if l.Comment != "" {
		loc.Comment = proto.String(l.Comment)
	}
	if l.ContextInfo != nil {
		loc.ContextInfo = l.ContextInfo.Build()
	}

	return &waE2E.Message{LocationMessage: loc}, nil
}

// MediaType implements Content.
func (l *LocationContent) MediaType() media.Type {
	return "" // Not a media message
}

// LiveLocationContent represents a live location message.
type LiveLocationContent struct {
	Latitude         float64
	Longitude        float64
	AccuracyInMeters uint32
	SpeedInMps       float32
	Heading          uint32
	Caption          string
	SequenceNumber   int64
	ContextInfo      *ContextInfo
}

// LiveLocation creates a live location message.
func LiveLocation(lat, lng float64, accuracy uint32) *LiveLocationContent {
	return &LiveLocationContent{
		Latitude:         lat,
		Longitude:        lng,
		AccuracyInMeters: accuracy,
	}
}

// WithSpeed sets the speed in meters per second.
func (l *LiveLocationContent) WithSpeed(mps float32) *LiveLocationContent {
	l.SpeedInMps = mps
	return l
}

// WithHeading sets the heading (degrees clockwise from magnetic north).
func (l *LiveLocationContent) WithHeading(degrees uint32) *LiveLocationContent {
	l.Heading = degrees
	return l
}

// WithCaption adds a caption.
func (l *LiveLocationContent) WithCaption(caption string) *LiveLocationContent {
	l.Caption = caption
	return l
}

// WithSequence sets the sequence number for live location updates.
func (l *LiveLocationContent) WithSequence(seq int64) *LiveLocationContent {
	l.SequenceNumber = seq
	return l
}

// WithContext adds context info.
func (l *LiveLocationContent) WithContext(ctx *ContextInfo) *LiveLocationContent {
	l.ContextInfo = ctx
	return l
}

// ToMessage implements Content.
func (l *LiveLocationContent) ToMessage() (*waE2E.Message, error) {
	loc := &waE2E.LiveLocationMessage{
		DegreesLatitude:  proto.Float64(l.Latitude),
		DegreesLongitude: proto.Float64(l.Longitude),
	}

	if l.AccuracyInMeters > 0 {
		loc.AccuracyInMeters = proto.Uint32(l.AccuracyInMeters)
	}
	if l.SpeedInMps > 0 {
		loc.SpeedInMps = proto.Float32(l.SpeedInMps)
	}
	if l.Heading > 0 {
		loc.DegreesClockwiseFromMagneticNorth = proto.Uint32(l.Heading)
	}
	if l.Caption != "" {
		loc.Caption = proto.String(l.Caption)
	}
	if l.SequenceNumber > 0 {
		loc.SequenceNumber = proto.Int64(l.SequenceNumber)
	}
	if l.ContextInfo != nil {
		loc.ContextInfo = l.ContextInfo.Build()
	}

	return &waE2E.Message{LiveLocationMessage: loc}, nil
}

// MediaType implements Content.
func (l *LiveLocationContent) MediaType() media.Type {
	return "" // Not a media message
}

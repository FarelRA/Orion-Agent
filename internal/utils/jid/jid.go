package jid

import (
	"strings"

	"go.mau.fi/whatsmeow/types"
)

// Parse parses a JID string into types.JID.
func Parse(jidStr string) (types.JID, error) {
	return types.ParseJID(jidStr)
}

// MustParse parses a JID string, panics on error.
func MustParse(jidStr string) types.JID {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		panic(err)
	}
	return jid
}

// FromPhone creates a user JID from a phone number.
func FromPhone(phone string) types.JID {
	// Remove any non-digit characters
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	return types.JID{
		User:   cleaned,
		Server: types.DefaultUserServer,
	}
}

// IsUser returns true if the JID is a user (not group/newsletter).
func IsUser(jid types.JID) bool {
	return jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer
}

// IsGroup returns true if the JID is a group.
func IsGroup(jid types.JID) bool {
	return jid.Server == types.GroupServer
}

// IsNewsletter returns true if the JID is a newsletter/channel.
func IsNewsletter(jid types.JID) bool {
	return jid.Server == types.NewsletterServer
}

// IsBroadcast returns true if the JID is a broadcast list.
func IsBroadcast(jid types.JID) bool {
	return jid.Server == types.BroadcastServer
}

// IsStatus returns true if the JID is a status update.
func IsStatus(jid types.JID) bool {
	return jid.User == "status" && jid.Server == types.BroadcastServer
}

// IsLID returns true if this JID is a LID (local identifier).
func IsLID(jid types.JID) bool {
	return jid.Server == types.HiddenUserServer
}

// IsPN returns true if this JID is a PN (phone number).
func IsPN(jid types.JID) bool {
	return jid.Server == types.DefaultUserServer && jid.User != ""
}

// ToUserJID strips device info and returns the base user JID.
func ToUserJID(jid types.JID) types.JID {
	return types.JID{
		User:   jid.User,
		Server: jid.Server,
	}
}

// String returns the string representation of a JID.
func String(jid types.JID) string {
	return jid.String()
}

// IsEmpty returns true if the JID is empty.
func IsEmpty(jid types.JID) bool {
	return jid.IsEmpty()
}

// Equals compares two JIDs for equality (ignoring device).
func Equals(a, b types.JID) bool {
	return a.User == b.User && a.Server == b.Server
}

// EqualsStrict compares two JIDs including device info.
func EqualsStrict(a, b types.JID) bool {
	return a == b
}

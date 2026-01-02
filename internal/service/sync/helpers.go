// Package sync provides helper functions for sync operations.
package sync

import (
	"go.mau.fi/whatsmeow/types"
)

// FilterUserJIDs filters JIDs to only include user JIDs (not groups/newsletters).
func FilterUserJIDs(jids []types.JID) []types.JID {
	result := make([]types.JID, 0, len(jids))
	for _, jid := range jids {
		if jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer {
			result = append(result, jid)
		}
	}
	return result
}

// FilterGroupJIDs filters JIDs to only include group JIDs.
func FilterGroupJIDs(jids []types.JID) []types.JID {
	result := make([]types.JID, 0, len(jids))
	for _, jid := range jids {
		if jid.Server == types.GroupServer {
			result = append(result, jid)
		}
	}
	return result
}

// DeduplicateJIDs removes duplicate JIDs from a slice.
func DeduplicateJIDs(jids []types.JID) []types.JID {
	seen := make(map[string]bool)
	result := make([]types.JID, 0, len(jids))
	for _, jid := range jids {
		key := jid.String()
		if !seen[key] {
			seen[key] = true
			result = append(result, jid)
		}
	}
	return result
}

// ChunkJIDs splits a slice of JIDs into chunks of specified size.
func ChunkJIDs(jids []types.JID, chunkSize int) [][]types.JID {
	if chunkSize <= 0 {
		return nil
	}
	var chunks [][]types.JID
	for i := 0; i < len(jids); i += chunkSize {
		end := i + chunkSize
		if end > len(jids) {
			end = len(jids)
		}
		chunks = append(chunks, jids[i:end])
	}
	return chunks
}

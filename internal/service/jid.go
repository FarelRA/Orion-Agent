package service

import (
	"context"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/store"
	"orion-agent/internal/utils/jid"
)

// JIDService handles PN to LID conversion with caching.
type JIDService struct {
	client       *whatsmeow.Client
	contactStore *store.ContactStore
	log          waLog.Logger
}

// NewJIDService creates a new JIDService.
func NewJIDService(client *whatsmeow.Client, contactStore *store.ContactStore, log waLog.Logger) *JIDService {
	return &JIDService{
		client:       client,
		contactStore: contactStore,
		log:          log.Sub("JIDService"),
	}
}

// SetClient sets the WhatsApp client (for delayed initialization).
func (s *JIDService) SetClient(client *whatsmeow.Client) {
	s.client = client
}

// ToLID converts a PN to LID. Returns the LID if found, otherwise fetches from WhatsApp.
func (s *JIDService) ToLID(ctx context.Context, pn types.JID) types.JID {
	// If it's empty or a non-PN, return as-is
	if jid.IsEmpty(pn) || !jid.IsPN(pn) {
		return pn
	}

	// Check database
	lid, err := s.contactStore.GetLIDForPN(pn)
	if err != nil {
		s.log.Warnf("Failed to get LID for %s: %v", pn, err)
		return pn
	} else if !lid.IsEmpty() {
		return lid
	}

	// Fetch from WhatsApp
	lid, err = s.fetchLIDFromWhatsApp(ctx, pn)
	if err != nil {
		s.log.Warnf("Failed to fetch LID for %s: %v", pn, err)
		return pn
	} else if !lid.IsEmpty() {
		// Store the mapping
		if err := s.contactStore.UpdatePN(lid, pn); err != nil {
			s.log.Warnf("Failed to store JID mapping: %v", err)
		}
		return lid
	}

	// PN not found in mapping, return as-is
	return pn
}

// ToLIDs converts multiple PNs to LIDs.
func (s *JIDService) ToLIDs(ctx context.Context, pns []types.JID) []types.JID {
	var lids []types.JID
	for _, pn := range pns {
		lids = append(lids, s.ToLID(ctx, pn))
	}
	return lids
}

// ToPN converts a LID to PN.
func (s *JIDService) ToPN(lid types.JID) types.JID {
	// If it's empty or a non-LID, return as-is
	if jid.IsEmpty(lid) || !jid.IsLID(lid) {
		return lid
	}

	// Check database
	pn, err := s.contactStore.GetPNForLID(lid)
	if err != nil {
		s.log.Warnf("Failed to get PN for %s: %v", lid, err)
		return lid
	} else if !pn.IsEmpty() {
		return pn
	}

	// LID not found in mapping, return as-is
	return lid
}

// ToPNs converts multiple LIDs to PNs.
func (s *JIDService) ToPNs(ctx context.Context, lids []types.JID) []types.JID {
	var pns []types.JID
	for _, lid := range lids {
		pns = append(pns, s.ToPN(lid))
	}
	return pns
}

// NormalizeJID normalizes a JID to its LID form.
func (s *JIDService) NormalizeJID(ctx context.Context, jid types.JID) types.JID {
	return s.ToLID(ctx, jid)
}

// NormalizeManyJIDs normalizes multiple JIDs to their LIDs form.
func (s *JIDService) NormalizeJIDs(ctx context.Context, jids []types.JID) []types.JID {
	return s.ToLIDs(ctx, jids)
}

// StoreMappingFromEvent stores a PN/LID mapping from event data.
func (s *JIDService) StoreMappingFromEvent(pn, lid types.JID) {
	if pn.IsEmpty() || lid.IsEmpty() {
		return
	}
	if !jid.IsPN(pn) || !jid.IsLID(lid) {
		return
	}

	if err := s.contactStore.UpdatePN(lid, pn); err != nil {
		s.log.Warnf("Failed to store JID mapping from event: %v", err)
		return
	}
}

// fetchLIDFromWhatsApp fetches the LID for a PN from WhatsApp.
func (s *JIDService) fetchLIDFromWhatsApp(ctx context.Context, pn types.JID) (types.JID, error) {
	// Fetch from WhatsApp
	lid, err := s.client.Store.LIDs.GetLIDForPN(ctx, pn)
	if err == nil && !lid.IsEmpty() {
		return lid, nil
	}

	// Fallback return PN
	return pn, nil
}

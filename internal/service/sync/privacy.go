package sync

import (
	"context"

	"orion-agent/internal/data/store"
)

// =============================================================================
// Privacy Sync Methods - SAVES TO DATABASE
// =============================================================================

// SyncPrivacySettings fetches privacy settings and saves to database.
func (s *SyncService) SyncPrivacySettings(ctx context.Context) error {
	s.log.Debugf("Syncing privacy settings")
	settings := s.client.GetPrivacySettings(ctx)

	// Save to database
	privacySettings := &store.PrivacySettings{
		GroupAdd:     string(settings.GroupAdd),
		LastSeen:     string(settings.LastSeen),
		Status:       string(settings.Status),
		Profile:      string(settings.Profile),
		ReadReceipts: string(settings.ReadReceipts),
		Online:       string(settings.Online),
		CallAdd:      string(settings.CallAdd),
	}

	if err := s.privacy.Put(privacySettings); err != nil {
		s.log.Errorf("Failed to save privacy settings: %v", err)
		return err
	}

	s.log.Infof("Synced and saved privacy settings")
	s.recordSync("privacy")
	return nil
}

// SyncStatusPrivacy fetches status privacy settings.
func (s *SyncService) SyncStatusPrivacy(ctx context.Context) error {
	s.log.Debugf("Syncing status privacy")
	privacy, err := s.client.GetStatusPrivacy(ctx)
	if err != nil {
		s.log.Errorf("Failed to get status privacy: %v", err)
		return err
	}

	// Save to database
	for _, prc := range privacy {
		statusPrivacyType := &store.StatusPrivacyType{
			Type:      string(prc.Type),
			IsDefault: prc.IsDefault,
		}
		if err := s.privacy.PutStatusPrivacyType(statusPrivacyType); err != nil {
			s.log.Errorf("Failed to save status privacy type: %v", err)
			return err
		}
		for _, jid := range prc.List {
			jid = s.utils.NormalizeJID(ctx, jid)
			statusPrivacyMember := &store.StatusPrivacyMember{
				Type: string(prc.Type),
				JID:  jid,
			}

			if err := s.privacy.PutStatusPrivacyMember(statusPrivacyMember); err != nil {
				s.log.Errorf("Failed to save status privacy member: %v", err)
				return err
			}
		}
	}

	s.log.Infof("Synced status privacy")
	s.recordSync("status_privacy")
	return nil
}

// TryFetchPrivacySettings attempts to fetch privacy with fallback.
func (s *SyncService) TryFetchPrivacySettings(ctx context.Context) error {
	s.log.Debugf("Trying to fetch privacy settings")
	settings, err := s.client.TryFetchPrivacySettings(ctx, true)
	if err != nil {
		s.log.Errorf("Failed to fetch privacy settings: %v", err)
		return err
	}

	// Save to database
	privacySettings := &store.PrivacySettings{
		GroupAdd:     string(settings.GroupAdd),
		LastSeen:     string(settings.LastSeen),
		Status:       string(settings.Status),
		Profile:      string(settings.Profile),
		ReadReceipts: string(settings.ReadReceipts),
		Online:       string(settings.Online),
		CallAdd:      string(settings.CallAdd),
	}

	if err := s.privacy.Put(privacySettings); err != nil {
		s.log.Errorf("Failed to save privacy settings: %v", err)
		return err
	}

	s.log.Infof("Fetched and saved privacy settings via TryFetch")
	s.recordSync("privacy")
	return nil
}

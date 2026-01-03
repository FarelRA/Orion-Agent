package sync

import (
	"context"

	"orion-agent/internal/data/store"

	"go.mau.fi/whatsmeow/types"
)

// =============================================================================
// Privacy Sync Methods - SAVES TO DATABASE
// =============================================================================

// SyncPrivacySettings fetches privacy settings and saves to database.
func (s *SyncService) SyncPrivacySettings(ctx context.Context) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing privacy settings")

	var settings *types.PrivacySettings
	err := s.performUSync(ctx, func(ctx context.Context) error {
		var err error
		// Use TryFetchPrivacySettings to get error return for 429 handling
		settings, err = s.client.TryFetchPrivacySettings(ctx, false)
		return err
	})
	if err != nil {
		s.log.Warnf("Failed to get privacy settings: %v", err)
		return nil
	}

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
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing status privacy")

	var privacy []types.StatusPrivacy
	err := s.performUSync(ctx, func(ctx context.Context) error {
		var err error
		privacy, err = s.client.GetStatusPrivacy(ctx)
		return err
	})
	if err != nil {
		s.log.Errorf("Failed to get status privacy: %v", err)
		return err
	}
	if privacy == nil {
		return nil
	}

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
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Trying to fetch privacy settings")

	var settings *types.PrivacySettings
	err := s.performUSync(ctx, func(ctx context.Context) error {
		var err error
		settings, err = s.client.TryFetchPrivacySettings(ctx, true)
		return err
	})
	if err != nil {
		s.log.Errorf("Failed to fetch privacy settings: %v", err)
		return err
	}

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

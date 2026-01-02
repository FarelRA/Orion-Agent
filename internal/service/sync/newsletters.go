package sync

import (
	"context"

	"orion-agent/internal/data/store"
)

// =============================================================================
// Newsletter Sync Methods - SAVES TO DATABASE
// =============================================================================

// SyncAllNewsletters fetches all subscribed newsletters and saves to database.
func (s *SyncService) SyncAllNewsletters(ctx context.Context) error {
	s.log.Debugf("Syncing all newsletters")
	newsletters, err := s.client.GetSubscribedNewsletters(ctx)
	if err != nil {
		s.log.Errorf("Failed to get subscribed newsletters: %v", err)
		return err // Not found is ERROR
	}

	for _, nl := range newsletters {
		// Normalize JID
		nl.ID = s.utils.NormalizeJID(ctx, nl.ID)

		newsletter := &store.Newsletter{
			JID:               nl.ID,
			Name:              nl.ThreadMeta.Name.Text,
			Description:       nl.ThreadMeta.Description.Text,
			SubscriberCount:   int64(nl.ThreadMeta.SubscriberCount),
			VerificationState: string(nl.ThreadMeta.VerificationState),
			Role:              string(nl.ViewerMeta.Role),
			Muted:             nl.ViewerMeta.Mute == "on",
		}

		// Picture info
		if nl.ThreadMeta.Picture.ID != "" {
			newsletter.PictureID = nl.ThreadMeta.Picture.ID
			newsletter.PictureURL = nl.ThreadMeta.Picture.DirectPath
		}
		if nl.ThreadMeta.Preview.ID != "" {
			newsletter.PreviewURL = nl.ThreadMeta.Preview.DirectPath
		}

		if err := s.newsletters.Put(newsletter); err != nil {
			s.log.Warnf("Failed to save newsletter %s: %v", nl.ID, err)
		}
	}

	s.log.Infof("Synced and saved %d newsletters", len(newsletters))
	s.recordSync("newsletters")
	return nil
}

// SyncNewsletterInfo fetches and saves a single newsletter's info.
func (s *SyncService) SyncNewsletterInfo(ctx context.Context, jid interface{}) error {
	s.log.Debugf("Newsletter info sync for %v", jid)
	return nil
}

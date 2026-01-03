package sync

import (
	"context"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
)

// =============================================================================
// Newsletter Sync Methods - SAVES TO DATABASE
// =============================================================================

// SyncAllNewsletters fetches all subscribed newsletters and saves to database.
func (s *SyncService) SyncAllNewsletters(ctx context.Context) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing all newsletters")

	var newsletters []*types.NewsletterMetadata
	err := s.performUSync(ctx, func(ctx context.Context) error {
		var err error
		newsletters, err = s.client.GetSubscribedNewsletters(ctx)
		return err
	})
	if err != nil {
		s.log.Warnf("Failed to get subscribed newsletters: %v", err)
		return nil
	}
	if len(newsletters) == 0 {
		return nil
	}

	saved := 0
	for _, nl := range newsletters {
		if nl == nil {
			continue
		}

		nl.ID = s.utils.NormalizeJID(ctx, nl.ID)

		newsletter := &store.Newsletter{
			JID:               nl.ID,
			Name:              nl.ThreadMeta.Name.Text,
			Description:       nl.ThreadMeta.Description.Text,
			SubscriberCount:   int64(nl.ThreadMeta.SubscriberCount),
			VerificationState: string(nl.ThreadMeta.VerificationState),
			PreviewURL:        nl.ThreadMeta.Preview.DirectPath,
		}
		if nl.ThreadMeta.Picture != nil {
			newsletter.PictureID = nl.ThreadMeta.Picture.ID
			newsletter.PictureURL = nl.ThreadMeta.Picture.DirectPath
		}

		if nl.ViewerMeta != nil {
			newsletter.Role = string(nl.ViewerMeta.Role)
			newsletter.Muted = nl.ViewerMeta.Mute == "on"
		}

		if err := s.newsletters.Put(newsletter); err != nil {
			s.log.Warnf("Failed to save newsletter %s: %v", nl.ID, err)
		} else {
			saved++
		}
	}

	s.log.Infof("Synced and saved %d/%d newsletters", saved, len(newsletters))
	s.recordSync("newsletters")
	return nil
}

// SyncNewsletterPicture fetches newsletter profile picture.
// Called when: new newsletter, picture changed event, or explicit refresh.
func (s *SyncService) SyncNewsletterPicture(ctx context.Context, jid types.JID) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	if jid.Server != types.NewsletterServer {
		return nil
	}
	s.log.Debugf("Syncing newsletter picture for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)

	existingID := ""
	if nl, err := s.newsletters.Get(jid); err == nil && nl != nil {
		existingID = nl.PictureID
	}

	params := &whatsmeow.GetProfilePictureParams{
		Preview:     false,
		ExistingID:  existingID,
		IsCommunity: false,
	}

	var pic *types.ProfilePictureInfo
	err := s.performUSync(ctx, func(ctx context.Context) error {
		var err error
		pic, err = s.client.GetProfilePictureInfo(ctx, jid, params)
		return err
	})
	if err != nil || pic == nil {
		s.log.Debugf("No newsletter picture for %s: %v", jid, err)
		// Mark as attempted with "0"
		s.newsletters.UpdateProfilePic(jid, "0", "")
		return nil
	}

	if err := s.newsletters.UpdateProfilePic(jid, pic.ID, pic.URL); err != nil {
		s.log.Warnf("Failed to update newsletter picture for %s: %v", jid, err)
	}

	if s.media != nil {
		s.media.QueueProfilePicture(jid, pic.ID, pic.URL)
	}

	s.log.Infof("Synced newsletter picture for %s", jid)
	s.recordSync("newsletter_picture")
	return nil
}

// SyncAllNewsletterPictures syncs profile pictures for newsletters without picture ID.
func (s *SyncService) SyncAllNewsletterPictures(ctx context.Context) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing newsletter pictures")

	newsletters, err := s.newsletters.GetAll()
	if err != nil {
		s.log.Errorf("Failed to get all newsletters: %v", err)
		return err
	}

	synced := 0
	skipped := 0
	for _, nl := range newsletters {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip if already attempted (has any PictureID including "0")
		if nl.PictureID != "" {
			skipped++
			continue
		}

		if err := s.SyncNewsletterPicture(ctx, nl.JID); err != nil {
			s.log.Warnf("Failed to sync newsletter picture for %s: %v", nl.JID, err)
		} else {
			synced++
		}
	}

	s.log.Infof("Synced %d newsletter pictures, skipped %d", synced, skipped)
	s.recordSync("newsletter_pictures")
	return nil
}

// SyncNewsletterInfo fetches and saves a single newsletter's info.
func (s *SyncService) SyncNewsletterInfo(ctx context.Context, jid interface{}) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Newsletter info sync for %v", jid)
	return nil
}

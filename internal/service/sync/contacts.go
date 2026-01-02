package sync

import (
	"context"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
)

// =============================================================================
// Contact Sync Methods
// =============================================================================

// SyncUserInfo fetches user info for the given JIDs using GetUserInfo.
func (s *SyncService) SyncUserInfo(ctx context.Context, jids ...types.JID) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing user info for %v", jids)

	jids = s.utils.NormalizeJIDs(ctx, jids)
	infos, err := s.client.GetUserInfo(ctx, jids)
	if err != nil || infos == nil {
		s.log.Errorf("Failed to get user info for %v: %v", jids, err)
		return err
	}

	for jid, info := range infos {
		contact := &store.Contact{
			LID:          jid,
			Status:       info.Status,
			ProfilePicID: info.PictureID,
		}
		if info.VerifiedName != nil && info.VerifiedName.Details != nil && info.VerifiedName.Details.VerifiedName != nil {
			contact.VerifiedName = *info.VerifiedName.Details.VerifiedName
		}

		if err := s.contacts.Put(contact); err != nil {
			s.log.Warnf("Failed to save contact info for %s: %v", jid, err)
		}
	}

	s.log.Infof("Synced user info for %d contacts", len(infos))
	s.recordSync("user_info")
	return nil
}

// SyncProfilePicture fetches profile picture for a user JID.
// Called when: new contact, picture changed event, or explicit refresh.
func (s *SyncService) SyncProfilePicture(ctx context.Context, jid types.JID) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	if jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer {
		return nil
	}
	s.log.Debugf("Syncing profile picture for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)

	existingID := ""
	if contact, err := s.contacts.Get(jid); err == nil && contact != nil {
		existingID = contact.ProfilePicID
	}

	params := &whatsmeow.GetProfilePictureParams{
		Preview:     false,
		ExistingID:  existingID,
		IsCommunity: false,
	}

	pic, err := s.client.GetProfilePictureInfo(ctx, jid, params)
	if err != nil || pic == nil {
		s.log.Debugf("No profile picture for %s: %v", jid, err)
		// Mark as attempted with "0" so we don't retry on next initial sync
		s.contacts.UpdateProfilePic(jid, "0", "")
		return nil
	}

	if err := s.contacts.UpdateProfilePic(jid, pic.ID, pic.URL); err != nil {
		s.log.Warnf("Failed to update profile picture for %s: %v", jid, err)
	}

	s.log.Infof("Synced profile picture for %s", jid)
	s.recordSync("profile_picture")
	return nil
}

// SyncBlocklist fetches the blocklist and saves to database.
func (s *SyncService) SyncBlocklist(ctx context.Context) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing blocklist")

	blocklist, err := s.client.GetBlocklist(ctx)
	if err != nil {
		s.log.Errorf("Failed to get blocklist: %v", err)
		return err
	}
	if blocklist == nil {
		return nil
	}

	blocklist.JIDs = s.utils.NormalizeJIDs(ctx, blocklist.JIDs)

	if err := s.blocklist.Replace(blocklist.JIDs); err != nil {
		s.log.Warnf("Failed to save blocklist: %v", err)
	}

	s.log.Infof("Synced and saved blocklist: %d blocked JIDs", len(blocklist.JIDs))
	s.recordSync("blocklist")
	return nil
}

// ResolvePhoneNumbers resolves phone numbers to JIDs using IsOnWhatsApp.
func (s *SyncService) ResolvePhoneNumbers(ctx context.Context, phones ...string) (map[string]types.JID, error) {
	if s.client == nil || ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.log.Debugf("Resolving phone numbers: %v", phones)

	result := make(map[string]types.JID)
	responses, err := s.client.IsOnWhatsApp(ctx, phones)
	if err != nil || responses == nil {
		s.log.Errorf("Failed to resolve phone numbers: %v", err)
		return nil, err
	}

	for _, resp := range responses {
		if resp.IsIn {
			jid := s.utils.NormalizeJID(ctx, resp.JID)
			result[resp.Query] = jid
		} else {
			s.log.Warnf("Phone number %s is not on WhatsApp", resp.Query)
		}
	}

	s.log.Infof("Resolved %d phone numbers", len(result))
	s.recordSync("resolve_phone_numbers")
	return result, nil
}

// SyncUserDevices fetches device list for JIDs.
func (s *SyncService) SyncUserDevices(ctx context.Context, jids ...types.JID) ([]types.JID, error) {
	if s.client == nil || ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.log.Debugf("Syncing user devices for %v", jids)

	jids = s.utils.NormalizeJIDs(ctx, jids)

	jids, err := s.client.GetUserDevicesContext(ctx, jids)
	if err != nil || jids == nil {
		s.log.Errorf("Failed to get user devices: %v", err)
		return nil, err
	}

	jids = s.utils.NormalizeJIDs(ctx, jids)

	s.log.Infof("Synced user devices for %d JIDs", len(jids))
	s.recordSync("user_devices")
	return jids, nil
}

// SubscribePresence subscribes to presence updates for a JID.
func (s *SyncService) SubscribePresence(ctx context.Context, jid types.JID) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Subscribing to presence for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	err := s.client.SubscribePresence(ctx, jid)
	if err != nil {
		s.log.Errorf("Failed to subscribe presence for %s: %v", jid, err)
		return err
	}

	s.log.Infof("Subscribed to presence for %s", jid)
	s.recordSync("subscribe_presence")
	return nil
}

// SyncAllContactPictures syncs profile pictures for contacts without picture ID.
func (s *SyncService) SyncAllContactPictures(ctx context.Context) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing contact pictures (missing only)")

	contacts, err := s.contacts.GetAll()
	if err != nil || contacts == nil {
		s.log.Errorf("Failed to get all contacts: %v", err)
		return err
	}

	synced := 0
	skipped := 0
	for _, contact := range contacts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip if already attempted (has any ProfilePicID including "0")
		if contact.ProfilePicID != "" {
			skipped++
			continue
		}

		lid := s.utils.NormalizeJID(ctx, contact.LID)
		if err := s.SyncProfilePicture(ctx, lid); err != nil {
			s.log.Errorf("Failed to sync profile picture for %s: %v", lid, err)
		} else {
			synced++
		}
	}

	s.log.Infof("Synced %d contact pictures, skipped %d", synced, skipped)
	s.recordSync("contact_pictures")
	return nil
}

// =============================================================================
// COMPREHENSIVE COALESCENCE TRIGGERS
// Events trigger syncs to fill in ALL missing data
// =============================================================================

// OnNewContact syncs all info for a new/unknown contact.
func (s *SyncService) OnNewContact(ctx context.Context, jid types.JID) {
	if ctx.Err() != nil {
		return
	}
	// Only sync user JIDs, not groups/newsletters
	if jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer {
		return
	}
	s.log.Debugf("Full sync for new contact %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)

	if err := s.SyncUserInfo(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync user info for %s: %v", jid, err)
	}

	if err := s.SyncProfilePicture(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync profile pic for %s: %v", jid, err)
	}

	if err := s.SubscribePresence(ctx, jid); err != nil {
		s.log.Warnf("Failed to subscribe presence for %s: %v", jid, err)
	}

	s.log.Infof("Full sync for new contact %s completed", jid)
	s.recordSync("new_contact")
}

// OnNewMessage handles coalescence when a new message is received.
func (s *SyncService) OnNewMessage(ctx context.Context, chatJID, senderJID types.JID, isGroup bool) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("New message for %s from %s", chatJID, senderJID)

	senderJID = s.utils.NormalizeJID(ctx, senderJID)
	chatJID = s.utils.NormalizeJID(ctx, chatJID)

	exists, err := s.contacts.Exists(senderJID)
	if err != nil || !exists {
		s.OnNewContact(ctx, senderJID)
	}

	exists, err = s.chats.Exists(chatJID)
	if err != nil || !exists {
		chatType := store.ChatTypeUser
		if isGroup {
			chatType = store.ChatTypeGroup
		}
		s.chats.EnsureExists(chatJID, chatType)
	}

	if isGroup {
		exists, err = s.groups.Exists(chatJID)
		if err != nil || !exists {
			s.OnGroupNeedsSync(ctx, chatJID)
		}
	}

	s.log.Infof("New message for %s completed", chatJID)
	s.recordSync("new_message")
}

// OnPushNameUpdate handles coalescence when push name changes.
// Does NOT sync picture - push name change doesn't mean picture changed.
func (s *SyncService) OnPushNameUpdate(ctx context.Context, jid types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Push name update for %s", jid)
	s.recordSync("push_name_update")
}

// OnPresenceUpdate handles coalescence when presence is received.
func (s *SyncService) OnPresenceUpdate(ctx context.Context, jid types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Sync data for presence update %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	exists, err := s.contacts.Exists(jid)
	if err != nil || !exists {
		s.OnNewContact(ctx, jid)
	}

	s.log.Infof("Sync data for presence update %s completed", jid)
	s.recordSync("presence_update")
}

// OnChatPresenceUpdate handles coalescence when chat presence is received.
func (s *SyncService) OnChatPresenceUpdate(ctx context.Context, chatJID, senderJID types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Sync data for chat presence update %s from %s", chatJID, senderJID)

	senderJID = s.utils.NormalizeJID(ctx, senderJID)
	chatJID = s.utils.NormalizeJID(ctx, chatJID)
	exists, err := s.contacts.Exists(senderJID)
	if err != nil || !exists {
		s.OnNewContact(ctx, senderJID)
	}

	s.log.Infof("Sync data for chat presence update %s from %s completed", chatJID, senderJID)
	s.recordSync("chat_presence_update")
}

// OnReceipt handles coalescence when a receipt is received.
func (s *SyncService) OnReceipt(ctx context.Context, senderJIDs []types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Sync data for receipt %s", senderJIDs)

	senderJIDs = s.utils.NormalizeJIDs(ctx, senderJIDs)
	for _, jid := range senderJIDs {
		if ctx.Err() != nil {
			return
		}
		exists, err := s.contacts.Exists(jid)
		if err != nil || !exists {
			s.OnNewContact(ctx, jid)
		}
	}

	s.log.Infof("Sync data for receipt %s completed", senderJIDs)
	s.recordSync("receipt")
}

// OnPictureUpdate handles coalescence when picture changes.
func (s *SyncService) OnPictureUpdate(ctx context.Context, jid types.JID, pictureID string) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing picture update for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	if err := s.SyncProfilePicture(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync picture for %s: %v", jid, err)
	}

	s.log.Infof("Syncing picture update for %s completed", jid)
	s.recordSync("picture_update")
}

// OnGroupNeedsSync syncs full group info when incomplete.
func (s *SyncService) OnGroupNeedsSync(ctx context.Context, jid types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Full sync for group %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)

	if err := s.SyncGroupInfo(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync group info for %s: %v", jid, err)
	}

	if err := s.SyncGroupInviteLink(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync invite link for %s: %v", jid, err)
	}

	// Get group to check if community
	group, _ := s.groups.Get(jid)
	isCommunity := group != nil && group.IsCommunity
	if err := s.SyncGroupPicture(ctx, jid, isCommunity); err != nil {
		s.log.Warnf("Failed to sync picture for %s: %v", jid, err)
	}

	s.log.Infof("Full sync for group %s completed", jid)
	s.recordSync("group_sync")
}

// OnGroupJoined handles coalescence when joining a new group.
func (s *SyncService) OnGroupJoined(ctx context.Context, jid types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing newly joined group %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	s.OnGroupNeedsSync(ctx, jid)

	s.log.Infof("Syncing newly joined group %s completed", jid)
	s.recordSync("group_joined")
}

// OnGroupInfoChange handles coalescence when group info changes.
func (s *SyncService) OnGroupInfoChange(ctx context.Context, jid types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing group info change for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	if err := s.SyncGroupInfo(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync group info for %s: %v", jid, err)
	}

	s.log.Infof("Syncing group info change for %s completed", jid)
	s.recordSync("group_info_change")
}

// OnGroupParticipantsChange handles coalescence when participants change.
func (s *SyncService) OnGroupParticipantsChange(ctx context.Context, groupJID types.JID, participantJIDs []types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing participant change for group %s", groupJID)

	groupJID = s.utils.NormalizeJID(ctx, groupJID)

	if err := s.SyncGroupInfo(ctx, groupJID); err != nil {
		s.log.Warnf("Failed to sync group info for %s: %v", groupJID, err)
	}

	for _, pJID := range participantJIDs {
		if ctx.Err() != nil {
			return
		}
		pJID = s.utils.NormalizeJID(ctx, pJID)
		exists, err := s.contacts.Exists(pJID)
		if err != nil || !exists {
			s.OnNewContact(ctx, pJID)
		}
	}

	s.log.Infof("Syncing participant change for group %s completed", groupJID)
	s.recordSync("group_participants_change")
}

// OnHistorySyncContacts handles batch contact sync from history.
func (s *SyncService) OnHistorySyncContacts(ctx context.Context, jids []types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing %d contacts from history", len(jids))

	jids = s.utils.NormalizeJIDs(ctx, jids)

	if err := s.SyncUserInfo(ctx, jids...); err != nil {
		s.log.Warnf("Failed to batch sync user info: %v", err)
	}

	for _, jid := range jids {
		if ctx.Err() != nil {
			return
		}
		if err := s.SyncProfilePicture(ctx, jid); err != nil {
			s.log.Warnf("Failed to sync picture for %s: %v", jid, err)
		}
	}

	s.log.Infof("Syncing %d contacts from history completed", len(jids))
	s.recordSync("history_sync_contacts")
}

// OnHistorySyncGroups handles batch group sync from history.
func (s *SyncService) OnHistorySyncGroups(ctx context.Context, jids []types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing %d groups from history", len(jids))

	jids = s.utils.NormalizeJIDs(ctx, jids)
	for _, jid := range jids {
		if ctx.Err() != nil {
			return
		}
		s.OnGroupNeedsSync(ctx, jid)
	}

	s.log.Infof("Syncing %d groups from history completed", len(jids))
	s.recordSync("history_sync_groups")
}

// OnCallReceived handles coalescence when a call is received.
func (s *SyncService) OnCallReceived(ctx context.Context, callerJID types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing caller %s", callerJID)

	callerJID = s.utils.NormalizeJID(ctx, callerJID)
	exists, err := s.contacts.Exists(callerJID)
	if err != nil || !exists {
		s.OnNewContact(ctx, callerJID)
	}

	s.log.Infof("Syncing caller %s completed", callerJID)
	s.recordSync("call_received")
}

// OnNewsletterMessage handles coalescence when a newsletter message is received.
// Only syncs newsletter info if unknown, does NOT sync picture on every message.
func (s *SyncService) OnNewsletterMessage(ctx context.Context, newsletterJID types.JID) {
	if ctx.Err() != nil {
		return
	}

	newsletterJID = s.utils.NormalizeJID(ctx, newsletterJID)
	exists, err := s.newsletters.Exists(newsletterJID)
	if err != nil || !exists {
		s.log.Debugf("Syncing unknown newsletter %s", newsletterJID)
		// SyncAllNewsletters fetches metadata including picture info
		if err := s.SyncAllNewsletters(ctx); err != nil {
			s.log.Warnf("Failed to sync newsletters: %v", err)
		}
	}
	s.recordSync("newsletter_message")
}

// OnReaction handles coalescence when a reaction is received.
func (s *SyncService) OnReaction(ctx context.Context, senderJID types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing reaction from %s", senderJID)

	senderJID = s.utils.NormalizeJID(ctx, senderJID)
	exists, err := s.contacts.Exists(senderJID)
	if err != nil || !exists {
		s.OnNewContact(ctx, senderJID)
	}

	s.log.Infof("Syncing reaction from %s completed", senderJID)
	s.recordSync("reaction")
}

// OnBlocklistChange handles coalescence when blocklist changes.
func (s *SyncService) OnBlocklistChange(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing blocklist change")
	if err := s.SyncBlocklist(ctx); err != nil {
		s.log.Warnf("Failed to sync blocklist: %v", err)
	}

	s.log.Infof("Syncing blocklist change completed")
	s.recordSync("blocklist_change")
}

// OnPrivacySettingsChange handles coalescence when privacy settings change.
func (s *SyncService) OnPrivacySettingsChange(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing privacy settings change")
	if err := s.SyncPrivacySettings(ctx); err != nil {
		s.log.Warnf("Failed to sync privacy settings: %v", err)
	}

	s.log.Infof("Syncing privacy settings change completed")
	s.recordSync("privacy_settings_change")
}

// OnStatusMessage handles coalescence when a status/story is received.
func (s *SyncService) OnStatusMessage(ctx context.Context, senderJID types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing status message from %s", senderJID)

	senderJID = s.utils.NormalizeJID(ctx, senderJID)
	exists, err := s.contacts.Exists(senderJID)
	if err != nil || !exists {
		s.OnNewContact(ctx, senderJID)
	}

	s.log.Infof("Syncing status message from %s completed", senderJID)
	s.recordSync("status_message")
}

// OnAny is a catch-all that can sync any JID encountered.
func (s *SyncService) OnAny(ctx context.Context, jids ...types.JID) {
	if ctx.Err() != nil {
		return
	}
	s.log.Debugf("Syncing any %d jids", len(jids))

	jids = s.utils.NormalizeJIDs(ctx, jids)
	for _, jid := range jids {
		if ctx.Err() != nil {
			return
		}
		switch jid.Server {
		case types.HiddenUserServer, types.DefaultUserServer:
			exists, err := s.contacts.Exists(jid)
			if err != nil || !exists {
				s.OnNewContact(ctx, jid)
			}
		case types.GroupServer:
			exists, err := s.groups.Exists(jid)
			if err != nil || !exists {
				s.OnGroupNeedsSync(ctx, jid)
			}
		}
	}

	s.log.Infof("Syncing any %d jids completed", len(jids))
	s.recordSync("any")
}

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
	s.log.Debugf("Syncing user info for %v", jids)

	jids = s.utils.NormalizeJIDs(ctx, jids)
	infos, err := s.client.GetUserInfo(ctx, jids)
	if err != nil || infos == nil {
		s.log.Errorf("Failed to get user info for %v: %v", jids, err)
		return err // Not found is ERROR
	}

	for jid, info := range infos {
		contact := &store.Contact{
			LID:          jid,
			Status:       info.Status,
			ProfilePicID: info.PictureID,
			VerifiedName: *info.VerifiedName.Details.VerifiedName,
		}

		if err := s.contacts.Put(contact); err != nil {
			s.log.Warnf("Failed to save contact info for %s: %v", jid, err)
		}
	}

	s.log.Infof("Synced user info for %d contacts", len(infos))
	s.recordSync("user_info")
	return nil
}

// SyncProfilePicture fetches profile picture info for a JID.
func (s *SyncService) SyncProfilePicture(ctx context.Context, jid types.JID) error {
	s.log.Debugf("Syncing profile picture for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	pic, err := s.client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{})
	if err != nil || pic == nil {
		s.log.Warnf("Failed to get profile picture info for %s: %v", jid, err)
		return nil // Not found is OK
	}

	switch jid.Server {
	case types.DefaultUserServer, types.HiddenUserServer:
		err = s.contacts.UpdateProfilePic(jid, pic.ID, pic.URL)
	case types.GroupServer:
		err = s.groups.UpdateProfilePic(jid, pic.ID, pic.URL)
	case types.NewsletterServer:
		err = s.newsletters.UpdateProfilePic(jid, pic.ID, pic.URL)
	}

	if err != nil {
		s.log.Warnf("Failed to update profile picture for %s: %v", jid, err)
	}

	s.log.Infof("Synced profile picture for %s", jid)
	s.recordSync("profile_picture")
	return nil
}

// SyncBlocklist fetches the blocklist and saves to database.
func (s *SyncService) SyncBlocklist(ctx context.Context) error {
	s.log.Debugf("Syncing blocklist")

	blocklist, err := s.client.GetBlocklist(ctx)
	if err != nil || blocklist == nil {
		s.log.Errorf("Failed to get blocklist: %v", err)
		return err // Not found is ERROR
	}

	// Normalize JIDs
	blocklist.JIDs = s.utils.NormalizeJIDs(ctx, blocklist.JIDs)

	// Save with replace entire blocklist
	if err := s.blocklist.Replace(blocklist.JIDs); err != nil {
		s.log.Warnf("Failed to save blocklist: %v", err)
	}

	s.log.Infof("Synced and saved blocklist: %d blocked JIDs", len(blocklist.JIDs))
	s.recordSync("blocklist")
	return nil
}

// ResolvePhoneNumbers resolves phone numbers to JIDs using IsOnWhatsApp.
func (s *SyncService) ResolvePhoneNumbers(ctx context.Context, phones ...string) (map[string]types.JID, error) {
	s.log.Debugf("Resolving phone numbers: %v", phones)

	result := make(map[string]types.JID)
	responses, err := s.client.IsOnWhatsApp(ctx, phones)
	if err != nil || responses == nil {
		s.log.Errorf("Failed to resolve phone numbers: %v", err)
		return nil, err // Not found is ERROR
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
	s.log.Debugf("Syncing user devices for %v", jids)

	// Normalize JIDs
	jids = s.utils.NormalizeJIDs(ctx, jids)

	jids, err := s.client.GetUserDevicesContext(ctx, jids)
	if err != nil || jids == nil {
		s.log.Errorf("Failed to get user devices: %v", err)
		return nil, err // Not found is ERROR
	}

	// Normalize JIDs
	jids = s.utils.NormalizeJIDs(ctx, jids)

	s.log.Infof("Synced user devices for %d JIDs", len(jids))
	s.recordSync("user_devices")
	return jids, nil
}

// SubscribePresence subscribes to presence updates for a JID.
func (s *SyncService) SubscribePresence(ctx context.Context, jid types.JID) error {
	s.log.Debugf("Subscribing to presence for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	err := s.client.SubscribePresence(ctx, jid)
	if err != nil {
		s.log.Errorf("Failed to subscribe presence for %s: %v", jid, err)
		return err // Not found is ERROR
	}

	s.log.Infof("Subscribed to presence for %s", jid)
	s.recordSync("subscribe_presence")
	return nil
}

// SyncAllContactPictures syncs profile pictures for all known contacts.
func (s *SyncService) SyncAllContactPictures(ctx context.Context) error {
	s.log.Debugf("Syncing all contact pictures")
	contacts, err := s.contacts.GetAll()
	if err != nil || contacts == nil {
		s.log.Errorf("Failed to get all contacts: %v", err)
		return err // Not found is ERROR
	}

	synced := 0
	for _, contact := range contacts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lid := s.utils.NormalizeJID(ctx, contact.LID)
		err := s.SyncProfilePicture(ctx, lid)
		if err != nil {
			s.log.Errorf("Failed to sync profile picture for %s: %v", lid, err)
		} else {
			synced++
		}
	}

	s.log.Infof("Synced %d/%d contact pictures", synced, len(contacts))
	s.recordSync("contact_pictures")
	return nil
}

// =============================================================================
// COMPREHENSIVE COALESCENCE TRIGGERS
// Events trigger syncs to fill in ALL missing data
// =============================================================================

// OnNewContact syncs all info for a new/unknown contact.
// Triggered when encountering an unknown sender.
func (s *SyncService) OnNewContact(ctx context.Context, jid types.JID) {
	s.log.Debugf("Full sync for new contact %s", jid)

	// 0. Convert to LID
	jid = s.utils.NormalizeJID(ctx, jid)

	// 1. Sync user info (status, picture ID)
	if err := s.SyncUserInfo(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync user info for %s: %v", jid, err)
	}

	// 2. Sync profile picture
	if err := s.SyncProfilePicture(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync profile pic for %s: %v", jid, err)
	}

	// 3. Subscribe to presence updates
	if err := s.SubscribePresence(ctx, jid); err != nil {
		s.log.Warnf("Failed to subscribe presence for %s: %v", jid, err)
	}

	s.log.Infof("Full sync for new contact %s completed", jid)
	s.recordSync("new_contact")
}

// OnNewMessage handles coalescence when a new message is received.
// Fills: sender info, chat info, group info if applicable.
func (s *SyncService) OnNewMessage(ctx context.Context, chatJID, senderJID types.JID, isGroup bool) {
	s.log.Debugf("New message for %s from %s", chatJID, senderJID)

	// 0. Convert to LID
	senderJID = s.utils.NormalizeJID(ctx, senderJID)
	chatJID = s.utils.NormalizeJID(ctx, chatJID)

	// 1. Check and sync sender
	exists, err := s.contacts.Exists(senderJID)
	if err != nil || !exists {
		s.log.Warnf("Failed to check contact existence for %s: %v", senderJID, err)
		s.OnNewContact(ctx, senderJID)
	}

	// 2. Sync chat info
	exists, err = s.chats.Exists(chatJID)
	if err != nil || !exists {
		s.log.Warnf("Failed to check chat existence for %s: %v", chatJID, err)
		// Ensure chat exists with appropriate type
		chatType := store.ChatTypeUser
		if isGroup {
			chatType = store.ChatTypeGroup
		}
		s.chats.EnsureExists(chatJID, chatType)
	}

	// 3. If group message, ensure group is fully synced
	if isGroup {
		exists, err = s.groups.Exists(chatJID)
		if err != nil || !exists {
			s.log.Warnf("Failed to check group existence for %s: %v", chatJID, err)
			s.OnGroupNeedsSync(ctx, chatJID)
		}
	}

	s.log.Infof("New message for %s completed", chatJID)
	s.recordSync("new_message")
}

// OnPushNameUpdate handles coalescence when push name changes.
// Fills: profile picture, business profile if applicable.
func (s *SyncService) OnPushNameUpdate(ctx context.Context, jid types.JID) {
	s.log.Debugf("Sync data for push name update %s", jid)

	// 0. Convert to LID
	jid = s.utils.NormalizeJID(ctx, jid)

	// 1. Sync profile picture (push name often comes with new pic)
	if err := s.SyncProfilePicture(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync profile pic for %s: %v", jid, err)
	}

	s.log.Infof("Sync data for push name update %s completed", jid)
	s.recordSync("push_name_update")
}

// OnPresenceUpdate handles coalescence when presence is received.
// Fills: contact basic info if unknown.
func (s *SyncService) OnPresenceUpdate(ctx context.Context, jid types.JID) {
	s.log.Debugf("Sync data for presence update %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	exists, err := s.contacts.Exists(jid)
	if err != nil || !exists {
		s.log.Warnf("Failed to check contact existence for %s: %v", jid, err)
		s.OnNewContact(ctx, jid)
	}

	s.log.Infof("Sync data for presence update %s completed", jid)
	s.recordSync("presence_update")
}

// OnChatPresenceUpdate handles coalescence when chat presence (typing, etc.) is received.
// Fills: contact and chat info.
func (s *SyncService) OnChatPresenceUpdate(ctx context.Context, chatJID, senderJID types.JID) {
	s.log.Debugf("Sync data for chat presence update %s from %s", chatJID, senderJID)

	senderJID = s.utils.NormalizeJID(ctx, senderJID)
	chatJID = s.utils.NormalizeJID(ctx, chatJID)
	exists, err := s.contacts.Exists(senderJID)
	if err != nil || !exists {
		s.log.Warnf("Failed to check contact existence for %s: %v", senderJID, err)
		s.OnNewContact(ctx, senderJID)
	}

	s.log.Infof("Sync data for chat presence update %s from %s completed", chatJID, senderJID)
	s.recordSync("chat_presence_update")
}

// OnReceipt handles coalescence when a receipt is received.
// Fills: recipient info if unknown.
func (s *SyncService) OnReceipt(ctx context.Context, senderJIDs []types.JID) {
	s.log.Debugf("Sync data for receipt %s", senderJIDs)

	senderJIDs = s.utils.NormalizeJIDs(ctx, senderJIDs)
	for _, jid := range senderJIDs {
		exists, err := s.contacts.Exists(jid)
		if err != nil || !exists {
			s.log.Warnf("Failed to check contact existence for %s: %v", jid, err)
			s.OnNewContact(ctx, jid)
		}
	}

	s.log.Infof("Sync data for receipt %s completed", senderJIDs)
	s.recordSync("receipt")
}

// OnPictureUpdate handles coalescence when picture changes.
// Fills: full picture URL if we only got the ID.
func (s *SyncService) OnPictureUpdate(ctx context.Context, jid types.JID, pictureID string) {
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
	s.log.Debugf("Full sync for group %s", jid)

	// 0. Convert to LID
	jid = s.utils.NormalizeJID(ctx, jid)

	// 1. Sync full group info + participants
	if err := s.SyncGroupInfo(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync group info for %s: %v", jid, err)
	}

	// 2. Sync group invite link
	if err := s.SyncGroupInviteLink(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync invite link for %s: %v", jid, err)
	}

	// 3. Sync group profile picture
	if err := s.SyncProfilePicture(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync picture for %s: %v", jid, err)
	}

	s.log.Infof("Full sync for group %s completed", jid)
	s.recordSync("group_sync")
}

// OnGroupJoined handles coalescence when joining a new group.
// Fills: group info, participants, invite link, picture.
func (s *SyncService) OnGroupJoined(ctx context.Context, jid types.JID) {
	s.log.Debugf("Syncing newly joined group %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	s.OnGroupNeedsSync(ctx, jid)

	s.log.Infof("Syncing newly joined group %s completed", jid)
	s.recordSync("group_joined")
}

// OnGroupInfoChange handles coalescence when group info changes.
// Fills: updated group info, participants.
func (s *SyncService) OnGroupInfoChange(ctx context.Context, jid types.JID) {
	s.log.Debugf("Syncing group info change for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	if err := s.SyncGroupInfo(ctx, jid); err != nil {
		s.log.Warnf("Failed to sync group info for %s: %v", jid, err)
	}

	s.log.Infof("Syncing group info change for %s completed", jid)
	s.recordSync("group_info_change")
}

// OnGroupParticipantsChange handles coalescence when participants change.
// Fills: new participant info + updated group state.
func (s *SyncService) OnGroupParticipantsChange(ctx context.Context, groupJID types.JID, participantJIDs []types.JID) {
	s.log.Debugf("Syncing participant change for group %s", groupJID)

	// 0. Normalize group JID
	groupJID = s.utils.NormalizeJID(ctx, groupJID)

	// 1. Sync full group info
	if err := s.SyncGroupInfo(ctx, groupJID); err != nil {
		s.log.Warnf("Failed to sync group info for %s: %v", groupJID, err)
	}

	// 2. Sync info for new participants
	for _, pJID := range participantJIDs {
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
// Fills: all contact info for synced contacts.
func (s *SyncService) OnHistorySyncContacts(ctx context.Context, jids []types.JID) {
	s.log.Debugf("Syncing %d contacts from history", len(jids))

	// Convert to LID
	jids = s.utils.NormalizeJIDs(ctx, jids)

	// Batch sync user info
	if err := s.SyncUserInfo(ctx, jids...); err != nil {
		s.log.Warnf("Failed to batch sync user info: %v", err)
	}

	// Sync profile pictures
	for _, jid := range jids {
		if err := s.SyncProfilePicture(ctx, jid); err != nil {
			s.log.Warnf("Failed to sync picture for %s: %v", jid, err)
		}
	}

	s.log.Infof("Syncing %d contacts from history completed", len(jids))
	s.recordSync("history_sync_contacts")
}

// OnHistorySyncGroups handles batch group sync from history.
// Fills: all group info for synced groups.
func (s *SyncService) OnHistorySyncGroups(ctx context.Context, jids []types.JID) {
	s.log.Debugf("Syncing %d groups from history", len(jids))

	jids = s.utils.NormalizeJIDs(ctx, jids)
	for _, jid := range jids {
		s.OnGroupNeedsSync(ctx, jid)
	}

	s.log.Infof("Syncing %d groups from history completed", len(jids))
	s.recordSync("history_sync_groups")
}

// OnCallReceived handles coalescence when a call is received.
// Fills: caller info if unknown.
func (s *SyncService) OnCallReceived(ctx context.Context, callerJID types.JID) {
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
// Fills: newsletter info if incomplete.
func (s *SyncService) OnNewsletterMessage(ctx context.Context, newsletterJID types.JID) {
	s.log.Debugf("Syncing newsletter %s", newsletterJID)

	newsletterJID = s.utils.NormalizeJID(ctx, newsletterJID)
	exists, err := s.newsletters.Exists(newsletterJID)
	if err != nil || !exists {
		s.log.Debugf("Syncing unknown newsletter %s", newsletterJID)
		if err := s.SyncAllNewsletters(ctx); err != nil {
			s.log.Warnf("Failed to sync newsletters: %v", err)
		}
	}

	s.log.Infof("Syncing newsletter %s completed", newsletterJID)
	s.recordSync("newsletter_message")
}

// OnReaction handles coalescence when a reaction is received.
// Fills: reactor info if unknown.
func (s *SyncService) OnReaction(ctx context.Context, senderJID types.JID) {
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
// Fills: refreshes full blocklist.
func (s *SyncService) OnBlocklistChange(ctx context.Context) {
	s.log.Debugf("Syncing blocklist change")
	if err := s.SyncBlocklist(ctx); err != nil {
		s.log.Warnf("Failed to sync blocklist: %v", err)
	}

	s.log.Infof("Syncing blocklist change completed")
	s.recordSync("blocklist_change")
}

// OnPrivacySettingsChange handles coalescence when privacy settings change.
// Fills: refreshes all privacy settings.
func (s *SyncService) OnPrivacySettingsChange(ctx context.Context) {
	s.log.Debugf("Syncing privacy settings change")
	if err := s.SyncPrivacySettings(ctx); err != nil {
		s.log.Warnf("Failed to sync privacy settings: %v", err)
	}

	s.log.Infof("Syncing privacy settings change completed")
	s.recordSync("privacy_settings_change")
}

// OnStatusMessage handles coalescence when a status/story is received.
// Fills: poster info if unknown.
func (s *SyncService) OnStatusMessage(ctx context.Context, senderJID types.JID) {
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
// Use this for events where we just want to ensure contact exists.
func (s *SyncService) OnAny(ctx context.Context, jids ...types.JID) {
	s.log.Debugf("Syncing any %d jids", len(jids))

	jids = s.utils.NormalizeJIDs(ctx, jids)
	for _, jid := range jids {
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

package sync

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
)

// =============================================================================
// Group Sync Methods
// =============================================================================

// SyncAllGroups fetches all joined groups using GetJoinedGroups.
func (s *SyncService) SyncAllGroups(ctx context.Context) error {
	s.log.Debugf("Syncing all groups")

	groups, err := s.client.GetJoinedGroups(ctx)
	if err != nil || groups == nil {
		s.log.Errorf("Failed to get joined groups: %v", err)
		return err // Not found is ERROR
	}

	for _, g := range groups {
		// Normalize JID
		g.JID = s.utils.NormalizeJID(ctx, g.JID)
		g.OwnerJID = s.utils.NormalizeJID(ctx, g.OwnerJID)
		g.NameSetBy = s.utils.NormalizeJID(ctx, g.NameSetBy)
		g.TopicSetBy = s.utils.NormalizeJID(ctx, g.TopicSetBy)

		// Convert to store group
		group := groupInfoToStore(g)

		// Save group
		if err := s.groups.Put(group); err != nil {
			s.log.Warnf("Failed to save group %s: %v", g.JID, err)
			continue
		}

		// Save participants
		for _, p := range g.Participants {
			// Normalize JID
			p.JID = s.utils.NormalizeJID(ctx, p.JID)

			participant := &store.GroupParticipant{
				GroupJID:     g.JID,
				MemberLID:    p.JID,
				IsAdmin:      p.IsAdmin,
				IsSuperAdmin: p.IsSuperAdmin,
				DisplayName:  p.DisplayName,
				ErrorCode:    int(p.Error),
			}
			if err := s.groups.PutParticipant(participant); err != nil {
				s.log.Warnf("Failed to save participant %s: %v", p.JID, err)
				continue
			}
		}
	}

	s.log.Infof("Synced %d groups", len(groups))
	s.recordSync("groups")
	return nil
}

// SyncGroupInfo fetches full group info using GetGroupInfo.
func (s *SyncService) SyncGroupInfo(ctx context.Context, jid types.JID) error {
	s.log.Debugf("Syncing group info for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	info, err := s.client.GetGroupInfo(ctx, jid)
	if err != nil || info == nil {
		s.log.Errorf("Failed to get group info for %s: %v", jid, err)
		return err // Not found is ERROR
	}

	// Normalize JID
	info.JID = s.utils.NormalizeJID(ctx, info.JID)
	info.OwnerJID = s.utils.NormalizeJID(ctx, info.OwnerJID)
	info.NameSetBy = s.utils.NormalizeJID(ctx, info.NameSetBy)
	info.TopicSetBy = s.utils.NormalizeJID(ctx, info.TopicSetBy)

	group := groupInfoToStore(info)
	if err := s.groups.Put(group); err != nil {
		s.log.Errorf("Failed to save group %s: %v", jid, err)
		return err
	}

	// Save participants
	for _, p := range info.Participants {
		// Normalize JID
		p.JID = s.utils.NormalizeJID(ctx, p.JID)

		participant := &store.GroupParticipant{
			GroupJID:     jid,
			MemberLID:    p.JID,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			DisplayName:  p.DisplayName,
			ErrorCode:    int(p.Error),
		}
		if err := s.groups.PutParticipant(participant); err != nil {
			s.log.Errorf("Failed to save participant %s: %v", p.JID, err)
			return err
		}
	}

	s.log.Infof("Synced group info for %s", jid)
	s.recordSync("group_info")
	return nil
}

// SyncGroupInviteLink fetches group invite link.
func (s *SyncService) SyncGroupInviteLink(ctx context.Context, jid types.JID) error {
	s.log.Debugf("Syncing invite link for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	link, err := s.client.GetGroupInviteLink(ctx, jid, false)
	if err != nil || link == "" {
		s.log.Errorf("Failed to get invite link for %s: %v", jid, err)
		return nil // Permission denied is OK
	}

	code := extractInviteCode(link)
	if err := s.groups.UpdateInviteLink(jid, link, code, time.Time{}); err != nil {
		s.log.Errorf("Failed to update invite link for %s: %v", jid, err)
		return err
	}

	s.log.Infof("Synced invite link for %s", jid)
	s.recordSync("group_invite_link")
	return nil
}

// SyncGroupPicture fetches group profile picture.
func (s *SyncService) SyncGroupPicture(ctx context.Context, jid types.JID) error {
	s.log.Debugf("Syncing profile picture for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	pic, err := s.client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{})
	if err != nil || pic == nil {
		s.log.Errorf("Failed to get profile picture for %s: %v", jid, err)
		return nil // Not found is OK
	}

	if err := s.groups.UpdateProfilePic(jid, pic.ID, pic.URL); err != nil {
		s.log.Errorf("Failed to update profile picture for %s: %v", jid, err)
		return err
	}

	s.log.Infof("Synced profile picture for %s", jid)
	s.recordSync("group_picture")
	return nil
}

// SyncGroupFromLink fetches group info from invite link.
func (s *SyncService) SyncGroupFromLink(ctx context.Context, code string) (*types.GroupInfo, error) {
	s.log.Debugf("Syncing group from link %s", code)
	info, err := s.client.GetGroupInfoFromLink(ctx, code)
	if err != nil || info == nil {
		s.log.Errorf("Failed to get group info from link %s: %v", code, err)
		return nil, err
	}

	// Normalize JID
	info.JID = s.utils.NormalizeJID(ctx, info.JID)
	info.OwnerJID = s.utils.NormalizeJID(ctx, info.OwnerJID)
	info.NameSetBy = s.utils.NormalizeJID(ctx, info.NameSetBy)
	info.TopicSetBy = s.utils.NormalizeJID(ctx, info.TopicSetBy)

	group := groupInfoToStore(info)
	if err := s.groups.Put(group); err != nil {
		s.log.Errorf("Failed to save group from link: %v", err)
		return nil, err
	}

	s.log.Infof("Synced group from link %s", code)
	s.recordSync("group_from_link")
	return info, nil
}

// SyncAllGroupPictures syncs profile pictures for all groups.
func (s *SyncService) SyncAllGroupPictures(ctx context.Context) error {
	s.log.Debugf("Syncing all group pictures")
	groups, err := s.groups.GetAll()
	if err != nil || groups == nil {
		s.log.Errorf("Failed to get all groups: %v", err)
		return err // Not found is OK
	}

	synced := 0
	for _, group := range groups {
		// Normalize JID
		group.JID = s.utils.NormalizeJID(ctx, group.JID)

		if err := s.SyncGroupPicture(ctx, group.JID); err != nil {
			s.log.Warnf("Failed to sync group picture for %s: %v", group.JID, err)
		} else {
			synced++
		}
	}

	s.log.Infof("Synced %d/%d group pictures", synced, len(groups))
	s.recordSync("group_pictures")
	return nil
}

// Helper functions

func groupInfoToStore(g *types.GroupInfo) *store.Group {
	// Convert to store group
	group := &store.Group{
		JID:               g.JID,
		Name:              g.Name,
		NameSetAt:         g.NameSetAt,
		NameSetByLID:      g.NameSetBy,
		Topic:             g.Topic,
		TopicID:           g.TopicID,
		TopicSetAt:        g.TopicSetAt,
		TopicSetByLID:     g.TopicSetBy,
		OwnerLID:          g.OwnerJID,
		CreatedAtWA:       g.GroupCreated,
		IsAnnounce:        g.IsAnnounce,
		IsLocked:          g.IsLocked,
		IsIncognito:       g.IsIncognito,
		EphemeralDuration: uint32(g.DisappearingTimer),
		MemberAddMode:     string(g.MemberAddMode),
		IsCommunity:       g.IsParent,
		IsParentGroup:     g.IsParent,
		IsDefaultSubgroup: g.IsDefaultSubGroup,
		ParticipantCount:  g.ParticipantCount,
		UpdatedAt:         time.Now(),
	}

	if !g.LinkedParentJID.IsEmpty() {
		group.LinkedParentJID = g.LinkedParentJID
	}

	return group
}

func extractInviteCode(link string) string {
	// Link format: https://chat.whatsapp.com/XXXX
	if len(link) > 24 {
		return link[len(link)-22:]
	}
	return ""
}

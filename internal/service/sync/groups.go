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
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing all groups")

	groups, err := s.client.GetJoinedGroups(ctx)
	if err != nil {
		s.log.Errorf("Failed to get joined groups: %v", err)
		return err
	}
	if groups == nil {
		return nil
	}

	for _, g := range groups {
		if g == nil {
			continue
		}
		g.JID = s.utils.NormalizeJID(ctx, g.JID)
		g.OwnerJID = s.utils.NormalizeJID(ctx, g.OwnerJID)
		g.NameSetBy = s.utils.NormalizeJID(ctx, g.NameSetBy)
		g.TopicSetBy = s.utils.NormalizeJID(ctx, g.TopicSetBy)

		group := groupInfoToStore(g)

		if err := s.groups.Put(group); err != nil {
			s.log.Warnf("Failed to save group %s: %v", g.JID, err)
			continue
		}

		for _, p := range g.Participants {
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
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	// Only handle actual groups, not broadcasts
	if jid.Server != types.GroupServer {
		return nil
	}
	s.log.Debugf("Syncing group info for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)
	info, err := s.client.GetGroupInfo(ctx, jid)
	if err != nil {
		s.log.Errorf("Failed to get group info for %s: %v", jid, err)
		return err
	}
	if info == nil {
		return nil
	}

	info.JID = s.utils.NormalizeJID(ctx, info.JID)
	info.OwnerJID = s.utils.NormalizeJID(ctx, info.OwnerJID)
	info.NameSetBy = s.utils.NormalizeJID(ctx, info.NameSetBy)
	info.TopicSetBy = s.utils.NormalizeJID(ctx, info.TopicSetBy)

	group := groupInfoToStore(info)
	if err := s.groups.Put(group); err != nil {
		s.log.Errorf("Failed to save group %s: %v", jid, err)
		return err
	}

	for _, p := range info.Participants {
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
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	if jid.Server != types.GroupServer {
		return nil
	}
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
// Called when: new group, picture changed event, or explicit refresh.
func (s *SyncService) SyncGroupPicture(ctx context.Context, jid types.JID, isCommunity bool) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	if jid.Server != types.GroupServer {
		return nil
	}
	s.log.Debugf("Syncing group picture for %s", jid)

	jid = s.utils.NormalizeJID(ctx, jid)

	existingID := ""
	if group, err := s.groups.Get(jid); err == nil && group != nil {
		existingID = group.ProfilePicID
	}

	params := &whatsmeow.GetProfilePictureParams{
		Preview:     false,
		ExistingID:  existingID,
		IsCommunity: isCommunity,
	}

	pic, err := s.client.GetProfilePictureInfo(ctx, jid, params)
	if err != nil || pic == nil {
		s.log.Debugf("No group picture for %s: %v", jid, err)
		// Mark as attempted with "0"
		s.groups.UpdateProfilePic(jid, "0", "")
		return nil
	}

	if err := s.groups.UpdateProfilePic(jid, pic.ID, pic.URL); err != nil {
		s.log.Errorf("Failed to update group picture for %s: %v", jid, err)
		return err
	}

	s.log.Infof("Synced group picture for %s", jid)
	s.recordSync("group_picture")
	return nil
}

// SyncGroupFromLink fetches group info from invite link.
func (s *SyncService) SyncGroupFromLink(ctx context.Context, code string) (*types.GroupInfo, error) {
	if s.client == nil || ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.log.Debugf("Syncing group from link %s", code)

	info, err := s.client.GetGroupInfoFromLink(ctx, code)
	if err != nil {
		s.log.Errorf("Failed to get group info from link %s: %v", code, err)
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

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

// SyncAllGroupPictures syncs profile pictures for groups without picture ID.
func (s *SyncService) SyncAllGroupPictures(ctx context.Context) error {
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Debugf("Syncing group pictures (missing only)")

	groups, err := s.groups.GetAll()
	if err != nil || groups == nil {
		s.log.Errorf("Failed to get all groups: %v", err)
		return err
	}

	synced := 0
	skipped := 0
	for _, group := range groups {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip if already attempted (has any ProfilePicID including "0")
		if group.ProfilePicID != "" {
			skipped++
			continue
		}

		group.JID = s.utils.NormalizeJID(ctx, group.JID)

		if err := s.SyncGroupPicture(ctx, group.JID, group.IsCommunity); err != nil {
			s.log.Warnf("Failed to sync group picture for %s: %v", group.JID, err)
		} else {
			synced++
		}
	}

	s.log.Infof("Synced %d group pictures, skipped %d", synced, skipped)
	s.recordSync("group_pictures")
	return nil
}

// Helper functions

func groupInfoToStore(g *types.GroupInfo) *store.Group {
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
	if len(link) > 24 {
		return link[len(link)-22:]
	}
	return ""
}

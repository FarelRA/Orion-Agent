package store

import (
	"database/sql"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Group represents a complete group with all fields.
type Group struct {
	JID types.JID

	// Basic info
	Name         string
	NameSetAt    time.Time
	NameSetByLID types.JID

	// Topic/Description
	Topic         string
	TopicID       string
	TopicSetAt    time.Time
	TopicSetByLID types.JID

	// Owner
	OwnerLID     types.JID
	CreatedAtWA  time.Time
	CreatedByLID types.JID

	// Settings
	IsAnnounce        bool
	IsLocked          bool
	IsIncognito       bool
	EphemeralDuration uint32
	MemberAddMode     string

	// Community
	IsCommunity       bool
	IsParentGroup     bool
	ParentGroupJID    types.JID
	IsDefaultSubgroup bool
	LinkedParentJID   types.JID

	// Participants
	ParticipantCount int

	// Invite
	InviteLink       string
	InviteCode       string
	InviteExpiration time.Time

	// Picture
	ProfilePicID  string
	ProfilePicURL string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// GroupParticipant represents a group participant.
type GroupParticipant struct {
	GroupJID     types.JID
	MemberLID    types.JID
	IsAdmin      bool
	IsSuperAdmin bool
	DisplayName  string
	JoinedAt     time.Time
	ErrorCode    int
	AddedByLID   types.JID
}

// PastParticipant represents someone who left a group.
type PastParticipant struct {
	GroupJID       types.JID
	MemberLID      types.JID
	LeaveReason    int
	LeaveTimestamp time.Time
}

// GroupStore handles group operations.
type GroupStore struct {
	store *Store
}

// NewGroupStore creates a new GroupStore.
func NewGroupStore(s *Store) *GroupStore {
	return &GroupStore{store: s}
}

// Put stores or updates a group.
func (s *GroupStore) Put(g *Group) error {
	now := time.Now().Unix()

	var nameSetAt, topicSetAt, createdAtWA, inviteExp sql.NullInt64
	if !g.NameSetAt.IsZero() {
		nameSetAt.Int64 = g.NameSetAt.Unix()
		nameSetAt.Valid = true
	}
	if !g.TopicSetAt.IsZero() {
		topicSetAt.Int64 = g.TopicSetAt.Unix()
		topicSetAt.Valid = true
	}
	if !g.CreatedAtWA.IsZero() {
		createdAtWA.Int64 = g.CreatedAtWA.Unix()
		createdAtWA.Valid = true
	}
	if !g.InviteExpiration.IsZero() {
		inviteExp.Int64 = g.InviteExpiration.Unix()
		inviteExp.Valid = true
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_groups (
			jid, name, name_set_at, name_set_by_lid,
			topic, topic_id, topic_set_at, topic_set_by_lid,
			owner_lid, created_at_wa, created_by_lid,
			is_announce, is_locked, is_incognito, ephemeral_duration, member_add_mode,
			is_community, is_parent_group, parent_group_jid, is_default_subgroup, linked_parent_jid,
			participant_count,
			invite_link, invite_code, invite_expiration,
			profile_pic_id, profile_pic_url,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = COALESCE(excluded.name, orion_groups.name),
			name_set_at = COALESCE(excluded.name_set_at, orion_groups.name_set_at),
			name_set_by_lid = COALESCE(excluded.name_set_by_lid, orion_groups.name_set_by_lid),
			topic = COALESCE(excluded.topic, orion_groups.topic),
			topic_id = COALESCE(excluded.topic_id, orion_groups.topic_id),
			topic_set_at = COALESCE(excluded.topic_set_at, orion_groups.topic_set_at),
			topic_set_by_lid = COALESCE(excluded.topic_set_by_lid, orion_groups.topic_set_by_lid),
			owner_lid = COALESCE(excluded.owner_lid, orion_groups.owner_lid),
			created_at_wa = COALESCE(excluded.created_at_wa, orion_groups.created_at_wa),
			created_by_lid = COALESCE(excluded.created_by_lid, orion_groups.created_by_lid),
			is_announce = excluded.is_announce,
			is_locked = excluded.is_locked,
			is_incognito = excluded.is_incognito,
			ephemeral_duration = COALESCE(excluded.ephemeral_duration, orion_groups.ephemeral_duration),
			member_add_mode = COALESCE(excluded.member_add_mode, orion_groups.member_add_mode),
			is_community = excluded.is_community,
			is_parent_group = excluded.is_parent_group,
			parent_group_jid = COALESCE(excluded.parent_group_jid, orion_groups.parent_group_jid),
			is_default_subgroup = excluded.is_default_subgroup,
			linked_parent_jid = COALESCE(excluded.linked_parent_jid, orion_groups.linked_parent_jid),
			participant_count = excluded.participant_count,
			invite_link = COALESCE(excluded.invite_link, orion_groups.invite_link),
			invite_code = COALESCE(excluded.invite_code, orion_groups.invite_code),
			invite_expiration = COALESCE(excluded.invite_expiration, orion_groups.invite_expiration),
			profile_pic_id = COALESCE(excluded.profile_pic_id, orion_groups.profile_pic_id),
			profile_pic_url = COALESCE(excluded.profile_pic_url, orion_groups.profile_pic_url),
			updated_at = excluded.updated_at
	`,
		g.JID.String(), nullString(g.Name), nameSetAt, nullJID(g.NameSetByLID),
		nullString(g.Topic), nullString(g.TopicID), topicSetAt, nullJID(g.TopicSetByLID),
		nullJID(g.OwnerLID), createdAtWA, nullJID(g.CreatedByLID),
		boolToInt(g.IsAnnounce), boolToInt(g.IsLocked), boolToInt(g.IsIncognito), int(g.EphemeralDuration), nullString(g.MemberAddMode),
		boolToInt(g.IsCommunity), boolToInt(g.IsParentGroup), nullJID(g.ParentGroupJID), boolToInt(g.IsDefaultSubgroup), nullJID(g.LinkedParentJID),
		g.ParticipantCount,
		nullString(g.InviteLink), nullString(g.InviteCode), inviteExp,
		nullString(g.ProfilePicID), nullString(g.ProfilePicURL),
		now, now,
	)
	return err
}

// Get retrieves a group by JID.
func (s *GroupStore) Get(jid types.JID) (*Group, error) {
	row := s.store.QueryRow(`
		SELECT jid, name, name_set_at, name_set_by_lid,
			topic, topic_id, topic_set_at, topic_set_by_lid,
			owner_lid, created_at_wa, created_by_lid,
			is_announce, is_locked, is_incognito, ephemeral_duration, member_add_mode,
			is_community, is_parent_group, parent_group_jid, is_default_subgroup, linked_parent_jid,
			participant_count,
			invite_link, invite_code, invite_expiration,
			profile_pic_id, profile_pic_url,
			created_at, updated_at
		FROM orion_groups WHERE jid = ?
	`, jid.String())

	return s.scanGroup(row)
}

// GetAll retrieves all groups.
func (s *GroupStore) GetAll() ([]*Group, error) {
	rows, err := s.store.Query(`
		SELECT jid, name, name_set_at, name_set_by_lid,
			topic, topic_id, topic_set_at, topic_set_by_lid,
			owner_lid, created_at_wa, created_by_lid,
			is_announce, is_locked, is_incognito, ephemeral_duration, member_add_mode,
			is_community, is_parent_group, parent_group_jid, is_default_subgroup, linked_parent_jid,
			participant_count,
			invite_link, invite_code, invite_expiration,
			profile_pic_id, profile_pic_url,
			created_at, updated_at
		FROM orion_groups ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		g, err := s.scanGroupRow(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// PutParticipant stores or updates a group participant.
func (s *GroupStore) PutParticipant(p *GroupParticipant) error {
	var joinedAt sql.NullInt64
	if !p.JoinedAt.IsZero() {
		joinedAt.Int64 = p.JoinedAt.Unix()
		joinedAt.Valid = true
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_group_participants (group_jid, member_lid, is_admin, is_superadmin, display_name, joined_at, error_code, added_by_lid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_jid, member_lid) DO UPDATE SET
			is_admin = excluded.is_admin,
			is_superadmin = excluded.is_superadmin,
			display_name = COALESCE(excluded.display_name, orion_group_participants.display_name),
			error_code = excluded.error_code
	`, p.GroupJID.String(), p.MemberLID.String(), boolToInt(p.IsAdmin), boolToInt(p.IsSuperAdmin),
		nullString(p.DisplayName), joinedAt, nullInt(p.ErrorCode), nullJID(p.AddedByLID))
	return err
}

// PutParticipants stores or updates multiple participants.
func (s *GroupStore) PutParticipants(participants []GroupParticipant) error {
	tx, err := s.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO orion_group_participants (group_jid, member_lid, is_admin, is_superadmin, display_name, joined_at, error_code, added_by_lid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_jid, member_lid) DO UPDATE SET
			is_admin = excluded.is_admin,
			is_superadmin = excluded.is_superadmin,
			display_name = COALESCE(excluded.display_name, orion_group_participants.display_name)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range participants {
		var joinedAt sql.NullInt64
		if !p.JoinedAt.IsZero() {
			joinedAt.Int64 = p.JoinedAt.Unix()
			joinedAt.Valid = true
		}
		if _, err := stmt.Exec(p.GroupJID.String(), p.MemberLID.String(), boolToInt(p.IsAdmin), boolToInt(p.IsSuperAdmin),
			nullString(p.DisplayName), joinedAt, nullInt(p.ErrorCode), nullJID(p.AddedByLID)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetParticipants retrieves all participants for a group.
func (s *GroupStore) GetParticipants(groupJID types.JID) ([]GroupParticipant, error) {
	rows, err := s.store.Query(`
		SELECT group_jid, member_lid, is_admin, is_superadmin, display_name, joined_at, error_code, added_by_lid
		FROM orion_group_participants WHERE group_jid = ?
	`, groupJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []GroupParticipant
	for rows.Next() {
		var groupJIDStr, memberLIDStr string
		var displayName, addedByLID sql.NullString
		var isAdmin, isSuperAdmin, errorCode int
		var joinedAt sql.NullInt64

		if err := rows.Scan(&groupJIDStr, &memberLIDStr, &isAdmin, &isSuperAdmin, &displayName, &joinedAt, &errorCode, &addedByLID); err != nil {
			return nil, err
		}

		groupJID, _ := types.ParseJID(groupJIDStr)
		memberLID, _ := types.ParseJID(memberLIDStr)

		p := GroupParticipant{
			GroupJID:     groupJID,
			MemberLID:    memberLID,
			IsAdmin:      isAdmin == 1,
			IsSuperAdmin: isSuperAdmin == 1,
			DisplayName:  displayName.String,
			ErrorCode:    errorCode,
		}
		if joinedAt.Valid {
			p.JoinedAt = time.Unix(joinedAt.Int64, 0)
		}
		if addedByLID.Valid {
			p.AddedByLID, _ = types.ParseJID(addedByLID.String)
		}

		participants = append(participants, p)
	}
	return participants, nil
}

// RemoveParticipant removes a participant from a group.
func (s *GroupStore) RemoveParticipant(groupJID, memberLID types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_group_participants WHERE group_jid = ? AND member_lid = ?`,
		groupJID.String(), memberLID.String())
	return err
}

// ClearParticipants removes all participants for a group.
func (s *GroupStore) ClearParticipants(groupJID types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_group_participants WHERE group_jid = ?`, groupJID.String())
	return err
}

// PutPastParticipant stores a past participant.
func (s *GroupStore) PutPastParticipant(p *PastParticipant) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_past_participants (group_jid, member_lid, leave_reason, leave_timestamp)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(group_jid, member_lid, leave_timestamp) DO NOTHING
	`, p.GroupJID.String(), p.MemberLID.String(), p.LeaveReason, p.LeaveTimestamp.Unix())
	return err
}

// UpdateInviteLink updates the group invite link.
func (s *GroupStore) UpdateInviteLink(jid types.JID, link, code string, expiration time.Time) error {
	now := time.Now().Unix()
	var exp sql.NullInt64
	if !expiration.IsZero() {
		exp.Int64 = expiration.Unix()
		exp.Valid = true
	}
	_, err := s.store.Exec(`
		UPDATE orion_groups SET invite_link = ?, invite_code = ?, invite_expiration = ?, updated_at = ? WHERE jid = ?
	`, link, code, exp, now, jid.String())
	return err
}

// UpdateProfilePic updates the group profile picture.
func (s *GroupStore) UpdateProfilePic(jid types.JID, picID, picURL string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_groups SET profile_pic_id = ?, profile_pic_url = ?, updated_at = ? WHERE jid = ?
	`, picID, picURL, now, jid.String())
	return err
}

// Exists checks if a group exists.
func (s *GroupStore) Exists(jid types.JID) (bool, error) {
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_groups WHERE jid = ?`, jid.String()).Scan(&count)
	return count > 0, err
}

func (s *GroupStore) scanGroup(row *sql.Row) (*Group, error) {
	var jidStr string
	var name, topicID, topic, memberAddMode sql.NullString
	var nameSetByLID, topicSetByLID, ownerLID, createdByLID sql.NullString
	var parentGroupJID, linkedParentJID sql.NullString
	var inviteLink, inviteCode sql.NullString
	var profilePicID, profilePicURL sql.NullString
	var nameSetAt, topicSetAt, createdAtWA, inviteExp sql.NullInt64
	var isAnnounce, isLocked, isIncognito, ephemeralDur int
	var isCommunity, isParentGroup, isDefaultSubgroup int
	var participantCount int
	var createdAt, updatedAt int64

	err := row.Scan(
		&jidStr, &name, &nameSetAt, &nameSetByLID,
		&topic, &topicID, &topicSetAt, &topicSetByLID,
		&ownerLID, &createdAtWA, &createdByLID,
		&isAnnounce, &isLocked, &isIncognito, &ephemeralDur, &memberAddMode,
		&isCommunity, &isParentGroup, &parentGroupJID, &isDefaultSubgroup, &linkedParentJID,
		&participantCount,
		&inviteLink, &inviteCode, &inviteExp,
		&profilePicID, &profilePicURL,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	jid, _ := types.ParseJID(jidStr)

	g := &Group{
		JID:               jid,
		Name:              name.String,
		Topic:             topic.String,
		TopicID:           topicID.String,
		IsAnnounce:        isAnnounce == 1,
		IsLocked:          isLocked == 1,
		IsIncognito:       isIncognito == 1,
		EphemeralDuration: uint32(ephemeralDur),
		MemberAddMode:     memberAddMode.String,
		IsCommunity:       isCommunity == 1,
		IsParentGroup:     isParentGroup == 1,
		IsDefaultSubgroup: isDefaultSubgroup == 1,
		ParticipantCount:  participantCount,
		InviteLink:        inviteLink.String,
		InviteCode:        inviteCode.String,
		ProfilePicID:      profilePicID.String,
		ProfilePicURL:     profilePicURL.String,
		CreatedAt:         time.Unix(createdAt, 0),
		UpdatedAt:         time.Unix(updatedAt, 0),
	}

	if nameSetAt.Valid {
		g.NameSetAt = time.Unix(nameSetAt.Int64, 0)
	}
	if nameSetByLID.Valid {
		g.NameSetByLID, _ = types.ParseJID(nameSetByLID.String)
	}
	if topicSetAt.Valid {
		g.TopicSetAt = time.Unix(topicSetAt.Int64, 0)
	}
	if topicSetByLID.Valid {
		g.TopicSetByLID, _ = types.ParseJID(topicSetByLID.String)
	}
	if ownerLID.Valid {
		g.OwnerLID, _ = types.ParseJID(ownerLID.String)
	}
	if createdAtWA.Valid {
		g.CreatedAtWA = time.Unix(createdAtWA.Int64, 0)
	}
	if createdByLID.Valid {
		g.CreatedByLID, _ = types.ParseJID(createdByLID.String)
	}
	if parentGroupJID.Valid {
		g.ParentGroupJID, _ = types.ParseJID(parentGroupJID.String)
	}
	if linkedParentJID.Valid {
		g.LinkedParentJID, _ = types.ParseJID(linkedParentJID.String)
	}
	if inviteExp.Valid {
		g.InviteExpiration = time.Unix(inviteExp.Int64, 0)
	}

	return g, nil
}

func (s *GroupStore) scanGroupRow(rows *sql.Rows) (*Group, error) {
	var jidStr string
	var name, topicID, topic, memberAddMode sql.NullString
	var nameSetByLID, topicSetByLID, ownerLID, createdByLID sql.NullString
	var parentGroupJID, linkedParentJID sql.NullString
	var inviteLink, inviteCode sql.NullString
	var profilePicID, profilePicURL sql.NullString
	var nameSetAt, topicSetAt, createdAtWA, inviteExp sql.NullInt64
	var isAnnounce, isLocked, isIncognito, ephemeralDur int
	var isCommunity, isParentGroup, isDefaultSubgroup int
	var participantCount int
	var createdAt, updatedAt int64

	err := rows.Scan(
		&jidStr, &name, &nameSetAt, &nameSetByLID,
		&topic, &topicID, &topicSetAt, &topicSetByLID,
		&ownerLID, &createdAtWA, &createdByLID,
		&isAnnounce, &isLocked, &isIncognito, &ephemeralDur, &memberAddMode,
		&isCommunity, &isParentGroup, &parentGroupJID, &isDefaultSubgroup, &linkedParentJID,
		&participantCount,
		&inviteLink, &inviteCode, &inviteExp,
		&profilePicID, &profilePicURL,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	jid, _ := types.ParseJID(jidStr)

	g := &Group{
		JID:               jid,
		Name:              name.String,
		Topic:             topic.String,
		TopicID:           topicID.String,
		IsAnnounce:        isAnnounce == 1,
		IsLocked:          isLocked == 1,
		IsIncognito:       isIncognito == 1,
		EphemeralDuration: uint32(ephemeralDur),
		MemberAddMode:     memberAddMode.String,
		IsCommunity:       isCommunity == 1,
		IsParentGroup:     isParentGroup == 1,
		IsDefaultSubgroup: isDefaultSubgroup == 1,
		ParticipantCount:  participantCount,
		InviteLink:        inviteLink.String,
		InviteCode:        inviteCode.String,
		ProfilePicID:      profilePicID.String,
		ProfilePicURL:     profilePicURL.String,
		CreatedAt:         time.Unix(createdAt, 0),
		UpdatedAt:         time.Unix(updatedAt, 0),
	}

	if nameSetAt.Valid {
		g.NameSetAt = time.Unix(nameSetAt.Int64, 0)
	}
	if nameSetByLID.Valid {
		g.NameSetByLID, _ = types.ParseJID(nameSetByLID.String)
	}
	if topicSetAt.Valid {
		g.TopicSetAt = time.Unix(topicSetAt.Int64, 0)
	}
	if topicSetByLID.Valid {
		g.TopicSetByLID, _ = types.ParseJID(topicSetByLID.String)
	}
	if ownerLID.Valid {
		g.OwnerLID, _ = types.ParseJID(ownerLID.String)
	}
	if createdAtWA.Valid {
		g.CreatedAtWA = time.Unix(createdAtWA.Int64, 0)
	}
	if createdByLID.Valid {
		g.CreatedByLID, _ = types.ParseJID(createdByLID.String)
	}
	if parentGroupJID.Valid {
		g.ParentGroupJID, _ = types.ParseJID(parentGroupJID.String)
	}
	if linkedParentJID.Valid {
		g.LinkedParentJID, _ = types.ParseJID(linkedParentJID.String)
	}
	if inviteExp.Valid {
		g.InviteExpiration = time.Unix(inviteExp.Int64, 0)
	}

	return g, nil
}

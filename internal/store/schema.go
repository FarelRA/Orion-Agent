package store

// schema contains all app-specific table definitions.
// Tables are designed to capture ALL whatsmeow event data + sync data.
//
// Tables:
//   - orion_contacts - Contact data with PN (replaces jid_mapping)
//   - orion_chats - Chat metadata
//   - orion_messages - Comprehensive message storage
//   - orion_message_receipts - Delivery receipts
//   - orion_message_edits - Edit history
//   - orion_reactions - Message reactions
//   - orion_groups - Full group metadata
//   - orion_group_participants - Current participants
//   - orion_past_participants - Participant history
//   - orion_broadcast_lists - Broadcast lists
//   - orion_broadcast_recipients - Broadcast recipients
//   - orion_newsletters - Channel/newsletter data
//   - orion_status_updates - Status updates
//   - orion_polls - Poll data
//   - orion_poll_votes - Poll votes
//   - orion_blocklist - Blocked contacts
//   - orion_labels - Business labels
//   - orion_label_associations - Label assignments
//   - orion_calls - Call history
//   - orion_privacy_settings - Privacy settings
//   - orion_settings - Global settings
//   - orion_media_cache - Downloaded media cache
//   - orion_sync_state - Sync progress tracking
const schema = `
-- ============================================================
-- Contacts (with PN - replaces jid_mapping)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_contacts (
    lid TEXT PRIMARY KEY,
    pn TEXT,
    
    -- Names
    push_name TEXT,
    business_name TEXT,
    server_name TEXT,
    full_name TEXT,
    first_name TEXT,
    
    -- Profile Picture
    profile_pic_id TEXT,
    profile_pic_url TEXT,
    
    -- Status
    status TEXT,
    status_set_at INTEGER,
    
    -- Presence
    is_online INTEGER DEFAULT 0,
    last_seen INTEGER,
    
    -- Business Profile
    is_business INTEGER DEFAULT 0,
    business_description TEXT,
    business_category TEXT,
    business_email TEXT,
    business_website TEXT,
    business_address TEXT,
    
    -- Verification
    verified_name TEXT,
    verified_level INTEGER,
    
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_orion_contacts_pn ON orion_contacts(pn);

-- ============================================================
-- Chats
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_chats (
    jid TEXT PRIMARY KEY,
    chat_type TEXT NOT NULL,
    name TEXT,
    
    -- Message counts
    unread_count INTEGER DEFAULT 0,
    unread_mention_count INTEGER DEFAULT 0,
    last_message_id TEXT,
    last_message_at INTEGER,
    
    -- State
    marked_as_unread INTEGER DEFAULT 0,
    is_archived INTEGER DEFAULT 0,
    is_pinned INTEGER DEFAULT 0,
    pin_timestamp INTEGER,
    muted_until INTEGER,
    
    -- Ephemeral
    ephemeral_duration INTEGER,
    ephemeral_setting_timestamp INTEGER,
    
    -- Sync
    conversation_timestamp INTEGER,
    end_of_history_transfer INTEGER DEFAULT 0,
    
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Messages (cleaned - removed media_url, thumbnail hashes, etc)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_messages (
    id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    sender_lid TEXT NOT NULL,
    from_me INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    server_id INTEGER,
    push_name TEXT,
    
    -- Message type
    message_type TEXT NOT NULL,
    
    -- Text content
    text_content TEXT,
    caption TEXT,
    
    -- Media fields (required for download)
    media_direct_path TEXT,
    media_key BLOB,
    media_key_timestamp INTEGER,
    file_sha256 BLOB,
    file_enc_sha256 BLOB,
    file_length INTEGER,
    mimetype TEXT,
    media_url TEXT,
    
    -- Media metadata
    width INTEGER,
    height INTEGER,
    duration_seconds INTEGER,
    thumbnail BLOB,
    thumbnail_direct_path TEXT,
    thumbnail_sha256 BLOB,
    thumbnail_enc_sha256 BLOB,
    
    -- Audio specific
    is_ptt INTEGER DEFAULT 0,
    waveform BLOB,
    
    -- Video specific
    is_gif INTEGER DEFAULT 0,
    is_animated INTEGER DEFAULT 0,
    streaming_sidecar BLOB,
    
    -- Quote/Reply context
    quoted_message_id TEXT,
    quoted_sender_lid TEXT,
    quoted_message_type TEXT,
    quoted_content TEXT,
    
    -- Mentions
    mentioned_jids TEXT,
    group_mentions TEXT,
    
    -- Forwarding
    is_forwarded INTEGER DEFAULT 0,
    forwarding_score INTEGER,
    
    -- Location (if location message)
    latitude REAL,
    longitude REAL,
    location_name TEXT,
    location_address TEXT,
    location_url TEXT,
    is_live_location INTEGER DEFAULT 0,
    accuracy_meters INTEGER,
    speed_mps REAL,
    degrees_clockwise INTEGER,
    live_location_sequence INTEGER,
    
    -- Contact (if contact message)
    vcard TEXT,
    display_name TEXT,
    
    -- Poll (if poll message)
    poll_name TEXT,
    poll_options TEXT,
    poll_select_max INTEGER,
    poll_encryption_key BLOB,
    
    -- Flags
    is_broadcast INTEGER DEFAULT 0,
    broadcast_list_jid TEXT,
    is_ephemeral INTEGER DEFAULT 0,
    is_view_once INTEGER DEFAULT 0,
    is_starred INTEGER DEFAULT 0,
    is_edited INTEGER DEFAULT 0,
    edit_timestamp INTEGER,
    is_revoked INTEGER DEFAULT 0,
    
    -- Protocol message info
    protocol_type INTEGER,
    
    created_at INTEGER NOT NULL,
    PRIMARY KEY (id, chat_jid)
);
CREATE INDEX IF NOT EXISTS idx_orion_messages_chat ON orion_messages(chat_jid, timestamp);
CREATE INDEX IF NOT EXISTS idx_orion_messages_sender ON orion_messages(sender_lid);
CREATE INDEX IF NOT EXISTS idx_orion_messages_starred ON orion_messages(is_starred) WHERE is_starred = 1;

-- ============================================================
-- Message receipts (delivery/read status)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_message_receipts (
    message_id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    recipient_lid TEXT NOT NULL,
    receipt_type TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    PRIMARY KEY (message_id, chat_jid, recipient_lid, receipt_type)
);

-- ============================================================
-- Message edits history
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_message_edits (
    message_id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    edit_number INTEGER NOT NULL,
    old_content TEXT,
    new_content TEXT,
    edited_at INTEGER NOT NULL,
    PRIMARY KEY (message_id, chat_jid, edit_number)
);

-- ============================================================
-- Reactions
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_reactions (
    message_id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    sender_lid TEXT NOT NULL,
    emoji TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    PRIMARY KEY (message_id, chat_jid, sender_lid)
);

-- ============================================================
-- Groups
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_groups (
    jid TEXT PRIMARY KEY,
    
    -- Basic info
    name TEXT,
    name_set_at INTEGER,
    name_set_by_lid TEXT,
    
    -- Topic/Description
    topic TEXT,
    topic_id TEXT,
    topic_set_at INTEGER,
    topic_set_by_lid TEXT,
    
    -- Owner
    owner_lid TEXT,
    created_at_wa INTEGER,
    created_by_lid TEXT,
    
    -- Settings
    is_announce INTEGER DEFAULT 0,
    is_locked INTEGER DEFAULT 0,
    is_incognito INTEGER DEFAULT 0,
    ephemeral_duration INTEGER,
    
    -- Membership
    member_add_mode TEXT,
    
    -- Community
    is_community INTEGER DEFAULT 0,
    is_parent_group INTEGER DEFAULT 0,
    parent_group_jid TEXT,
    is_default_subgroup INTEGER DEFAULT 0,
    linked_parent_jid TEXT,
    
    -- Participants
    participant_count INTEGER,
    
    -- Invite
    invite_link TEXT,
    invite_code TEXT,
    invite_expiration INTEGER,
    
    -- Picture
    profile_pic_id TEXT,
    profile_pic_url TEXT,
    
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Group participants
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_group_participants (
    group_jid TEXT NOT NULL,
    member_lid TEXT NOT NULL,
    is_admin INTEGER DEFAULT 0,
    is_superadmin INTEGER DEFAULT 0,
    display_name TEXT,
    joined_at INTEGER,
    error_code INTEGER,
    added_by_lid TEXT,
    PRIMARY KEY (group_jid, member_lid)
);
CREATE INDEX IF NOT EXISTS idx_orion_group_participants_member ON orion_group_participants(member_lid);

-- ============================================================
-- Past group participants
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_past_participants (
    group_jid TEXT NOT NULL,
    member_lid TEXT NOT NULL,
    leave_reason INTEGER,
    leave_timestamp INTEGER,
    PRIMARY KEY (group_jid, member_lid, leave_timestamp)
);

-- ============================================================
-- Broadcast lists
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_broadcast_lists (
    jid TEXT PRIMARY KEY,
    name TEXT,
    recipient_count INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Broadcast recipients
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_broadcast_recipients (
    broadcast_jid TEXT NOT NULL,
    recipient_lid TEXT NOT NULL,
    PRIMARY KEY (broadcast_jid, recipient_lid)
);

-- ============================================================
-- Newsletters/Channels
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_newsletters (
    jid TEXT PRIMARY KEY,
    
    -- Basic info
    name TEXT,
    description TEXT,
    
    -- State
    verification_state TEXT,
    
    -- Counts
    subscriber_count INTEGER,
    
    -- Access
    invite_code TEXT,
    invite_link TEXT,
    
    -- User state
    role TEXT,
    muted INTEGER DEFAULT 0,
    
    -- Picture
    picture_id TEXT,
    picture_url TEXT,
    preview_url TEXT,
    
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Status updates
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_status_updates (
    id TEXT PRIMARY KEY,
    sender_lid TEXT NOT NULL,
    message_type TEXT NOT NULL,
    
    -- Content
    text_content TEXT,
    caption TEXT,
    
    -- Media
    media_direct_path TEXT,
    media_key BLOB,
    file_sha256 BLOB,
    file_enc_sha256 BLOB,
    file_length INTEGER,
    mimetype TEXT,
    thumbnail BLOB,
    
    -- Timing
    timestamp INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_orion_status_sender ON orion_status_updates(sender_lid);

-- ============================================================
-- Polls (separate for complex structure)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_polls (
    message_id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    creator_lid TEXT,
    question TEXT NOT NULL,
    options TEXT NOT NULL,
    is_multi_select INTEGER DEFAULT 0,
    select_max INTEGER,
    encryption_key BLOB,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (message_id, chat_jid)
);

-- ============================================================
-- Poll votes
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_poll_votes (
    message_id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    voter_lid TEXT NOT NULL,
    selected_options TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    PRIMARY KEY (message_id, chat_jid, voter_lid)
);

-- ============================================================
-- Blocklist
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_blocklist (
    lid TEXT PRIMARY KEY,
    blocked_at INTEGER NOT NULL
);

-- ============================================================
-- Labels (for business)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_labels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    color INTEGER,
    sort_order INTEGER,
    predefined_id INTEGER,
    deleted INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Label associations
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_label_associations (
    label_id TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_jid TEXT NOT NULL,
    message_id TEXT,
    timestamp INTEGER NOT NULL,
    PRIMARY KEY (label_id, target_type, target_jid, COALESCE(message_id, ''))
);
CREATE INDEX IF NOT EXISTS idx_orion_label_assoc_target ON orion_label_associations(target_jid);

-- ============================================================
-- Calls
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_calls (
    call_id TEXT PRIMARY KEY,
    caller_lid TEXT NOT NULL,
    group_jid TEXT,
    call_type TEXT,
    is_video INTEGER DEFAULT 0,
    is_group INTEGER DEFAULT 0,
    timestamp INTEGER NOT NULL,
    duration_seconds INTEGER,
    outcome TEXT,
    participants TEXT
);
CREATE INDEX IF NOT EXISTS idx_orion_calls_time ON orion_calls(timestamp);

-- Privacy settings
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_privacy_settings (
    id INTEGER PRIMARY KEY DEFAULT 1,
    group_add TEXT,
    last_seen TEXT,
    status TEXT,
    profile TEXT,
    read_receipts TEXT,
    online TEXT,
    call_add TEXT,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Status privacy types
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_status_privacy_types (
    type TEXT PRIMARY KEY,
    is_default INTEGER DEFAULT 0,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Status privacy members
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_status_privacy_members (
    type TEXT NOT NULL,
    jid TEXT NOT NULL,
    PRIMARY KEY (type, jid)
);

-- ============================================================
-- Global settings
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

-- ============================================================
-- Media cache (for downloaded files)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_media_cache (
    message_id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    media_type TEXT NOT NULL,
    local_path TEXT,
    downloaded_at INTEGER,
    file_size INTEGER,
    PRIMARY KEY (message_id, chat_jid)
);
CREATE INDEX IF NOT EXISTS idx_orion_media_cache_path ON orion_media_cache(local_path);

-- ============================================================
-- Sync state (track what's been synced)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_sync_state (
    sync_type TEXT PRIMARY KEY,
    last_sync_at INTEGER NOT NULL,
    sync_progress INTEGER,
    sync_data TEXT
);

-- ============================================================
-- Bots (AI/Business bots)
-- ============================================================
CREATE TABLE IF NOT EXISTS orion_bots (
    jid TEXT PRIMARY KEY,
    name TEXT,
    description TEXT,
    category TEXT,
    persona_id TEXT,
    profile_pic_id TEXT,
    profile_pic_url TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
`

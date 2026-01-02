package store

import (
	"database/sql"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Contact represents a complete contact with all fields.
// The PN field provides JID mapping (replaces orion_jid_mapping table).
type Contact struct {
	LID types.JID
	PN  types.JID

	// Names
	PushName     string
	BusinessName string
	ServerName   string
	FullName     string
	FirstName    string

	// Profile
	ProfilePicID  string
	ProfilePicURL string

	// Status
	Status      string
	StatusSetAt time.Time

	// Presence
	LastSeen time.Time
	IsOnline bool

	// Business profile
	IsBusiness          bool
	BusinessDescription string
	BusinessCategory    string
	BusinessEmail       string
	BusinessWebsite     string
	BusinessAddress     string

	// Verification
	VerifiedName  string
	VerifiedLevel int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ContactStore handles contact operations.
type ContactStore struct {
	store *Store
}

// NewContactStore creates a new ContactStore.
func NewContactStore(s *Store) *ContactStore {
	return &ContactStore{store: s}
}

// Put stores or updates a contact.
func (s *ContactStore) Put(c *Contact) error {
	now := time.Now().Unix()
	var statusSetAt, lastSeen sql.NullInt64

	if !c.StatusSetAt.IsZero() {
		statusSetAt.Int64 = c.StatusSetAt.Unix()
		statusSetAt.Valid = true
	}
	if !c.LastSeen.IsZero() {
		lastSeen.Int64 = c.LastSeen.Unix()
		lastSeen.Valid = true
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_contacts (
			lid, pn, push_name, business_name, server_name, full_name, first_name,
			profile_pic_id, profile_pic_url,
			status, status_set_at, last_seen, is_online,
			is_business, business_description, business_category, business_email, business_website, business_address,
			verified_name, verified_level, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET
			pn = COALESCE(excluded.pn, orion_contacts.pn),
			push_name = COALESCE(excluded.push_name, orion_contacts.push_name),
			business_name = COALESCE(excluded.business_name, orion_contacts.business_name),
			server_name = COALESCE(excluded.server_name, orion_contacts.server_name),
			full_name = COALESCE(excluded.full_name, orion_contacts.full_name),
			first_name = COALESCE(excluded.first_name, orion_contacts.first_name),
			profile_pic_id = COALESCE(excluded.profile_pic_id, orion_contacts.profile_pic_id),
			profile_pic_url = COALESCE(excluded.profile_pic_url, orion_contacts.profile_pic_url),
			status = COALESCE(excluded.status, orion_contacts.status),
			status_set_at = COALESCE(excluded.status_set_at, orion_contacts.status_set_at),
			last_seen = COALESCE(excluded.last_seen, orion_contacts.last_seen),
			is_online = excluded.is_online,
			is_business = CASE WHEN excluded.is_business = 1 THEN 1 ELSE orion_contacts.is_business END,
			business_description = COALESCE(excluded.business_description, orion_contacts.business_description),
			business_category = COALESCE(excluded.business_category, orion_contacts.business_category),
			business_email = COALESCE(excluded.business_email, orion_contacts.business_email),
			business_website = COALESCE(excluded.business_website, orion_contacts.business_website),
			business_address = COALESCE(excluded.business_address, orion_contacts.business_address),
			verified_name = COALESCE(excluded.verified_name, orion_contacts.verified_name),
			verified_level = COALESCE(excluded.verified_level, orion_contacts.verified_level),
			updated_at = excluded.updated_at
	`,
		c.LID.String(), nullJID(c.PN), nullString(c.PushName), nullString(c.BusinessName),
		nullString(c.ServerName), nullString(c.FullName), nullString(c.FirstName),
		nullString(c.ProfilePicID), nullString(c.ProfilePicURL),
		nullString(c.Status), statusSetAt, lastSeen, boolToInt(c.IsOnline),
		boolToInt(c.IsBusiness), nullString(c.BusinessDescription), nullString(c.BusinessCategory),
		nullString(c.BusinessEmail), nullString(c.BusinessWebsite), nullString(c.BusinessAddress),
		nullString(c.VerifiedName), nullInt(c.VerifiedLevel), now, now,
	)
	return err
}

// Get retrieves a contact by matching either LID or PN.
func (s *ContactStore) Get(jid types.JID) (*Contact, error) {
	jidStr := jid.String()
	row := s.store.QueryRow(`
		SELECT lid, pn, push_name, business_name, server_name, full_name, first_name,
			profile_pic_id, profile_pic_url,
			status, status_set_at, last_seen, is_online,
			is_business, business_description, business_category, business_email, business_website, business_address,
			verified_name, verified_level, created_at, updated_at
		FROM orion_contacts WHERE lid = ? OR pn = ?
	`, jidStr, jidStr)

	return s.scanContact(row)
}

// GetByLID retrieves a contact by LID.
func (s *ContactStore) GetByLID(lid types.JID) (*Contact, error) {
	row := s.store.QueryRow(`
		SELECT lid, pn, push_name, business_name, server_name, full_name, first_name,
			profile_pic_id, profile_pic_url,
			status, status_set_at, last_seen, is_online,
			is_business, business_description, business_category, business_email, business_website, business_address,
			verified_name, verified_level, created_at, updated_at
		FROM orion_contacts WHERE lid = ?
	`, lid.String())

	return s.scanContact(row)
}

// GetByPN retrieves a contact by phone number JID.
func (s *ContactStore) GetByPN(pn types.JID) (*Contact, error) {
	row := s.store.QueryRow(`
		SELECT lid, pn, push_name, business_name, server_name, full_name, first_name,
			profile_pic_id, profile_pic_url,
			status, status_set_at, last_seen, is_online,
			is_business, business_description, business_category, business_email, business_website, business_address,
			verified_name, verified_level, created_at, updated_at
		FROM orion_contacts WHERE pn = ?
	`, pn.String())

	return s.scanContact(row)
}

// GetAll retrieves all contacts.
func (s *ContactStore) GetAll() ([]*Contact, error) {
	rows, err := s.store.Query(`
		SELECT lid, pn, push_name, business_name, server_name, full_name, first_name,
			profile_pic_id, profile_pic_url,
			status, status_set_at, last_seen, is_online,
			is_business, business_description, business_category, business_email, business_website, business_address,
			verified_name, verified_level, created_at, updated_at
		FROM orion_contacts ORDER BY COALESCE(push_name, full_name, lid)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []*Contact
	for rows.Next() {
		c, err := s.scanContactRow(rows)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

// UpdatePresence updates online/last seen status.
func (s *ContactStore) UpdatePresence(lid types.JID, isOnline bool, lastSeen time.Time) error {
	now := time.Now().Unix()
	var lastSeenTs sql.NullInt64
	if !lastSeen.IsZero() {
		lastSeenTs.Int64 = lastSeen.Unix()
		lastSeenTs.Valid = true
	}
	_, err := s.store.Exec(`
		UPDATE orion_contacts SET is_online = ?, last_seen = COALESCE(?, last_seen), updated_at = ? WHERE lid = ?
	`, boolToInt(isOnline), lastSeenTs, now, lid.String())
	return err
}

// UpdatePushName updates the push name.
func (s *ContactStore) UpdatePushName(lid types.JID, pushName string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_contacts (lid, push_name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET push_name = excluded.push_name, updated_at = excluded.updated_at
	`, lid.String(), pushName, now, now)
	return err
}

// UpdateBusinessName updates the business name.
func (s *ContactStore) UpdateBusinessName(lid types.JID, businessName string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_contacts (lid, business_name, is_business, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET business_name = excluded.business_name, is_business = 1, updated_at = excluded.updated_at
	`, lid.String(), businessName, now, now)
	return err
}

// UpdateProfilePic updates the profile picture.
func (s *ContactStore) UpdateProfilePic(lid types.JID, picID, picURL string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_contacts (lid, profile_pic_id, profile_pic_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET 
			profile_pic_id = excluded.profile_pic_id, 
			profile_pic_url = excluded.profile_pic_url,
			updated_at = excluded.updated_at
	`, lid.String(), picID, picURL, now, now)
	return err
}

// UpdateStatus updates the user status.
func (s *ContactStore) UpdateStatus(lid types.JID, status string, setAt time.Time) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_contacts (lid, status, status_set_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET status = excluded.status, status_set_at = excluded.status_set_at, updated_at = excluded.updated_at
	`, lid.String(), status, setAt.Unix(), now, now)
	return err
}

// UpdateBusinessProfile updates business profile fields.
func (s *ContactStore) UpdateBusinessProfile(lid types.JID, desc, category, email, website, address string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_contacts SET 
			is_business = 1,
			business_description = COALESCE(?, business_description),
			business_category = COALESCE(?, business_category),
			business_email = COALESCE(?, business_email),
			business_website = COALESCE(?, business_website),
			business_address = COALESCE(?, business_address),
			updated_at = ?
		WHERE lid = ?
	`, nullString(desc), nullString(category), nullString(email), nullString(website), nullString(address), now, lid.String())
	return err
}

func (s *ContactStore) scanContact(row *sql.Row) (*Contact, error) {
	var lidStr string
	var pn, pushName, businessName, serverName, fullName, firstName sql.NullString
	var profilePicID, profilePicURL sql.NullString
	var status sql.NullString
	var businessDesc, businessCat, businessEmail, businessWeb, businessAddr sql.NullString
	var verifiedName sql.NullString
	var verifiedLevel sql.NullInt64
	var statusSetAt, lastSeen sql.NullInt64
	var isBusiness, isOnline int
	var createdAt, updatedAt int64

	err := row.Scan(
		&lidStr, &pn, &pushName, &businessName, &serverName, &fullName, &firstName,
		&profilePicID, &profilePicURL,
		&status, &statusSetAt, &lastSeen, &isOnline,
		&isBusiness, &businessDesc, &businessCat, &businessEmail, &businessWeb, &businessAddr,
		&verifiedName, &verifiedLevel, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	lid, _ := types.ParseJID(lidStr)

	c := &Contact{
		LID:                 lid,
		PushName:            pushName.String,
		BusinessName:        businessName.String,
		ServerName:          serverName.String,
		FullName:            fullName.String,
		FirstName:           firstName.String,
		ProfilePicID:        profilePicID.String,
		ProfilePicURL:       profilePicURL.String,
		Status:              status.String,
		IsBusiness:          isBusiness == 1,
		IsOnline:            isOnline == 1,
		BusinessDescription: businessDesc.String,
		BusinessCategory:    businessCat.String,
		BusinessEmail:       businessEmail.String,
		BusinessWebsite:     businessWeb.String,
		BusinessAddress:     businessAddr.String,
		VerifiedName:        verifiedName.String,
		CreatedAt:           time.Unix(createdAt, 0),
		UpdatedAt:           time.Unix(updatedAt, 0),
	}

	if pn.Valid {
		c.PN, _ = types.ParseJID(pn.String)
	}
	if statusSetAt.Valid {
		c.StatusSetAt = time.Unix(statusSetAt.Int64, 0)
	}
	if lastSeen.Valid {
		c.LastSeen = time.Unix(lastSeen.Int64, 0)
	}
	if verifiedLevel.Valid {
		c.VerifiedLevel = int(verifiedLevel.Int64)
	}

	return c, nil
}

func (s *ContactStore) scanContactRow(rows *sql.Rows) (*Contact, error) {
	var lidStr string
	var pn, pushName, businessName, serverName, fullName, firstName sql.NullString
	var profilePicID, profilePicURL sql.NullString
	var status sql.NullString
	var businessDesc, businessCat, businessEmail, businessWeb, businessAddr sql.NullString
	var verifiedName sql.NullString
	var verifiedLevel sql.NullInt64
	var statusSetAt, lastSeen sql.NullInt64
	var isBusiness, isOnline int
	var createdAt, updatedAt int64

	err := rows.Scan(
		&lidStr, &pn, &pushName, &businessName, &serverName, &fullName, &firstName,
		&profilePicID, &profilePicURL,
		&status, &statusSetAt, &lastSeen, &isOnline,
		&isBusiness, &businessDesc, &businessCat, &businessEmail, &businessWeb, &businessAddr,
		&verifiedName, &verifiedLevel, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	lid, _ := types.ParseJID(lidStr)

	c := &Contact{
		LID:                 lid,
		PushName:            pushName.String,
		BusinessName:        businessName.String,
		ServerName:          serverName.String,
		FullName:            fullName.String,
		FirstName:           firstName.String,
		ProfilePicID:        profilePicID.String,
		ProfilePicURL:       profilePicURL.String,
		Status:              status.String,
		IsBusiness:          isBusiness == 1,
		IsOnline:            isOnline == 1,
		BusinessDescription: businessDesc.String,
		BusinessCategory:    businessCat.String,
		BusinessEmail:       businessEmail.String,
		BusinessWebsite:     businessWeb.String,
		BusinessAddress:     businessAddr.String,
		VerifiedName:        verifiedName.String,
		CreatedAt:           time.Unix(createdAt, 0),
		UpdatedAt:           time.Unix(updatedAt, 0),
	}

	if pn.Valid {
		c.PN, _ = types.ParseJID(pn.String)
	}
	if statusSetAt.Valid {
		c.StatusSetAt = time.Unix(statusSetAt.Int64, 0)
	}
	if lastSeen.Valid {
		c.LastSeen = time.Unix(lastSeen.Int64, 0)
	}
	if verifiedLevel.Valid {
		c.VerifiedLevel = int(verifiedLevel.Int64)
	}

	return c, nil
}

// =============================================================================
// JID Mapping Methods (replaces orion_jid_mapping table)
// =============================================================================

// JIDMapping represents a LID to PN mapping.
type JIDMapping struct {
	LID types.JID
	PN  types.JID
}

// GetLIDForPN returns the LID for a phone number JID.
func (s *ContactStore) GetLIDForPN(pn types.JID) (types.JID, error) {
	var lidStr string
	err := s.store.QueryRow(`SELECT lid FROM orion_contacts WHERE pn = ?`, pn.String()).Scan(&lidStr)
	if err == sql.ErrNoRows {
		return types.JID{}, nil
	}
	if err != nil {
		return types.JID{}, err
	}
	lid, err := types.ParseJID(lidStr)
	return lid, err
}

// GetPNForLID returns the PN (phone number) for a LID.
func (s *ContactStore) GetPNForLID(lid types.JID) (types.JID, error) {
	var pnStr sql.NullString
	err := s.store.QueryRow(`SELECT pn FROM orion_contacts WHERE lid = ?`, lid.String()).Scan(&pnStr)
	if err == sql.ErrNoRows {
		return types.JID{}, nil
	}
	if err != nil {
		return types.JID{}, err
	}
	if !pnStr.Valid || pnStr.String == "" {
		return types.JID{}, nil
	}
	pn, err := types.ParseJID(pnStr.String)
	return pn, err
}

// UpdatePN updates the phone number for a contact.
func (s *ContactStore) UpdatePN(lid, pn types.JID) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_contacts (lid, pn, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET pn = excluded.pn, updated_at = excluded.updated_at
	`, lid.String(), pn.String(), now, now)
	return err
}

// PutJIDMappings stores multiple JID mappings.
func (s *ContactStore) PutJIDMappings(mappings []JIDMapping) error {
	if len(mappings) == 0 {
		return nil
	}
	tx, err := s.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	stmt, err := tx.Prepare(`
		INSERT INTO orion_contacts (lid, pn, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(lid) DO UPDATE SET pn = excluded.pn, updated_at = excluded.updated_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range mappings {
		if _, err := stmt.Exec(m.LID.String(), m.PN.String(), now, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Exists checks if a contact exists by matching either LID or PN.
func (s *ContactStore) Exists(jid types.JID) (bool, error) {
	jidStr := jid.String()
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_contacts WHERE lid = ? OR pn = ?`, jidStr, jidStr).Scan(&count)
	return count > 0, err
}

package send

import (
	"fmt"
	"orion-agent/internal/utils"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// ContactContent represents a single contact message.
type ContactContent struct {
	DisplayName string
	VCard       string
	ContextInfo *ContextInfo
}

// Contact creates a contact message from a vCard string.
func Contact(displayName, vcard string) *ContactContent {
	return &ContactContent{
		DisplayName: displayName,
		VCard:       vcard,
	}
}

// ContactFromVCardBuilder creates a contact message from a VCardBuilder.
func ContactFromVCardBuilder(builder *VCardBuilder) *ContactContent {
	return &ContactContent{
		DisplayName: builder.name,
		VCard:       builder.Build(),
	}
}

// WithContext adds context info.
func (c *ContactContent) WithContext(ctx *ContextInfo) *ContactContent {
	c.ContextInfo = ctx
	return c
}

// ToMessage implements Content.
func (c *ContactContent) ToMessage() (*waE2E.Message, error) {
	contact := &waE2E.ContactMessage{
		DisplayName: proto.String(c.DisplayName),
		Vcard:       proto.String(c.VCard),
	}

	if c.ContextInfo != nil {
		contact.ContextInfo = c.ContextInfo.Build()
	}

	return &waE2E.Message{ContactMessage: contact}, nil
}

// MediaType implements Content.
func (c *ContactContent) MediaType() utils.Type {
	return "" // Not a media message
}

// ContactsArrayContent represents a multi-contact message.
type ContactsArrayContent struct {
	DisplayName string
	Contacts    []*ContactContent
	ContextInfo *ContextInfo
}

// Contacts creates a multi-contact message.
func Contacts(displayName string, contacts ...*ContactContent) *ContactsArrayContent {
	return &ContactsArrayContent{
		DisplayName: displayName,
		Contacts:    contacts,
	}
}

// WithContext adds context info.
func (c *ContactsArrayContent) WithContext(ctx *ContextInfo) *ContactsArrayContent {
	c.ContextInfo = ctx
	return c
}

// ToMessage implements Content.
func (c *ContactsArrayContent) ToMessage() (*waE2E.Message, error) {
	contactMessages := make([]*waE2E.ContactMessage, len(c.Contacts))
	for i, contact := range c.Contacts {
		contactMessages[i] = &waE2E.ContactMessage{
			DisplayName: proto.String(contact.DisplayName),
			Vcard:       proto.String(contact.VCard),
		}
	}

	arr := &waE2E.ContactsArrayMessage{
		DisplayName: proto.String(c.DisplayName),
		Contacts:    contactMessages,
	}

	if c.ContextInfo != nil {
		arr.ContextInfo = c.ContextInfo.Build()
	}

	return &waE2E.Message{ContactsArrayMessage: arr}, nil
}

// MediaType implements Content.
func (c *ContactsArrayContent) MediaType() utils.Type {
	return "" // Not a media message
}

// VCardBuilder helps build vCard format strings.
type VCardBuilder struct {
	name          string
	formattedName string
	phones        []vCardPhone
	emails        []string
	org           string
	title         string
	note          string
}

type vCardPhone struct {
	number string
	types  []string // HOME, WORK, CELL, etc.
}

// NewVCard creates a new vCard builder.
func NewVCard(name string) *VCardBuilder {
	return &VCardBuilder{
		name:          name,
		formattedName: name,
	}
}

// FormattedName sets the formatted name (FN field).
func (v *VCardBuilder) FormattedName(fn string) *VCardBuilder {
	v.formattedName = fn
	return v
}

// Phone adds a phone number.
func (v *VCardBuilder) Phone(number string, types ...string) *VCardBuilder {
	if len(types) == 0 {
		types = []string{"CELL"}
	}
	v.phones = append(v.phones, vCardPhone{number: number, types: types})
	return v
}

// CellPhone adds a cell phone number.
func (v *VCardBuilder) CellPhone(number string) *VCardBuilder {
	return v.Phone(number, "CELL")
}

// WorkPhone adds a work phone number.
func (v *VCardBuilder) WorkPhone(number string) *VCardBuilder {
	return v.Phone(number, "WORK")
}

// HomePhone adds a home phone number.
func (v *VCardBuilder) HomePhone(number string) *VCardBuilder {
	return v.Phone(number, "HOME")
}

// Email adds an email address.
func (v *VCardBuilder) Email(email string) *VCardBuilder {
	v.emails = append(v.emails, email)
	return v
}

// Organization sets the organization.
func (v *VCardBuilder) Organization(org string) *VCardBuilder {
	v.org = org
	return v
}

// Title sets the job title.
func (v *VCardBuilder) Title(title string) *VCardBuilder {
	v.title = title
	return v
}

// Note adds a note.
func (v *VCardBuilder) Note(note string) *VCardBuilder {
	v.note = note
	return v
}

// Build generates the vCard string.
func (v *VCardBuilder) Build() string {
	var lines []string
	lines = append(lines, "BEGIN:VCARD")
	lines = append(lines, "VERSION:3.0")
	lines = append(lines, fmt.Sprintf("N:%s", v.name))
	lines = append(lines, fmt.Sprintf("FN:%s", v.formattedName))

	for _, phone := range v.phones {
		typeStr := strings.Join(phone.types, ",")
		lines = append(lines, fmt.Sprintf("TEL;TYPE=%s:%s", typeStr, phone.number))
	}

	for _, email := range v.emails {
		lines = append(lines, fmt.Sprintf("EMAIL:%s", email))
	}

	if v.org != "" {
		lines = append(lines, fmt.Sprintf("ORG:%s", v.org))
	}
	if v.title != "" {
		lines = append(lines, fmt.Sprintf("TITLE:%s", v.title))
	}
	if v.note != "" {
		lines = append(lines, fmt.Sprintf("NOTE:%s", v.note))
	}

	lines = append(lines, "END:VCARD")
	return strings.Join(lines, "\n")
}

// ToContact creates a ContactContent from this builder.
func (v *VCardBuilder) ToContact() *ContactContent {
	return ContactFromVCardBuilder(v)
}

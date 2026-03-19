package vocab

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/microcosm-cc/bluemonday"
)

// ErrLocalAuthorRequired is returned by LocalStatusToNote when the author
// is a remote account (Account.Domain != nil). Use LocalStatusToNote only
// for statuses authored on this instance; for remote statuses use the Note
// as received from federation.
var ErrLocalAuthorRequired = errors.New("vocab: LocalStatusToNote requires local author (Account.Domain == nil)")

// Note represents an AP Note (status/post). The core content type in the
// Mastodon federation protocol.
//
// ContentMap is a Mastodon extension that maps language codes to localised
// content.
type Note struct {
	Context      any               `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         ObjectType        `json:"type"` // "Note"
	AttributedTo string            `json:"attributedTo"`
	Content      string            `json:"content"` // rendered HTML
	ContentMap   map[string]string `json:"contentMap,omitempty"`
	Source       *NoteSource       `json:"source,omitempty"`
	To           []string          `json:"to"`
	Cc           []string          `json:"cc,omitempty"`
	InReplyTo    *string           `json:"inReplyTo"` // null or parent Note IRI
	Published    string            `json:"published"` // ISO 8601
	Updated      string            `json:"updated,omitempty"`
	URL          string            `json:"url"`
	Sensitive    bool              `json:"sensitive"`
	Summary      *string           `json:"summary"` // content warning; null if none
	Tag          []Tag             `json:"tag,omitempty"`
	Attachment   []Attachment      `json:"attachment,omitempty"`
	Replies      json.RawMessage   `json:"replies,omitempty"`
}

// NoteSource preserves the original plain-text or Markdown source.
// Mastodon includes this for editable posts.
type NoteSource struct {
	Content   string `json:"content"`
	MediaType string `json:"mediaType"` // "text/plain"
}

// LocalStatusToNoteInput bundles all data needed to build an outbound AP Note
// for a local status (author and status on this instance).
type LocalStatusToNoteInput struct {
	Status       *domain.Status
	Author       *domain.Account
	InstanceBase string
	Mentions     []*domain.Account
	Tags         []domain.Hashtag
	Media        []domain.MediaAttachment
	ParentAPID   string // APID of the parent status (for InReplyTo); empty if not a reply
}

// LocalStatusToNote builds an ActivityPub Note from a domain status and its
// author account. Author must be a local account (Account.Domain == nil);
// returns ErrLocalAuthorRequired for remote authors. For remote statuses,
// use the Note as received from federation instead of this function.
func LocalStatusToNote(in LocalStatusToNoteInput) (*Note, error) {
	if in.Author == nil || in.Status == nil {
		return nil, errors.New("vocab: LocalStatusToNote requires non-nil Author and Status")
	}
	if in.Author.Domain != nil && *in.Author.Domain != "" {
		return nil, ErrLocalAuthorRequired
	}
	s := in.Status
	account := in.Author
	instanceBase := in.InstanceBase

	content := ""
	if s.Content != nil {
		content = *s.Content
	} else if s.Text != nil {
		content = bluemonday.UGCPolicy().Sanitize(*s.Text)
	}
	noteID := StatusNoteID(s, instanceBase)
	actorID := AccountActorID(account, instanceBase)
	published := s.CreatedAt.Format(time.RFC3339)

	var inReplyTo *string
	if s.InReplyToID != nil && *s.InReplyToID != "" {
		if in.ParentAPID != "" {
			inReplyTo = &in.ParentAPID
		} else {
			iri := fmt.Sprintf("%s/statuses/%s", instanceBase, *s.InReplyToID)
			inReplyTo = &iri
		}
	}

	updated := ""
	if s.EditedAt != nil {
		updated = s.EditedAt.Format(time.RFC3339)
	}

	followersURL := account.FollowersURL
	if followersURL == "" {
		followersURL = actorID + "/followers"
	}
	mentionIRIs := mentionActorIRIs(in.Mentions, instanceBase)
	to, cc := noteAddressing(s.Visibility, followersURL, mentionIRIs)
	tags := buildTags(in.Tags, in.Mentions, instanceBase)
	attachments := buildAttachments(in.Media)

	var contentMap map[string]string
	if s.Language != nil && *s.Language != "" {
		contentMap = map[string]string{*s.Language: content}
	}

	return &Note{
		Context:      DefaultContext,
		ID:           noteID,
		Type:         ObjectTypeNote,
		AttributedTo: actorID,
		Content:      content,
		ContentMap:   contentMap,
		To:           to,
		Cc:           cc,
		InReplyTo:    inReplyTo,
		Published:    published,
		Updated:      updated,
		URL:          fmt.Sprintf("%s/@%s/%s", instanceBase, account.Username, s.ID),
		Sensitive:    s.Sensitive,
		Summary:      s.ContentWarning,
		Tag:          tags,
		Attachment:   attachments,
	}, nil
}

// noteAddressing returns To and Cc slices per Mastodon/AP convention.
func noteAddressing(visibility, followersURL string, mentionIRIs []string) (to, cc []string) {
	switch visibility {
	case domain.VisibilityPublic:
		to = []string{PublicAddress}
		cc = append([]string{followersURL}, mentionIRIs...)
	case domain.VisibilityUnlisted:
		to = []string{followersURL}
		cc = append([]string{PublicAddress}, mentionIRIs...)
	case domain.VisibilityPrivate:
		to = []string{followersURL}
		cc = mentionIRIs
	case domain.VisibilityDirect:
		to = mentionIRIs
		cc = nil
	default:
		to = []string{PublicAddress}
		cc = append([]string{followersURL}, mentionIRIs...)
	}
	if to == nil {
		to = []string{}
	}
	return to, cc
}

func mentionActorIRIs(mentions []*domain.Account, instanceBase string) []string {
	out := make([]string, 0, len(mentions))
	for _, m := range mentions {
		iri := AccountActorID(m, instanceBase)
		if iri != "" {
			out = append(out, iri)
		}
	}
	return out
}

func buildTags(hashtags []domain.Hashtag, mentions []*domain.Account, instanceBase string) []Tag {
	tags := make([]Tag, 0, len(hashtags)+len(mentions))
	for _, h := range hashtags {
		tags = append(tags, Tag{
			Type: ObjectTypeHashtag,
			Href: fmt.Sprintf("%s/tags/%s", instanceBase, h.Name),
			Name: "#" + h.Name,
		})
	}
	for _, m := range mentions {
		name := "@" + m.Username
		if m.Domain != nil && *m.Domain != "" {
			name += "@" + *m.Domain
		}
		tags = append(tags, Tag{
			Type: ObjectTypeMention,
			Href: AccountActorID(m, instanceBase),
			Name: name,
		})
	}
	return tags
}

func buildAttachments(media []domain.MediaAttachment) []Attachment {
	if len(media) == 0 {
		return nil
	}
	out := make([]Attachment, len(media))
	for i, m := range media {
		desc := ""
		if m.Description != nil {
			desc = *m.Description
		}
		blurhash := ""
		if m.Blurhash != nil {
			blurhash = *m.Blurhash
		}
		mediaType := ""
		if m.ContentType != nil {
			mediaType = *m.ContentType
		}
		out[i] = Attachment{
			Type:      mediaTypeToObjectType(m.Type),
			MediaType: mediaType,
			URL:       m.URL,
			Name:      desc,
			Blurhash:  blurhash,
		}
	}
	return out
}

// mediaTypeToObjectType maps a domain media category to the appropriate
// ActivityStreams object type per AS2 Vocabulary §3.3.
func mediaTypeToObjectType(mediaCategory string) ObjectType {
	switch mediaCategory {
	case domain.MediaTypeImage, domain.MediaTypeGifv:
		return ObjectTypeImage
	case domain.MediaTypeVideo:
		return ObjectTypeVideo
	case domain.MediaTypeAudio:
		return ObjectTypeAudio
	default:
		return ObjectTypeDocument
	}
}

func isPublicAddress(addr string) bool {
	switch addr {
	case PublicAddress, "as:Public", "Public":
		return true
	default:
		return false
	}
}

// NoteVisibility derives domain visibility from a Note's To/Cc addressing.
// followersURL is the author's AP followers collection IRI.
// Callers must sanitize note content before use; this function only inspects addressing fields.
func NoteVisibility(note *Note, followersURL string) string {
	if slices.ContainsFunc(note.To, isPublicAddress) {
		return domain.VisibilityPublic
	}
	if slices.ContainsFunc(note.Cc, isPublicAddress) {
		return domain.VisibilityUnlisted
	}
	if followersURL != "" && slices.Contains(note.To, followersURL) {
		return domain.VisibilityPrivate
	}
	return domain.VisibilityDirect
}

// NoteLanguage returns the first language code from ContentMap, or nil.
func NoteLanguage(note *Note) *string {
	if len(note.ContentMap) == 0 {
		return nil
	}
	for k := range note.ContentMap {
		return &k
	}
	return nil
}

// NoteStatusFields holds the pure field-mapping result of an inbound Note.
// Content and ContentWarning are excluded — callers must sanitize those at the
// trust boundary before building service.CreateRemoteStatusInput.
// InReplyToID and MediaIDs require I/O and are filled in by the caller.
type NoteStatusFields struct {
	URI         string
	APID        string
	Sensitive   bool
	Language    *string
	PublishedAt *time.Time
}

// NoteToStatusFields extracts the pure (non-sanitized, non-I/O) fields from
// an inbound Note. Callers supply visibility (from NoteVisibility).
func NoteToStatusFields(note *Note) NoteStatusFields {
	fields := NoteStatusFields{
		URI:       note.ID,
		APID:      note.ID,
		Sensitive: note.Sensitive,
		Language:  NoteLanguage(note),
	}
	if note.Published != "" {
		if t, err := time.Parse(time.RFC3339, note.Published); err == nil {
			fields.PublishedAt = &t
		}
	}
	return fields
}

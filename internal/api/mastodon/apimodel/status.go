package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// ToStatus converts a domain status to the Mastodon API status shape.
func ToStatus(s *domain.Status, author Account, mentions []Mention, tags []Tag, media []MediaAttachment, instanceDomain string) Status {
	spoiler := ""
	if s.ContentWarning != nil {
		spoiler = *s.ContentWarning
	}
	content := ""
	if s.Content != nil {
		content = *s.Content
	}
	var urlStr *string
	if s.Local {
		u := "https://" + instanceDomain + "/@" + author.Username + "/" + s.ID
		urlStr = &u
	}
	st := Status{
		ID:                  s.ID,
		CreatedAt:           s.CreatedAt.UTC().Format(time.RFC3339),
		InReplyToID:         s.InReplyToID,
		InReplyToAccountID:  s.InReplyToAccountID,
		Sensitive:           s.Sensitive,
		SpoilerText:         spoiler,
		Visibility:          s.Visibility,
		Language:            s.Language,
		URI:                 s.URI,
		URL:                 urlStr,
		RepliesCount:        s.RepliesCount,
		ReblogsCount:        s.ReblogsCount,
		FavouritesCount:     s.FavouritesCount,
		QuotesCount:         s.QuotesCount,
		QuoteApprovalPolicy: s.QuoteApprovalPolicy,
		Content:             content,
		Account:             author,
		MediaAttachments:    media,
		Mentions:            mentions,
		Tags:                tags,
		Emojis:              []any{},
		Card:                nil,
		Poll:                nil,
		Favourited:          false,
		Reblogged:           false,
		Muted:               false,
		Bookmarked:          false,
		Pinned:              false,
	}
	return st
}

// MentionFromAccount builds a Mention from a domain account and instance domain.
func MentionFromAccount(a *domain.Account, instanceDomain string) Mention {
	acct := a.Username
	if a.Domain != nil && *a.Domain != "" {
		acct = a.Username + "@" + *a.Domain
	}
	urlStr := a.APID
	if a.Domain == nil || *a.Domain == "" {
		urlStr = "https://" + instanceDomain + "/@" + a.Username
	}
	return Mention{
		ID:       a.ID,
		Username: a.Username,
		Acct:     acct,
		URL:      urlStr,
	}
}

// TagFromName builds a Tag for a hashtag name.
func TagFromName(name, instanceDomain string) Tag {
	return Tag{
		Name: name,
		URL:  "https://" + instanceDomain + "/tags/" + name,
	}
}

// FollowedTagFromDomain builds a Tag for the followed_tags list (id, name, url, following: true).
func FollowedTagFromDomain(h domain.Hashtag, instanceDomain string) Tag {
	return Tag{
		ID:        h.ID,
		Name:      h.Name,
		URL:       "https://" + instanceDomain + "/tags/" + h.Name,
		Following: true,
	}
}

// FeaturedTagFromDomain builds a FeaturedTag API response. Profile URL is base/@{username}/tagged/{name}.
func FeaturedTagFromDomain(ft domain.FeaturedTag, profileTagURL string) FeaturedTag {
	out := FeaturedTag{
		ID:            ft.ID,
		Name:          ft.Name,
		URL:           profileTagURL,
		StatusesCount: ft.StatusesCount,
	}
	if ft.LastStatusAt != nil {
		s := ft.LastStatusAt.Format("2006-01-02")
		out.LastStatusAt = &s
	}
	return out
}

// StatusEditFromDomain converts a domain status edit to the API model with the given author account.
func StatusEditFromDomain(e domain.StatusEdit, author Account) StatusEdit {
	content := ""
	if e.Content != nil {
		content = *e.Content
	}
	spoiler := ""
	if e.ContentWarning != nil {
		spoiler = *e.ContentWarning
	}
	return StatusEdit{
		Content:          content,
		SpoilerText:      spoiler,
		Sensitive:        e.Sensitive,
		CreatedAt:        e.CreatedAt.UTC().Format(time.RFC3339),
		Account:          author,
		MediaAttachments: nil,
		Emojis:           []any{},
	}
}

// MediaFromDomain converts a domain media attachment to the API model shape.
func MediaFromDomain(m *domain.MediaAttachment) MediaAttachment {
	preview := ""
	if m.PreviewURL != nil {
		preview = *m.PreviewURL
	}
	remote := ""
	if m.RemoteURL != nil {
		remote = *m.RemoteURL
	}
	desc := ""
	if m.Description != nil {
		desc = *m.Description
	}
	blur := ""
	if m.Blurhash != nil {
		blur = *m.Blurhash
	}
	return MediaAttachment{
		ID:          m.ID,
		Type:        m.Type,
		URL:         m.URL,
		PreviewURL:  preview,
		RemoteURL:   remote,
		Description: desc,
		Blurhash:    blur,
	}
}

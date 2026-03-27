package apimodel

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/microcosm-cc/bluemonday"
)

// CardFromDomain converts a domain card to the API model. Returns nil if the card is nil or not yet fetched.
func CardFromDomain(c *domain.Card) *Card {
	if c == nil || c.ProcessingState != domain.CardStateFetched {
		return nil
	}
	var img *string
	if c.ImageURL != "" {
		img = &c.ImageURL
	}
	return &Card{
		URL:          c.URL,
		Title:        c.Title,
		Description:  c.Description,
		Type:         c.Type,
		ProviderName: c.ProviderName,
		ProviderURL:  c.ProviderURL,
		HTML:         "",
		Width:        c.Width,
		Height:       c.Height,
		Image:        img,
		Blurhash:     nil,
		PublishedAt:  nil,
	}
}

// ToStatus converts a domain status to the Mastodon API status shape.
func ToStatus(s *domain.Status, author Account, mentions []Mention, tags []Tag, media []MediaAttachment, card *domain.Card, instanceDomain string) Status {
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
		Card:                CardFromDomain(card),
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
	var urlStr string
	switch {
	case a.Domain == nil || *a.Domain == "":
		urlStr = "https://" + instanceDomain + "/@" + a.Username
	case a.ProfileURL != "":
		urlStr = a.ProfileURL
	default:
		urlStr = a.APID
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
		Name:    name,
		URL:     "https://" + instanceDomain + "/tags/" + name,
		History: []TagHistory{},
	}
}

// TagFromDomain builds a Tag from a domain.Hashtag, optionally with following state.
// Pass nil for following to omit the field (unauthenticated requests).
func TagFromDomain(h *domain.Hashtag, instanceDomain string, following *bool) Tag {
	return Tag{
		Name:      h.Name,
		URL:       "https://" + instanceDomain + "/tags/" + h.Name,
		Following: following,
		History:   []TagHistory{},
	}
}

// FollowedTagFromDomain builds a Tag for the followed_tags list (id, name, url, following: true).
func FollowedTagFromDomain(h domain.Hashtag, instanceDomain string) Tag {
	t := true
	return Tag{
		ID:        h.ID,
		Name:      h.Name,
		URL:       "https://" + instanceDomain + "/tags/" + h.Name,
		Following: &t,
		History:   []TagHistory{},
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

// TrendingTagFromDomain converts a domain.TrendingTag to the Mastodon API Tag shape with history.
// followedNames is the set of tag names the authenticated user follows; pass nil for unauthenticated
// requests to omit the following field entirely.
func TrendingTagFromDomain(t domain.TrendingTag, instanceDomain string, followedNames map[string]bool) *Tag {
	history := make([]TagHistory, len(t.History))
	for i, h := range t.History {
		history[i] = TagHistory{
			Day:      strconv.FormatInt(h.Day.Unix(), 10),
			Uses:     strconv.FormatInt(h.Uses, 10),
			Accounts: strconv.FormatInt(h.Accounts, 10),
		}
	}
	tag := &Tag{
		Name:    t.Hashtag.Name,
		URL:     "https://" + instanceDomain + "/tags/" + t.Hashtag.Name,
		History: history,
	}
	if followedNames != nil {
		f := followedNames[t.Hashtag.Name]
		tag.Following = &f
	}
	return tag
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

// CreateStatusRequest is the request body for POST /api/v1/statuses.
type CreateStatusRequest struct {
	Status              string   `json:"status"`
	Visibility          string   `json:"visibility"`
	SpoilerText         string   `json:"spoiler_text"`
	Sensitive           bool     `json:"sensitive"`
	Language            string   `json:"language"`
	InReplyToID         string   `json:"in_reply_to_id"`
	MediaIDs            []string `json:"media_ids"`
	QuotedStatusID      string   `json:"quoted_status_id"`
	QuoteApprovalPolicy string   `json:"quote_approval_policy"`
	ScheduledAt         string   `json:"scheduled_at"` // if non-empty, return 422 (Phase 1)
	Poll                *struct {
		Options   []string `json:"options"`
		ExpiresIn int      `json:"expires_in"`
		Multiple  bool     `json:"multiple"`
	} `json:"poll"`
}

func (r *CreateStatusRequest) Validate() error {
	if r.QuotedStatusID != "" && (len(r.MediaIDs) > 0 || (r.Poll != nil && len(r.Poll.Options) > 0)) {
		return fmt.Errorf("%w: cannot attach media or poll to a quote post", api.ErrUnprocessable)
	}

	return nil
}

func (r *CreateStatusRequest) Sanitize() {
	r.Status = bluemonday.UGCPolicy().Sanitize(r.Status)
	r.SpoilerText = bluemonday.UGCPolicy().Sanitize(r.SpoilerText)
	r.Language = bluemonday.StrictPolicy().Sanitize(r.Language)

	if r.Poll != nil {
		for i, option := range r.Poll.Options {
			r.Poll.Options[i] = bluemonday.UGCPolicy().Sanitize(option)
		}
	}
}

// UpdateStatusRequest is the request body for PUT /api/v1/statuses/:id.
type UpdateStatusRequest struct {
	Status      string `json:"status"`
	SpoilerText string `json:"spoiler_text"`
	Sensitive   bool   `json:"sensitive"`
}

func (r *UpdateStatusRequest) Validate() error {
	if strings.TrimSpace(r.Status) == "" {
		return fmt.Errorf("validate status: %w", api.NewMissingRequiredFieldError("status"))
	}
	return nil
}

func (r *UpdateStatusRequest) Sanitize() {
	r.Status = bluemonday.UGCPolicy().Sanitize(r.Status)
	r.SpoilerText = bluemonday.UGCPolicy().Sanitize(r.SpoilerText)
}

// PUTInteractionPolicyRequest is the request body for PUT /api/v1/statuses/:id/interaction_policy.
type PUTInteractionPolicyRequest struct {
	QuoteApprovalPolicy string `json:"quote_approval_policy"`
}

func (r *PUTInteractionPolicyRequest) Validate() error {
	policy := strings.TrimSpace(r.QuoteApprovalPolicy)
	if policy == "" {
		return fmt.Errorf("quote_approval_policy: %w", api.NewMissingRequiredFieldError("quote_approval_policy"))
	}
	r.QuoteApprovalPolicy = policy
	return nil
}

// ParseCreateStatusRequest parses JSON or form body into CreateStatusRequest.
// Returns an error with a client-safe message on validation or parse failure.
func ParseCreateStatusRequest(r *http.Request) (CreateStatusRequest, error) {
	var req CreateStatusRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := api.DecodeJSONBody(r, &req); err != nil {
			return CreateStatusRequest{}, fmt.Errorf("decode body: %w", err)
		}
	} else {
		if err := r.ParseForm(); err != nil {
			// nolint:wrapcheck
			return CreateStatusRequest{}, api.NewInvalidRequestBodyError()
		}
		req.Status = r.FormValue("status")
		req.Visibility = r.FormValue("visibility")
		req.SpoilerText = r.FormValue("spoiler_text")
		req.Sensitive = api.FormValueIsTruthy(r.Form, "sensitive")
		req.Language = r.FormValue("language")
		req.InReplyToID = r.FormValue("in_reply_to_id")
		req.QuotedStatusID = r.FormValue("quoted_status_id")
		req.QuoteApprovalPolicy = r.FormValue("quote_approval_policy")
		req.ScheduledAt = r.FormValue("scheduled_at")
		if ids := r.Form["media_ids[]"]; len(ids) > 0 {
			req.MediaIDs = ids
		} else if ids := r.Form["media_ids"]; len(ids) > 0 {
			req.MediaIDs = ids
		}
		if opts := r.Form["poll[options][]"]; len(opts) > 0 {
			if req.Poll == nil {
				req.Poll = &struct {
					Options   []string `json:"options"`
					ExpiresIn int      `json:"expires_in"`
					Multiple  bool     `json:"multiple"`
				}{}
			}
			req.Poll.Options = opts
			if e := r.FormValue("poll[expires_in]"); e != "" {
				if n, err := strconv.Atoi(e); err == nil {
					req.Poll.ExpiresIn = n
				}
			}
			req.Poll.Multiple = api.FormValueIsTruthy(r.Form, "poll[multiple]")
		}
	}
	req.Status = strings.TrimSpace(req.Status)

	// Require at least one of the following fields: status, media_ids, poll.
	if req.Status == "" && len(req.MediaIDs) == 0 && req.Poll == nil {
		return CreateStatusRequest{}, api.NewMissingRequiredFieldsError([]string{"status", "media_ids", "poll"})
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		// nolint:wrapcheck
		return CreateStatusRequest{}, fmt.Errorf("validate: %w", err)
	}
	return req, nil
}

// StatusFromEnriched converts a service.EnrichedStatus to the Mastodon API Status model.
func StatusFromEnriched(result service.EnrichedStatus, instanceDomain string) Status {
	authorAcc := ToAccount(result.Author, instanceDomain)
	mentionsResp := make([]Mention, 0, len(result.Mentions))
	for _, a := range result.Mentions {
		mentionsResp = append(mentionsResp, MentionFromAccount(a, instanceDomain))
	}
	tagsResp := make([]Tag, 0, len(result.Tags))
	for _, t := range result.Tags {
		tagsResp = append(tagsResp, TagFromName(t.Name, instanceDomain))
	}
	mediaResp := make([]MediaAttachment, 0, len(result.Media))
	for i := range result.Media {
		mediaResp = append(mediaResp, MediaFromDomain(&result.Media[i]))
	}
	out := ToStatus(result.Status, authorAcc, mentionsResp, tagsResp, mediaResp, result.Card, instanceDomain)
	if result.Poll != nil {
		p := PollFromEnriched(result.Poll)
		out.Poll = &p
	}
	out.Favourited = result.Favourited
	out.Reblogged = result.Reblogged
	out.Bookmarked = result.Bookmarked
	out.Pinned = result.Pinned
	out.Muted = result.Muted
	if result.ReblogOf != nil {
		reblogAPI := StatusFromEnriched(*result.ReblogOf, instanceDomain)
		out.Reblog = &reblogAPI
	}
	return out
}

// StatusFromParts converts individual domain components into a Mastodon API Status.
// Use this when the parts are available separately (e.g. from event payloads)
// rather than bundled in an EnrichedStatus.
func StatusFromParts(status *domain.Status, author *domain.Account, mentions []*domain.Account, tags []domain.Hashtag, media []domain.MediaAttachment, instanceDomain string) Status {
	acc := ToAccount(author, instanceDomain)
	apiMentions := make([]Mention, 0, len(mentions))
	for _, m := range mentions {
		if m != nil {
			apiMentions = append(apiMentions, MentionFromAccount(m, instanceDomain))
		}
	}
	apiTags := make([]Tag, 0, len(tags))
	for _, t := range tags {
		apiTags = append(apiTags, TagFromName(t.Name, instanceDomain))
	}
	apiMedia := make([]MediaAttachment, 0, len(media))
	for i := range media {
		apiMedia = append(apiMedia, MediaFromDomain(&media[i]))
	}
	return ToStatus(status, acc, apiMentions, apiTags, apiMedia, nil, instanceDomain)
}

// StatusesFromEnriched converts a slice of EnrichedStatus to API Status models.
func StatusesFromEnriched(enriched []service.EnrichedStatus, instanceDomain string) []Status {
	out := make([]Status, 0, len(enriched))
	for i := range enriched {
		out = append(out, StatusFromEnriched(enriched[i], instanceDomain))
	}
	return out
}

// PollFromEnriched converts a service.EnrichedPoll to the Mastodon API Poll model.
func PollFromEnriched(p *service.EnrichedPoll) Poll {
	var expiresAt *string
	if p.Poll.ExpiresAt != nil {
		s := p.Poll.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}
	expired := p.Poll.ExpiresAt != nil && p.Poll.ExpiresAt.Before(time.Now())
	options := make([]PollOption, 0, len(p.Options))
	var votesCount int
	for _, o := range p.Options {
		votesCount += o.VotesCount
		options = append(options, PollOption{Title: o.Title, VotesCount: o.VotesCount})
	}
	return Poll{
		ID:          p.Poll.ID,
		ExpiresAt:   expiresAt,
		Expired:     expired,
		Multiple:    p.Poll.Multiple,
		VotesCount:  votesCount,
		VotersCount: nil,
		Voted:       p.Voted,
		OwnVotes:    p.OwnVotes,
		Options:     options,
		Emojis:      []any{},
	}
}

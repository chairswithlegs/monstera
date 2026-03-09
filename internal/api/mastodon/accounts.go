package mastodon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// AccountsHandler handles account-related Mastodon API endpoints.
type AccountsHandler struct {
	accounts       service.AccountService
	follows        service.FollowService
	timeline       service.TimelineService
	instanceDomain string
}

// NewAccountsHandler returns a new AccountsHandler. follows may be nil to disable follow endpoints; timeline is required for GET account statuses.
func NewAccountsHandler(accounts service.AccountService, follows service.FollowService, timeline service.TimelineService, instanceDomain string) *AccountsHandler {
	return &AccountsHandler{accounts: accounts, follows: follows, timeline: timeline, instanceDomain: instanceDomain}
}

// GETVerifyCredentials handles GET /api/v1/accounts/verify_credentials.
func (h *AccountsHandler) GETVerifyCredentials(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	acc, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := apimodel.ToAccountWithSource(acc, user, h.instanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// GETAccounts handles GET /api/v1/accounts/:id. Auth optional. id is the account's internal ID (ULID).
func (h *AccountsHandler) GETAccounts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := api.ValidateRequiredField(id, "id"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	acc, err := h.accounts.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	if acc.Suspended {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccount(acc, h.instanceDomain))
}

// GETAccountsLookup handles GET /api/v1/accounts/lookup?acct=username or username@domain.
// Returns the account only if this instance already has it (local by username, remote by username@domain).
// No remote resolution: it does not fetch from federation. For resolving unknown remote users, use GET /api/v2/search?q=user@domain&resolve=true.
// Mastodon-compatible: clients use this to get the account (and ID) when they have the handle and the account is already known to the instance.
func (h *AccountsHandler) GETAccountsLookup(w http.ResponseWriter, r *http.Request) {
	acct := strings.TrimSpace(r.URL.Query().Get("acct"))
	if acct == "" {
		api.HandleError(w, r, api.NewUnprocessableError("acct parameter is required"))
		return
	}
	var username string
	var accountDomain *string
	if idx := strings.Index(acct, "@"); idx >= 0 {
		username = acct[:idx]
		d := strings.TrimSpace(acct[idx+1:])
		if d != "" {
			accountDomain = &d
		}
	} else {
		username = acct
	}
	if username == "" {
		api.HandleError(w, r, api.NewUnprocessableError("acct must contain a username"))
		return
	}
	acc, err := h.accounts.GetByUsername(r.Context(), username, accountDomain)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	if acc.Suspended {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccount(acc, h.instanceDomain))
}

// GETDirectory handles GET /api/v1/directory. Auth optional. Returns discoverable/local accounts.
func (h *AccountsHandler) GETDirectory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := DefaultListLimit
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > MaxListLimit {
				limit = MaxListLimit
			}
		}
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n > 0 {
			offset = n
		}
	}
	order := q.Get("order")
	if order != "active" && order != "new" {
		order = "active"
	}
	localOnly := q.Get("local") == "true"

	accounts, err := h.accounts.ListDirectory(r.Context(), order, localOnly, offset, limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, apimodel.ToAccount(&accounts[i], h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTFollow handles POST /api/v1/accounts/:id/follow. Auth required.
func (h *AccountsHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if err := api.ValidateRequiredField(targetID, "target_id"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Follow(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnfollow handles POST /api/v1/accounts/:id/unfollow. Auth required.
func (h *AccountsHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if err := api.ValidateRequiredField(targetID, "target_id"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unfollow(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// GETRelationships handles GET /api/v1/accounts/relationships?id[]=... Returns []Relationship for each requested id.
func (h *AccountsHandler) GETRelationships(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	ids := r.URL.Query()["id[]"]
	if len(ids) == 0 {
		api.WriteJSON(w, http.StatusOK, []apimodel.Relationship{})
		return
	}
	out := make([]apimodel.Relationship, 0, len(ids))
	for _, targetID := range ids {
		if targetID == "" {
			continue
		}
		target, err := h.accounts.GetByID(r.Context(), targetID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				api.HandleError(w, r, api.ErrNotFound)
				return
			}
			api.HandleError(w, r, err)
			return
		}
		rel, err := h.accounts.GetRelationship(r.Context(), account.ID, target.ID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		out = append(out, apimodel.ToRelationship(rel))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// PATCHUpdateCredentials handles PATCH /api/v1/accounts/update_credentials.
func (h *AccountsHandler) PATCHUpdateCredentials(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil && r.Form == nil {
		_ = r.ParseForm()
	}
	form := r.Form
	if form == nil {
		form = make(map[string][]string)
	}
	displayName := formValue(form, "display_name")
	note := formValue(form, "note")
	locked := formValue(form, "locked") == resolveQueryTrue || formValue(form, "locked") == "1"
	bot := formValue(form, "bot") == resolveQueryTrue || formValue(form, "bot") == "1"
	avatarID := formValue(form, "avatar")
	if avatarID == "" {
		avatarID = formValue(form, "avatar_media_id")
	}
	headerID := formValue(form, "header")
	if headerID == "" {
		headerID = formValue(form, "header_media_id")
	}
	var avatarMediaID, headerMediaID *string
	if avatarID != "" {
		avatarMediaID = &avatarID
	}
	if headerID != "" {
		headerMediaID = &headerID
	}
	var displayNamePtr, notePtr *string
	if displayName != "" {
		displayNamePtr = &displayName
	}
	if note != "" {
		notePtr = &note
	}
	fields := parseFieldsAttributes(form)
	quotePolicy := formValue(form, "source[quote_policy]")
	if quotePolicy == "" {
		quotePolicy = formValue(form, "quote_policy")
	}
	var defaultQuotePolicy *string
	if quotePolicy != "" {
		defaultQuotePolicy = &quotePolicy
	}
	acc, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if len(fields) == 0 {
		fields = acc.Fields
	}
	updated, updatedUser, err := h.accounts.UpdateCredentials(r.Context(), service.UpdateCredentialsInput{
		AccountID:          account.ID,
		DisplayName:        displayNamePtr,
		Note:               notePtr,
		AvatarMediaID:      avatarMediaID,
		HeaderMediaID:      headerMediaID,
		Locked:             locked,
		Bot:                bot,
		DefaultQuotePolicy: defaultQuotePolicy,
		Fields:             fields,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if updatedUser == nil {
		updatedUser = user
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccountWithSource(updated, updatedUser, h.instanceDomain))
}

func formValue(form map[string][]string, key string) string {
	if v := form[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func parseFieldsAttributes(form map[string][]string) json.RawMessage {
	type field struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	var fields []field
	for i := 0; i < 4; i++ {
		key := strconv.Itoa(i)
		name := ""
		value := ""
		if n := form["fields_attributes["+key+"][name]"]; len(n) > 0 {
			name = n[0]
		}
		if v := form["fields_attributes["+key+"][value]"]; len(v) > 0 {
			value = v[0]
		}
		if name != "" || value != "" {
			fields = append(fields, field{Name: name, Value: value})
		}
	}
	if len(fields) == 0 {
		return nil
	}
	b, _ := json.Marshal(fields)
	return b
}

// GETAccountStatuses handles GET /api/v1/accounts/:id/statuses.
func (h *AccountsHandler) GETAccountStatuses(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	if h.timeline == nil {
		api.HandleError(w, r, api.NewUnprocessableError("timeline not configured"))
		return
	}
	params := PageParamsFromRequest(r)
	viewerID := &account.ID
	enriched, err := h.timeline.AccountStatusesEnriched(r.Context(), target.ID, viewerID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		e := &enriched[i]
		authorAcc := apimodel.ToAccount(e.Author, h.instanceDomain)
		mentionsResp := make([]apimodel.Mention, 0, len(e.Mentions))
		for _, a := range e.Mentions {
			mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, h.instanceDomain))
		}
		tagsResp := make([]apimodel.Tag, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, h.instanceDomain))
		}
		mediaResp := make([]apimodel.MediaAttachment, 0, len(e.Media))
		for j := range e.Media {
			mediaResp = append(mediaResp, apimodel.MediaFromDomain(&e.Media[j]))
		}
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccountStatuses(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

func firstLastIDsFromAccountStatuses(enriched []service.EnrichedStatus) (firstID, lastID string) {
	if len(enriched) == 0 {
		return "", ""
	}
	return enriched[0].Status.ID, enriched[len(enriched)-1].Status.ID
}

// GETFollowers handles GET /api/v1/accounts/:id/followers.
func (h *AccountsHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	// TODO: if target locked and viewer does not follow, return empty
	params := PageParamsFromRequest(r)
	followers, err := h.follows.GetFollowers(r.Context(), target.ID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(followers))
	for _, a := range followers {
		out = append(out, apimodel.ToAccount(&a, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccounts(followers)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

func firstLastIDsFromAccounts(list []domain.Account) (firstID, lastID string) {
	if len(list) == 0 {
		return "", ""
	}
	return list[0].ID, list[len(list)-1].ID
}

// GETFollowing handles GET /api/v1/accounts/:id/following.
func (h *AccountsHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	params := PageParamsFromRequest(r)
	list, err := h.follows.GetFollowing(r.Context(), target.ID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(&a, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccounts(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETBlocks handles GET /api/v1/blocks. Returns paginated blocked accounts for the authenticated user.
func (h *AccountsHandler) GETBlocks(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := DefaultListLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > MaxListLimit {
				limit = MaxListLimit
			}
		}
	}
	blocks, nextCursor, err := h.follows.ListBlockedAccounts(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(blocks))
	for i := range blocks {
		out = append(out, apimodel.ToAccount(&blocks[i], h.instanceDomain))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETMutes handles GET /api/v1/mutes. Returns paginated muted accounts for the authenticated user.
func (h *AccountsHandler) GETMutes(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := DefaultListLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > MaxListLimit {
				limit = MaxListLimit
			}
		}
	}
	mutes, nextCursor, err := h.follows.ListMutedAccounts(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(mutes))
	for i := range mutes {
		out = append(out, apimodel.ToAccount(&mutes[i], h.instanceDomain))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTBlock handles POST /api/v1/accounts/:id/block.
func (h *AccountsHandler) POSTBlock(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Block(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnblock handles POST /api/v1/accounts/:id/unblock.
func (h *AccountsHandler) POSTUnblock(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unblock(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTMute handles POST /api/v1/accounts/:id/mute.
func (h *AccountsHandler) POSTMute(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	hideNotifications := r.FormValue("notifications") == resolveQueryTrue || r.FormValue("notifications") == "1"
	if r.Form == nil {
		_ = r.ParseForm()
	}
	if r.Form != nil {
		hideNotifications = r.Form.Get("notifications") == resolveQueryTrue || r.Form.Get("notifications") == "1"
	}
	rel, err := h.follows.Mute(r.Context(), account.ID, target.ID, hideNotifications)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnmute handles POST /api/v1/accounts/:id/unmute.
func (h *AccountsHandler) POSTUnmute(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unmute(r.Context(), account.ID, target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// GETFollowedTags handles GET /api/v1/followed_tags.
func (h *AccountsHandler) GETFollowedTags(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := DefaultListLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
			if limit > MaxListLimit {
				limit = MaxListLimit
			}
		}
	}
	tags, nextCursor, err := h.follows.ListFollowedTags(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Tag, 0, len(tags))
	for i := range tags {
		out = append(out, apimodel.FollowedTagFromDomain(tags[i], h.instanceDomain))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

type postFollowedTagRequest struct {
	Name string `json:"name"`
}

func (r *postFollowedTagRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Name, "name"); err != nil {
		return fmt.Errorf("name: %w", err)
	}
	return nil
}

// POSTFollowedTags handles POST /api/v1/followed_tags. Body: { "name": "hashtag" }.
func (h *AccountsHandler) POSTFollowedTags(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body postFollowedTagRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	tag, err := h.follows.FollowTag(r.Context(), account.ID, body.Name)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("Validation failed: Tag is invalid"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.FollowedTagFromDomain(*tag, h.instanceDomain))
}

// DELETEFollowedTag handles DELETE /api/v1/followed_tags/:id. id is the tag ID.
func (h *AccountsHandler) DELETEFollowedTag(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	tagID := chi.URLParam(r, "id")
	if tagID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.follows.UnfollowTag(r.Context(), account.ID, tagID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{})
}

package mastodon

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// AccountsHandler handles account-related Mastodon API endpoints.
type AccountsHandler struct {
	accounts       *service.AccountService
	follows        *service.FollowService
	timeline       *service.TimelineService
	instanceDomain string
}

// NewAccountsHandler returns a new AccountsHandler. follows may be nil to disable follow endpoints; timeline is required for GET account statuses.
func NewAccountsHandler(accounts *service.AccountService, follows *service.FollowService, timeline *service.TimelineService, instanceDomain string) *AccountsHandler {
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

// GETAccounts handles GETAccounts /api/v1/accounts/:id. Auth optional.
func (h *AccountsHandler) GETAccounts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := api.ValidateRequiredString(id); err != nil {
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

// POSTFollow handles POST /api/v1/accounts/:id/follow. Auth required.
func (h *AccountsHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if err := api.ValidateRequiredString(targetID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Follow(r.Context(), account.ID, targetID)
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
	if err := api.ValidateRequiredString(targetID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unfollow(r.Context(), account.ID, targetID)
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
		rel, err := h.accounts.GetRelationship(r.Context(), account.ID, targetID)
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
	acc, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if len(fields) == 0 {
		fields = acc.Fields
	}
	updated, updatedUser, err := h.accounts.UpdateCredentials(r.Context(), service.UpdateCredentialsInput{
		AccountID:     account.ID,
		DisplayName:   displayNamePtr,
		Note:          notePtr,
		AvatarMediaID: avatarMediaID,
		HeaderMediaID: headerMediaID,
		Locked:        locked,
		Bot:           bot,
		Fields:        fields,
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
	if h.timeline == nil {
		api.HandleError(w, r, api.NewUnprocessableError("timeline not configured"))
		return
	}
	params := PageParamsFromRequest(r)
	var viewerID *string
	if account != nil {
		viewerID = &account.ID
	}
	enriched, err := h.timeline.AccountStatusesEnriched(r.Context(), id, viewerID, optionalString(params.MaxID), params.Limit)
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
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
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
	// TODO: if target locked and viewer does not follow, return empty
	params := PageParamsFromRequest(r)
	_, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	followers, err := h.follows.GetFollowers(r.Context(), targetID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(followers))
	for _, a := range followers {
		out = append(out, apimodel.ToAccount(&a, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccounts(followers)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
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
	_, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	params := PageParamsFromRequest(r)
	list, err := h.follows.GetFollowing(r.Context(), targetID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(&a, h.instanceDomain))
	}
	firstID, lastID := firstLastIDsFromAccounts(list)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
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
	rel, err := h.follows.Block(r.Context(), account.ID, targetID)
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
	rel, err := h.follows.Unblock(r.Context(), account.ID, targetID)
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
	hideNotifications := r.FormValue("notifications") == resolveQueryTrue || r.FormValue("notifications") == "1"
	if r.Form == nil {
		_ = r.ParseForm()
	}
	if r.Form != nil {
		hideNotifications = r.Form.Get("notifications") == resolveQueryTrue || r.Form.Get("notifications") == "1"
	}
	rel, err := h.follows.Mute(r.Context(), account.ID, targetID, hideNotifications)
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
	rel, err := h.follows.Unmute(r.Context(), account.ID, targetID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

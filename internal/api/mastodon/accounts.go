package mastodon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/microcosm-cc/bluemonday"

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
	tagFollows     service.TagFollowService
	timeline       service.TimelineService
	settings       service.MonsteraSettingsService
	media          service.MediaService
	mediaMaxBytes  int64
	instanceDomain string
}

// NewAccountsHandler returns a new AccountsHandler. follows may be nil to disable follow endpoints; timeline is required for GET account statuses.
func NewAccountsHandler(accounts service.AccountService, follows service.FollowService, tagFollows service.TagFollowService, timeline service.TimelineService, settings service.MonsteraSettingsService, media service.MediaService, mediaMaxBytes int64, instanceDomain string) *AccountsHandler {
	return &AccountsHandler{accounts: accounts, follows: follows, tagFollows: tagFollows, timeline: timeline, settings: settings, media: media, mediaMaxBytes: mediaMaxBytes, instanceDomain: instanceDomain}
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
	limit := parseLimitParam(r, DefaultListLimit, MaxListLimit)
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
	localOnly := api.QueryParamIsTrue(r, "local")

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

// PATCHUpdateCredentials handles PATCH /api/v1/accounts/update_credentials.
func (h *AccountsHandler) PATCHUpdateCredentials(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	input, err := h.parseUpdateCredentialsRequest(w, r, account)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	acc, _, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if len(input.Fields) == 0 {
		input.Fields = acc.Fields
	}
	updated, updatedUser, err := h.accounts.UpdateCredentials(r.Context(), *input)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccountWithSource(updated, updatedUser, h.instanceDomain))
}

// parseUpdateCredentialsRequest parses the multipart/form request body for
// PATCHUpdateCredentials, uploading avatar/header files and returning the
// service input. w is needed for http.MaxBytesReader.
func (h *AccountsHandler) parseUpdateCredentialsRequest(w http.ResponseWriter, r *http.Request, account *domain.Account) (*service.UpdateCredentialsInput, error) {
	const formOverhead = 64 * 1024
	maxBody := 2*h.mediaMaxBytes + formOverhead
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(h.mediaMaxBytes); err != nil {
		slog.DebugContext(r.Context(), "failed to parse multipart form, falling back to form parse", slog.Any("error", err))
		if r.Form == nil {
			if err := r.ParseForm(); err != nil {
				return nil, fmt.Errorf("parse form: %w", api.NewBadRequestError("failed to parse form"))
			}
		}
	}

	form := r.Form
	displayName := bluemonday.UGCPolicy().Sanitize(api.FormValue(form, "display_name"))
	note := bluemonday.UGCPolicy().Sanitize(api.FormValue(form, "note"))
	locked := api.FormValueIsTruthy(form, "locked")
	bot := api.FormValueIsTruthy(form, "bot")

	var avatarMediaID, headerMediaID *string
	if h.media != nil {
		var err error
		avatarMediaID, err = h.uploadFormFile(r, "avatar", account.ID, h.media.UploadAvatar)
		if err != nil {
			return nil, err
		}
		headerMediaID, err = h.uploadFormFile(r, "header", account.ID, h.media.UploadHeader)
		if err != nil {
			return nil, err
		}
	}

	var displayNamePtr, notePtr *string
	if displayName != "" {
		displayNamePtr = &displayName
	}
	if note != "" {
		notePtr = &note
	}
	quotePolicy := api.FormValue(form, "source[quote_policy]")
	if quotePolicy == "" {
		quotePolicy = api.FormValue(form, "quote_policy")
	}
	var defaultQuotePolicy *string
	if quotePolicy != "" {
		defaultQuotePolicy = &quotePolicy
	}

	return &service.UpdateCredentialsInput{
		AccountID:          account.ID,
		DisplayName:        displayNamePtr,
		Note:               notePtr,
		AvatarMediaID:      avatarMediaID,
		HeaderMediaID:      headerMediaID,
		Locked:             locked,
		Bot:                bot,
		DefaultQuotePolicy: defaultQuotePolicy,
		Fields:             parseFieldsAttributes(form),
	}, nil
}

type uploadFunc func(ctx context.Context, accountID string, file io.Reader, contentType string) (*service.UploadResult, error)

// uploadFormFile extracts a named file from the multipart form, uploads it
// via the provided upload function, and returns the resulting media attachment ID.
func (h *AccountsHandler) uploadFormFile(r *http.Request, field string, accountID string, upload uploadFunc) (*string, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	file, fh, err := r.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get "+field+" file from request", slog.Any("error", err))
		return nil, nil
	}
	defer func() { _ = file.Close() }()
	ct := fh.Header.Get("Content-Type")
	if ct == "" {
		ct = contentTypeOctetStream
	}
	result, err := upload(r.Context(), accountID, file, ct)
	if err != nil {
		return nil, err
	}
	return &result.Attachment.ID, nil
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
		out = append(out, apimodel.ToStatus(e.Status, authorAcc, mentionsResp, tagsResp, mediaResp, e.Card, h.instanceDomain))
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

// POSTAccounts handles POST /api/v1/accounts (public registration endpoint).
func (h *AccountsHandler) POSTAccounts(w http.ResponseWriter, r *http.Request) {
	var body apimodel.RegisterAccountRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}

	settings, err := h.settings.Get(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if settings.RegistrationMode == domain.MonsteraRegistrationModeClosed {
		api.HandleError(w, r, api.NewForbiddenError("registrations are closed"))
		return
	}

	acc, err := h.accounts.Register(r.Context(), service.RegisterInput{
		Username:           body.Username,
		Email:              body.Email,
		Password:           body.Password,
		Role:               domain.RoleUser,
		RegistrationReason: body.Reason,
		InviteCode:         body.InviteCode,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	pending := settings.RegistrationMode == domain.MonsteraRegistrationModeApproval
	api.WriteJSON(w, http.StatusOK, apimodel.RegisterAccountResponse{
		Account: apimodel.ToAccount(acc, h.instanceDomain),
		Pending: pending,
	})
}

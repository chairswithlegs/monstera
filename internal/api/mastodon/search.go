package mastodon

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// SearchHandler handles the Mastodon search API.
type SearchHandler struct {
	search         service.SearchService
	instanceDomain string
}

// NewSearchHandler returns a new SearchHandler.
func NewSearchHandler(search service.SearchService, instanceDomain string) *SearchHandler {
	return &SearchHandler{search: search, instanceDomain: instanceDomain}
}

type searchRequest struct {
	Q       string
	Type    string
	Resolve bool
	Limit   int
}

const resolveQueryTrue = "true"

func parseSearchRequest(r *http.Request) (*searchRequest, error) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return nil, fmt.Errorf("parse search request: %w", api.NewUnprocessableError("q is required"))
	}
	typ := r.URL.Query().Get("type")
	resolve := r.URL.Query().Get("resolve") == resolveQueryTrue
	limit := 5
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("parse search request: %w", api.NewUnprocessableError("limit must be a positive integer"))
		}
		if n > 40 {
			n = 40
		}
		limit = n
	}
	return &searchRequest{Q: q, Type: typ, Resolve: resolve, Limit: limit}, nil
}

func mapTypeToSearchType(typ string) service.SearchType {
	switch strings.ToLower(typ) {
	case "accounts":
		return service.SearchTypeAccounts
	case "statuses":
		return service.SearchTypeStatuses
	case "hashtags":
		return service.SearchTypeHashtags
	default:
		return service.SearchTypeAll
	}
}

// GETSearch handles GET /api/v2/search.
func (h *SearchHandler) GETSearch(w http.ResponseWriter, r *http.Request) {
	req, err := parseSearchRequest(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	viewer := middleware.AccountFromContext(r.Context())
	filter := mapTypeToSearchType(req.Type)
	res, err := h.search.Search(r.Context(), viewer, req.Q, filter, req.Resolve, req.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	accounts := make([]apimodel.Account, 0, len(res.Accounts))
	for _, a := range res.Accounts {
		accounts = append(accounts, apimodel.ToAccount(a, h.instanceDomain))
	}
	hashtags := make([]apimodel.Tag, 0, len(res.Hashtags))
	for _, t := range res.Hashtags {
		hashtags = append(hashtags, apimodel.TagFromName(t.Name, h.instanceDomain))
	}
	body := struct {
		Accounts []apimodel.Account `json:"accounts"`
		Statuses []apimodel.Status  `json:"statuses"`
		Hashtags []apimodel.Tag     `json:"hashtags"`
	}{
		Accounts: accounts,
		Statuses: []apimodel.Status{}, // Phase 1: always empty
		Hashtags: hashtags,
	}
	api.WriteJSON(w, http.StatusOK, body)
}

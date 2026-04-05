package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/ssrf"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/microcosm-cc/bluemonday"
)

const (
	cardMaxBodyBytes = 1 << 20 // 1 MB
	cardUserAgent    = "Monstera/1.0"
)

// DomainBlockChecker checks whether a domain is blocked.
type DomainBlockChecker interface {
	IsSuspended(ctx context.Context, domain string) bool
}

// CardService fetches and stores link preview cards for statuses.
type CardService interface {
	FetchAndStoreCard(ctx context.Context, statusID string) error
}

type cardService struct {
	store      store.Store
	blocklist  DomainBlockChecker
	httpClient *http.Client
}

// NewCardService returns a CardService backed by the given store.
func NewCardService(s store.Store, bl DomainBlockChecker) CardService {
	// Use a secure egress HTTP client to protect against SSRF attacks.
	client := ssrf.NewHTTPClient(ssrf.HTTPClientOptions{})
	return &cardService{store: s, blocklist: bl, httpClient: client}
}

func (svc *cardService) FetchAndStoreCard(ctx context.Context, statusID string) error {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return fmt.Errorf("GetStatusByID: %w", err)
	}

	content := ""
	if st.Content != nil {
		content = *st.Content
	}

	rawURL := extractFirstURL(content)
	if rawURL != "" && svc.blocklist != nil {
		if parsed, parseErr := url.Parse(rawURL); parseErr == nil && parsed.Host != "" {
			if svc.blocklist.IsSuspended(ctx, parsed.Hostname()) {
				if upsertErr := svc.store.UpsertStatusCard(ctx, store.UpsertStatusCardInput{
					StatusID:        statusID,
					ProcessingState: domain.CardStateFetchFailed,
					URL:             rawURL,
					CardType:        "link",
				}); upsertErr != nil {
					return fmt.Errorf("UpsertStatusCard: %w", upsertErr)
				}
				return nil
			}
		}
	}
	if rawURL == "" {
		if err := svc.store.UpsertStatusCard(ctx, store.UpsertStatusCardInput{
			StatusID:        statusID,
			ProcessingState: domain.CardStateNoURL,
			CardType:        "link",
		}); err != nil {
			return fmt.Errorf("UpsertStatusCard: %w", err)
		}
		return nil
	}

	og, err := svc.fetchOGMetadata(ctx, rawURL)
	if err != nil {
		slog.WarnContext(ctx, "card: og fetch failed", slog.String("url", rawURL), slog.Any("error", err))
		if upsertErr := svc.store.UpsertStatusCard(ctx, store.UpsertStatusCardInput{
			StatusID:        statusID,
			ProcessingState: domain.CardStateFetchFailed,
			URL:             rawURL,
			CardType:        "link",
		}); upsertErr != nil {
			return fmt.Errorf("UpsertStatusCard: %w", upsertErr)
		}
		return nil
	}

	if err := svc.store.UpsertStatusCard(ctx, store.UpsertStatusCardInput{
		StatusID:        statusID,
		ProcessingState: domain.CardStateFetched,
		URL:             rawURL,
		Title:           og.title,
		Description:     og.description,
		CardType:        "link",
		ImageURL:        og.image,
	}); err != nil {
		return fmt.Errorf("UpsertStatusCard: %w", err)
	}
	return nil
}

type ogMetadata struct {
	title       string
	description string
	image       string
}

func (svc *cardService) fetchOGMetadata(ctx context.Context, rawURL string) (*ogMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", cardUserAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := svc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, cardMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseOGMetadata(body), nil
}

// parseOGMetadata parses and sanitizes OG meta tags and title from raw HTML bytes.
func parseOGMetadata(body []byte) *ogMetadata {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return &ogMetadata{}
	}

	var og ogMetadata
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if n.FirstChild != nil && og.title == "" {
					og.title = n.FirstChild.Data
				}
			case "meta":
				prop := attrVal(n, "property")
				name := attrVal(n, "name")
				content := attrVal(n, "content")
				switch prop {
				case "og:title":
					if content != "" {
						og.title = content
					}
				case "og:description":
					og.description = content
				case "og:image":
					og.image = content
				}
				if name == "description" && og.description == "" {
					og.description = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Sanitize extracted metadata to prevent XSS via injected OpenGraph tags
	og.title = bluemonday.StrictPolicy().Sanitize(og.title)
	og.description = bluemonday.StrictPolicy().Sanitize(og.description)

	// Image must be a valid URL
	if og.image != "" {
		if _, err := url.Parse(og.image); err != nil {
			og.image = ""
		}
	}
	return &og
}

// extractFirstURL finds the first external http/https URL in an <a href> that is
// not a mention or hashtag link.
func extractFirstURL(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var found string
	var walk func(*html.Node) bool
	walk = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "a" {
			cls := attrVal(n, "class")
			rel := attrVal(n, "rel")
			if strings.Contains(cls, "mention") || strings.Contains(cls, "hashtag") ||
				strings.Contains(rel, "mention") || strings.Contains(rel, "tag") {
				return false
			}
			href := attrVal(n, "href")
			if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
				found = href
				return true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(doc)
	return found
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

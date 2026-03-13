package domain

// Card processing states.
const (
	CardStateNoURL       = "no_url"       // status had no external URL
	CardStateFetchFailed = "fetch_failed" // URL found but fetch/parse failed
	CardStateFetched     = "fetched"      // card data successfully retrieved
)

// Card holds the link preview data for a status.
type Card struct {
	StatusID        string
	ProcessingState string // CardStateNoURL | CardStateFetchFailed | CardStateFetched
	URL             string
	Title           string
	Description     string
	Type            string // always "link" for now
	ProviderName    string
	ProviderURL     string
	ImageURL        string
	Width           int
	Height          int
}

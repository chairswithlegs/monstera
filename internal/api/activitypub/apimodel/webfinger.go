package apimodel

type WebFingerResponse struct {
	Subject string          `json:"subject"`
	Aliases []string        `json:"aliases"`
	Links   []WebFingerLink `json:"links"`
}

type WebFingerLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}

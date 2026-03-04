package apimodel

// AdminSettings is the response for GET /admin/settings.
type AdminSettings struct {
	Settings map[string]string `json:"settings"`
}

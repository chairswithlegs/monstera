package apimodel

type NodeInfoPointerResponse struct {
	Links []NodeInfoPointerLink `json:"links"`
}

type NodeInfoPointerLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

type NodeInfoResponse struct {
	Version           string           `json:"version"`
	Software          NodeInfoSoftware `json:"software"`
	Protocols         []string         `json:"protocols"`
	Usage             NodeInfoUsage    `json:"usage"`
	OpenRegistrations bool             `json:"openRegistrations"`
	Metadata          map[string]any   `json:"metadata"`
}

type NodeInfoSoftware struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type NodeInfoUsage struct {
	Users      NodeInfoUsers `json:"users"`
	LocalPosts int64         `json:"localPosts"`
}

type NodeInfoUsers struct {
	Total int64 `json:"total"`
}

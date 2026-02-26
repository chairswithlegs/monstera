package activitypub

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// NodeInfoPointerHandler serves the well-known nodeinfo pointer document.
// GET /.well-known/nodeinfo
type NodeInfoPointerHandler struct {
	deps Deps
}

// NewNodeInfoPointerHandler constructs a NodeInfoPointerHandler.
func NewNodeInfoPointerHandler(deps Deps) *NodeInfoPointerHandler {
	return &NodeInfoPointerHandler{deps: deps}
}

type nodeInfoPointerResponse struct {
	Links []nodeInfoPointerLink `json:"links"`
}

type nodeInfoPointerLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// ServeHTTP serves the nodeinfo pointer.
func (h *NodeInfoPointerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := nodeInfoPointerResponse{
		Links: []nodeInfoPointerLink{
			{
				Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
				Href: fmt.Sprintf("https://%s/nodeinfo/2.0", h.deps.Config.InstanceDomain),
			},
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=1800")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// NodeInfoHandler serves the full NodeInfo 2.0 document.
// GET /nodeinfo/2.0
type NodeInfoHandler struct {
	deps Deps
}

// NewNodeInfoHandler constructs a NodeInfoHandler.
func NewNodeInfoHandler(deps Deps) *NodeInfoHandler {
	return &NodeInfoHandler{deps: deps}
}

type nodeInfoResponse struct {
	Version           string           `json:"version"`
	Software          nodeInfoSoftware `json:"software"`
	Protocols         []string         `json:"protocols"`
	Usage             nodeInfoUsage    `json:"usage"`
	OpenRegistrations bool             `json:"openRegistrations"`
	Metadata          map[string]any   `json:"metadata"`
}

type nodeInfoSoftware struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type nodeInfoUsage struct {
	Users      nodeInfoUsers `json:"users"`
	LocalPosts int64         `json:"localPosts"`
}

type nodeInfoUsers struct {
	Total int64 `json:"total"`
}

// ServeHTTP serves the NodeInfo 2.0 document.
func (h *NodeInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.deps.Instance.GetNodeInfoStats(ctx)
	if err != nil {
		h.deps.Logger.Error("nodeinfo: get stats", slog.Any("error", err))
		stats = &service.NodeInfoStats{}
	}
	resp := nodeInfoResponse{
		Version: "2.0",
		Software: nodeInfoSoftware{
			Name:    "monstera-fed",
			Version: h.deps.Config.Version,
		},
		Protocols: []string{"activitypub"},
		Usage: nodeInfoUsage{
			Users:      nodeInfoUsers{Total: stats.UserCount},
			LocalPosts: stats.LocalPostCount,
		},
		OpenRegistrations: stats.OpenRegistrations,
		Metadata:          map[string]any{},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=1800")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

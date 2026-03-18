package activitypub

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/activitypub/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// NodeInfoPointerHandler serves the well-known nodeinfo pointer document.
// GET /.well-known/nodeinfo
type NodeInfoPointerHandler struct {
	instanceBaseURL string
}

// NewNodeInfoPointerHandler returns a new NodeInfoPointerHandler.
func NewNodeInfoPointerHandler(instanceBaseURL string) *NodeInfoPointerHandler {
	return &NodeInfoPointerHandler{instanceBaseURL: instanceBaseURL}
}

// GETNodeInfoPointer serves the nodeinfo pointer.
func (h *NodeInfoPointerHandler) GETNodeInfoPointer(w http.ResponseWriter, r *http.Request) {
	resp := apimodel.NodeInfoPointerResponse{
		Links: []apimodel.NodeInfoPointerLink{
			{
				Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
				Href: h.instanceBaseURL + "/nodeinfo/2.0",
			},
		},
	}
	w.Header().Set("Cache-Control", "max-age=1800")
	api.WriteJSON(w, http.StatusOK, resp)
}

// NodeInfoHandler serves the full NodeInfo 2.0 document.
// GET /nodeinfo/2.0
type NodeInfoHandler struct {
	instance service.InstanceService
	version  string
}

// NewNodeInfoHandler returns a new NodeInfoHandler.
func NewNodeInfoHandler(instance service.InstanceService, version string) *NodeInfoHandler {
	return &NodeInfoHandler{instance: instance, version: version}
}

// GETNodeInfo serves the NodeInfo 2.0 document.
func (h *NodeInfoHandler) GETNodeInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.instance.GetNodeInfoStats(ctx)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	resp := apimodel.NodeInfoResponse{
		Version: "2.0",
		Software: apimodel.NodeInfoSoftware{
			Name:    "monstera",
			Version: h.version,
		},
		Protocols: []string{"activitypub"},
		Usage: apimodel.NodeInfoUsage{
			Users:      apimodel.NodeInfoUsers{Total: stats.UserCount},
			LocalPosts: stats.LocalPostCount,
		},
		OpenRegistrations: stats.OpenRegistrations,
		Metadata: map[string]any{
			"registration_mode":  stats.RegistrationMode,
			"server_name":        stats.ServerName,
			"server_description": stats.ServerDescription,
			"server_rules":       stats.ServerRules,
		},
	}
	api.WriteJSON(w, http.StatusOK, resp)
}

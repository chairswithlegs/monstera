package activitypub

import (
	"fmt"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/activitypub/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// NodeInfoPointerHandler serves the well-known nodeinfo pointer document.
// GET /.well-known/nodeinfo
type NodeInfoPointerHandler struct {
	config *config.Config
}

// NewNodeInfoPointerHandler returns a new NodeInfoPointerHandler.
func NewNodeInfoPointerHandler(config *config.Config) *NodeInfoPointerHandler {
	return &NodeInfoPointerHandler{config: config}
}

// GETNodeInfoPointer serves the nodeinfo pointer.
func (h *NodeInfoPointerHandler) GETNodeInfoPointer(w http.ResponseWriter, r *http.Request) {
	resp := apimodel.NodeInfoPointerResponse{
		Links: []apimodel.NodeInfoPointerLink{
			{
				Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
				Href: fmt.Sprintf("https://%s/nodeinfo/2.0", h.config.InstanceDomain),
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
	config   *config.Config
}

// NewNodeInfoHandler returns a new NodeInfoHandler.
func NewNodeInfoHandler(instance service.InstanceService, config *config.Config) *NodeInfoHandler {
	return &NodeInfoHandler{instance: instance, config: config}
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
			Name:    "monstera-fed",
			Version: h.config.Version,
		},
		Protocols: []string{"activitypub"},
		Usage: apimodel.NodeInfoUsage{
			Users:      apimodel.NodeInfoUsers{Total: stats.UserCount},
			LocalPosts: stats.LocalPostCount,
		},
		OpenRegistrations: stats.OpenRegistrations,
		Metadata:          map[string]any{},
	}
	w.Header().Set("Cache-Control", "max-age=1800")
	api.WriteJSON(w, http.StatusOK, resp)
}

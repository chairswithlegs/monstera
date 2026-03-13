package apimodel

import (
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/microcosm-cc/bluemonday"
)

// AdminKnownInstance is one known instance in the admin API.
type AdminKnownInstance struct {
	ID              string    `json:"id"`
	Domain          string    `json:"domain"`
	Software        *string   `json:"software,omitempty"`
	SoftwareVersion *string   `json:"software_version,omitempty"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	AccountsCount   int64     `json:"accounts_count"`
}

// ToAdminKnownInstance converts a domain known instance to the admin API shape.
func ToAdminKnownInstance(k *domain.KnownInstance) AdminKnownInstance {
	return AdminKnownInstance{
		ID:              k.ID,
		Domain:          k.Domain,
		Software:        k.Software,
		SoftwareVersion: k.SoftwareVersion,
		FirstSeenAt:     k.FirstSeenAt,
		LastSeenAt:      k.LastSeenAt,
		AccountsCount:   k.AccountsCount,
	}
}

// AdminKnownInstanceList is the response for GET /admin/federation/instances.
type AdminKnownInstanceList struct {
	Instances []AdminKnownInstance `json:"instances"`
}

// AdminDomainBlock is one domain block in the admin API.
type AdminDomainBlock struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Severity  string    `json:"severity"`
	Reason    *string   `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ToAdminDomainBlock converts a domain block to the admin API shape.
func ToAdminDomainBlock(d *domain.DomainBlock) AdminDomainBlock {
	return AdminDomainBlock{
		ID:        d.ID,
		Domain:    d.Domain,
		Severity:  d.Severity,
		Reason:    d.Reason,
		CreatedAt: d.CreatedAt,
	}
}

// AdminDomainBlockList is the response for GET /admin/federation/domain-blocks.
type AdminDomainBlockList struct {
	DomainBlocks []AdminDomainBlock `json:"domain_blocks"`
}

type PostDomainBlocksRequest struct {
	Domain   string  `json:"domain"`
	Severity string  `json:"severity"`
	Reason   *string `json:"reason"`
}

func (r *PostDomainBlocksRequest) Sanitize() {
	if r.Reason != nil {
		*r.Reason = bluemonday.StrictPolicy().Sanitize(*r.Reason)
	}
}

func (r *PostDomainBlocksRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Domain, "domain"); err != nil {
		return fmt.Errorf("domain: %w", err)
	}
	if r.Severity != domain.DomainBlockSeveritySilence && r.Severity != domain.DomainBlockSeveritySuspend {
		r.Severity = domain.DomainBlockSeveritySilence
	}
	return nil
}

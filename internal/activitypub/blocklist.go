package activitypub

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// BlocklistCache caches the set of blocked domains for fast inbox/outbox checks.
// It keeps an in-memory map populated from the store; call Refresh at startup
// and after admin domain block create/update/delete. On lookup miss, it loads
// from the store once to repopulate the map.
type BlocklistCache struct {
	store    store.Store
	mu       sync.RWMutex
	byDomain map[string]domain.DomainBlock
}

// NewBlocklistCache constructs a BlocklistCache.
func NewBlocklistCache(store store.Store) *BlocklistCache {
	return &BlocklistCache{
		store:    store,
		byDomain: make(map[string]domain.DomainBlock),
	}
}

// Refresh loads all domain blocks from the store and updates the in-memory map.
// Call at startup and after any admin domain block create/update/delete.
func (b *BlocklistCache) Refresh(ctx context.Context) error {
	blocks, err := b.store.ListDomainBlocks(ctx)
	if err != nil {
		return fmt.Errorf("list domain blocks: %w", err)
	}
	b.mu.Lock()
	b.byDomain = make(map[string]domain.DomainBlock, len(blocks))
	for _, blk := range blocks {
		b.byDomain[normalizeDomain(blk.Domain)] = blk
	}
	b.mu.Unlock()
	return nil
}

// loadFromStore repopulates byDomain from the store (e.g. on first use or after in-memory miss).
func (b *BlocklistCache) loadFromStore(ctx context.Context) {
	if err := b.Refresh(ctx); err != nil {
		slog.WarnContext(ctx, "blocklist: refresh failed", slog.Any("err", err))
	}
}

// lookup returns the block for the domain if present, loading from the store on in-memory miss.
func (b *BlocklistCache) lookup(ctx context.Context, domainName string) (domain.DomainBlock, bool) {
	key := normalizeDomain(domainName)
	b.mu.RLock()
	blk, ok := b.byDomain[key]
	b.mu.RUnlock()
	if ok {
		return blk, true
	}
	b.loadFromStore(ctx)
	b.mu.RLock()
	defer b.mu.RUnlock()
	blk, ok = b.byDomain[key]
	return blk, ok
}

// IsBlocked returns true if the domain is in the block list (any severity).
func (b *BlocklistCache) IsBlocked(ctx context.Context, domainName string) bool {
	_, ok := b.lookup(ctx, domainName)
	return ok
}

// Severity returns the block severity for the domain ("suspend", "silence", or "" if not blocked).
func (b *BlocklistCache) Severity(ctx context.Context, domainName string) string {
	blk, ok := b.lookup(ctx, domainName)
	if !ok {
		return ""
	}
	return blk.Severity
}

// IsSuspended returns true if the domain is blocked with severity "suspend".
func (b *BlocklistCache) IsSuspended(ctx context.Context, domainName string) bool {
	return b.Severity(ctx, domainName) == domain.DomainBlockSeveritySuspend
}

// IsSilenced returns true if the domain is blocked with severity "silence".
func (b *BlocklistCache) IsSilenced(ctx context.Context, domainName string) bool {
	return b.Severity(ctx, domainName) == domain.DomainBlockSeveritySilence
}

func normalizeDomain(d string) string {
	return strings.ToLower(strings.TrimSpace(d))
}

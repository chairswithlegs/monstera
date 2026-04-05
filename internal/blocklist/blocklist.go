package blocklist

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

const negativeCacheTTL = 5 * time.Minute

// blocklistStore is the narrow interface the BlocklistCache needs.
type blocklistStore interface {
	ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error)
}

// BlocklistCache caches the set of blocked domains for fast inbox/outbox checks.
// It keeps an in-memory map populated from the store; call Refresh at startup
// and after admin domain block create/update/delete. On lookup miss, it loads
// from the store once to repopulate the map. Domains confirmed as not-blocked
// are cached with a TTL to avoid repeated full-table scans.
type BlocklistCache struct {
	store    blocklistStore
	mu       sync.RWMutex
	byDomain map[string]domain.DomainBlock
	negative map[string]time.Time // domain → expiry for confirmed-not-blocked entries
}

// NewBlocklistCache constructs a BlocklistCache.
func NewBlocklistCache(store blocklistStore) *BlocklistCache {
	return &BlocklistCache{
		store:    store,
		byDomain: make(map[string]domain.DomainBlock),
		negative: make(map[string]time.Time),
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
	b.negative = make(map[string]time.Time)
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
	if ok {
		b.mu.RUnlock()
		return blk, true
	}
	if expiry, neg := b.negative[key]; neg && time.Now().Before(expiry) {
		b.mu.RUnlock()
		return domain.DomainBlock{}, false
	}
	b.mu.RUnlock()

	b.loadFromStore(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()
	blk, ok = b.byDomain[key]
	if !ok {
		b.negative[key] = time.Now().Add(negativeCacheTTL)
	}
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

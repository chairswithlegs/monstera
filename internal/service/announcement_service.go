package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// AnnouncementItem is an active announcement with read state and reactions for a viewer.
type AnnouncementItem struct {
	Announcement domain.Announcement
	Read         bool
	Reactions    []domain.AnnouncementReactionCount
}

// AnnouncementService handles announcement listing, dismiss, and reactions.
type AnnouncementService interface {
	ListActive(ctx context.Context, accountID string) ([]AnnouncementItem, error)
	Dismiss(ctx context.Context, accountID, announcementID string) error
	AddReaction(ctx context.Context, accountID, announcementID, name string) error
	RemoveReaction(ctx context.Context, accountID, announcementID, name string) error
	GetByID(ctx context.Context, id string) (*domain.Announcement, error)
	Create(ctx context.Context, in store.CreateAnnouncementInput) (*domain.Announcement, error)
	Update(ctx context.Context, in store.UpdateAnnouncementInput) error
	ListAll(ctx context.Context) ([]domain.Announcement, error)
}

type announcementService struct {
	store store.Store
}

// NewAnnouncementService returns an AnnouncementService.
func NewAnnouncementService(s store.Store) AnnouncementService {
	return &announcementService{store: s}
}

// ListActive returns active announcements for the account with read flags and reactions (Me set for viewer).
func (svc *announcementService) ListActive(ctx context.Context, accountID string) ([]AnnouncementItem, error) {
	announcements, err := svc.store.ListActiveAnnouncements(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListActiveAnnouncements: %w", err)
	}
	readIDs, err := svc.store.ListReadAnnouncementIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListReadAnnouncementIDs: %w", err)
	}
	readSet := make(map[string]struct{})
	for _, id := range readIDs {
		readSet[id] = struct{}{}
	}
	out := make([]AnnouncementItem, 0, len(announcements))
	for _, a := range announcements {
		read := false
		if _, ok := readSet[a.ID]; ok {
			read = true
		}
		reactions, err := svc.store.ListAnnouncementReactionCounts(ctx, a.ID)
		if err != nil {
			return nil, fmt.Errorf("ListAnnouncementReactionCounts(%s): %w", a.ID, err)
		}
		myNames, err := svc.store.ListAccountAnnouncementReactionNames(ctx, a.ID, accountID)
		if err != nil {
			return nil, fmt.Errorf("ListAccountAnnouncementReactionNames(%s): %w", a.ID, err)
		}
		mySet := make(map[string]struct{})
		for _, n := range myNames {
			mySet[n] = struct{}{}
		}
		for i := range reactions {
			if _, ok := mySet[reactions[i].Name]; ok {
				reactions[i].Me = true
			}
		}
		out = append(out, AnnouncementItem{
			Announcement: a,
			Read:         read,
			Reactions:    reactions,
		})
	}
	return out, nil
}

// Dismiss marks an announcement as read for the account.
func (svc *announcementService) Dismiss(ctx context.Context, accountID, announcementID string) error {
	if _, err := svc.store.GetAnnouncementByID(ctx, announcementID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("Dismiss: %w", err)
		}
		return fmt.Errorf("GetAnnouncementByID: %w", err)
	}
	if err := svc.store.DismissAnnouncement(ctx, accountID, announcementID); err != nil {
		return fmt.Errorf("DismissAnnouncement: %w", err)
	}
	return nil
}

// AddReaction adds an emoji reaction from the account to the announcement.
func (svc *announcementService) AddReaction(ctx context.Context, accountID, announcementID, name string) error {
	if name == "" {
		return domain.ErrValidation
	}
	if _, err := svc.store.GetAnnouncementByID(ctx, announcementID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("AddReaction: %w", err)
		}
		return fmt.Errorf("GetAnnouncementByID: %w", err)
	}
	if err := svc.store.AddAnnouncementReaction(ctx, announcementID, accountID, name); err != nil {
		return fmt.Errorf("AddAnnouncementReaction: %w", err)
	}
	return nil
}

// RemoveReaction removes an emoji reaction from the account for the announcement.
func (svc *announcementService) RemoveReaction(ctx context.Context, accountID, announcementID, name string) error {
	if name == "" {
		return domain.ErrValidation
	}
	if _, err := svc.store.GetAnnouncementByID(ctx, announcementID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("RemoveReaction: %w", err)
		}
		return fmt.Errorf("GetAnnouncementByID: %w", err)
	}
	if err := svc.store.RemoveAnnouncementReaction(ctx, announcementID, accountID, name); err != nil {
		return fmt.Errorf("RemoveAnnouncementReaction: %w", err)
	}
	return nil
}

// GetByID returns an announcement by ID.
func (svc *announcementService) GetByID(ctx context.Context, id string) (*domain.Announcement, error) {
	a, err := svc.store.GetAnnouncementByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetAnnouncementByID: %w", err)
	}
	return a, nil
}

// Create creates an announcement (admin).
func (svc *announcementService) Create(ctx context.Context, in store.CreateAnnouncementInput) (*domain.Announcement, error) {
	if in.ID == "" {
		in.ID = uid.New()
	}
	if in.PublishedAt.IsZero() {
		in.PublishedAt = time.Now()
	}
	a, err := svc.store.CreateAnnouncement(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("CreateAnnouncement: %w", err)
	}
	return a, nil
}

// Update updates an announcement (admin).
func (svc *announcementService) Update(ctx context.Context, in store.UpdateAnnouncementInput) error {
	if _, err := svc.store.GetAnnouncementByID(ctx, in.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("Update: %w", err)
		}
		return fmt.Errorf("GetAnnouncementByID: %w", err)
	}
	if err := svc.store.UpdateAnnouncement(ctx, in); err != nil {
		return fmt.Errorf("UpdateAnnouncement: %w", err)
	}
	return nil
}

// ListAll returns all announcements for admin (no read/reaction enrichment).
func (svc *announcementService) ListAll(ctx context.Context) ([]domain.Announcement, error) {
	list, err := svc.store.ListAllAnnouncements(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListAllAnnouncements: %w", err)
	}
	return list, nil
}

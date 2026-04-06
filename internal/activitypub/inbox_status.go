package activitypub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"

	"github.com/microcosm-cc/bluemonday"
)

// handleCreate handles a Create{Note} or Create{Question} activity.
func (p *inbox) handleCreate(ctx context.Context, activity *vocab.Activity, _ string) error {
	note, err := activity.ObjectNote()
	if err != nil {
		return fmt.Errorf("%w: create object is not a note: %w", ErrInboxFatal, err)
	}
	if note.Type != vocab.ObjectTypeNote && note.Type != vocab.ObjectTypeQuestion {
		return fmt.Errorf("%w: create object type %q is not supported", ErrInboxFatal, note.Type)
	}
	if note.ID != "" {
		if _, err := p.statuses.GetByAPID(ctx, note.ID); err == nil {
			return nil
		}
	}
	author, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	visibility := vocab.NoteVisibility(note, author.FollowersURL)

	// If the visibility is private, the status is only meant for local followers.
	if visibility == domain.VisibilityPrivate {
		hasLocal, err := p.hasLocalFollower(ctx, author.ID)
		if err != nil {
			return err
		}
		// If the author has no local followers, the status is not meant for anyone. Drop it.
		if !hasLocal {
			return nil
		}
	}
	// If the visibility is direct, the status is only meant for local recipients.
	if visibility == domain.VisibilityDirect {
		hasLocal, err := p.hasLocalRecipient(ctx, note.To)
		if err != nil {
			return err
		}
		// If the status is not meant for local recipients, drop it.
		if !hasLocal {
			return nil
		}
	}
	createInput := p.buildCreateStatusInput(ctx, note, author, visibility)
	// AP Note -> domain status
	_, err = p.remoteStatusWrites.CreateRemote(ctx, createInput)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create status: %w", err)
	}
	return nil
}

// handleAnnounce handles an Announce activity.
func (p *inbox) handleAnnounce(ctx context.Context, activity *vocab.Activity, _ string) error {
	if activity.ID != "" {
		if _, err := p.statuses.GetByAPID(ctx, activity.ID); err == nil {
			return nil
		}
	}
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: announce object is not a status IRI", ErrInboxFatal)
	}
	_, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("inbox: GetByAPID (Announce): %w", err)
		}
		objectDomain := vocab.DomainFromIRI(objectID)
		if objectDomain != "" && p.blocklist.IsSuspended(ctx, objectDomain) {
			slog.DebugContext(ctx, "inbox: dropped Announce referencing suspended domain",
				slog.String("object", objectID), slog.String("domain", objectDomain))
			return nil
		}
		var note vocab.Note
		if fetchErr := p.remoteResolver.resolveIRIDocument(ctx, objectID, &note); fetchErr != nil {
			return fmt.Errorf("inbox: fetch Note for Announce: %w", fetchErr)
		}
		if note.Type != vocab.ObjectTypeNote && note.Type != vocab.ObjectTypeQuestion {
			return fmt.Errorf("%w: announce object is not a Note or Question", ErrInboxFatal)
		}
		author, resolveErr := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, note.AttributedTo)
		if resolveErr != nil {
			return fmt.Errorf("inbox: resolve author for Announce %q: %w", note.AttributedTo, resolveErr)
		}
		visibility := vocab.NoteVisibility(&note, author.FollowersURL)
		if visibility == domain.VisibilityPrivate {
			hasLocal, checkErr := p.hasLocalFollower(ctx, author.ID)
			if checkErr != nil {
				return checkErr
			}
			if !hasLocal {
				return nil
			}
		}
		if visibility == domain.VisibilityDirect {
			hasLocal, checkErr := p.hasLocalRecipient(ctx, note.To)
			if checkErr != nil {
				return checkErr
			}
			if !hasLocal {
				return nil
			}
		}
		createInput := p.buildCreateStatusInput(ctx, &note, author, visibility)
		_, createErr := p.remoteStatusWrites.CreateRemote(ctx, createInput)
		if createErr != nil {
			if errors.Is(createErr, domain.ErrConflict) {
				return nil
			}
			return fmt.Errorf("inbox: create status for Announce: %w", createErr)
		}
	}
	actor, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	// AP Announce -> domain reblog
	_, err = p.remoteStatusWrites.CreateRemoteReblog(ctx, service.CreateRemoteReblogInput{
		AccountID:        actor.ID,
		ActivityAPID:     activity.ID,
		ObjectStatusAPID: objectID,
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create reblog: %w", err)
	}
	// reblog.created event emitted by CreateRemoteReblog; notification subscriber handles notifications.
	return nil
}

// handleLike handles a Like activity.
func (p *inbox) handleLike(ctx context.Context, activity *vocab.Activity) error {
	objectID, ok := activity.ObjectID()
	if !ok {
		return fmt.Errorf("%w: like object is not a status IRI", ErrInboxFatal)
	}
	status, err := p.statuses.GetByAPID(ctx, objectID)
	if err != nil {
		slog.DebugContext(ctx, "inbox: Like of unknown status", slog.String("object", objectID))
		return fmt.Errorf("inbox: GetByAPID (Like): %w", err)
	}
	actor, err := p.remoteResolver.ResolveRemoteAccountByIRI(ctx, activity.Actor)
	if err != nil {
		return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
	}
	var apID *string
	if activity.ID != "" {
		apID = &activity.ID
	}
	// AP Like -> domain favourite
	_, err = p.remoteStatusWrites.CreateRemoteFavourite(ctx, actor.ID, status.ID, apID)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return fmt.Errorf("inbox: create favourite: %w", err)
	}
	// favourite.created event emitted by CreateRemoteFavourite; notification subscriber handles notifications.
	return nil
}

// handleDelete handles a Delete activity.
func (p *inbox) handleDelete(ctx context.Context, activity *vocab.Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case vocab.ObjectTypeTombstone, vocab.ObjectTypeNote, vocab.ObjectTypeQuestion, "":
		objectID, ok := activity.ObjectID()
		if !ok || objectID == "" {
			return nil
		}
		status, err := p.statuses.GetByAPID(ctx, objectID)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Delete): %w", err)
		}
		statusAuthor, err := p.accounts.GetByID(ctx, status.AccountID)
		if err != nil {
			return fmt.Errorf("inbox: GetByID author (Delete): %w", err)
		}
		if statusAuthor.APID != activity.Actor {
			return fmt.Errorf("%w: delete: actor %q is not the author", ErrInboxFatal, activity.Actor)
		}
		// AP Delete{Note/Tombstone} -> domain delete status
		if err := p.remoteStatusWrites.DeleteRemote(ctx, status.ID); err != nil {
			return fmt.Errorf("inbox: DeleteRemote (Delete): %w", err)
		}
		return nil
	case vocab.ObjectTypePerson, vocab.ObjectTypeService:
		account, err := p.accounts.GetByAPID(ctx, activity.Actor)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Delete Person): %w", err)
		}
		if account.IsLocal() {
			return fmt.Errorf("%w: delete: refusing to suspend local account %s", ErrInboxFatal, account.ID)
		}
		if suspendErr := p.accounts.SuspendRemote(ctx, account.ID); suspendErr != nil {
			return fmt.Errorf("inbox: SuspendRemote: %w", suspendErr)
		}
		return nil
	default:
		slog.DebugContext(ctx, "inbox: unsupported Delete object type", slog.String("type", string(objectType)))
		return nil
	}
}

// handleUpdate handles an Update{Note} activity.
func (p *inbox) handleUpdate(ctx context.Context, activity *vocab.Activity) error {
	objectType := activity.ObjectType()
	switch objectType {
	case vocab.ObjectTypeNote, vocab.ObjectTypeQuestion:
		note, err := activity.ObjectNote()
		if err != nil {
			return fmt.Errorf("%w: update{Note}: %w", ErrInboxFatal, err)
		}
		status, err := p.statuses.GetByAPID(ctx, note.ID)
		if err != nil {
			return fmt.Errorf("inbox: GetByAPID (Update Note): %w", err)
		}
		author, err := p.accounts.GetByID(ctx, status.AccountID)
		if err != nil {
			return fmt.Errorf("inbox: GetByID author (Update Note): %w", err)
		}
		if author.APID != activity.Actor {
			return fmt.Errorf("%w: update: actor is not the author", ErrInboxFatal)
		}

		var cw *string
		if note.Summary != nil {
			sanitized := bluemonday.StrictPolicy().Sanitize(*note.Summary)
			cw = &sanitized
		}
		content := remoteContentPolicy().Sanitize(note.Content)
		text := noteSourceText(note, content)
		if updateErr := p.remoteStatusWrites.UpdateRemote(ctx, status.ID, status, service.UpdateRemoteStatusInput{
			Text:           &text,
			Content:        &content,
			ContentWarning: cw,
			Sensitive:      note.Sensitive,
		}); updateErr != nil {
			return fmt.Errorf("inbox: UpdateRemote: %w", updateErr)
		}
		// For Question objects, also update poll vote counts from the authoritative server.
		if pollFields := vocab.NoteToPollFields(note); pollFields != nil {
			if updateErr := p.remoteStatusWrites.UpdateRemotePollVoteCounts(ctx, note.ID, pollFields.VoteCounts); updateErr != nil {
				slog.WarnContext(ctx, "inbox: UpdateRemotePollVoteCounts failed", slog.Any("error", updateErr), slog.String("status_apid", note.ID))
			}
		}
		return nil
	case vocab.ObjectTypePerson, vocab.ObjectTypeService:
		actor, err := activity.ObjectActor()
		if err != nil {
			return fmt.Errorf("%w: Update{Person}: %w", ErrInboxFatal, err)
		}
		if activity.Actor != actor.ID {
			return fmt.Errorf("%w: update: actor %q is not the object being updated", ErrInboxFatal, activity.Actor)
		}
		_, err = p.remoteResolver.SyncActorToStore(ctx, actor)
		return err
	default:
		slog.DebugContext(ctx, "inbox: unsupported Update object type", slog.String("type", string(objectType)))
		return nil
	}
}

func (p *inbox) buildCreateStatusInput(ctx context.Context, note *vocab.Note, author *domain.Account, visibility string) service.CreateRemoteStatusInput {
	return buildCreateStatusInput(ctx, note, author, visibility, p.statuses)
}

func extractTagsFromNote(note *vocab.Note) (hashtagNames, mentionIRIs []string) {
	for _, tag := range note.Tag {
		switch tag.Type { //nolint:exhaustive // only Hashtag and Mention are relevant for Note tags
		case vocab.ObjectTypeHashtag:
			name := strings.TrimPrefix(tag.Name, "#")
			name = strings.ToLower(strings.TrimSpace(name))
			if name != "" {
				hashtagNames = append(hashtagNames, name)
			}
		case vocab.ObjectTypeMention:
			if tag.Href != "" {
				mentionIRIs = append(mentionIRIs, tag.Href)
			}
		}
	}
	return hashtagNames, mentionIRIs
}

// noteSourceText returns the plain-text source from note.Source if available,
// falling back to sanitizedContent (the HTML-rendered version).
func noteSourceText(note *vocab.Note, sanitizedContent string) string {
	if note.Source != nil && note.Source.Content != "" {
		return note.Source.Content
	}
	return sanitizedContent
}

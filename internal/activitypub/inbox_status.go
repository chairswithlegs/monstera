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

// handleCreate handles a Create{Note} activity.
func (p *inbox) handleCreate(ctx context.Context, activity *vocab.Activity, _ string) error {
	note, err := activity.ObjectNote()
	if err != nil {
		return fmt.Errorf("%w: create object is not a note: %w", ErrInboxFatal, err)
	}
	if note.Type != vocab.ObjectTypeNote {
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
		var note vocab.Note
		if fetchErr := p.remoteResolver.resolveIRIDocument(ctx, objectID, &note); fetchErr != nil {
			return fmt.Errorf("inbox: fetch Note for Announce: %w", fetchErr)
		}
		if note.Type != vocab.ObjectTypeNote {
			return fmt.Errorf("%w: announce object is not a Note", ErrInboxFatal)
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
	case vocab.ObjectTypeTombstone, vocab.ObjectTypeNote, "":
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
		if account.Domain == nil {
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
	case vocab.ObjectTypeNote:
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
		content := bluemonday.UGCPolicy().Sanitize(note.Content)
		text := noteSourceText(note, content)
		if updateErr := p.remoteStatusWrites.UpdateRemote(ctx, status.ID, status, service.UpdateRemoteStatusInput{
			Text:           &text,
			Content:        &content,
			ContentWarning: cw,
			Sensitive:      note.Sensitive,
		}); updateErr != nil {
			return fmt.Errorf("inbox: UpdateRemote: %w", updateErr)
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
	var inReplyToID *string
	if note.InReplyTo != nil && *note.InReplyTo != "" {
		if parent, err := p.statuses.GetByAPID(ctx, *note.InReplyTo); err == nil {
			inReplyToID = &parent.ID
		}
	}
	mediaIDs := p.storeRemoteMedia(ctx, note.Attachment, author.ID)
	var contentWarning *string
	if note.Summary != nil && *note.Summary != "" {
		cw := bluemonday.StrictPolicy().Sanitize(*note.Summary)
		contentWarning = &cw
	}

	content := bluemonday.UGCPolicy().Sanitize(note.Content)
	text := noteSourceText(note, content)
	hashtagNames, mentionIRIs := extractTagsFromNote(note)

	fields := vocab.NoteToStatusFields(note)
	return service.CreateRemoteStatusInput{
		AccountID:      author.ID,
		URI:            fields.URI,
		Text:           &text,
		Content:        &content,
		ContentWarning: contentWarning,
		Visibility:     visibility,
		Language:       fields.Language,
		InReplyToID:    inReplyToID,
		MediaIDs:       mediaIDs,
		APID:           fields.APID,
		Sensitive:      fields.Sensitive,
		HashtagNames:   hashtagNames,
		MentionIRIs:    mentionIRIs,
	}
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

func (p *inbox) storeRemoteMedia(ctx context.Context, attachments []vocab.Attachment, accountID string) []string {
	var ids []string
	for _, att := range attachments {
		if att.URL == "" {
			continue
		}
		in := service.CreateRemoteMediaInput{
			AccountID: accountID,
			RemoteURL: att.URL,
			MediaType: att.MediaType,
			Width:     att.Width,
			Height:    att.Height,
		}
		if att.Name != "" {
			in.Description = &att.Name
		}
		if att.Blurhash != "" {
			in.Blurhash = &att.Blurhash
		}
		m, err := p.media.CreateRemote(ctx, in)
		if err != nil {
			continue
		}
		ids = append(ids, m.ID)
	}
	return ids
}

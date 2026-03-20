package activitypub

import (
	"context"

	"github.com/microcosm-cc/bluemonday"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// buildCreateStatusInput builds a CreateRemoteStatusInput from an AP Note.
// Used by both inbox handlers and the backfill worker.
func buildCreateStatusInput(ctx context.Context, note *vocab.Note, author *domain.Account,
	visibility string, statusSvc service.StatusService) service.CreateRemoteStatusInput {

	var inReplyToID *string
	if note.InReplyTo != nil && *note.InReplyTo != "" {
		if parent, err := statusSvc.GetByAPID(ctx, *note.InReplyTo); err == nil {
			inReplyToID = &parent.ID
		}
	}
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
		Attachments:    noteAttachmentsToServiceInput(note.Attachment, author.ID),
		APID:           fields.APID,
		Sensitive:      fields.Sensitive,
		HashtagNames:   hashtagNames,
		MentionIRIs:    mentionIRIs,
		PublishedAt:    fields.PublishedAt,
	}
}

// noteAttachmentsToServiceInput converts AP Note attachments to service input structs.
func noteAttachmentsToServiceInput(attachments []vocab.Attachment, accountID string) []service.CreateRemoteMediaInput {
	var out []service.CreateRemoteMediaInput
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
		out = append(out, in)
	}
	return out
}

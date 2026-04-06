package vocab

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

const (
	testInstanceBase = "https://example.com"
	testRemoteDomain = "remote.example"
)

func TestStatusToNote(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	content := "<p>Hello world</p>"
	cw := "spoiler"

	status := &domain.Status{
		ID:             "01STATUS",
		APID:           "https://example.com/statuses/01STATUS",
		URI:            "https://example.com/statuses/01STATUS",
		AccountID:      "01ACC",
		Visibility:     domain.VisibilityPublic,
		Content:        &content,
		ContentWarning: &cw,
		Sensitive:      true,
		CreatedAt:      now,
	}
	account := &domain.Account{
		ID:           "01ACC",
		Username:     "alice",
		APID:         "https://example.com/users/alice",
		FollowersURL: "https://example.com/users/alice/followers",
	}

	note, err := LocalStatusToNote(LocalStatusToNoteInput{
		Status:       status,
		Author:       account,
		InstanceBase: testInstanceBase,
	})
	require.NoError(t, err)
	require.NotNil(t, note)
	require.Equal(t, "https://example.com/statuses/01STATUS", note.ID)
	require.Equal(t, ObjectTypeNote, note.Type)
	require.Equal(t, "https://example.com/users/alice", note.AttributedTo)
	require.Equal(t, "<p>Hello world</p>", note.Content)
	require.Equal(t, []string{PublicAddress}, note.To)
	require.Equal(t, []string{"https://example.com/users/alice/followers"}, note.Cc)
	require.Equal(t, "2026-02-25T12:00:00Z", note.Published)
	require.Equal(t, "https://example.com/@alice/01STATUS", note.URL)
	require.True(t, note.Sensitive)
	require.NotNil(t, note.Summary)
	require.Equal(t, "spoiler", *note.Summary)
}

func TestLocalStatusToNote_rejects_remote_and_nil(t *testing.T) {
	t.Parallel()

	t.Run("remote author returns ErrLocalAuthorRequired", func(t *testing.T) {
		t.Parallel()
		domainStr := testRemoteDomain
		account := &domain.Account{ID: "01A", Username: "bob", Domain: &domainStr, APID: "https://" + testRemoteDomain + "/users/bob"}
		status := &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: time.Now()}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.Nil(t, note)
		require.ErrorIs(t, err, ErrLocalAuthorRequired)
	})

	t.Run("nil author returns error", func(t *testing.T) {
		t.Parallel()
		status := &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: time.Now()}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: nil, InstanceBase: testInstanceBase})
		require.Nil(t, note)
		require.Error(t, err)
	})

	t.Run("nil status returns error", func(t *testing.T) {
		t.Parallel()
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: nil, Author: account, InstanceBase: testInstanceBase})
		require.Nil(t, note)
		require.Error(t, err)
	})
}

func TestStatusToNote_fallbacks(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("no APID or URI uses instanceBase and status ID", func(t *testing.T) {
		status := &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob", APID: "https://example.com/users/bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		require.Equal(t, "https://example.com/statuses/01X", note.ID)
		require.Equal(t, "https://example.com/@bob/01X", note.URL)
	})

	t.Run("account no APID uses username fallback", func(t *testing.T) {
		status := &domain.Status{ID: "01X", APID: "https://example.com/statuses/01X", AccountID: "01A", CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		require.Equal(t, "https://example.com/users/bob", note.AttributedTo)
	})

	t.Run("Content nil uses Text with sanitization", func(t *testing.T) {
		text := "<script>alert('xss')</script>"
		status := &domain.Status{ID: "01X", AccountID: "01A", Text: &text, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		assert.Empty(t, note.Content, "script tags should be stripped by sanitizer")
	})

	t.Run("Content nil uses Text preserving safe HTML", func(t *testing.T) {
		text := "hello <b>world</b>"
		status := &domain.Status{ID: "01X", AccountID: "01A", Text: &text, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		assert.Equal(t, "hello <b>world</b>", note.Content)
	})

	t.Run("InReplyToID without ParentAPID uses local IRI", func(t *testing.T) {
		parentID := "01PARENT"
		status := &domain.Status{ID: "01X", AccountID: "01A", InReplyToID: &parentID, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		require.NotNil(t, note.InReplyTo)
		require.Equal(t, "https://example.com/statuses/01PARENT", *note.InReplyTo)
	})

	t.Run("InReplyToID with ParentAPID uses remote APID", func(t *testing.T) {
		parentID := "01PARENT"
		status := &domain.Status{ID: "01X", AccountID: "01A", InReplyToID: &parentID, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{
			Status:       status,
			Author:       account,
			InstanceBase: testInstanceBase,
			ParentAPID:   "https://" + testRemoteDomain + "/users/alice/statuses/109",
		})
		require.NoError(t, err)
		require.NotNil(t, note)
		require.NotNil(t, note.InReplyTo)
		require.Equal(t, "https://"+testRemoteDomain+"/users/alice/statuses/109", *note.InReplyTo)
	})

	t.Run("EditedAt populates Updated", func(t *testing.T) {
		editedAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
		status := &domain.Status{ID: "01X", AccountID: "01A", EditedAt: &editedAt, CreatedAt: now}
		account := &domain.Account{ID: "01A", Username: "bob"}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		require.Equal(t, "2026-03-01T10:00:00Z", note.Updated)
	})
}

func TestStatusToNote_contentMap(t *testing.T) {
	t.Parallel()
	now := time.Now()
	account := &domain.Account{ID: "01A", Username: "alice", APID: "https://example.com/users/alice"}

	t.Run("Language set populates ContentMap", func(t *testing.T) {
		t.Parallel()
		content := "<p>Hello</p>"
		lang := "en"
		status := &domain.Status{
			ID:        "01X",
			AccountID: "01A",
			Content:   &content,
			Language:  &lang,
			CreatedAt: now,
		}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		require.NotNil(t, note.ContentMap)
		assert.Equal(t, map[string]string{"en": "<p>Hello</p>"}, note.ContentMap)
	})

	t.Run("Language nil leaves ContentMap nil", func(t *testing.T) {
		t.Parallel()
		content := "<p>Hello</p>"
		status := &domain.Status{
			ID:        "01X",
			AccountID: "01A",
			Content:   &content,
			Language:  nil,
			CreatedAt: now,
		}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{Status: status, Author: account, InstanceBase: testInstanceBase})
		require.NoError(t, err)
		require.NotNil(t, note)
		assert.Nil(t, note.ContentMap)
	})
}

func TestStatusToNote_addressing(t *testing.T) {
	t.Parallel()
	now := time.Now()
	account := &domain.Account{
		ID:           "01A",
		Username:     "alice",
		APID:         "https://example.com/users/alice",
		FollowersURL: "https://example.com/users/alice/followers",
	}
	remoteDomain := testRemoteDomain
	mention := &domain.Account{
		ID:       "02B",
		Username: "bob",
		Domain:   &remoteDomain,
		APID:     "https://" + testRemoteDomain + "/users/bob",
	}

	t.Run("public", func(t *testing.T) {
		t.Parallel()
		status := &domain.Status{ID: "01X", AccountID: "01A", Visibility: domain.VisibilityPublic, CreatedAt: now}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{
			Status: status, Author: account, InstanceBase: testInstanceBase, Mentions: []*domain.Account{mention},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{PublicAddress}, note.To)
		assert.Equal(t, []string{"https://example.com/users/alice/followers", "https://" + testRemoteDomain + "/users/bob"}, note.Cc)
	})

	t.Run("unlisted", func(t *testing.T) {
		t.Parallel()
		status := &domain.Status{ID: "01X", AccountID: "01A", Visibility: domain.VisibilityUnlisted, CreatedAt: now}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{
			Status: status, Author: account, InstanceBase: testInstanceBase, Mentions: []*domain.Account{mention},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"https://example.com/users/alice/followers"}, note.To)
		assert.Equal(t, []string{PublicAddress, "https://" + testRemoteDomain + "/users/bob"}, note.Cc)
	})

	t.Run("private", func(t *testing.T) {
		t.Parallel()
		status := &domain.Status{ID: "01X", AccountID: "01A", Visibility: domain.VisibilityPrivate, CreatedAt: now}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{
			Status: status, Author: account, InstanceBase: testInstanceBase, Mentions: []*domain.Account{mention},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"https://example.com/users/alice/followers"}, note.To)
		assert.Equal(t, []string{"https://" + testRemoteDomain + "/users/bob"}, note.Cc)
	})

	t.Run("direct", func(t *testing.T) {
		t.Parallel()
		status := &domain.Status{ID: "01X", AccountID: "01A", Visibility: domain.VisibilityDirect, CreatedAt: now}
		note, err := LocalStatusToNote(LocalStatusToNoteInput{
			Status: status, Author: account, InstanceBase: testInstanceBase, Mentions: []*domain.Account{mention},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"https://" + testRemoteDomain + "/users/bob"}, note.To)
		assert.Empty(t, note.Cc)
	})
}

func TestStatusToNote_tags_and_attachments(t *testing.T) {
	t.Parallel()
	now := time.Now()
	account := &domain.Account{ID: "01A", Username: "alice", APID: "https://example.com/users/alice"}
	remoteDomain := testRemoteDomain
	mention := &domain.Account{ID: "02B", Username: "bob", Domain: &remoteDomain, APID: "https://" + testRemoteDomain + "/users/bob"}
	desc := "photo of a cat"
	blurhash := "LEHV6nWB2y"

	contentType := "image/png"
	note, err := LocalStatusToNote(LocalStatusToNoteInput{
		Status:       &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: now},
		Author:       account,
		InstanceBase: testInstanceBase,
		Mentions:     []*domain.Account{mention},
		Tags:         []domain.Hashtag{{ID: "t1", Name: "golang"}, {ID: "t2", Name: "fediverse"}},
		Media: []domain.MediaAttachment{
			{ID: "m1", Type: "image", ContentType: &contentType, URL: "https://example.com/media/cat.png", Description: &desc, Blurhash: &blurhash},
		},
	})
	require.NoError(t, err)

	require.Len(t, note.Tag, 3)
	assert.Equal(t, Tag{Type: ObjectTypeHashtag, Href: "https://example.com/tags/golang", Name: "#golang"}, note.Tag[0])
	assert.Equal(t, Tag{Type: ObjectTypeHashtag, Href: "https://example.com/tags/fediverse", Name: "#fediverse"}, note.Tag[1])
	assert.Equal(t, Tag{Type: ObjectTypeMention, Href: "https://" + testRemoteDomain + "/users/bob", Name: "@bob@" + testRemoteDomain}, note.Tag[2])

	require.Len(t, note.Attachment, 1)
	assert.Equal(t, Attachment{Type: ObjectTypeImage, MediaType: "image/png", URL: "https://example.com/media/cat.png", Name: "photo of a cat", Blurhash: "LEHV6nWB2y"}, note.Attachment[0])
}

func TestStatusToNote_attachment_nil_content_type(t *testing.T) {
	t.Parallel()
	now := time.Now()
	account := &domain.Account{ID: "01A", Username: "alice", APID: "https://example.com/users/alice"}
	desc := "old upload"

	note, err := LocalStatusToNote(LocalStatusToNoteInput{
		Status:       &domain.Status{ID: "01X", AccountID: "01A", CreatedAt: now},
		Author:       account,
		InstanceBase: testInstanceBase,
		Media: []domain.MediaAttachment{
			{ID: "m1", Type: "image", ContentType: nil, URL: "https://example.com/media/old.jpg", Description: &desc},
		},
	})
	require.NoError(t, err)

	require.Len(t, note.Attachment, 1)
	assert.Empty(t, note.Attachment[0].MediaType, "nil ContentType should produce empty mediaType")
	assert.Equal(t, "https://example.com/media/old.jpg", note.Attachment[0].URL)
}

func TestNote_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	note := Note{
		Context:      DefaultContext,
		ID:           "https://example.com/statuses/1",
		Type:         ObjectTypeNote,
		AttributedTo: "https://example.com/users/alice",
		Content:      "<p>Hello world</p>",
		To:           []string{PublicAddress},
		Published:    "2026-02-25T12:00:00Z",
		URL:          "https://example.com/statuses/1",
		Sensitive:    false,
	}
	data, err := json.Marshal(note)
	require.NoError(t, err)
	var decoded Note
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, note.ID, decoded.ID)
	assert.Equal(t, note.Content, decoded.Content)
	assert.Equal(t, note.AttributedTo, decoded.AttributedTo)
}

func TestNoteVisibility(t *testing.T) {
	t.Parallel()
	followersURL := "https://example.com/users/alice/followers"

	t.Run("public when PublicAddress in To", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{PublicAddress}, Cc: []string{followersURL}}
		assert.Equal(t, domain.VisibilityPublic, NoteVisibility(note, followersURL))
	})

	t.Run("unlisted when PublicAddress in Cc", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{followersURL}, Cc: []string{PublicAddress}}
		assert.Equal(t, domain.VisibilityUnlisted, NoteVisibility(note, followersURL))
	})

	t.Run("private when followersURL in To", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{followersURL}, Cc: []string{}}
		assert.Equal(t, domain.VisibilityPrivate, NoteVisibility(note, followersURL))
	})

	t.Run("direct when no match", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{"https://example.com/users/bob"}, Cc: []string{}}
		assert.Equal(t, domain.VisibilityDirect, NoteVisibility(note, followersURL))
	})

	t.Run("public when as:Public in To", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{"as:Public"}, Cc: []string{followersURL}}
		assert.Equal(t, domain.VisibilityPublic, NoteVisibility(note, followersURL))
	})

	t.Run("public when Public in To", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{"Public"}, Cc: []string{followersURL}}
		assert.Equal(t, domain.VisibilityPublic, NoteVisibility(note, followersURL))
	})

	t.Run("unlisted when as:Public in Cc", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{followersURL}, Cc: []string{"as:Public"}}
		assert.Equal(t, domain.VisibilityUnlisted, NoteVisibility(note, followersURL))
	})

	t.Run("unlisted when Public in Cc", func(t *testing.T) {
		t.Parallel()
		note := &Note{To: []string{followersURL}, Cc: []string{"Public"}}
		assert.Equal(t, domain.VisibilityUnlisted, NoteVisibility(note, followersURL))
	})
}

func TestNoteLanguage(t *testing.T) {
	t.Parallel()

	t.Run("returns language from ContentMap", func(t *testing.T) {
		t.Parallel()
		note := &Note{ContentMap: map[string]string{"en": "<p>Hello</p>"}}
		lang := NoteLanguage(note)
		require.NotNil(t, lang)
		assert.Equal(t, "en", *lang)
	})

	t.Run("returns nil for empty ContentMap", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, NoteLanguage(&Note{}))
	})
}

func TestBuildAttachments_ObjectTypes(t *testing.T) {
	t.Parallel()

	mimeJPEG := "image/jpeg"
	mimeMP4 := "video/mp4"
	mimeMP3 := "audio/mpeg"
	mimeGIF := "image/gif"
	desc := "alt text"
	blurhash := "LGF5]+Yk^6#M@-5c,1J5@[or[Q6."

	cases := []struct {
		name        string
		mediaType   string
		contentType *string
		wantObjType ObjectType
		wantMIME    string
	}{
		{"image", domain.MediaTypeImage, &mimeJPEG, ObjectTypeImage, "image/jpeg"},
		{"gifv", domain.MediaTypeGifv, &mimeGIF, ObjectTypeImage, "image/gif"},
		{"video", domain.MediaTypeVideo, &mimeMP4, ObjectTypeVideo, "video/mp4"},
		{"audio", domain.MediaTypeAudio, &mimeMP3, ObjectTypeAudio, "audio/mpeg"},
		{"unknown falls back to Document", "unknown", nil, ObjectTypeDocument, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			media := []domain.MediaAttachment{
				{
					Type:        tc.mediaType,
					ContentType: tc.contentType,
					URL:         "https://example.com/media/file",
					Description: &desc,
					Blurhash:    &blurhash,
				},
			}
			attachments := buildAttachments(media)
			require.Len(t, attachments, 1)
			assert.Equal(t, tc.wantObjType, attachments[0].Type)
			assert.Equal(t, tc.wantMIME, attachments[0].MediaType)
			assert.Equal(t, "https://example.com/media/file", attachments[0].URL)
			assert.Equal(t, desc, attachments[0].Name)
			assert.Equal(t, blurhash, attachments[0].Blurhash)
		})
	}
}

func TestBuildAttachments_Empty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, buildAttachments(nil))
	assert.Nil(t, buildAttachments([]domain.MediaAttachment{}))
}

// TestNote_RepliesPolymorphic verifies that a Note with replies.first as an
// inline object (as sent by some servers, e.g. theforkiverse.com) unmarshals
// without error instead of failing with "cannot unmarshal object into string".
func TestNote_RepliesPolymorphic(t *testing.T) {
	t.Parallel()

	raw := `{
		"id": "https://remote.example/statuses/1",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "<p>hello</p>",
		"to": ["https://www.w3.org/ns/activitystreams#Public"],
		"published": "2026-03-18T12:00:00Z",
		"url": "https://remote.example/@alice/1",
		"sensitive": false,
		"summary": null,
		"inReplyTo": null,
		"replies": {
			"id": "https://remote.example/statuses/1/replies",
			"type": "Collection",
			"first": {
				"type": "CollectionPage",
				"next": "https://remote.example/statuses/1/replies?only_other_accounts=true&page=true",
				"partOf": "https://remote.example/statuses/1/replies",
				"items": []
			}
		}
	}`

	var note Note
	err := json.Unmarshal([]byte(raw), &note)
	require.NoError(t, err)
	assert.Equal(t, "https://remote.example/statuses/1", note.ID)
	assert.Equal(t, "<p>hello</p>", note.Content)
}

func TestNoteToStatusFields(t *testing.T) {
	t.Parallel()

	t.Run("standard note", func(t *testing.T) {
		t.Parallel()
		note := &Note{
			ID:        "https://example.com/statuses/01S",
			Sensitive: true,
		}
		fields := NoteToStatusFields(note)
		assert.Equal(t, "https://example.com/statuses/01S", fields.URI)
		assert.Equal(t, "https://example.com/statuses/01S", fields.APID)
		assert.True(t, fields.Sensitive)
		assert.Nil(t, fields.Language)
	})

	t.Run("with ContentMap sets Language", func(t *testing.T) {
		t.Parallel()
		note := &Note{
			ID:         "https://example.com/statuses/01S",
			ContentMap: map[string]string{"fr": "<p>Bonjour</p>"},
		}
		fields := NoteToStatusFields(note)
		require.NotNil(t, fields.Language)
		assert.Equal(t, "fr", *fields.Language)
	})
}

// --- Poll tests ---

func TestNoteToPollFields_SingleChoice(t *testing.T) {
	t.Parallel()
	note := &Note{
		Type: ObjectTypeQuestion,
		OneOf: []PollOptionEntry{
			{Type: ObjectTypeNote, Name: "Option A", Replies: &PollOptionReplies{Type: "Collection", TotalItems: 5}},
			{Type: ObjectTypeNote, Name: "Option B", Replies: &PollOptionReplies{Type: "Collection", TotalItems: 3}},
		},
		EndTime: "2026-04-10T12:00:00Z",
	}
	fields := NoteToPollFields(note)
	require.NotNil(t, fields)
	assert.False(t, fields.Multiple)
	assert.Equal(t, []string{"Option A", "Option B"}, fields.Options)
	assert.Equal(t, []int{5, 3}, fields.VoteCounts)
	require.NotNil(t, fields.ExpiresAt)
	assert.Equal(t, 2026, fields.ExpiresAt.Year())
}

func TestNoteToPollFields_MultipleChoice(t *testing.T) {
	t.Parallel()
	note := &Note{
		Type: ObjectTypeQuestion,
		AnyOf: []PollOptionEntry{
			{Type: ObjectTypeNote, Name: "Red", Replies: &PollOptionReplies{Type: "Collection", TotalItems: 10}},
			{Type: ObjectTypeNote, Name: "Blue"},
		},
	}
	fields := NoteToPollFields(note)
	require.NotNil(t, fields)
	assert.True(t, fields.Multiple)
	assert.Equal(t, []string{"Red", "Blue"}, fields.Options)
	assert.Equal(t, []int{10, 0}, fields.VoteCounts)
	assert.Nil(t, fields.ExpiresAt)
}

func TestNoteToPollFields_NotAPoll(t *testing.T) {
	t.Parallel()
	note := &Note{Type: ObjectTypeNote, Content: "just a note"}
	assert.Nil(t, NoteToPollFields(note))
}

func TestNoteToPollFields_NoEndTime(t *testing.T) {
	t.Parallel()
	note := &Note{
		Type:  ObjectTypeQuestion,
		OneOf: []PollOptionEntry{{Type: ObjectTypeNote, Name: "Yes"}, {Type: ObjectTypeNote, Name: "No"}},
	}
	fields := NoteToPollFields(note)
	require.NotNil(t, fields)
	assert.Nil(t, fields.ExpiresAt)
}

func TestLocalStatusToNote_WithPoll(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	expires := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	content := "<p>What do you think?</p>"
	status := &domain.Status{ID: "01POLL", AccountID: "01ACC", Content: &content, Visibility: domain.VisibilityPublic, CreatedAt: now}
	account := &domain.Account{ID: "01ACC", Username: "alice", FollowersURL: "https://example.com/users/alice/followers"}
	votersCount := 8

	note, err := LocalStatusToNote(LocalStatusToNoteInput{
		Status:       status,
		Author:       account,
		InstanceBase: testInstanceBase,
		Poll: &LocalPollData{
			Multiple:    false,
			ExpiresAt:   &expires,
			VotersCount: votersCount,
			Options: []LocalPollOptionData{
				{Title: "Option A", VotesCount: 5},
				{Title: "Option B", VotesCount: 3},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ObjectTypeQuestion, note.Type)
	require.Len(t, note.OneOf, 2)
	assert.Empty(t, note.AnyOf)
	assert.Equal(t, "Option A", note.OneOf[0].Name)
	assert.Equal(t, 5, note.OneOf[0].Replies.TotalItems)
	assert.Equal(t, "Option B", note.OneOf[1].Name)
	assert.Equal(t, 3, note.OneOf[1].Replies.TotalItems)
	assert.Equal(t, "2026-04-07T12:00:00Z", note.EndTime)
	require.NotNil(t, note.VotersCount)
	assert.Equal(t, 8, *note.VotersCount)
}

func TestLocalStatusToNote_WithMultiplePoll(t *testing.T) {
	t.Parallel()
	now := time.Now()
	content := "<p>Pick all that apply</p>"
	status := &domain.Status{ID: "01POLL2", AccountID: "01ACC", Content: &content, Visibility: domain.VisibilityPublic, CreatedAt: now}
	account := &domain.Account{ID: "01ACC", Username: "alice"}

	note, err := LocalStatusToNote(LocalStatusToNoteInput{
		Status:       status,
		Author:       account,
		InstanceBase: testInstanceBase,
		Poll: &LocalPollData{
			Multiple: true,
			Options: []LocalPollOptionData{
				{Title: "Red", VotesCount: 1},
				{Title: "Blue", VotesCount: 2},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ObjectTypeQuestion, note.Type)
	assert.Empty(t, note.OneOf)
	require.Len(t, note.AnyOf, 2)
	assert.Equal(t, "Red", note.AnyOf[0].Name)
	assert.Equal(t, "Blue", note.AnyOf[1].Name)
}

func TestLocalStatusToNote_NoPoll(t *testing.T) {
	t.Parallel()
	now := time.Now()
	content := "<p>No poll here</p>"
	status := &domain.Status{ID: "01NOPOLL", AccountID: "01ACC", Content: &content, Visibility: domain.VisibilityPublic, CreatedAt: now}
	account := &domain.Account{ID: "01ACC", Username: "alice"}

	note, err := LocalStatusToNote(LocalStatusToNoteInput{
		Status:       status,
		Author:       account,
		InstanceBase: testInstanceBase,
	})
	require.NoError(t, err)
	assert.Equal(t, ObjectTypeNote, note.Type)
	assert.Empty(t, note.OneOf)
	assert.Empty(t, note.AnyOf)
	assert.Empty(t, note.EndTime)
	assert.Nil(t, note.VotersCount)
}

func TestQuestion_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	votersCount := 12
	note := Note{
		Context:      DefaultContext,
		ID:           "https://example.com/statuses/poll1",
		Type:         ObjectTypeQuestion,
		AttributedTo: "https://example.com/users/alice",
		Content:      "<p>Favorite color?</p>",
		To:           []string{PublicAddress},
		Published:    "2026-04-06T12:00:00Z",
		OneOf: []PollOptionEntry{
			{Type: ObjectTypeNote, Name: "Red", Replies: &PollOptionReplies{Type: "Collection", TotalItems: 7}},
			{Type: ObjectTypeNote, Name: "Blue", Replies: &PollOptionReplies{Type: "Collection", TotalItems: 5}},
		},
		EndTime:     "2026-04-07T12:00:00Z",
		VotersCount: &votersCount,
	}
	data, err := json.Marshal(note)
	require.NoError(t, err)

	var decoded Note
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, ObjectTypeQuestion, decoded.Type)
	require.Len(t, decoded.OneOf, 2)
	assert.Equal(t, "Red", decoded.OneOf[0].Name)
	assert.Equal(t, 7, decoded.OneOf[0].Replies.TotalItems)
	assert.Equal(t, "2026-04-07T12:00:00Z", decoded.EndTime)
	require.NotNil(t, decoded.VotersCount)
	assert.Equal(t, 12, *decoded.VotersCount)
}

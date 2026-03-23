package activitypub

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/blocklist"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/media"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const (
	testAliceAPID = "https://example.com/users/alice"
	testBobAPID   = "https://bob.com/users/bob"
)

func TestInboxProcessor_Process_unsupportedType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	activity := &vocab.Activity{Type: "Unknown", ID: "https://remote.example/activities/1", Actor: "https://remote.example/users/alice"}
	err := proc.Process(ctx, activity)
	assert.NoError(t, err)
}

func TestInboxProcessor_Process_emptyActorDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	activity := &vocab.Activity{Type: "Follow", Actor: "not-a-url"}
	err := proc.Process(ctx, activity)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInboxProcessor_Process_ownDomain_returnsErrInboxFatal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	objectRaw, err := json.Marshal("https://remote.example/users/bob")
	require.NoError(t, err)
	activity := &vocab.Activity{Type: "Follow", ID: "https://example.com/activities/1", Actor: "https://example.com/users/alice", ObjectRaw: objectRaw}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInbox_Process_AcceptFollow_forgedActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)

	aliceID := uid.New()
	bobID := uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: uid.New(), AccountID: bobID, TargetID: aliceID, State: domain.FollowStatePending, APID: nil,
	})
	require.NoError(t, err)

	innerFollow := map[string]string{"type": "Follow", "actor": testBobAPID, "object": testAliceAPID}
	objectRaw, err := json.Marshal(innerFollow)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Accept",
		ID:        "https://evil.com/accept/1",
		Actor:     "https://evil.com/users/attacker",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInbox_Process_RejectFollow_forgedActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)

	aliceID := uid.New()
	bobID := uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: uid.New(), AccountID: bobID, TargetID: aliceID, State: domain.FollowStatePending, APID: nil,
	})
	require.NoError(t, err)

	innerFollow := map[string]string{"type": "Follow", "actor": testBobAPID, "object": testAliceAPID}
	objectRaw, err := json.Marshal(innerFollow)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Reject",
		ID:        "https://evil.com/reject/1",
		Actor:     "https://evil.com/users/attacker",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInbox_Process_UpdatePerson_forgedActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)

	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: uid.New(), Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)

	objectActor := map[string]string{"id": testBobAPID, "type": string(vocab.ObjectTypePerson), "preferredUsername": "bob"}
	objectRaw, err := json.Marshal(objectActor)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Update",
		ID:        "https://evil.com/update/1",
		Actor:     "https://evil.com/users/attacker",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInbox_Process_Delete_statusWrongActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)

	aliceID := uid.New()
	bobID := uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	statusAPID := "https://example.com/statuses/1"
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: aliceID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: true,
	})
	require.NoError(t, err)
	require.NotNil(t, st)

	objectRaw, err := json.Marshal(map[string]string{"id": statusAPID, "type": "Note"})
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Delete",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInbox_Process_Delete_Person_suspendsRemoteAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	_, bobID := seedInboxAccounts(t, ctx, fake, false)

	objectRaw, err := json.Marshal(map[string]string{"id": testBobAPID, "type": "Person"})
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Delete",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)

	acc, err := fake.GetAccountByID(ctx, bobID)
	require.NoError(t, err)
	assert.True(t, acc.Suspended, "remote account should be suspended after Delete{Person}")
}

func TestInbox_Process_Delete_authorLookupFailsReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)

	statusAPID := "https://example.com/statuses/orphan"
	orphanAccountID := "01orphan"
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: orphanAccountID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: false,
	})
	require.NoError(t, err)
	require.NotNil(t, st)

	objectRaw, err := json.Marshal(map[string]string{"id": statusAPID, "type": "Note"})
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Delete",
		Actor:     "https://example.com/users/someone",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
}

// seedInboxAccounts creates local alice and remote bob; alice can be locked to avoid outbox.SendAcceptFollow.
func seedInboxAccounts(t *testing.T, ctx context.Context, fake *testutil.FakeStore, aliceLocked bool) (aliceID, bobID string) {
	t.Helper()
	aliceID = uid.New()
	bobID = uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil, Locked: aliceLocked,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	return aliceID, bobID
}

func TestInbox_Process_Follow_happyPath_createsFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, bobID := seedInboxAccounts(t, ctx, fake, true) // locked so no outbox call

	activity := &vocab.Activity{
		Type:      "Follow",
		ID:        "https://bob.com/activities/follow-1",
		Actor:     testBobAPID,
		ObjectRaw: json.RawMessage(`"` + testAliceAPID + `"`),
	}
	err := proc.Process(ctx, activity)
	require.NoError(t, err)
	follow, err := fake.GetFollow(ctx, bobID, aliceID)
	require.NoError(t, err)
	require.NotNil(t, follow)
	require.Equal(t, domain.FollowStatePending, follow.State)
}

func TestInbox_Process_AcceptFollow_happyPath_updatesFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, bobID := seedInboxAccounts(t, ctx, fake, false)
	followID := uid.New()
	_, err := fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: followID, AccountID: aliceID, TargetID: bobID, State: domain.FollowStatePending, APID: nil,
	})
	require.NoError(t, err)

	innerFollow := map[string]string{"type": "Follow", "actor": testAliceAPID, "object": testBobAPID}
	objectRaw, err := json.Marshal(innerFollow)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Accept",
		ID:        "https://bob.com/accept/1",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)
	follow, err := fake.GetFollow(ctx, aliceID, bobID)
	require.NoError(t, err)
	require.NotNil(t, follow)
	require.Equal(t, domain.FollowStateAccepted, follow.State)
}

func TestInbox_Process_RejectFollow_happyPath_removesFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, bobID := seedInboxAccounts(t, ctx, fake, false)
	_, err := fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: uid.New(), AccountID: aliceID, TargetID: bobID, State: domain.FollowStatePending, APID: nil,
	})
	require.NoError(t, err)

	innerFollow := map[string]string{"type": "Follow", "actor": testAliceAPID, "object": testBobAPID}
	objectRaw, err := json.Marshal(innerFollow)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Reject",
		ID:        "https://bob.com/reject/1",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)
	_, err = fake.GetFollow(ctx, aliceID, bobID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestInbox_Process_suspendedDomain_dropsActivity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID: uid.New(), Domain: "bob.com", Severity: domain.DomainBlockSeveritySuspend, Reason: nil,
	})
	require.NoError(t, err)
	proc := newInboxProcessorForTest(t, fake)
	aliceID, bobID := seedInboxAccounts(t, ctx, fake, true)

	activity := &vocab.Activity{
		Type:      "Follow",
		ID:        "https://bob.com/activities/follow-1",
		Actor:     testBobAPID,
		ObjectRaw: json.RawMessage(`"` + testAliceAPID + `"`),
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)
	// Follow should not be created because actor's domain is suspended
	_, err = fake.GetFollow(ctx, bobID, aliceID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestInbox_Process_Create_happyPath_createsStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	_, bobID := seedInboxAccounts(t, ctx, fake, false)

	note := map[string]any{
		"type":         "Note",
		"id":           "https://bob.com/notes/1",
		"content":      "<p>Hello from bob</p>",
		"attributedTo": testBobAPID,
		"to":           []string{vocab.PublicAddress},
		"published":    "2026-02-25T12:00:00Z",
		"url":          "https://bob.com/notes/1",
	}
	objectRaw, err := json.Marshal(note)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Create",
		ID:        "https://bob.com/activities/create-1",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)
	st, err := fake.GetStatusByAPID(ctx, "https://bob.com/notes/1")
	require.NoError(t, err)
	require.NotNil(t, st)
	require.Equal(t, bobID, st.AccountID)
}

func TestInbox_Process_Announce_happyPath_createsBoost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, _ := seedInboxAccounts(t, ctx, fake, false)
	statusAPID := "https://example.com/statuses/boost-target"
	_, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: aliceID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: true,
	})
	require.NoError(t, err)

	activity := &vocab.Activity{
		Type:      "Announce",
		ID:        "https://bob.com/activities/announce-1",
		Actor:     testBobAPID,
		ObjectRaw: json.RawMessage(`"` + statusAPID + `"`),
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)
}

func TestInbox_Process_Like_happyPath_createsFavourite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, bobID := seedInboxAccounts(t, ctx, fake, false)
	statusAPID := "https://example.com/statuses/1"
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: aliceID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: true,
	})
	require.NoError(t, err)

	activity := &vocab.Activity{
		Type:      "Like",
		ID:        "https://bob.com/activities/like-1",
		Actor:     testBobAPID,
		ObjectRaw: json.RawMessage(`"` + statusAPID + `"`),
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)
	fav, err := fake.GetFavouriteByAPID(ctx, "https://bob.com/activities/like-1")
	require.NoError(t, err)
	require.NotNil(t, fav)
	require.Equal(t, bobID, fav.AccountID)
	require.Equal(t, st.ID, fav.StatusID)
}

func TestInbox_Process_Delete_owner_deletesStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, _ := seedInboxAccounts(t, ctx, fake, false)
	statusAPID := "https://example.com/statuses/to-delete"
	_, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: aliceID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: true,
	})
	require.NoError(t, err)

	objectRaw, err := json.Marshal(map[string]string{"id": statusAPID, "type": "Note"})
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Delete",
		Actor:     testAliceAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInboxFatal)
}

func TestInbox_Process_Create_sanitizesContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	_, _ = seedInboxAccounts(t, ctx, fake, false)

	note := map[string]any{
		"type":         "Note",
		"id":           "https://bob.com/notes/xss-1",
		"content":      `<p>Hello <script>alert('xss')</script> world</p>`,
		"summary":      `cw <script>evil()</script>`,
		"attributedTo": testBobAPID,
		"to":           []string{vocab.PublicAddress},
	}
	objectRaw, err := json.Marshal(note)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Create",
		ID:        "https://bob.com/activities/create-xss-1",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)

	st, err := fake.GetStatusByAPID(ctx, "https://bob.com/notes/xss-1")
	require.NoError(t, err)
	require.NotNil(t, st.Content)
	assert.NotContains(t, *st.Content, "<script>", "dangerous script tag must be stripped from content")
	assert.Contains(t, *st.Content, "<p>", "safe <p> tag must be preserved")
	require.NotNil(t, st.ContentWarning)
	assert.NotContains(t, *st.ContentWarning, "<script>", "dangerous script tag must be stripped from content warning")
	assert.Equal(t, "cw ", *st.ContentWarning)
}

func TestInbox_Process_Create_preservesSafeHTML(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	_, _ = seedInboxAccounts(t, ctx, fake, false)

	note := map[string]any{
		"type":         "Note",
		"id":           "https://bob.com/notes/safe-1",
		"content":      `<p>Check out <a href="https://example.com">this link</a> and <strong>bold text</strong></p>`,
		"attributedTo": testBobAPID,
		"to":           []string{vocab.PublicAddress},
	}
	objectRaw, err := json.Marshal(note)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Create",
		ID:        "https://bob.com/activities/create-safe-1",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)

	st, err := fake.GetStatusByAPID(ctx, "https://bob.com/notes/safe-1")
	require.NoError(t, err)
	require.NotNil(t, st.Content)
	assert.Contains(t, *st.Content, "<p>", "paragraph tags must be preserved")
	assert.Contains(t, *st.Content, "<strong>", "strong tags must be preserved")
	assert.Contains(t, *st.Content, "<a href=", "anchor tags must be preserved")
}

func TestInbox_Process_UpdateNote_sanitizesContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	_, bobID := seedInboxAccounts(t, ctx, fake, false)

	statusAPID := "https://bob.com/notes/update-target"
	_, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: bobID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: false,
	})
	require.NoError(t, err)

	updatedNote := map[string]any{
		"type":         "Note",
		"id":           statusAPID,
		"content":      `<p>Updated <script>alert(1)</script> content</p>`,
		"summary":      `cw <iframe src="evil"/>`,
		"attributedTo": testBobAPID,
		"to":           []string{vocab.PublicAddress},
	}
	objectRaw, err := json.Marshal(updatedNote)
	require.NoError(t, err)
	activity := &vocab.Activity{
		Type:      "Update",
		ID:        "https://bob.com/activities/update-1",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.NoError(t, err)

	st, err := fake.GetStatusByAPID(ctx, statusAPID)
	require.NoError(t, err)
	require.NotNil(t, st.Content)
	assert.NotContains(t, *st.Content, "<script>", "script tag must be stripped on update")
	assert.Contains(t, *st.Content, "<p>", "safe tags must be preserved on update")
	require.NotNil(t, st.ContentWarning)
	assert.NotContains(t, *st.ContentWarning, "<iframe>", "iframe tag must be stripped from content warning")
}

func TestInbox_Process_Block_happyPath_createsBlock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake)
	aliceID, bobID := seedInboxAccounts(t, ctx, fake, false)

	activity := &vocab.Activity{
		Type:      "Block",
		ID:        "https://bob.com/activities/block-1",
		Actor:     testBobAPID,
		ObjectRaw: json.RawMessage(`"` + testAliceAPID + `"`),
	}
	err := proc.Process(ctx, activity)
	require.NoError(t, err)
	blocked, err := fake.IsBlockedEitherDirection(ctx, bobID, aliceID)
	require.NoError(t, err)
	assert.True(t, blocked)
}

type testMediaStore struct{}

func (testMediaStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	return nil
}
func (testMediaStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, media.ErrNotFound
}
func (testMediaStore) Delete(ctx context.Context, key string) error { return nil }
func (testMediaStore) URL(ctx context.Context, key string) (string, error) {
	return "https://example.com/" + key, nil
}

func newInboxProcessorForTest(t *testing.T, fake *testutil.FakeStore) Inbox {
	t.Helper()
	const instanceDomain = "example.com"
	bl := blocklist.NewBlocklistCache(fake)
	_ = bl.Refresh(context.Background())
	instanceBaseURL := "https://" + instanceDomain
	accountSvc := service.NewAccountService(fake, instanceBaseURL)
	remoteFollowSvc := service.NewRemoteFollowService(fake)
	followSvc := service.NewFollowService(fake, service.NewAccountService(fake, "https://example.com"), remoteFollowSvc)
	statusSvc := service.NewStatusService(fake, instanceBaseURL, instanceDomain, 5000)
	conversationSvc := service.NewConversationService(fake, statusSvc)
	mediaSvc := service.NewMediaService(fake, &testMediaStore{}, 1<<20)
	remoteStatusWriteSvc := service.NewRemoteStatusWriteService(fake, conversationSvc, mediaSvc, instanceBaseURL)
	remoteResolver := NewRemoteAccountResolver(accountSvc, "", false, instanceDomain)
	return NewInbox(accountSvc, followSvc, remoteFollowSvc, statusSvc, remoteStatusWriteSvc, mediaSvc, remoteResolver, bl, instanceDomain)
}

package testutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func strPtr(s string) *string { return &s }

func makeAccountInput(username string) store.CreateAccountInput {
	id := uid.New()
	return store.CreateAccountInput{
		ID:           id,
		Username:     username,
		PublicKey:    "pk-" + id,
		InboxURL:     "https://example.com/" + username + "/inbox",
		OutboxURL:    "https://example.com/" + username + "/outbox",
		FollowersURL: "https://example.com/" + username + "/followers",
		FollowingURL: "https://example.com/" + username + "/following",
		APID:         "https://example.com/users/" + username,
		DisplayName:  strPtr(username),
	}
}

func makeRemoteAccountInput(username, accountDomain string) store.CreateAccountInput {
	in := makeAccountInput(username)
	in.Domain = &accountDomain
	in.APID = "https://" + accountDomain + "/users/" + username
	in.InboxURL = "https://" + accountDomain + "/" + username + "/inbox"
	return in
}

func makeStatusInput(accountID string, vis string, local bool) store.CreateStatusInput {
	id := uid.New()
	text := "hello world"
	content := "<p>hello world</p>"
	return store.CreateStatusInput{
		ID:         id,
		URI:        "https://example.com/statuses/" + id,
		AccountID:  accountID,
		Text:       &text,
		Content:    &content,
		Visibility: vis,
		APID:       "https://example.com/statuses/" + id,
		Local:      local,
	}
}

// ---------- Account Operations ----------

func TestAccountCreateAndGet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeAccountInput("alice")
	acc, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, in.ID, acc.ID)
	assert.Equal(t, "alice", acc.Username)
	assert.Equal(t, in.APID, acc.APID)
	assert.Equal(t, *in.DisplayName, *acc.DisplayName)

	got, err := f.GetAccountByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, got.ID)
	assert.Equal(t, acc.Username, got.Username)
}

func TestAccountGetByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	_, err := f.GetAccountByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountCreateDuplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeAccountInput("alice")
	_, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)

	in2 := makeAccountInput("alice")
	_, err = f.CreateAccount(ctx, in2)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestAccountGetByAPID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeAccountInput("bob")
	acc, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)

	got, err := f.GetAccountByAPID(ctx, acc.APID)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, got.ID)

	_, err = f.GetAccountByAPID(ctx, "https://nope.example/nope")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountGetLocalByUsername(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeAccountInput("carol")
	_, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)

	got, err := f.GetLocalAccountByUsername(ctx, "carol")
	require.NoError(t, err)
	assert.Equal(t, "carol", got.Username)

	_, err = f.GetLocalAccountByUsername(ctx, "nobody")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountGetRemoteByUsername(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeRemoteAccountInput("dan", "remote.example")
	_, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)

	d := "remote.example"
	got, err := f.GetRemoteAccountByUsername(ctx, "dan", &d)
	require.NoError(t, err)
	assert.Equal(t, "dan", got.Username)

	_, err = f.GetRemoteAccountByUsername(ctx, "nobody", &d)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountGetsByIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in1 := makeAccountInput("u1")
	a1, err := f.CreateAccount(ctx, in1)
	require.NoError(t, err)

	in2 := makeAccountInput("u2")
	a2, err := f.CreateAccount(ctx, in2)
	require.NoError(t, err)

	accounts, err := f.GetAccountsByIDs(ctx, []string{a1.ID, a2.ID, "nonexistent"})
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
}

func TestAccountSearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	_, err := f.CreateAccount(ctx, makeAccountInput("alpha"))
	require.NoError(t, err)
	_, err = f.CreateAccount(ctx, makeAccountInput("alphabeta"))
	require.NoError(t, err)
	_, err = f.CreateAccount(ctx, makeAccountInput("beta"))
	require.NoError(t, err)

	results, err := f.SearchAccounts(ctx, "alpha", 10, 0)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAccountUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeAccountInput("eve")
	acc, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)

	newName := "Eve Updated"
	err = f.UpdateAccount(ctx, store.UpdateAccountInput{
		ID:          acc.ID,
		DisplayName: &newName,
	})
	require.NoError(t, err)

	got, err := f.GetAccountByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Eve Updated", *got.DisplayName)
}

func TestAccountUpdate_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	err := f.UpdateAccount(ctx, store.UpdateAccountInput{ID: "nonexistent"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAccountSuspend(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	in := makeAccountInput("frank")
	acc, err := f.CreateAccount(ctx, in)
	require.NoError(t, err)

	err = f.SuspendAccount(ctx, acc.ID)
	require.NoError(t, err)

	got, err := f.GetAccountByID(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, got.Suspended)
}

func TestCountLocalAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	_, err := f.CreateAccount(ctx, makeAccountInput("local1"))
	require.NoError(t, err)
	_, err = f.CreateAccount(ctx, makeAccountInput("local2"))
	require.NoError(t, err)
	_, err = f.CreateAccount(ctx, makeRemoteAccountInput("remote1", "r.example"))
	require.NoError(t, err)

	count, err := f.CountLocalAccounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// ---------- Status Operations ----------

func TestStatusCreateAndGet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("poster"))
	require.NoError(t, err)

	sIn := makeStatusInput(acc.ID, domain.VisibilityPublic, true)
	status, err := f.CreateStatus(ctx, sIn)
	require.NoError(t, err)
	assert.Equal(t, sIn.ID, status.ID)
	assert.Equal(t, acc.ID, status.AccountID)

	got, err := f.GetStatusByID(ctx, status.ID)
	require.NoError(t, err)
	assert.Equal(t, status.ID, got.ID)
	assert.Equal(t, *sIn.Text, *got.Text)
}

func TestStatusGetByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	_, err := f.GetStatusByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusGetByAPID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("poster2"))
	require.NoError(t, err)

	sIn := makeStatusInput(acc.ID, domain.VisibilityPublic, true)
	status, err := f.CreateStatus(ctx, sIn)
	require.NoError(t, err)

	got, err := f.GetStatusByAPID(ctx, status.APID)
	require.NoError(t, err)
	assert.Equal(t, status.ID, got.ID)

	_, err = f.GetStatusByAPID(ctx, "https://nope.example/nope")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatusSoftDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("deleter"))
	require.NoError(t, err)

	sIn := makeStatusInput(acc.ID, domain.VisibilityPublic, true)
	status, err := f.CreateStatus(ctx, sIn)
	require.NoError(t, err)

	err = f.SoftDeleteStatus(ctx, status.ID)
	require.NoError(t, err)

	_, err = f.GetStatusByID(ctx, status.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetAccountStatuses_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("pager"))
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
	}

	page1, err := f.GetAccountStatuses(ctx, acc.ID, nil, 3)
	require.NoError(t, err)
	assert.Len(t, page1, 3)

	cursor := page1[len(page1)-1].ID
	page2, err := f.GetAccountStatuses(ctx, acc.ID, &cursor, 3)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	for _, s := range page2 {
		assert.Less(t, s.ID, cursor)
	}
}

func TestGetPublicTimeline_LocalOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("tl"))
	require.NoError(t, err)

	_, err = f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)

	remoteIn := makeStatusInput(acc.ID, domain.VisibilityPublic, false)
	_, err = f.CreateStatus(ctx, remoteIn)
	require.NoError(t, err)

	all, err := f.GetPublicTimeline(ctx, false, nil, 10)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	localOnly, err := f.GetPublicTimeline(ctx, true, nil, 10)
	require.NoError(t, err)
	assert.Len(t, localOnly, 1)
	assert.True(t, localOnly[0].Local)
}

func TestStatusesCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	accID := uid.New()
	err := f.IncrementStatusesCount(ctx, accID)
	require.NoError(t, err)
	err = f.IncrementStatusesCount(ctx, accID)
	require.NoError(t, err)
	err = f.DecrementStatusesCount(ctx, accID)
	require.NoError(t, err)

	f.mu.Lock()
	count := f.statusesCount[accID]
	f.mu.Unlock()
	assert.Equal(t, 1, count)
}

// ---------- Follow Operations ----------

func TestFollowLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	a1, err := f.CreateAccount(ctx, makeAccountInput("follower"))
	require.NoError(t, err)
	a2, err := f.CreateAccount(ctx, makeAccountInput("followee"))
	require.NoError(t, err)

	followID := uid.New()
	follow, err := f.CreateFollow(ctx, store.CreateFollowInput{
		ID:        followID,
		AccountID: a1.ID,
		TargetID:  a2.ID,
		State:     domain.FollowStatePending,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.FollowStatePending, follow.State)

	got, err := f.GetFollow(ctx, a1.ID, a2.ID)
	require.NoError(t, err)
	assert.Equal(t, followID, got.ID)

	gotByID, err := f.GetFollowByID(ctx, followID)
	require.NoError(t, err)
	assert.Equal(t, a1.ID, gotByID.AccountID)

	err = f.AcceptFollow(ctx, followID)
	require.NoError(t, err)

	accepted, err := f.GetFollow(ctx, a1.ID, a2.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.FollowStateAccepted, accepted.State)

	err = f.DeleteFollow(ctx, a1.ID, a2.ID)
	require.NoError(t, err)

	_, err = f.GetFollow(ctx, a1.ID, a2.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetFollow_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	_, err := f.GetFollow(ctx, "a", "b")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------- Block/Mute Operations ----------

func TestBlockLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	a1, err := f.CreateAccount(ctx, makeAccountInput("blocker"))
	require.NoError(t, err)
	a2, err := f.CreateAccount(ctx, makeAccountInput("blockee"))
	require.NoError(t, err)

	blockID := uid.New()
	err = f.CreateBlock(ctx, store.CreateBlockInput{ID: blockID, AccountID: a1.ID, TargetID: a2.ID})
	require.NoError(t, err)

	block, err := f.GetBlock(ctx, a1.ID, a2.ID)
	require.NoError(t, err)
	assert.Equal(t, a1.ID, block.AccountID)

	_, err = f.GetBlock(ctx, a2.ID, a1.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	err = f.DeleteBlock(ctx, a1.ID, a2.ID)
	require.NoError(t, err)

	_, err = f.GetBlock(ctx, a1.ID, a2.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestIsBlockedEitherDirection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	a1, err := f.CreateAccount(ctx, makeAccountInput("x1"))
	require.NoError(t, err)
	a2, err := f.CreateAccount(ctx, makeAccountInput("x2"))
	require.NoError(t, err)

	blocked, err := f.IsBlockedEitherDirection(ctx, a1.ID, a2.ID)
	require.NoError(t, err)
	assert.False(t, blocked)

	err = f.CreateBlock(ctx, store.CreateBlockInput{ID: uid.New(), AccountID: a2.ID, TargetID: a1.ID})
	require.NoError(t, err)

	blocked, err = f.IsBlockedEitherDirection(ctx, a1.ID, a2.ID)
	require.NoError(t, err)
	assert.True(t, blocked)

	blocked, err = f.IsBlockedEitherDirection(ctx, a2.ID, a1.ID)
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestListBlockedAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	blocker, err := f.CreateAccount(ctx, makeAccountInput("lb"))
	require.NoError(t, err)

	targets := make([]*domain.Account, 3)
	for i := 0; i < 3; i++ {
		a, err := f.CreateAccount(ctx, makeAccountInput("lbtarget"+uid.New()))
		require.NoError(t, err)
		targets[i] = a
		err = f.CreateBlock(ctx, store.CreateBlockInput{ID: uid.New(), AccountID: blocker.ID, TargetID: a.ID})
		require.NoError(t, err)
	}

	accounts, nextCursor, err := f.ListBlockedAccounts(ctx, blocker.ID, nil, 2)
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
	assert.NotNil(t, nextCursor)

	accounts2, _, err := f.ListBlockedAccounts(ctx, blocker.ID, nextCursor, 10)
	require.NoError(t, err)
	assert.Len(t, accounts2, 1)
}

func TestMuteLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	a1, err := f.CreateAccount(ctx, makeAccountInput("muter"))
	require.NoError(t, err)
	a2, err := f.CreateAccount(ctx, makeAccountInput("mutee"))
	require.NoError(t, err)

	err = f.CreateMute(ctx, store.CreateMuteInput{ID: uid.New(), AccountID: a1.ID, TargetID: a2.ID})
	require.NoError(t, err)

	mute, err := f.GetMute(ctx, a1.ID, a2.ID)
	require.NoError(t, err)
	assert.Equal(t, a1.ID, mute.AccountID)

	_, err = f.GetMute(ctx, a2.ID, a1.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	err = f.DeleteMute(ctx, a1.ID, a2.ID)
	require.NoError(t, err)

	_, err = f.GetMute(ctx, a1.ID, a2.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListMutedAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	muter, err := f.CreateAccount(ctx, makeAccountInput("lm"))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		a, err := f.CreateAccount(ctx, makeAccountInput("lmtarget"+uid.New()))
		require.NoError(t, err)
		err = f.CreateMute(ctx, store.CreateMuteInput{ID: uid.New(), AccountID: muter.ID, TargetID: a.ID})
		require.NoError(t, err)
	}

	accounts, nextCursor, err := f.ListMutedAccounts(ctx, muter.ID, nil, 2)
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
	assert.NotNil(t, nextCursor)

	accounts2, _, err := f.ListMutedAccounts(ctx, muter.ID, nextCursor, 10)
	require.NoError(t, err)
	assert.Len(t, accounts2, 1)
}

// ---------- Favourite Operations ----------

func TestFavouriteLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("faver"))
	require.NoError(t, err)
	status, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)

	apID := "https://example.com/fav/1"
	fav, err := f.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID:        uid.New(),
		AccountID: acc.ID,
		StatusID:  status.ID,
		APID:      &apID,
	})
	require.NoError(t, err)

	got, err := f.GetFavouriteByAccountAndStatus(ctx, acc.ID, status.ID)
	require.NoError(t, err)
	assert.Equal(t, fav.ID, got.ID)

	gotByAPID, err := f.GetFavouriteByAPID(ctx, apID)
	require.NoError(t, err)
	assert.Equal(t, fav.ID, gotByAPID.ID)

	err = f.DeleteFavourite(ctx, acc.ID, status.ID)
	require.NoError(t, err)

	_, err = f.GetFavouriteByAccountAndStatus(ctx, acc.ID, status.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestIncrementFavouritesCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("fc"))
	require.NoError(t, err)
	status, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)

	err = f.IncrementFavouritesCount(ctx, status.ID)
	require.NoError(t, err)
	err = f.IncrementFavouritesCount(ctx, status.ID)
	require.NoError(t, err)

	got, err := f.GetStatusByID(ctx, status.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.FavouritesCount)
}

// ---------- Bookmark Operations ----------

func TestBookmarkLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("bkmk"))
	require.NoError(t, err)
	status, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)

	err = f.CreateBookmark(ctx, store.CreateBookmarkInput{ID: uid.New(), AccountID: acc.ID, StatusID: status.ID})
	require.NoError(t, err)

	ok, err := f.IsBookmarked(ctx, acc.ID, status.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	err = f.CreateBookmark(ctx, store.CreateBookmarkInput{ID: uid.New(), AccountID: acc.ID, StatusID: status.ID})
	require.ErrorIs(t, err, domain.ErrConflict)

	err = f.DeleteBookmark(ctx, acc.ID, status.ID)
	require.NoError(t, err)

	ok, err = f.IsBookmarked(ctx, acc.ID, status.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

// ---------- OAuth Operations ----------

func TestOAuthApplicationLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	clientID := uid.New()
	app, err := f.CreateApplication(ctx, store.CreateApplicationInput{
		ID:           uid.New(),
		Name:         "TestApp",
		ClientID:     clientID,
		ClientSecret: "secret",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       "read write",
	})
	require.NoError(t, err)
	assert.Equal(t, "TestApp", app.Name)

	got, err := f.GetApplicationByClientID(ctx, clientID)
	require.NoError(t, err)
	assert.Equal(t, app.ID, got.ID)

	_, err = f.GetApplicationByClientID(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestOAuthAuthorizationCodeLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	code := uid.New()
	_, err := f.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
		ID:            uid.New(),
		Code:          code,
		ApplicationID: uid.New(),
		AccountID:     uid.New(),
		RedirectURI:   "urn:ietf:wg:oauth:2.0:oob",
		Scopes:        "read",
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	})
	require.NoError(t, err)

	got, err := f.GetAuthorizationCode(ctx, code)
	require.NoError(t, err)
	assert.Equal(t, code, got.Code)

	expiredCode := uid.New()
	_, err = f.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
		ID:            uid.New(),
		Code:          expiredCode,
		ApplicationID: uid.New(),
		AccountID:     uid.New(),
		RedirectURI:   "urn:ietf:wg:oauth:2.0:oob",
		Scopes:        "read",
		ExpiresAt:     time.Now().Add(-1 * time.Minute),
	})
	require.NoError(t, err)

	_, err = f.GetAuthorizationCode(ctx, expiredCode)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestOAuthAccessTokenLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	token := uid.New()
	accountID := uid.New()
	_, err := f.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		ID:            uid.New(),
		ApplicationID: uid.New(),
		AccountID:     &accountID,
		Token:         token,
		Scopes:        "read write",
	})
	require.NoError(t, err)

	got, err := f.GetAccessToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, token, got.Token)

	err = f.RevokeAccessToken(ctx, token)
	require.NoError(t, err)

	_, err = f.GetAccessToken(ctx, token)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------- User Operations ----------

func TestUserLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("usertest"))
	require.NoError(t, err)

	userID := uid.New()
	user, err := f.CreateUser(ctx, store.CreateUserInput{
		ID:           userID,
		AccountID:    acc.ID,
		Email:        "user@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)

	got, err := f.GetUserByAccountID(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, userID, got.ID)

	gotByEmail, err := f.GetUserByEmail(ctx, "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, userID, gotByEmail.ID)

	gotByID, err := f.GetUserByID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, gotByID.AccountID)

	err = f.ConfirmUser(ctx, userID)
	require.NoError(t, err)

	confirmed, err := f.GetUserByAccountID(ctx, acc.ID)
	require.NoError(t, err)
	assert.NotNil(t, confirmed.ConfirmedAt)

	err = f.UpdateUserRole(ctx, userID, domain.RoleAdmin)
	require.NoError(t, err)

	updated, err := f.GetUserByID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, updated.Role)
}

// ---------- List Operations ----------

func TestListLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("lister"))
	require.NoError(t, err)

	listID := uid.New()
	list, err := f.CreateList(ctx, store.CreateListInput{
		ID:            listID,
		AccountID:     acc.ID,
		Title:         "My List",
		RepliesPolicy: domain.ListRepliesPolicyFollowed,
	})
	require.NoError(t, err)
	assert.Equal(t, "My List", list.Title)

	got, err := f.GetListByID(ctx, listID)
	require.NoError(t, err)
	assert.Equal(t, "My List", got.Title)

	updated, err := f.UpdateList(ctx, store.UpdateListInput{
		ID:            listID,
		Title:         "Renamed",
		RepliesPolicy: domain.ListRepliesPolicyNone,
	})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Title)

	member, err := f.CreateAccount(ctx, makeAccountInput("member"))
	require.NoError(t, err)

	err = f.AddAccountToList(ctx, listID, member.ID)
	require.NoError(t, err)

	ids, err := f.ListListAccountIDs(ctx, listID)
	require.NoError(t, err)
	assert.Contains(t, ids, member.ID)

	err = f.RemoveAccountFromList(ctx, listID, member.ID)
	require.NoError(t, err)

	ids, err = f.ListListAccountIDs(ctx, listID)
	require.NoError(t, err)
	assert.NotContains(t, ids, member.ID)

	err = f.DeleteList(ctx, listID)
	require.NoError(t, err)

	_, err = f.GetListByID(ctx, listID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------- UserFilter Operations ----------

func TestUserFilterLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	accID := uid.New()
	filterID := uid.New()

	uf, err := f.CreateUserFilter(ctx, store.CreateUserFilterInput{
		ID:        filterID,
		AccountID: accID,
		Phrase:    "badword",
		Context:   []string{domain.FilterContextHome, domain.FilterContextPublic},
		WholeWord: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "badword", uf.Phrase)

	got, err := f.GetUserFilterByID(ctx, filterID)
	require.NoError(t, err)
	assert.Equal(t, "badword", got.Phrase)
	assert.True(t, got.WholeWord)

	filters, err := f.ListUserFilters(ctx, accID)
	require.NoError(t, err)
	assert.Len(t, filters, 1)

	updated, err := f.UpdateUserFilter(ctx, store.UpdateUserFilterInput{
		ID:      filterID,
		Phrase:  "worse",
		Context: []string{domain.FilterContextHome},
	})
	require.NoError(t, err)
	assert.Equal(t, "worse", updated.Phrase)

	err = f.DeleteUserFilter(ctx, filterID)
	require.NoError(t, err)

	_, err = f.GetUserFilterByID(ctx, filterID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------- Notification Operations ----------

func TestNotificationLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	recipientID := uid.New()
	fromID := uid.New()

	var notifIDs []string
	for i := 0; i < 5; i++ {
		nID := uid.New()
		notifIDs = append(notifIDs, nID)
		_, err := f.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        nID,
			AccountID: recipientID,
			FromID:    fromID,
			Type:      domain.NotificationTypeFavourite,
		})
		require.NoError(t, err)
	}

	got, err := f.GetNotification(ctx, notifIDs[0], recipientID)
	require.NoError(t, err)
	assert.Equal(t, notifIDs[0], got.ID)

	_, err = f.GetNotification(ctx, "nonexistent", recipientID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	list, err := f.ListNotifications(ctx, recipientID, nil, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, list, 3)
	for i := 1; i < len(list); i++ {
		assert.Greater(t, list[i-1].ID, list[i].ID)
	}

	cursor := list[len(list)-1].ID
	page2, err := f.ListNotifications(ctx, recipientID, &cursor, 10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
}

// ---------- Poll Operations ----------

func TestPollLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("pollster"))
	require.NoError(t, err)
	status, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)

	pollID := uid.New()
	expires := time.Now().Add(24 * time.Hour)
	poll, err := f.CreatePoll(ctx, store.CreatePollInput{
		ID:        pollID,
		StatusID:  status.ID,
		ExpiresAt: &expires,
		Multiple:  false,
	})
	require.NoError(t, err)
	assert.Equal(t, pollID, poll.ID)

	got, err := f.GetPollByID(ctx, pollID)
	require.NoError(t, err)
	assert.Equal(t, pollID, got.ID)

	opt1ID := uid.New()
	opt2ID := uid.New()
	_, err = f.CreatePollOption(ctx, store.CreatePollOptionInput{ID: opt1ID, PollID: pollID, Title: "Yes", Position: 0})
	require.NoError(t, err)
	_, err = f.CreatePollOption(ctx, store.CreatePollOptionInput{ID: opt2ID, PollID: pollID, Title: "No", Position: 1})
	require.NoError(t, err)

	opts, err := f.ListPollOptions(ctx, pollID)
	require.NoError(t, err)
	require.Len(t, opts, 2)
	assert.Equal(t, "Yes", opts[0].Title)
	assert.Equal(t, "No", opts[1].Title)

	voterID := uid.New()
	err = f.CreatePollVote(ctx, uid.New(), pollID, voterID, opt1ID)
	require.NoError(t, err)

	counts, err := f.GetVoteCountsByPoll(ctx, pollID)
	require.NoError(t, err)
	assert.Equal(t, 1, counts[opt1ID])

	voted, err := f.HasVotedOnPoll(ctx, pollID, voterID)
	require.NoError(t, err)
	assert.True(t, voted)

	notVoted, err := f.HasVotedOnPoll(ctx, pollID, uid.New())
	require.NoError(t, err)
	assert.False(t, notVoted)
}

// ---------- Announcement Operations ----------

func TestAnnouncementLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	annID := uid.New()
	now := time.Now()
	ann, err := f.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          annID,
		Content:     "Hello everyone!",
		PublishedAt: now.Add(-1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello everyone!", ann.Content)

	got, err := f.GetAnnouncementByID(ctx, annID)
	require.NoError(t, err)
	assert.Equal(t, annID, got.ID)

	_, err = f.GetAnnouncementByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListActiveAnnouncements(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	past := time.Now().Add(-2 * time.Hour)
	future := time.Now().Add(2 * time.Hour)

	_, err := f.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          uid.New(),
		Content:     "Active",
		PublishedAt: past,
	})
	require.NoError(t, err)

	_, err = f.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          uid.New(),
		Content:     "Future",
		PublishedAt: future,
	})
	require.NoError(t, err)

	expired := time.Now().Add(-1 * time.Hour)
	_, err = f.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          uid.New(),
		Content:     "Expired",
		PublishedAt: past,
		EndsAt:      &expired,
	})
	require.NoError(t, err)

	active, err := f.ListActiveAnnouncements(ctx)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, "Active", active[0].Content)
}

func TestAnnouncementDismissAndReactions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	annID := uid.New()
	accountID := uid.New()
	_, err := f.CreateAnnouncement(ctx, store.CreateAnnouncementInput{
		ID:          annID,
		Content:     "Notice",
		PublishedAt: time.Now().Add(-1 * time.Hour),
	})
	require.NoError(t, err)

	err = f.DismissAnnouncement(ctx, accountID, annID)
	require.NoError(t, err)

	readIDs, err := f.ListReadAnnouncementIDs(ctx, accountID)
	require.NoError(t, err)
	assert.Contains(t, readIDs, annID)

	err = f.AddAnnouncementReaction(ctx, annID, accountID, "👍")
	require.NoError(t, err)
	err = f.AddAnnouncementReaction(ctx, annID, uid.New(), "👍")
	require.NoError(t, err)

	counts, err := f.ListAnnouncementReactionCounts(ctx, annID)
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, "👍", counts[0].Name)
	assert.Equal(t, 2, counts[0].Count)
}

// ---------- Conversation Operations ----------

func TestConversationLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	convID := uid.New()
	err := f.CreateConversation(ctx, convID)
	require.NoError(t, err)

	acc, err := f.CreateAccount(ctx, makeAccountInput("conv"))
	require.NoError(t, err)
	status, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityDirect, true))
	require.NoError(t, err)

	err = f.SetStatusConversationID(ctx, status.ID, convID)
	require.NoError(t, err)

	cid, err := f.GetStatusConversationID(ctx, status.ID)
	require.NoError(t, err)
	require.NotNil(t, cid)
	assert.Equal(t, convID, *cid)
}

func TestConversationMute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	accountID := uid.New()
	convID := uid.New()

	muted, err := f.IsConversationMuted(ctx, accountID, convID)
	require.NoError(t, err)
	assert.False(t, muted)

	err = f.CreateConversationMute(ctx, accountID, convID)
	require.NoError(t, err)

	muted, err = f.IsConversationMuted(ctx, accountID, convID)
	require.NoError(t, err)
	assert.True(t, muted)
}

func TestAccountConversations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	accountID := uid.New()
	convID := uid.New()
	acID := uid.New()

	err := f.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
		ID:             acID,
		AccountID:      accountID,
		ConversationID: convID,
		LastStatusID:   uid.New(),
		Unread:         true,
	})
	require.NoError(t, err)

	list, _, err := f.ListAccountConversations(ctx, accountID, nil, 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Unread)
	assert.Equal(t, convID, list[0].ConversationID)
}

// ---------- Outbox Events ----------

func TestOutboxEventLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	id1 := uid.New()
	id2 := uid.New()
	payload := json.RawMessage(`{"foo":"bar"}`)

	err := f.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
		ID:            id1,
		EventType:     domain.EventStatusCreated,
		AggregateType: "status",
		AggregateID:   uid.New(),
		Payload:       payload,
	})
	require.NoError(t, err)

	err = f.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
		ID:            id2,
		EventType:     domain.EventFollowCreated,
		AggregateType: "follow",
		AggregateID:   uid.New(),
		Payload:       payload,
	})
	require.NoError(t, err)

	events, err := f.GetAndLockUnpublishedOutboxEvents(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, events, 2)

	err = f.MarkOutboxEventsPublished(ctx, []string{id1})
	require.NoError(t, err)

	remaining, err := f.GetAndLockUnpublishedOutboxEvents(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, id2, remaining[0].ID)
}

// ---------- Hashtag Operations ----------

func TestHashtagGetOrCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	h1, err := f.GetOrCreateHashtag(ctx, "GoLang")
	require.NoError(t, err)
	assert.Equal(t, "golang", h1.Name)

	h2, err := f.GetOrCreateHashtag(ctx, "golang")
	require.NoError(t, err)
	assert.Equal(t, h1.ID, h2.ID)
}

func TestHashtagFollowAndUnfollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	accountID := uid.New()
	tag, err := f.GetOrCreateHashtag(ctx, "testtag")
	require.NoError(t, err)

	followID := uid.New()
	err = f.FollowTag(ctx, followID, accountID, tag.ID)
	require.NoError(t, err)

	tags, _, err := f.ListFollowedTags(ctx, accountID, nil, 10)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "testtag", tags[0].Name)

	err = f.UnfollowTag(ctx, accountID, tag.ID)
	require.NoError(t, err)

	tags, _, err = f.ListFollowedTags(ctx, accountID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// ---------- ScheduledStatus Operations ----------

func TestScheduledStatusLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	accountID := uid.New()
	ssID := uid.New()
	params := json.RawMessage(`{"text":"scheduled post"}`)

	ss, err := f.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          ssID,
		AccountID:   accountID,
		Params:      params,
		ScheduledAt: time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, ssID, ss.ID)

	got, err := f.GetScheduledStatusByID(ctx, ssID)
	require.NoError(t, err)
	assert.Equal(t, ssID, got.ID)

	err = f.DeleteScheduledStatus(ctx, ssID)
	require.NoError(t, err)

	_, err = f.GetScheduledStatusByID(ctx, ssID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	err = f.DeleteScheduledStatus(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------- StatusCard Operations ----------

func TestStatusCardLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	statusID := uid.New()
	err := f.UpsertStatusCard(ctx, store.UpsertStatusCardInput{
		StatusID:        statusID,
		ProcessingState: domain.CardStateFetched,
		URL:             "https://example.com/article",
		Title:           "An Article",
		Description:     "Interesting content",
		CardType:        "link",
		ProviderName:    "Example",
		Width:           800,
		Height:          600,
	})
	require.NoError(t, err)

	card, err := f.GetStatusCard(ctx, statusID)
	require.NoError(t, err)
	assert.Equal(t, "An Article", card.Title)
	assert.Equal(t, "https://example.com/article", card.URL)
	assert.Equal(t, 800, card.Width)

	_, err = f.GetStatusCard(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------- Pin Operations ----------

func TestPinLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFakeStore()

	acc, err := f.CreateAccount(ctx, makeAccountInput("pinner"))
	require.NoError(t, err)
	s1, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)
	s2, err := f.CreateStatus(ctx, makeStatusInput(acc.ID, domain.VisibilityPublic, true))
	require.NoError(t, err)

	err = f.CreateAccountPin(ctx, acc.ID, s1.ID)
	require.NoError(t, err)
	err = f.CreateAccountPin(ctx, acc.ID, s2.ID)
	require.NoError(t, err)

	pinned, err := f.ListPinnedStatusIDs(ctx, acc.ID)
	require.NoError(t, err)
	assert.Len(t, pinned, 2)
	assert.Contains(t, pinned, s1.ID)
	assert.Contains(t, pinned, s2.ID)

	count, err := f.CountAccountPins(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	err = f.DeleteAccountPin(ctx, acc.ID, s1.ID)
	require.NoError(t, err)

	pinned, err = f.ListPinnedStatusIDs(ctx, acc.ID)
	require.NoError(t, err)
	assert.Len(t, pinned, 1)
	assert.Contains(t, pinned, s2.ID)

	count, err = f.CountAccountPins(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

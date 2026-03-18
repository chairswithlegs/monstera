package activitypub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestOutboxHandler_GETOutbox_collection(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "01USERALICE", AccountID: "01HXXX", Email: "alice@example.com", PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))
	accountSvc := service.NewAccountService(fake, "https://example.com")
	statusSvc := service.NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := service.NewTimelineService(fake, accountSvc, statusSvc)
	h := NewOutbox(accountSvc, timelineSvc, "https://example.com")
	r := httptest.NewRequest(http.MethodGet, "/users/alice/outbox", nil)
	r = r.WithContext(ctx)
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETOutbox(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var coll struct {
		Type       string `json:"type"`
		TotalItems int    `json:"totalItems"`
		First      string `json:"first"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&coll))
	assert.Equal(t, "OrderedCollection", coll.Type)
	assert.Equal(t, "https://example.com/users/alice/outbox?page=true", coll.First)
}

func TestOutboxHandler_GETOutbox_page(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: "01HXXX", Username: "alice", APID: "https://example.com/users/alice",
	})
	require.NoError(t, err)
	_, err = fake.CreateUser(ctx, store.CreateUserInput{
		ID: "01USERALICE", AccountID: "01HXXX", Email: "alice@example.com", PasswordHash: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, fake.ConfirmUser(ctx, "01USERALICE"))
	content := "<p>hello</p>"
	_, err = fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: "01HYYY", URI: "https://example.com/statuses/01HYYY", AccountID: "01HXXX",
		Content: &content, Visibility: "public", APID: "https://example.com/statuses/01HYYY", Local: true,
	})
	require.NoError(t, err)

	accountSvc := service.NewAccountService(fake, "https://example.com")
	statusSvc := service.NewStatusService(fake, "https://example.com", "example.com", 500)
	timelineSvc := service.NewTimelineService(fake, accountSvc, statusSvc)
	h := NewOutbox(accountSvc, timelineSvc, "https://example.com")
	r := httptest.NewRequest(http.MethodGet, "/users/alice/outbox?page=true", nil)
	r = r.WithContext(ctx)
	r = testutil.AddChiURLParam(r, "username", "alice")
	w := httptest.NewRecorder()
	h.GETOutbox(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var page struct {
		Type         string            `json:"type"`
		OrderedItems []json.RawMessage `json:"orderedItems"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
	assert.Equal(t, "OrderedCollectionPage", page.Type)
	require.Len(t, page.OrderedItems, 1)
	var create struct {
		Type  string `json:"type"`
		Actor string `json:"actor"`
	}
	require.NoError(t, json.Unmarshal(page.OrderedItems[0], &create))
	assert.Equal(t, "Create", create.Type)
	assert.Equal(t, "https://example.com/users/alice", create.Actor)
}

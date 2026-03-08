package activitypub

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

const (
	testKeyID = "https://example.com/users/alice#main-key"
	testHost  = "example.com"
)

func TestActorKeyID(t *testing.T) {
	t.Parallel()
	acc := &domain.Account{APID: "https://example.com/users/alice", Username: "alice"}
	assert.Equal(t, "https://example.com/users/alice#main-key", actorKeyID(acc, "example.com"))
	acc.APID = ""
	assert.Equal(t, "https://example.com/users/alice#main-key", actorKeyID(acc, "example.com"))
}

func TestParseRSAPrivateKeyPEM(t *testing.T) {
	t.Parallel()
	privPEM, err := generateTestKeyPEM(t)
	require.NoError(t, err)
	key, err := parseRSAPrivateKeyPEM(privPEM)
	require.NoError(t, err)
	require.NotNil(t, key)
}

func TestParseRSAPrivateKeyPEM_invalid(t *testing.T) {
	t.Parallel()
	_, err := parseRSAPrivateKeyPEM("not-pem")
	require.Error(t, err)
	_, err = parseRSAPrivateKeyPEM("-----BEGIN RSA PRIVATE KEY-----\ninvalid\n-----END RSA PRIVATE KEY-----")
	require.Error(t, err)
}

func TestSignWithSenderID_success(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	fake := testutil.NewFakeStore()
	_, err = fake.CreateAccount(context.Background(), store.CreateAccountInput{
		ID:           "01sender",
		Username:     "alice",
		Domain:       nil,
		DisplayName:  nil,
		Note:         nil,
		PublicKey:    "test-pub",
		PrivateKey:   &privPEM,
		InboxURL:     "",
		OutboxURL:    "",
		FollowersURL: "",
		FollowingURL: "",
		APID:         "https://example.com/users/alice",
		ApRaw:        nil,
		Bot:          false,
		Locked:       false,
	})
	require.NoError(t, err)

	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	cfg := &config.Config{InstanceDomain: "example.com"}
	accountSvc := service.NewAccountService(fake, "https://example.com")
	svc := NewHTTPSignatureService(cfg, c, accountSvc)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://remote.example/inbox", nil)
	require.NoError(t, err)

	err = svc.SignWithSenderID(context.Background(), req, "01sender")
	require.NoError(t, err)

	sig := req.Header.Get("Signature")
	require.NotEmpty(t, sig)
	assert.Contains(t, sig, `keyId="https://example.com/users/alice#main-key"`)
	assert.Contains(t, sig, "algorithm=\"rsa-sha256\"")
}

func TestSignWithSenderID_senderNotFound(t *testing.T) {
	t.Parallel()
	accountService := &mockAccountService{
		GetByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			return nil, nil
		},
		CreateFunc: func(ctx context.Context, in service.CreateAccountInput) (*domain.Account, error) {
			return nil, nil
		},
	}
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{InstanceDomain: "example.com"}, c, accountService)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://x/inbox", nil)
	err := svc.SignWithSenderID(context.Background(), req, "01nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "01nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestSignWithSenderID_noPrivateKey(t *testing.T) {
	t.Parallel()
	accountService := &mockAccountService{
		GetByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			return &domain.Account{ID: id, APID: "https://example.com/users/nokey"}, nil
		},
		CreateFunc: func(ctx context.Context, in service.CreateAccountInput) (*domain.Account, error) {
			return nil, nil
		},
	}
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{InstanceDomain: "example.com"}, c, accountService)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://x/inbox", nil)
	err := svc.SignWithSenderID(context.Background(), req, "01nokey")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no private key")
}

func TestNewHTTPSignatureService(t *testing.T) {
	t.Parallel()
	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	cfg := &config.Config{InstanceDomain: "example.com"}
	svc := NewHTTPSignatureService(cfg, c, nil)
	require.NotNil(t, svc)
	impl, ok := svc.(*httpSignatureService)
	require.True(t, ok)
	assert.Equal(t, testHost, impl.instanceDomain)
}

func TestSign_GET_setsHeaders(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil).(*httpSignatureService)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+testHost+"/users/alice", nil)
	require.NoError(t, err)

	err = svc.sign(req, testKeyID, key)
	require.NoError(t, err)

	assert.NotEmpty(t, req.Header.Get("Date"))
	assert.Equal(t, testHost, req.Header.Get("Host"))
	sig := req.Header.Get("Signature")
	require.NotEmpty(t, sig)
	assert.Contains(t, sig, "keyId=")
	assert.Contains(t, sig, "algorithm=\"rsa-sha256\"")
	assert.Contains(t, sig, "(request-target)")
	assert.NotContains(t, sig, "digest")
}

func TestSign_POST_withBody_setsDigest(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil).(*httpSignatureService)

	body := []byte(`{"type":"Create"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://"+testHost+"/inbox", bytes.NewReader(body))
	require.NoError(t, err)

	err = svc.sign(req, testKeyID, key)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(req.Header.Get("Digest"), "SHA-256="))
	sig := req.Header.Get("Signature")
	assert.Contains(t, sig, "digest")
}

func TestVerify_success(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	ck := pubKeyCacheKey(testKeyID)
	err = c.Set(context.Background(), ck, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), time.Hour)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously

	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	impl := svc.(*httpSignatureService)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+testHost+"/users/alice", nil)
	require.NoError(t, err)
	req.Host = testHost

	err = impl.sign(req, testKeyID, key)
	require.NoError(t, err)

	got, err := svc.Verify(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, testKeyID, got)
}

func TestVerify_missingSignature(t *testing.T) {
	t.Parallel()
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	_, err := svc.Verify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Signature")
}

func TestVerify_unsupportedAlgorithm(t *testing.T) {
	t.Parallel()
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Signature", `keyId="https://x#k",algorithm="hmac-sha256",headers="date",signature="YQ=="`)

	_, err := svc.Verify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "algorithm")
}

func TestVerify_missingDate(t *testing.T) {
	t.Parallel()
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Signature", `keyId="https://x#k",algorithm="rsa-sha256",headers="date",signature="YQ=="`)

	_, err := svc.Verify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Date")
}

func TestVerify_dateDrift(t *testing.T) {
	t.Parallel()
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Date", time.Now().Add(-2*time.Hour).UTC().Format(http.TimeFormat))
	req.Header.Set("Signature", `keyId="https://x#k",algorithm="rsa-sha256",headers="date",signature="YQ=="`)

	_, err := svc.Verify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift")
}

func TestVerify_digestMismatch(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	pubDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	_ = c.Set(context.Background(), pubKeyCacheKey(testKeyID), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), time.Hour)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously

	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	impl := svc.(*httpSignatureService)
	body := []byte(`{"type":"Create"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://"+testHost+"/inbox", bytes.NewReader(body))
	require.NoError(t, err)
	req.Host = testHost
	err = impl.sign(req, testKeyID, key)
	require.NoError(t, err)

	req.Header.Set("Digest", "SHA-256=wrong")
	_, err = svc.Verify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Digest")
}

func TestVerify_replayDetected(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	pubDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	err = c.Set(context.Background(), pubKeyCacheKey(testKeyID), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), time.Hour)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously

	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	impl := svc.(*httpSignatureService)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+testHost+"/users/alice", nil)
	require.NoError(t, err)
	req.Host = testHost
	err = impl.sign(req, testKeyID, key)
	require.NoError(t, err)

	_, err = svc.Verify(context.Background(), req)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously (replay entry)

	_, err = svc.Verify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replay")
}

func TestFetchRemotePublicKey_success(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"publicKey": map[string]string{
				"id":           r.URL.String() + "#main-key",
				"publicKeyPem": string(pubPEM),
			},
		})
	}))
	defer server.Close()

	keyID := server.URL + "#main-key"
	c, _ := cache.New(cache.Config{Driver: "memory"})
	defer func() { _ = c.Close() }()
	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	impl := svc.(*httpSignatureService)
	impl.client = server.Client()

	got, err := svc.FetchRemotePublicKey(context.Background(), keyID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, key.E, got.E)
	assert.Equal(t, key.N, got.N)
}

func TestFetchRemotePublicKey_cacheHit(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	keyID := "https://example.com/users/bob#main-key"
	ck := pubKeyCacheKey(keyID)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	require.NotEmpty(t, pubPEM, "PEM encoding must succeed")

	c, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	err = c.Set(context.Background(), ck, pubPEM, time.Hour)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ristretto admits asynchronously

	svc := NewHTTPSignatureService(&config.Config{}, c, nil)
	impl := svc.(*httpSignatureService)
	callCount := 0
	impl.client = &http.Client{
		Transport: &roundTripperFunc{fn: func(*http.Request) (*http.Response, error) {
			callCount++
			return nil, assert.AnError
		}},
	}

	got, err := svc.FetchRemotePublicKey(context.Background(), keyID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, key.N, got.N)
	assert.Equal(t, 0, callCount, "should not have called remote when cache hit")
}

type roundTripperFunc struct {
	fn func(*http.Request) (*http.Response, error)
}

func (r *roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.fn(req)
}

func generateTestKeyPEM(t *testing.T) (string, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), nil
}

type mockAccountService struct {
	service.AccountService
	GetByIDFunc func(ctx context.Context, id string) (*domain.Account, error)
	CreateFunc  func(ctx context.Context, in service.CreateAccountInput) (*domain.Account, error)
}

func (m *mockAccountService) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	return m.GetByIDFunc(ctx, id)
}

func (m *mockAccountService) Create(ctx context.Context, in service.CreateAccountInput) (*domain.Account, error) {
	return m.CreateFunc(ctx, in)
}

package cli

// actorserver.go — serves ActivityPub actor JSON with publicKeyPem for HTTP Signature verification.

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ActorServer serves actor documents on a random local port so the Monstera
// server can fetch the public key to verify incoming HTTP Signatures.
type ActorServer struct {
	BaseURL  string
	listener net.Listener
	server   *http.Server
	keys     map[string]*rsa.PublicKey
}

// NewActorServer creates and starts an HTTP server on a random local port.
// keys maps actorID (e.g. "actor0") to the RSA public key to serve.
// hostIP, when non-empty, causes the server to bind on 0.0.0.0 and advertise
// http://hostIP:PORT as BaseURL (needed when the verifying server runs in Docker).
func NewActorServer(keys map[string]*rsa.PublicKey, hostIP string) (*ActorServer, error) {
	bindAddr := "127.0.0.1:0"
	if hostIP != "" {
		bindAddr = "0.0.0.0:0"
	}
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("actorserver: listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	advertise := ln.Addr().String()
	if hostIP != "" {
		advertise = fmt.Sprintf("%s:%d", hostIP, port)
	}
	as := &ActorServer{
		BaseURL:  "http://" + advertise,
		listener: ln,
		keys:     keys,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/users/", as.handleActor)

	as.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = as.server.Serve(ln) }()
	return as, nil
}

// KeyID returns the keyId string for actorN (used in Signature headers).
func (as *ActorServer) KeyID(actorN int) string {
	return fmt.Sprintf("%s/users/actor%d#main-key", as.BaseURL, actorN)
}

// ActorID returns the actor URL for actorN.
func (as *ActorServer) ActorID(actorN int) string {
	return fmt.Sprintf("%s/users/actor%d", as.BaseURL, actorN)
}

func (as *ActorServer) handleActor(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/users/")
	if idx := strings.Index(name, "#"); idx >= 0 {
		name = name[:idx]
	}

	key, ok := as.keys[name]
	if !ok {
		http.NotFound(w, r)
		return
	}

	pubDER, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		http.Error(w, "marshal key", http.StatusInternalServerError)
		return
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	actorURL := fmt.Sprintf("%s/users/%s", as.BaseURL, name)
	keyID := actorURL + "#main-key"

	body, _ := json.Marshal(map[string]any{
		"@context":          "https://www.w3.org/ns/activitystreams",
		"id":                actorURL,
		"type":              "Person",
		"preferredUsername": name,
		"inbox":             actorURL + "/inbox",
		"publicKey": map[string]any{
			"id":           keyID,
			"owner":        actorURL,
			"publicKeyPem": pubPEM,
		},
	})

	w.Header().Set("Content-Type", "application/activity+json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// Shutdown gracefully stops the actor server.
func (as *ActorServer) Shutdown(ctx context.Context) error {
	if err := as.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("actorserver shutdown: %w", err)
	}
	return nil
}

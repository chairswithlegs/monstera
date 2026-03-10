package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Flood the ActivityPub inbox endpoint with signed activities",
	RunE:  runInbox,
}

var (
	inboxTarget       string
	inboxUsername     string
	inboxConcurrency  int
	inboxTotal        int
	inboxDuration     time.Duration
	inboxActivityType string
	inboxActors       int
	inboxHostIP       string
)

func init() {
	inboxCmd.Flags().StringVar(&inboxTarget, "target", "http://localhost:8080", "base URL of the Monstera server")
	inboxCmd.Flags().StringVar(&inboxUsername, "username", "alice", "local account username to target")
	inboxCmd.Flags().IntVar(&inboxConcurrency, "concurrency", 10, "number of concurrent workers")
	inboxCmd.Flags().IntVar(&inboxTotal, "total", 1000, "total requests to send (0 = use --duration)")
	inboxCmd.Flags().DurationVar(&inboxDuration, "duration", 0, "run for this duration (overrides --total when set)")
	inboxCmd.Flags().StringVar(&inboxActivityType, "activity-type", "create-note", "activity type: create-note|follow|like")
	inboxCmd.Flags().IntVar(&inboxActors, "actors", 0, "number of key pairs to generate (default = concurrency)")
	inboxCmd.Flags().StringVar(&inboxHostIP, "host-ip", "", "host IP reachable from the server container (e.g. 172.18.0.1 for Docker)")
}

func runInbox(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Default: one unique actor per request to avoid replay detection.
	// The inbox replay guard keys on (keyId, Date, requestTarget) with 1s Date resolution;
	// reusing an actor within the same second triggers a false replay.
	numActors := inboxActors
	if numActors <= 0 {
		if inboxDuration > 0 {
			numActors = inboxConcurrency * 10
		} else {
			numActors = inboxTotal
		}
	}

	// Generate a single RSA-2048 key pair shared by all actors.
	// Replay detection keys on (keyId, Date, requestTarget); each actor has a unique keyId,
	// so sharing key material does not trigger false replays.
	fmt.Fprintf(os.Stderr, "Generating RSA-2048 key pair (shared across %d actors)...\n", numActors)
	sharedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	privateKeys := make([]*rsa.PrivateKey, numActors)
	publicKeys := make(map[string]*rsa.PublicKey, numActors)
	for i := range numActors {
		privateKeys[i] = sharedKey
		publicKeys[fmt.Sprintf("actor%d", i)] = &sharedKey.PublicKey
	}

	// Start actor server so the inbox handler can verify signatures.
	actorSrv, err := NewActorServer(publicKeys, inboxHostIP)
	if err != nil {
		return fmt.Errorf("actor server: %w", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = actorSrv.Shutdown(shutCtx)
	}()
	fmt.Fprintf(os.Stderr, "Actor server listening at %s\n", actorSrv.BaseURL)

	stats := &Stats{}
	var sent atomic.Int64
	inboxURL := fmt.Sprintf("%s/users/%s/inbox", inboxTarget, inboxUsername)

	// Determine stop condition.
	var stop func() bool
	if inboxDuration > 0 {
		deadline := time.Now().Add(inboxDuration)
		stop = func() bool { return time.Now().After(deadline) }
	} else {
		stop = func() bool { return sent.Load() >= int64(inboxTotal) }
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()

	// actorSeq assigns a unique actor index per request to avoid replay detection.
	var actorSeq atomic.Int64

	var wg sync.WaitGroup
	for range inboxConcurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop() {
				n := sent.Add(1)
				if inboxDuration == 0 && n > int64(inboxTotal) {
					return
				}
				actorN := int(actorSeq.Add(1)-1) % numActors
				keyID := actorSrv.KeyID(actorN)
				actorID := actorSrv.ActorID(actorN)
				activity := buildActivity(inboxActivityType, actorID, inboxTarget, inboxUsername)
				body, _ := json.Marshal(activity)

				req, err := http.NewRequestWithContext(ctx, http.MethodPost, inboxURL, bytes.NewReader(body))
				if err != nil {
					return
				}
				req.Header.Set("Content-Type", "application/activity+json")

				privateKey := privateKeys[actorN]
				if err := SignRequest(req, keyID, privateKey); err != nil {
					return
				}

				t0 := time.Now()
				resp, err := client.Do(req)
				latency := time.Since(t0)
				if err != nil {
					stats.Record(latency, 0)
					continue
				}
				_ = resp.Body.Close()
				stats.Record(latency, resp.StatusCode)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	total := stats.total.Load()
	result := InboxResult{
		Requests:  total,
		Successes: stats.success.Load(),
		C4xx:      stats.client4xx.Load(),
		C5xx:      stats.server5xx.Load(),
		Duration:  elapsed.Seconds(),
		P50Ms:     stats.Percentile(50).Milliseconds(),
		P95Ms:     stats.Percentile(95).Milliseconds(),
		P99Ms:     stats.Percentile(99).Milliseconds(),
	}
	if elapsed.Seconds() > 0 {
		result.RPS = float64(total) / elapsed.Seconds()
	}

	PrintInboxResult(os.Stdout, result, jsonOutput)
	return nil
}

// buildActivity returns a minimal ActivityPub activity map.
func buildActivity(activityType, actorID, targetBase, username string) map[string]any {
	objectBase := fmt.Sprintf("%s/users/%s", targetBase, username)
	switch activityType {
	case "follow":
		return map[string]any{
			"@context": "https://www.w3.org/ns/activitystreams",
			"type":     "Follow",
			"actor":    actorID,
			"object":   objectBase,
		}
	case "like":
		return map[string]any{
			"@context": "https://www.w3.org/ns/activitystreams",
			"type":     "Like",
			"actor":    actorID,
			"object":   objectBase + "/statuses/01HZZZZZZZZZZZZZZZZZZZZZZ0",
		}
	default: // create-note
		return map[string]any{
			"@context": "https://www.w3.org/ns/activitystreams",
			"type":     "Create",
			"actor":    actorID,
			"object": map[string]any{
				"type":         "Note",
				"content":      "loadtest ping",
				"to":           []string{"https://www.w3.org/ns/activitystreams#Public"},
				"tag":          []map[string]any{{"type": "Mention", "href": objectBase}},
				"attributedTo": actorID,
			},
		}
	}
}

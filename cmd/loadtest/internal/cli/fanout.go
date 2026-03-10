package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
)

var fanoutCmd = &cobra.Command{
	Use:   "fanout",
	Short: "Measure outbox fan-out throughput to simulated remote followers",
	RunE:  runFanout,
}

var (
	fanoutFollowers int
	fanoutInstances int
	fanoutUsername  string
	fanoutServerURL string
	fanoutDBURL     string
	fanoutNATSURL   string
	fanoutToken     string
	fanoutTimeout   time.Duration
	fanoutCleanup   bool
	fanoutHostIP    string
)

func init() {
	fanoutCmd.Flags().IntVar(&fanoutFollowers, "followers", 1000, "number of remote followers to seed")
	fanoutCmd.Flags().IntVar(&fanoutInstances, "instances", 10, "number of mock inbox servers")
	fanoutCmd.Flags().StringVar(&fanoutUsername, "username", "alice", "local account username")
	fanoutCmd.Flags().StringVar(&fanoutServerURL, "server-url", "http://localhost:8080", "Monstera server base URL")
	fanoutCmd.Flags().StringVar(&fanoutDBURL, "db-url", "", "PostgreSQL connection URL")
	fanoutCmd.Flags().StringVar(&fanoutNATSURL, "nats-url", "nats://localhost:4222", "NATS server URL")
	fanoutCmd.Flags().StringVar(&fanoutToken, "token", "", "Mastodon access token for posting statuses")
	fanoutCmd.Flags().DurationVar(&fanoutTimeout, "timeout", 120*time.Second, "max time to wait for deliveries")
	fanoutCmd.Flags().BoolVar(&fanoutCleanup, "cleanup", false, "delete seeded accounts after the test")
	fanoutCmd.Flags().StringVar(&fanoutHostIP, "host-ip", "", "host IP reachable from the server container (e.g. 172.18.0.1 for Docker)")

	_ = fanoutCmd.MarkFlagRequired("db-url")
	_ = fanoutCmd.MarkFlagRequired("token")
}

func runFanout(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Connect to DB and look up target account.
	seeder, err := NewFanoutSeeder(ctx, fanoutDBURL)
	if err != nil {
		return fmt.Errorf("fanout: db: %w", err)
	}
	acc, err := seeder.GetLocalAccount(ctx, fanoutUsername)
	if err != nil {
		return fmt.Errorf("fanout: get account: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Target account: %s (%s)\n", acc.Username, acc.ID)

	// Start mock inbox servers.
	fmt.Fprintf(os.Stderr, "Starting %d mock inbox servers...\n", fanoutInstances)
	mockServers := make([]*MockInboxServer, fanoutInstances)
	inboxURLs := make([]string, fanoutInstances)
	totalBuf := fanoutFollowers + 100
	for i := range fanoutInstances {
		ms, err := NewMockInboxServer(totalBuf, fanoutHostIP)
		if err != nil {
			return fmt.Errorf("fanout: mock server %d: %w", i, err)
		}
		mockServers[i] = ms
		inboxURLs[i] = ms.InboxURL
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, ms := range mockServers {
			_ = ms.Shutdown(shutCtx)
		}
	}()

	// Seed followers.
	fmt.Fprintf(os.Stderr, "Seeding %d followers...\n", fanoutFollowers)
	accountIDs, err := seeder.SeedFollowers(ctx, acc.ID, fanoutFollowers, inboxURLs)
	if err != nil {
		return fmt.Errorf("fanout: seed: %w", err)
	}
	if fanoutCleanup {
		defer func() {
			cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fmt.Fprintf(os.Stderr, "Cleaning up %d seeded accounts...\n", len(accountIDs))
			if err := seeder.CleanupFollowers(cleanCtx, acc.ID, accountIDs); err != nil {
				fmt.Fprintf(os.Stderr, "cleanup error: %v\n", err)
			}
		}()
	}

	// Capture NATS lag before.
	nc, err := nats.Connect(fanoutNATSURL)
	if err != nil {
		return fmt.Errorf("fanout: nats connect: %w", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("fanout: jetstream: %w", err)
	}

	lagBefore, err := natsStreamLag(ctx, js, "ACTIVITYPUB_OUTBOUND_FANOUT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: NATS lag before: %v\n", err)
	}

	// POST a status to trigger fanout.
	fmt.Fprintf(os.Stderr, "Posting status to trigger fanout...\n")
	t0 := time.Now()
	if err := postStatus(ctx, fanoutServerURL, fanoutToken); err != nil {
		return fmt.Errorf("fanout: post status: %w", err)
	}

	// Wait for deliveries across all mock servers.
	timeoutCtx, cancel := context.WithTimeout(ctx, fanoutTimeout)
	defer cancel()

	var (
		mu            sync.Mutex
		firstDelivery time.Time
		lastDelivery  time.Time
		totalReceived int
	)

	// The fanout worker delivers to each distinct inbox URL once (shared-inbox optimisation).
	// With followers spread across fanoutInstances servers, the delivery count equals fanoutInstances.
	var wg sync.WaitGroup
	for _, ms := range mockServers {
		wg.Add(1)
		go func(ms *MockInboxServer) {
			defer wg.Done()
			records := ms.WaitForDeliveries(timeoutCtx, 1)
			mu.Lock()
			defer mu.Unlock()
			for _, rec := range records {
				totalReceived++
				if firstDelivery.IsZero() || rec.ReceivedAt.Before(firstDelivery) {
					firstDelivery = rec.ReceivedAt
				}
				if rec.ReceivedAt.After(lastDelivery) {
					lastDelivery = rec.ReceivedAt
				}
			}
		}(ms)
	}
	wg.Wait()

	lagAfter, err := natsStreamLag(ctx, js, "ACTIVITYPUB_OUTBOUND_FANOUT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: NATS lag after: %v\n", err)
	}

	var firstS, lastS, rate float64
	if !firstDelivery.IsZero() {
		firstS = firstDelivery.Sub(t0).Seconds()
		lastS = lastDelivery.Sub(t0).Seconds()
		span := lastDelivery.Sub(firstDelivery).Seconds()
		if span > 0 {
			rate = float64(totalReceived) / span
		}
	}

	result := FanoutResult{
		FollowersSeeded:    fanoutFollowers,
		Instances:          fanoutInstances,
		DeliveriesExpected: fanoutInstances,
		DeliveriesReceived: totalReceived,
		FirstDeliveryS:     firstS,
		LastDeliveryS:      lastS,
		DeliveryRate:       rate,
		NATSLagBefore:      lagBefore,
		NATSLagAfter:       lagAfter,
	}

	PrintFanoutResult(os.Stdout, result, jsonOutput)
	return nil
}

func natsStreamLag(ctx context.Context, js jetstream.JetStream, streamName string) (uint64, error) {
	info, err := js.Stream(ctx, streamName)
	if err != nil {
		return 0, fmt.Errorf("stream info %s: %w", streamName, err)
	}
	si, err := info.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("stream state %s: %w", streamName, err)
	}
	return si.State.Msgs, nil
}

func postStatus(ctx context.Context, serverURL, token string) error {
	body, _ := json.Marshal(map[string]string{
		"status":     "loadtest fanout ping",
		"visibility": "public",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		serverURL+"/api/v1/statuses", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

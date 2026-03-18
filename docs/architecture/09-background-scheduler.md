# Background scheduler

This document describes the distributed background job scheduler.

## Problem

Several operations need to run periodically — publishing scheduled statuses, refreshing trending indexes, fetching link preview cards, cleaning up old outbox events. In a multi-pod deployment, these jobs must run **exactly once per interval** across all replicas, not once per pod.

## Design

The scheduler uses NATS JetStream for distributed coordination. Each registered job gets two goroutines:

1. **Publisher** — fires on a `time.Ticker` at the job's interval and publishes a "tick" message to the `SCHEDULER` stream. The message ID encodes the job name and the time slot (interval-truncated Unix timestamp), so NATS deduplication ensures only one tick per interval is accepted, regardless of how many pods publish.
2. **Consumer** — pulls from a durable per-job consumer. When a tick arrives, it calls the job's handler. On success it ACKs; on failure it NAKs with exponential backoff (30s, 2m). Max 3 delivery attempts.

This means every pod publishes ticks (redundantly, deduplicated by NATS), but only one pod processes each tick (work-queue retention with `MaxAckPending: 1`).

### Why not cron or a single leader?

- Cron requires a persistent scheduler process or external cron service. The NATS approach is embedded and stateless — any pod can drive the scheduler.
- Leader election adds complexity and a failure mode (leader crashes, election delay). The NATS dedup + work-queue pattern achieves the same result without explicit coordination.

## NATS stream

| Setting | Value |
|---------|-------|
| Stream | `SCHEDULER` |
| Subjects | `scheduler.tick.>` |
| Retention | WorkQueue (message deleted after ACK) |
| Dedup window | 5 minutes |
| Max age | 10 minutes |
| Storage | File |

Per-job consumers are created dynamically when `Start` is called, not statically in the stream config. Each consumer filters on `scheduler.tick.{jobName}`.

## Registered jobs

| Job name | Interval | Purpose |
|----------|----------|---------|
| `scheduled-statuses` | 1 minute | Publishes statuses whose scheduled time has passed |
| `trending-indexes` | 5 minutes | Refreshes pre-computed trending status and hashtag indexes |
| `pending-cards` | 1 minute | Fetches link preview cards for recent statuses |
| `cleanup-outbox-events` | 1 hour | Deletes published outbox events older than 24 hours |

Jobs that process variable-size backlogs (scheduled statuses, pending cards) use a drain-batches pattern: the handler processes items in fixed-size batches until fewer items than the batch size are returned.

## Failure handling

- **Handler error** — NAK with backoff; NATS redelivers up to 3 times.
- **Handler panic** — recovered; NAK so the message is redelivered.
- **Tick publish failure** — logged; the next tick interval will try again.
- **NATS unavailable** — ticks are not published; when NATS recovers, the next tick fires normally. No persistent queue of missed ticks is needed because jobs are idempotent (they process whatever is due at execution time).

## Key files

| File | Responsibility |
|------|----------------|
| `internal/scheduler/scheduler.go` | Scheduler interface, NATS-backed implementation |
| `internal/scheduler/streams.go` | `SCHEDULER` stream configuration |
| `internal/scheduler/jobs/jobs.go` | Job handler factories |

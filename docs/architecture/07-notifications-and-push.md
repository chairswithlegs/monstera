# Notifications and Web Push

This document describes how notifications are created reactively from domain events and how Web Push delivery works.

## Overview

Notifications are not created inline by services or inbox handlers. Instead, two event-driven subscribers handle the full lifecycle:

1. **Notification subscriber** — consumes domain events and creates notification records.
2. **Push delivery subscriber** — consumes `notification.created` events and delivers Web Push messages to subscribed devices.

This keeps the service layer and inbox free of notification logic and makes it easy to add new notification triggers without touching existing code.

## Notification creation

### Why event-driven?

Earlier designs created notifications inline (e.g. the inbox handler would call the notification service after processing a Follow). This had several problems:

- Notification logic was scattered across inbox handlers and service methods.
- Adding a new notification trigger meant editing multiple call sites.
- Failures in notification creation could interfere with the primary operation.

Moving to a subscriber centralizes the rules ("who gets notified for what") in one place and decouples notification delivery from the request path.

### Notification subscriber

`internal/events/notification_subscriber.go` consumes from the `domain-events-notifications` durable consumer on the `DOMAIN_EVENTS` stream. It reacts to these events:

| Event | Notification type | Recipient |
|-------|-------------------|-----------|
| `follow.created` | `follow` | Target account (if local) |
| `follow.requested` | `follow_request` | Target account (if local) |
| `favourite.created` | `favourite` | Status author (if local, not self) |
| `reblog.created` | `reblog` | Original status author (if local, not self) |
| `status.created` / `status.created.remote` | `mention` | Each local mentioned account (not self, not conversation-muted) |

All other event types are acknowledged and skipped.

### Locality filtering

The subscriber only creates notifications for **local** recipients. Remote accounts do not have users on this instance, so notifications for them are meaningless. Locality is checked via `account.Domain == nil`.

### Conversation muting

For mention notifications, the subscriber checks whether the recipient has muted the conversation containing the status. If so, the notification is suppressed.

## Web Push delivery

### Subscription model

Each push subscription is tied to a specific **OAuth access token**. When a client registers for push via `POST /api/v1/push/subscription`, it provides:

- **Endpoint** — the push service URL (e.g. `https://fcm.googleapis.com/...`)
- **Keys** — P-256 DH public key and auth secret for payload encryption
- **Alerts** — per-notification-type toggles (follow, favourite, reblog, mention, follow_request)
- **Policy** — `all`, `followed`, `follower`, or `none`

The subscription is stored in the database keyed by access token ID. One account can have multiple subscriptions (one per client/device).

### Delivery flow

```
notification.created event
        │
        ▼
Push Delivery Subscriber
        │
        ├── Look up push subscriptions for recipient account
        │
        ├── For each subscription:
        │     ├── Check if alert type is enabled
        │     ├── Build JSON payload (notification ID, type, title, body)
        │     └── Send via VAPID-signed Web Push (RFC 8030/8291/8292)
        │
        └── If endpoint returns 410 Gone → delete the subscription
```

The push delivery subscriber consumes from `domain-events-push-delivery`, which is filtered to `domain.events.notification.>` subjects. This means it only wakes up for notification events, not all domain events.

### VAPID authentication

Outbound push messages are signed with VAPID (Voluntary Application Server Identification). The server holds a VAPID key pair; the public key is given to clients during subscription so the push service can verify the sender. The `internal/webpush` package wraps the signing and delivery.

### Gone subscriptions

Push endpoints are ephemeral — a user may uninstall the app or revoke browser permissions. When a push service returns HTTP 410 Gone, the subscriber deletes the subscription to avoid future wasted delivery attempts.

## Mastodon API endpoints

The push subscription endpoints follow the Mastodon API specification:

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/push/subscription` | Create subscription for the current access token |
| GET | `/api/v1/push/subscription` | Get current subscription (404 if none) |
| PUT | `/api/v1/push/subscription` | Update alerts and policy |
| DELETE | `/api/v1/push/subscription` | Remove subscription |

Handlers live in `internal/api/mastodon/push.go`. The service layer is `internal/service/push_subscription_service.go`.

## Key files

| File | Responsibility |
|------|----------------|
| `internal/events/notification_subscriber.go` | Domain events → notification creation |
| `internal/events/push_delivery_subscriber.go` | Notification events → Web Push delivery |
| `internal/service/push_subscription_service.go` | Push subscription CRUD |
| `internal/webpush/webpush.go` | VAPID-signed Web Push sender |
| `internal/domain/push_subscription.go` | Domain types (PushSubscription, PushAlerts) |
| `internal/api/mastodon/push.go` | Mastodon API handlers |

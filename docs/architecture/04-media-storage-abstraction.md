# Media storage abstraction

This document describes the desired media store interface and drivers (local, S3).

---

## Design decisions

| Question | Decision |
|----------|----------|
| Storage key ID | **nanoid** (`github.com/jaevor/go-nanoid`) — short, URL-safe, no time-sortability needed |
| Storage key format | `media/{year}/{month}/{day}/{nanoid}{ext}` — date-sharded for filesystem and S3 listing |
| Upload buffering | Buffer entire body in memory (≤ `MEDIA_MAX_BYTES`, default 10 MB) — simplifies content detection, blurhash, and dimension extraction |
| Blurhash | Synchronous for images in Phase 1; skipped for video/audio (Phase 2) |
| S3 endpoint override | `s3.Options.BaseEndpoint` + `UsePathStyle: true` — the modern AWS SDK v2 approach |
| S3 URL strategy | CDN base URL if `MEDIA_CDN_BASE` is set; otherwise presigned `GetObject` (1-hour TTL) |
| Partial upload rollback | Best-effort `Delete` on storage key when DB insert fails; documented as non-atomic |
| Local file write safety | Write to `.tmp` file then `os.Rename` — prevents serving incomplete files |

---

## File layout and responsibilities

- **Media store interface**: `internal/media/store.go` defines the `MediaStore` interface, storage key strategy, and configuration used by all drivers.
- **Local driver**: `internal/media/local` implements filesystem-backed storage for dev/small deployments; files are served under a `/system/...` prefix on the main HTTP server.
- **S3 driver**: `internal/media/s3` implements S3-compatible storage for larger/multi-replica deployments, optionally fronted by a CDN.
- **Service layer**: `internal/service/media_service.go` coordinates uploads, validates size/type, updates `media_attachments`, and hands URLs to presenters.
- **HTTP API**: `internal/api/mastodon/media.go` exposes the Mastodon media endpoints (`POST /api/v2/media`, `GET/PUT /api/v1/media/:id`) and delegates to the media service.

---

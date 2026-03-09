# Media Storage Layer

Design doc: `docs/architecture/01-high-level-system-architecture.md` (media store as component)

## Conventions

- Implements the `media.MediaStore` interface (`Put`, `Get`, `Delete`, `URL`).
- Returns `media.ErrNotFound` when a storage key doesn't exist.
- Storage keys are nanoid-generated, not ULIDs (no time-sortability needed).
- Two drivers: `local` (filesystem, dev) and `s3` (AWS SDK v2, prod).
- BlurHash generation uses `github.com/buckket/go-blurhash`.
- Media processing (resize, metadata extraction) happens in the service layer, not the store driver.

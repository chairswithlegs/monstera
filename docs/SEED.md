# Database seeder (local testing)

The `monstera-fed-seed` binary seeds the database with example users so you can sign in via the OAuth page and use a client (e.g. Pinafore) for local testing.

## Usage

### Against the docker-compose stack

With the stack running (`docker compose up -d`), seed the database with one command:

```bash
make seed
```

This builds the seeder (if needed), then runs it with the same env as the compose app (postgres on `localhost:5433`, instance at `localhost:8080`).

### Manual run

1. Ensure migrations are applied and the same environment as `serve` is set (e.g. `.env` with `DATABASE_URL`, `INSTANCE_DOMAIN`, and other required vars).
2. Build and run the seeder:

   ```bash
   make build-seed
   ./bin/monstera-fed-seed
   ```

   Or with `go run`:

   ```bash
   go run ./cmd/monstera-fed-seed
   ```

3. Sign in at your instance’s OAuth page with one of the seeded accounts.

## Default test accounts

| Username | Email              | Password  | Role  |
|----------|--------------------|-----------|-------|
| admin    | admin@example.com  | password  | admin |
| alice    | alice@example.com  | password  | user  |

## Notes

- The seeder is a **separate executable**; production deployments do not ship or run it.
- Re-running the seeder is **idempotent**: existing users are left as-is (and confirmed if they were not already).
- Seed users and passwords are hardcoded for local/dev only.

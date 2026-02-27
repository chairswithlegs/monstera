# Two-instance federation stack

Runs two Monstera-fed instances (**app-a** and **app-b**) behind Caddy so you can test federation locally (follow, deliver activities, receive replies).

## Prerequisites

1. **Hostnames** — Add to `/etc/hosts`:
   ```
   127.0.0.1   monstera.local monstera2.local
   ```

2. **Build the seeder** — From the repo root, run:
   ```bash
   make build-seed
   ```

## Start the stack

From this directory (`local-federation`):

```bash
docker compose up -d
```

This starts:

- **Caddy** — HTTPS on 443 for `monstera.local` and `monstera2.local`
- **app-a** — Monstera-fed for `monstera.local` (DB: postgres-a)
- **app-b** — Monstera-fed for `monstera2.local` (DB: postgres-b)
- **postgres-a** — DB for app-a (host port 5433)
- **postgres-b** — DB for app-b (host port 5434)
- **nats-a** — NATs (host port 4222)
- **nats-b** - NATs (host port 4223)

## Migrations and seed

From the **repo root**, run migrations and seed for each instance:

```bash
# Instance A (monstera.local)
DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
  ./bin/monstera-fed migrate up

INSTANCE_DOMAIN=monstera.local \
DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
NATS_URL="nats://localhost:4222" \
MEDIA_BASE_URL="https://monstera.local/media" \
MEDIA_LOCAL_PATH=./data/media \
EMAIL_FROM=noreply@monstera.local \
SECRET_KEY_BASE="0000000000000000000000000000000000000000000000000000000000000001" \
./bin/seed

# Instance B (monstera2.local)
DATABASE_URL="postgres://monstera:monstera@localhost:5434/monstera_fed?sslmode=disable" \
  ./bin/monstera-fed migrate up

INSTANCE_DOMAIN=monstera2.local \
DATABASE_URL="postgres://monstera:monstera@localhost:5434/monstera_fed?sslmode=disable" \
NATS_URL="nats://localhost:4223" \
MEDIA_BASE_URL="https://monstera2.local/media" \
MEDIA_LOCAL_PATH=./data/media \
EMAIL_FROM=noreply@monstera2.local \
SECRET_KEY_BASE="0000000000000000000000000000000000000000000000000000000000000002" \
./bin/seed
```

(If `make seed` uses different env var names, adjust accordingly; the seed binary reads from the same config as the app.)

## Use

- **Instance A:** https://monstera.local — accept the self-signed certificate when prompted.
- **Instance B:** https://monstera2.local

Default seeded users (from `make seed`) are typically the same on both (e.g. `admin` / `password`, `alice` / `password`).

### Getting the two instances to talk (e.g. from Pinafore)

1. **Connect Pinafore to one instance**  
   In Pinafore, add the instance URL (e.g. `https://monstera.local`) and accept the self-signed cert. Log in as `admin` / `password` or `alice` / `password`.

2. **Follow a user on the other instance**  
   Use the search (magnifying glass or search field) and type the **full handle** of a user on the other server:
   - If you’re on **monstera.local**, search for: `alice@monstera2.local` (or `admin@monstera2.local`).
   - If you’re on **monstera2.local**, search for: `alice@monstera.local` (or `admin@monstera.local`).  
   Pinafore will do WebFinger and load the profile. Click **Follow**.

3. **Accept the follow (on the other instance)**  
   Monstera-fed accepts follows automatically, so the follow should show as accepted. You can confirm by opening the other instance in another tab (e.g. https://monstera2.local), logging in as that user, and checking the followers list.

4. **Post from one and see it on the other**  
   From the account you’re logged in as (e.g. on monstera.local), post a status. Then open the other instance (monstera2.local) in the same or another client, log in as the user you followed (e.g. alice), and check the **Home** timeline. The post from the first instance should appear there (delivered via the federation worker).

5. **Reply from the other instance**  
   From the second instance, reply to that post. The reply is sent to the first instance’s inbox; the original author should get a notification and see the reply in their timeline.

If search doesn’t find the remote user, check that both instances are up and that you’re using the exact handle `username@domain` (e.g. `alice@monstera2.local`). You can also try opening `https://monstera.local/.well-known/webfinger?resource=acct:alice@monstera.local` in a browser to confirm WebFinger works.

## TLS and self-signed certs

Caddy uses `local_certs`, so it serves self-signed certificates for `monstera.local` and `monstera2.local`. Browsers will prompt to accept them. The stack sets `APP_ENV=development`, so the app defaults `FEDERATION_INSECURE_SKIP_TLS_VERIFY=true`: the federation HTTP client (WebFinger and actor fetch) skips TLS verification. Search for remote users (e.g. `admin@monstera2.local`) from one instance therefore works without extra setup. In production, leave `APP_ENV=production` so TLS is verified.

## Stop

```bash
docker compose down
```

Data in named volumes (`pg_a_data`, `pg_b_data`, `app_a_media`, `app_b_media`, `nats_data`) persists. Use `docker compose down -v` to remove volumes.

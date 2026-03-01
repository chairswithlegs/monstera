# Federation testing

Runs two Monstera-fed instances (**app-a** and **app-b**) behind Caddy so you can test federation locally (follow, deliver activities, receive replies).

## Prerequisites

1. **Hostnames** — Add to `/etc/hosts`:
   ```
   127.0.0.1   monstera.local monstera2.local
   ```

## Start the stack

From this directory (`federation-testing`):

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

From this directory, run the script:

```bash
./migrate-and-seed.sh
```

## Use

Open your browser and navigate to:

- **Instance A:** https://monstera.local — accept the self-signed certificate when prompted.
- **Instance B:** https://monstera2.local — do the same.


## Use a client to connect to the app

Using a client such a Pinafore or tusky, connect to the app at monstera.local. Federation should work between the two instances.

## TLS and self-signed certs

Caddy uses `local_certs`, so it serves self-signed certificates for `monstera.local` and `monstera2.local`. Browsers will prompt to accept them. The stack sets `APP_ENV=development`, so the app defaults `FEDERATION_INSECURE_SKIP_TLS_VERIFY=true`: the federation HTTP client (WebFinger and actor fetch) skips TLS verification. Search for remote users (e.g. `admin@monstera2.local`) from one instance therefore works without extra setup. In production, leave `APP_ENV=production` so TLS is verified.

## Stop

```bash
docker compose down
```

Data in named volumes (`pg_a_data`, `pg_b_data`, `app_a_media`, `app_b_media`, `nats_data`) persists. Use `docker compose down -v` to remove volumes.

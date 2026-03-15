# Federation testing

Runs two Monstera instances behind Caddy so you can test federation locally (follow, deliver activities, receive replies).

## Prerequisites

Hostname resolution is required for this work correctly. On linux, you can configure it like so:

**Hostnames** — Add to `/etc/hosts`:
   ```
   127.0.0.1   monstera.local monstera2.local
   ```

## Start the stack

From the repo root:

```bash
docker compose -f federation-testing/docker-compose.yaml up --build -d
```

Next, you can seed the users by running:

```bash
# Seed an admin user in monstera.local (email: admin@example.com, password: password)
docker compose -f federation-testing/docker-compose.yaml exec server-a ./server user create admin admin@example.com password Admin admin

# Seed an admin user in monstera2.local (email: admin@example.com, password: password)
docker compose -f federation-testing/docker-compose.yaml exec server-b ./server user create admin admin@example.com password Admin admin
```
To **reset the databases** and start from scratch (drops all data), remove volumes then bring the stack back up:

```bash
docker compose -f federation-testing/docker-compose.yaml down -v
docker compose -f federation-testing/docker-compose.yaml up --build -d
```

## Use

Open your browser and navigate to:

- **Instance A:** https://monstera.local — accept the self-signed certificate when prompted.
- **Instance B:** https://monstera2.local — do the same.


## Use a client to connect to the app

The docker compose stack brings up a local instance of Elk at localhost:5314. Navigate to the URL and trying signing in. Federation should work between the two instances.

## TLS and self-signed certs

Caddy uses `local_certs`, so it serves self-signed certificates for `monstera.local` and `monstera2.local`. Browsers will prompt to accept them. The stack sets `APP_ENV=development`, so the app defaults `FEDERATION_INSECURE_SKIP_TLS_VERIFY=true`: the federation HTTP client (WebFinger and actor fetch) skips TLS verification. Search for remote users (e.g. `admin@monstera2.local`) from one instance therefore works without extra setup.

## Stop

```bash
docker compose -f federation-testing/docker-compose.yaml down
```

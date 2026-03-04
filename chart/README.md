# Monstera Helm Chart

Helm chart for deploying [Monstera](https://github.com/chairswithlegs/monstera): the server (API), the UI, and optionally PostgreSQL and NATS. When PostgreSQL or NATS are disabled, you supply your own instances and set the connection details in the server Secret.

## Prerequisites

- Kubernetes 1.23+
- Helm 3.x
- (Optional) Ingress controller if you use the chart ingress

## Creating the server Secret (required)

The server and the optional migration Job read **all** configuration from a single Secret. The chart does **not** create this Secret; you must create it before installing or upgrading. The server and migration Job will not start until this Secret exists.

### Required keys

Your Secret must contain at least these keys (as plain key/value; the server reads them as environment variables):

| Key | Description |
|-----|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `NATS_URL` | NATS server URL (with credentials if NATS uses auth) |
| `SECRET_KEY_BASE` | 64+ hex characters for signing and key derivation |
| `INSTANCE_DOMAIN` | Public hostname (e.g. `social.example.com`) |
| `MEDIA_BASE_URL` | Base URL for media (e.g. `https://social.example.com/media`) |
| `EMAIL_FROM` | From address for outgoing email |

Optional keys (see Monstera docs / `internal/config`): `MEDIA_DRIVER`, `MEDIA_LOCAL_PATH`, `EMAIL_DRIVER`, `APP_ENV`, `APP_PORT`, `LOG_LEVEL`, `UI_DOMAIN`, `NATS_CREDS_FILE`, and others.

**Important:** `SECRET_KEY_BASE` must be at least 64 hex characters (32 bytes). Generate one with:

```bash
openssl rand -hex 32
```

### When using the chart’s PostgreSQL

- The service name is `<release-name>-postgresql` (e.g. `monstera-postgresql` if release is `monstera`).
- Use the same username, password, and database as in `postgresql.auth` in your values.
- Example (replace `<release>`, `<namespace>`, user, password, and db with your values):

  ```
  postgres://monstera:YOUR_PASSWORD@<release>-postgresql.<namespace>.svc.cluster.local:5432/monstera_fed?sslmode=disable
  ```

  In the same namespace you can use the short form:

  ```
  postgres://monstera:YOUR_PASSWORD@<release>-postgresql:5432/monstera_fed?sslmode=disable
  ```

- PostgreSQL is always deployed with username/password authentication; do not use an empty or trust URL.

### When using the chart’s NATS

- The service name is `<release-name>-nats` (e.g. `monstera-nats`).
- NATS is configured with authentication (username/password in the chart’s `nats.config.merge.authorization`). Use the **same** username and password in `NATS_URL`.
- Example:

  ```
  nats://monstera:YOUR_NATS_PASSWORD@<release>-nats:4222
  ```

- Override the default NATS password in values (e.g. `nats.config.merge.authorization.users[0].password`) and use that same value in your Secret’s `NATS_URL`.

### Example: create the Secret with kubectl

Create the Secret in the same namespace where you will install the chart. The name must match `server.existingSecret` in your values (e.g. `monstera-server-env`).

```bash
kubectl create secret generic monstera-server-env \
  --from-literal=DATABASE_URL='postgres://monstera:YOUR_DB_PASSWORD@monstera-postgresql:5432/monstera_fed?sslmode=disable' \
  --from-literal=NATS_URL='nats://monstera:YOUR_NATS_PASSWORD@monstera-nats:4222' \
  --from-literal=SECRET_KEY_BASE="$(openssl rand -hex 32)" \
  --from-literal=INSTANCE_DOMAIN=social.example.com \
  --from-literal=MEDIA_BASE_URL=https://social.example.com/media \
  --from-literal=EMAIL_FROM=noreply@example.com
```

Or from an env file (no quotes in the file; one `KEY=value` per line):

```bash
kubectl create secret generic monstera-server-env --from-env-file=monstera.env
```

## Install

1. Create the server Secret (see above) in the target namespace.
2. Add the chart (if using from repo) or use the local `chart/` directory.
3. Update dependencies and install:

```bash
cd chart
helm dependency update
helm install monstera . -f myvalues.yaml -n <namespace>
```

Set `server.existingSecret` in `myvalues.yaml` to the name of the Secret you created (e.g. `monstera-server-env`).

## Configuration

### Key values

| Value | Description |
|-------|-------------|
| `server.existingSecret` | **Required.** Name of the Secret containing server env (DATABASE_URL, NATS_URL, etc.). |
| `postgresql.enabled` | If `true`, deploy PostgreSQL with the chart. If `false`, you must provide `DATABASE_URL` in your Secret. |
| `nats.enabled` | If `true`, deploy NATS with the chart (JetStream + auth). If `false`, you must provide `NATS_URL` in your Secret. |
| `instanceDomain` | Instance hostname (can also be set only in the Secret as `INSTANCE_DOMAIN`). |
| `ui.config.server_url` | Public API URL for the UI (e.g. `https://social.example.com`). |
| `media.driver` | `local` (chart provisions a PVC) or `s3`. |
| `media.persistence.storageClass` | Optional. StorageClass for the local media PVC; omit for cluster default. |
| `media.persistence.size` | Size of the local media PVC (e.g. `10Gi`). |
| `ingress.enabled` | Set to `true` and set `ingress.hosts` (and optionally `ingress.tls`) to expose the app. |
| `migrationJob.enabled` | If `true`, run a pre-install/pre-upgrade Job that executes migrations. |

### Securing PostgreSQL and NATS

- **PostgreSQL:** The chart always configures Bitnami PostgreSQL with username/password (`postgresql.auth`). Use the same credentials in your Secret’s `DATABASE_URL`. For production, consider `postgresql.auth.existingSecret` so the password comes from an existing Secret.
- **NATS:** The chart enables NATS with authentication via `nats.config.merge.authorization.users`. Override the default password and use the same credentials in your Secret’s `NATS_URL`.

### Local media and StorageClass

When `media.driver` is `local`, the chart creates a PVC for the server. You can optionally set `media.persistence.storageClass` to use a specific StorageClass; leave it empty to use the cluster default. Set `media.persistence.size` (e.g. `10Gi`) as needed.

### External PostgreSQL or NATS

Set `postgresql.enabled: false` or `nats.enabled: false` and put the correct `DATABASE_URL` or `NATS_URL` in your server Secret (pointing at your own Postgres/NATS).

## Upgrading

1. Ensure the server Secret exists and is up to date.
2. Run `helm dependency update` if Chart.yaml or dependencies changed.
3. Run `helm upgrade monstera . -f myvalues.yaml -n <namespace>`.

The migration Job (if enabled) runs as a pre-upgrade hook and will run before the new server pods are created.

## Uninstall

```bash
helm uninstall monstera -n <namespace>
```

Note: The server Secret you created is not removed by Helm; delete it manually if desired.

## More

- Application configuration and env vars: see the main [Monstera README](https://github.com/chairswithlegs/monstera) and `internal/config` in the repo.

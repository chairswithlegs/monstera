#!/usr/bin/env bash
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Instance A (monstera.local): migrate..."
DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
  go run ./cmd/server migrate up

echo "Instance A (monstera.local): seed..."
INSTANCE_DOMAIN=monstera.local \
  DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
  MONSTERA_UI_URL="http://localhost:3000" \
  NATS_URL="nats://localhost:4222" \
  MEDIA_BASE_URL="https://monstera.local/media" \
  MEDIA_LOCAL_PATH=./data/media \
  EMAIL_FROM=noreply@monstera.local \
  SECRET_KEY_BASE="0000000000000000000000000000000000000000000000000000000000000001" \
  go run ./cmd/seed

echo "Instance B (monstera2.local): migrate..."
DATABASE_URL="postgres://monstera:monstera@localhost:5434/monstera_fed?sslmode=disable" \
  go run ./cmd/server migrate up

echo "Instance B (monstera2.local): seed..."
INSTANCE_DOMAIN=monstera2.local \
  DATABASE_URL="postgres://monstera:monstera@localhost:5434/monstera_fed?sslmode=disable" \
  MONSTERA_UI_URL="http://localhost:3001" \
  NATS_URL="nats://localhost:4223" \
  MEDIA_BASE_URL="https://monstera2.local/media" \
  MEDIA_LOCAL_PATH=./data/media \
  EMAIL_FROM=noreply@monstera2.local \
  SECRET_KEY_BASE="0000000000000000000000000000000000000000000000000000000000000002" \
  go run ./cmd/seed

echo "Done."

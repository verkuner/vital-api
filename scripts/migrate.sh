#!/usr/bin/env bash
set -euo pipefail

# Production migration helper using golang-migrate.
# Usage:
#   ./scripts/migrate.sh up          # Apply all pending migrations
#   ./scripts/migrate.sh down        # Rollback one step
#   ./scripts/migrate.sh version     # Show current version
#   ./scripts/migrate.sh create NAME # Create new migration pair

MIGRATIONS_PATH="${MIGRATIONS_PATH:-internal/database/migrations}"
DATABASE_URL="${DATABASE_URL:?DATABASE_URL environment variable required}"

command="${1:?Usage: migrate.sh <up|down|version|create> [name]}"

case "$command" in
  up)
    echo "Applying all pending migrations..."
    migrate -database "$DATABASE_URL" -path "$MIGRATIONS_PATH" up
    echo "Done."
    ;;
  down)
    echo "Rolling back one migration..."
    migrate -database "$DATABASE_URL" -path "$MIGRATIONS_PATH" down 1
    echo "Done."
    ;;
  version)
    migrate -database "$DATABASE_URL" -path "$MIGRATIONS_PATH" version
    ;;
  create)
    name="${2:?Usage: migrate.sh create <name>}"
    migrate create -ext sql -dir "$MIGRATIONS_PATH" -seq "$name"
    echo "Created migration: $name"
    ;;
  *)
    echo "Unknown command: $command"
    echo "Usage: migrate.sh <up|down|version|create> [name]"
    exit 1
    ;;
esac

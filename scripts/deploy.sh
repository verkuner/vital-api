#!/usr/bin/env bash
# Deploy vital-api to production VM.
# Usage:
#   ./scripts/deploy.sh          # full deploy (build + transfer + restart all)
#   ./scripts/deploy.sh api      # re-transfer image and restart api container only
#   ./scripts/deploy.sh infra    # sync compose files and restart infra only (no build)

set -euo pipefail

VM_USER=ubuntu
VM_HOST=101.42.46.218
VM_KEY="${HOME}/.ssh/vital-api"
VM_DIR="~/vital-api"
IMAGE="vital-api:latest"
COMPOSE_FILE="deployments/docker/docker-compose.yml"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
step()  { echo -e "\n${GREEN}==>${NC} $1"; }
warn()  { echo -e "${YELLOW}[warn]${NC} $1"; }
die()   { echo -e "${RED}[error]${NC} $1" >&2; exit 1; }

# Detect Docker socket (Rancher Desktop uses a non-standard path)
if [[ -z "${DOCKER_HOST:-}" ]]; then
  if [[ -S "${HOME}/.rd/docker.sock" ]]; then
    export DOCKER_HOST="unix://${HOME}/.rd/docker.sock"
  elif [[ -S "/var/run/docker.sock" ]]; then
    export DOCKER_HOST="unix:///var/run/docker.sock"
  fi
fi
export PATH="${HOME}/.rd/bin:/usr/local/bin:${PATH}"

ssh_vm() { ssh -i "$VM_KEY" "$VM_USER@$VM_HOST" "$@"; }

check_deps() {
  for cmd in encore docker rsync ssh; do
    command -v "$cmd" &>/dev/null || die "Required command not found: $cmd"
  done
}

build_image() {
  step "Building Docker image via Encore..."
  encore build docker --config=deployments/docker/infra.config.json "$IMAGE"
}

transfer_image() {
  step "Transferring image to VM (this may take a minute)..."
  docker save "$IMAGE" | gzip | ssh_vm "gunzip | docker load"
}

sync_files() {
  step "Syncing deployment files to VM..."
  ssh_vm "mkdir -p $VM_DIR"
  rsync -az --delete -e "ssh -i $VM_KEY" \
    deployments/docker/ \
    "$VM_USER@$VM_HOST:$VM_DIR/"
  # Warn if .env is missing on VM — required for secrets
  ssh_vm "test -f $VM_DIR/.env" || \
    warn ".env not found on VM at $VM_DIR/.env — create it from .env.production.example"
}

restart_all() {
  step "Starting all services..."
  ssh_vm "cd $VM_DIR && docker compose up -d --remove-orphans"
}

restart_api() {
  step "Restarting API container..."
  ssh_vm "cd $VM_DIR && docker compose up -d --no-deps --remove-orphans api"
}

restart_infra() {
  step "Restarting infra services (postgres, redis, keycloak, otel-collector)..."
  ssh_vm "cd $VM_DIR && docker compose up -d --remove-orphans \
    postgres redis keycloak otel-collector"
}

# ── Entrypoint ───────────────────────────────────────────────────────────────

check_deps

MODE="${1:-full}"

case "$MODE" in
  full)
    build_image
    transfer_image
    sync_files
    restart_all
    echo -e "\n${GREEN}Deploy complete.${NC} API → http://$VM_HOST:5080"
    ;;
  api)
    build_image
    transfer_image
    sync_files
    restart_api
    echo -e "\n${GREEN}API deploy complete.${NC} → http://$VM_HOST:5080"
    ;;
  infra)
    sync_files
    restart_infra
    echo -e "\n${GREEN}Infra deploy complete.${NC}"
    ;;
  *)
    die "Unknown mode '$MODE'. Use: full | api | infra"
    ;;
esac

#!/usr/bin/env bash
# Generate a self-signed TLS certificate on the VM for Nginx → Keycloak HTTPS.
# Usage: ./scripts/gen-certs.sh [IP_OR_HOST]
#
# The cert is written to ~/vital-api/certs/ on the VM and synced back
# to deployments/docker/certs/ locally so Nginx can mount it.
# The private key is never committed to git.

set -euo pipefail

VM_USER=ubuntu
VM_HOST=101.42.46.218
VM_KEY="${HOME}/.ssh/vital-api"
VM_DIR="~/vital-api"
LOCAL_CERTS="deployments/docker/certs"

# Detect Docker socket (Rancher Desktop)
if [[ -z "${DOCKER_HOST:-}" ]]; then
  if [[ -S "${HOME}/.rd/docker.sock" ]]; then
    export DOCKER_HOST="unix://${HOME}/.rd/docker.sock"
  fi
fi

HOST="${1:-$VM_HOST}"

GREEN='\033[0;32m'; NC='\033[0m'
step() { echo -e "\n${GREEN}==>${NC} $1"; }

step "Generating self-signed certificate for: $HOST"
ssh -i "$VM_KEY" "$VM_USER@$VM_HOST" "
  mkdir -p $VM_DIR/certs
  openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 \
    -nodes \
    -keyout $VM_DIR/certs/server.key \
    -out    $VM_DIR/certs/server.crt \
    -subj   '/CN=$HOST' \
    -addext 'subjectAltName=IP:$HOST' \
    -quiet 2>/dev/null || \
  openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 \
    -nodes \
    -keyout $VM_DIR/certs/server.key \
    -out    $VM_DIR/certs/server.crt \
    -subj   '/CN=$HOST'
  chmod 600 $VM_DIR/certs/server.key
  openssl x509 -in $VM_DIR/certs/server.crt -noout -subject -dates
"

step "Pulling certificate to local deployments/docker/certs/ (for git-ignored mount)"
mkdir -p "$LOCAL_CERTS"
scp -i "$VM_KEY" "$VM_USER@$VM_HOST:$VM_DIR/certs/server.crt" "$LOCAL_CERTS/server.crt"
# Private key stays on VM only — never committed to git.

echo -e "\n${GREEN}Done.${NC}"
echo "  VM cert:   $VM_DIR/certs/server.crt (+ server.key)"
echo "  Local CRT: $LOCAL_CERTS/server.crt (key NOT copied — VM only)"
echo ""
echo "To trust this cert on a device, transfer server.crt and install it as a CA."

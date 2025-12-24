#!/usr/bin/env bash
set -euo pipefail

AUTH_BASE="${AUTH_BASE:-http://localhost:8080}"
EMAIL="${EMAIL:-mqtest@example.com}"
RABBIT_CONTAINER="${RABBIT_CONTAINER:-cityevents-rabbitmq}"

WATCH_SECS="${WATCH_SECS:-25}"
SLEEP_SECS="${SLEEP_SECS:-1}"

# Peek behavior:
# - consume: ack_requeue_false (recommended; non-polluting for retry verification)
# - requeue: ack_requeue_true  (pollutes redelivered; use only when you must "peek without losing")
PEEK_MODE="${PEEK_MODE:-consume}"

log() { echo "[$(date +'%H:%M:%S')] $*"; }

ackmode() {
  case "$PEEK_MODE" in
    consume) echo "ack_requeue_false" ;;
    requeue) echo "ack_requeue_true" ;;
    *) echo "ack_requeue_false" ;;
  esac
}

consumers_snapshot() {
  docker exec "$RABBIT_CONTAINER" rabbitmqctl list_consumers 2>/dev/null \
    | tr -d '\r' \
    | awk 'NR==1 || $1 ~ /email-service\.q/ {print}'
}

queues_snapshot() {
  docker exec "$RABBIT_CONTAINER" rabbitmqctl list_queues \
      name messages_ready messages_unacknowledged messages 2>/dev/null \
    | tr -d '\r' \
    | awk '
      NR==1 ||
      $1 ~ /email-service\.(q|dlq|retry\.)/ ||
      $1 ~ /^debug\.(email|reset)$/ {print}
    '
}

# NEW: show queue arguments so we can verify:
# - retry queues have x-message-ttl + x-dead-letter-exchange
# - main queue has DLX safety net (optional)
queues_arguments_snapshot() {
  docker exec "$RABBIT_CONTAINER" rabbitmqctl list_queues name arguments 2>/dev/null \
    | tr -d '\r' \
    | awk '
      NR==1 ||
      $1 ~ /email-service\.(q|dlq|retry\.)/ {print}
    '
}

# NEW: show bindings to verify:
# - main queue bound to city.events with auth.* keys
# - retry queues bound to tier exchanges with "#"
# - dlq queue bound to dlx.final with rkFinalDLQ
bindings_snapshot() {
  docker exec "$RABBIT_CONTAINER" rabbitmqctl list_bindings \
    source_name destination_name destination_kind routing_key arguments 2>/dev/null \
    | tr -d '\r' \
    | awk '
      NR==1 ||
      $1 ~ /^city\.events/ ||
      $1 ~ /^city\.events\.dlx/ ||
      $2 ~ /email-service\.(q|dlq|retry\.)/ {print}
    '
}

topology_snapshot() {
  log ">>> Topology snapshot: queues (arguments)"
  queues_arguments_snapshot || echo "(no output)"
  echo
  log ">>> Topology snapshot: bindings"
  bindings_snapshot || echo "(no output)"
}

get_one_msg() {
  local q="$1"
  docker exec "$RABBIT_CONTAINER" rabbitmqadmin get \
    queue="$q" \
    count=1 \
    ackmode="$(ackmode)" \
    encoding=auto 2>/dev/null | tr -d '\r'
}

curl_verify_request() {
  log "POST /auth/v1/verify-email/request"
  curl -sS -i -X POST "${AUTH_BASE}/auth/v1/verify-email/request" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${EMAIL}\"}" | tr -d '\r'
}

curl_reset_request() {
  log "POST /auth/v1/password/reset/request"
  curl -sS -i -X POST "${AUTH_BASE}/auth/v1/password/reset/request" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${EMAIL}\"}" | tr -d '\r'
}

peek() {
  local q="$1"
  log ">>> Peek $q (ackmode=$(ackmode))"
  local out
  out="$(get_one_msg "$q" || true)"
  if [ -z "$out" ] || echo "$out" | grep -q "No items"; then
    echo "No items"
  else
    echo "$out"
  fi
  echo
}

log "=== MQ Retry Verification Script (Non-polluting) ==="
log "AUTH_BASE=${AUTH_BASE}"
log "EMAIL=${EMAIL}"
log "RABBIT_CONTAINER=${RABBIT_CONTAINER}"
log "PEEK_MODE=${PEEK_MODE}  (consume=non-polluting, requeue=polluting)"
log "WATCH_SECS=${WATCH_SECS} SLEEP_SECS=${SLEEP_SECS}"
log ""

log ">>> Consumers snapshot (before)"
consumers_snapshot || echo "(no output)"

log ""
log ">>> Initial queue snapshot"
queues_snapshot || echo "(no output)"

log ""
topology_snapshot

log ""
log ">>> Trigger verify-email request"
curl_verify_request

log ""
log ">>> Trigger password-reset request"
curl_reset_request

log ""
log ">>> Consumers snapshot (after triggers)"
consumers_snapshot || echo "(no output)"

log ""
log ">>> Watching queues for ${WATCH_SECS}s"
for ((i=1; i<=WATCH_SECS; i++)); do
  echo "---- t=${i}s ----"
  queues_snapshot || echo "(no output)"
  sleep "$SLEEP_SECS"
done

log ""
topology_snapshot

# Optional peeks (by default consume one message each to avoid redelivered pollution)
echo
peek "email-service.q"
peek "email-service.retry.10s"
peek "email-service.retry.1m"
peek "email-service.retry.10m"
peek "email-service.dlq"
peek "debug.email"
peek "debug.reset"

log "DONE"

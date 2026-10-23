#!/usr/bin/env bash
# Generates a website-issued DeepSeek device_id on a Linux host.
#
# Requirements:
#   - A real Chromium/Chrome binary (headful, not --headless).
#   - An X display. On a headless NAS, use the existing KasmVNC/Xvfb display
#     (for example DISPLAY=:12) or start Xvfb yourself.
#   - Node.js 22+ or Docker with the node:22-alpine image.
#
# The helper maps fp-it-acc.portal101.cn to a public IP because some NAS DNS
# resolvers return 0.0.0.0 for it, which makes the official SMSdk fall back to
# a non-login D... payload instead of a B... device id.
#
# Usage:
#   scripts/deepseek-device-linux.sh [--output FILE] [--timeout SECONDS]
#       [--display :12] [--browser PATH] [--api-ip IP] [--port 9222]
#       [--profile DIR] [--keep-profile]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CDP_SCRIPT="$ROOT_DIR/scripts/deepseek-device-cdp.mjs"
OUTPUT=""
TIMEOUT=120
DISPLAY_VALUE="${DS2API_DEVICE_DISPLAY:-${DISPLAY:-}}"
BROWSER="${DS2API_BROWSER_PATH:-}"
API_IP="${DS2API_DEVICE_API_IP:-}"
PORT="${DS2API_DEVICE_PORT:-9222}"
PROFILE=""
KEEP_PROFILE=0

usage() {
  sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    -o|--output) OUTPUT="${2:-}"; shift 2 ;;
    --timeout) TIMEOUT="${2:-}"; shift 2 ;;
    --display) DISPLAY_VALUE="${2:-}"; shift 2 ;;
    --browser) BROWSER="${2:-}"; shift 2 ;;
    --api-ip) API_IP="${2:-}"; shift 2 ;;
    --port) PORT="${2:-}"; shift 2 ;;
    --profile) PROFILE="${2:-}"; shift 2 ;;
    --keep-profile) KEEP_PROFILE=1; shift ;;
    *) echo "deepseek-device-linux: unknown option: $1" >&2; exit 1 ;;
  esac
done

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || (( TIMEOUT < 10 || TIMEOUT > 300 )); then
  echo "deepseek-device-linux: --timeout must be an integer between 10 and 300" >&2
  exit 1
fi
if ! [[ "$PORT" =~ ^[0-9]+$ ]] || (( PORT < 1 || PORT > 65535 )); then
  echo "deepseek-device-linux: invalid --port" >&2
  exit 1
fi
if [[ -n "$OUTPUT" && -e "$OUTPUT" ]]; then
  echo "deepseek-device-linux: refusing to overwrite existing output: $OUTPUT" >&2
  exit 1
fi

resolve_api_ip() {
  if [[ -n "$API_IP" ]]; then
    printf '%s\n' "$API_IP"
    return
  fi
  local ip=""
  if command -v getent >/dev/null 2>&1; then
    ip="$(getent ahostsv4 fp-it-acc.portal101.cn 2>/dev/null | awk '$1 != "0.0.0.0" {print $1; exit}')"
  fi
  if [[ -z "$ip" ]] && command -v nslookup >/dev/null 2>&1; then
    ip="$(nslookup fp-it-acc.portal101.cn 223.5.5.5 2>/dev/null | awk '/^Address: / && $2 !~ /#/ {print $2; exit}')"
  fi
  if [[ -z "$ip" ]]; then
    ip="183.232.4.46"
  fi
  printf '%s\n' "$ip"
}

find_browser() {
  if [[ -n "$BROWSER" ]]; then
    if [[ ! -x "$BROWSER" ]]; then
      echo "deepseek-device-linux: browser is not executable: $BROWSER" >&2
      exit 1
    fi
    printf '%s\n' "$BROWSER"
    return
  fi
  local candidate
  for candidate in \
    /vol1/@appcenter/fygo-browser/app/vendor/runtime/usr/lib/chromium/chromium \
    "$(command -v chromium 2>/dev/null || true)" \
    "$(command -v chromium-browser 2>/dev/null || true)" \
    "$(command -v google-chrome 2>/dev/null || true)" \
    "$(command -v google-chrome-stable 2>/dev/null || true)"; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  done
  echo "deepseek-device-linux: no Chromium/Chrome found; set DS2API_BROWSER_PATH" >&2
  exit 1
}

find_display() {
  if [[ -n "$DISPLAY_VALUE" ]]; then
    printf '%s\n' "$DISPLAY_VALUE"
    return
  fi
  local n
  for n in 12 11 10 99; do
    if [[ -S "/tmp/.X11-unix/X$n" ]]; then
      printf ':%s\n' "$n"
      return
    fi
  done
  echo "deepseek-device-linux: no X display found; start Xvfb or set --display" >&2
  exit 1
}

browser_ld_path() {
  local browser="$1"
  if [[ "$browser" == *"/fygo-browser/"* && "$browser" == *"/runtime/usr/lib/chromium/chromium" ]]; then
    local runtime="${browser%/lib/chromium/chromium}"
    printf '%s:%s\n' "$runtime/lib" "$runtime/lib/x86_64-linux-gnu"
  fi
}

run_cdp_client() {
  local timeout="$1"
  if command -v node >/dev/null 2>&1; then
    local major
    major="$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0)"
    if [[ "$major" =~ ^[0-9]+$ ]] && (( major >= 22 )); then
      node "$CDP_SCRIPT" --port "$PORT" --timeout "$timeout"
      return
    fi
  fi
  if command -v docker >/dev/null 2>&1; then
    docker run --rm --network host \
      -v "$CDP_SCRIPT:/tmp/deepseek-device-cdp.mjs:ro" \
      "${DS2API_NODE_IMAGE:-node:22-alpine}" \
      node /tmp/deepseek-device-cdp.mjs --port "$PORT" --timeout "$timeout"
    return
  fi
  echo "deepseek-device-linux: Node.js 22+ or Docker is required" >&2
  exit 1
}

BROWSER="$(find_browser)"
DISPLAY_VALUE="$(find_display)"
API_IP="$(resolve_api_ip)"
LD_PATH="$(browser_ld_path "$BROWSER")"

if curl -s --max-time 2 "http://127.0.0.1:$PORT/json/version" >/dev/null 2>&1; then
  echo "deepseek-device-linux: port $PORT is already in use; pass --port with a free port" >&2
  exit 1
fi

if [[ -z "$PROFILE" ]]; then
  PROFILE="$(mktemp -d "${TMPDIR:-/tmp}/ds2api-device-XXXXXX")"
  OWN_PROFILE=1
else
  mkdir -p "$PROFILE"
  OWN_PROFILE=0
fi
LOG_FILE="$PROFILE/chromium.log"
BROWSER_PID=""

cleanup() {
  if [[ -n "$BROWSER_PID" ]]; then
    kill "$BROWSER_PID" 2>/dev/null || true
    wait "$BROWSER_PID" 2>/dev/null || true
  fi
  if (( OWN_PROFILE == 1 && KEEP_PROFILE == 0 )); then
    rm -rf "$PROFILE"
  fi
}
trap cleanup EXIT INT TERM

echo "deepseek-device-linux: browser=$BROWSER display=$DISPLAY_VALUE api_ip=$API_IP port=$PORT" >&2

LAUNCH_ENV=(env "DISPLAY=$DISPLAY_VALUE")
if [[ -n "$LD_PATH" ]]; then
  LAUNCH_ENV+=("LD_LIBRARY_PATH=$LD_PATH")
fi
"${LAUNCH_ENV[@]}" "$BROWSER" \
  --no-sandbox --disable-dev-shm-usage --disable-gpu \
  --user-data-dir="$PROFILE" \
  --remote-debugging-address=127.0.0.1 --remote-debugging-port="$PORT" \
  --no-first-run --no-default-browser-check \
  --host-resolver-rules="MAP fp-it-acc.portal101.cn $API_IP" \
  "https://chat.deepseek.com/sign_in" >"$LOG_FILE" 2>&1 &
BROWSER_PID=$!

ready=0
for _ in $(seq 1 60); do
  if curl -s --max-time 2 "http://127.0.0.1:$PORT/json/version" >/dev/null 2>&1; then
    ready=1
    break
  fi
  if ! kill -0 "$BROWSER_PID" 2>/dev/null; then
    break
  fi
  sleep 0.5
done
if (( ready == 0 )); then
  echo "deepseek-device-linux: Chromium did not expose DevTools on port $PORT" >&2
  tail -30 "$LOG_FILE" >&2 || true
  exit 1
fi

RESULT="$(run_cdp_client "$TIMEOUT")" || {
  echo "deepseek-device-linux: device registration failed" >&2
  exit 1
}
DEVICE_ID="${RESULT#DEVICE_ID=}"
DEVICE_ID="${DEVICE_ID%%$'\n'*}"
if [[ ! "$DEVICE_ID" =~ ^B ]] || (( ${#DEVICE_ID} != 89 )); then
  echo "deepseek-device-linux: unexpected device id (length ${#DEVICE_ID}, prefix ${DEVICE_ID:0:1})" >&2
  exit 1
fi

if [[ -n "$OUTPUT" ]]; then
  umask 077
  printf '%s\n' "$DEVICE_ID" >"$OUTPUT"
  echo "deepseek-device-linux: device registration succeeded; saved to $OUTPUT" >&2
else
  printf '%s\n' "$DEVICE_ID"
  echo "deepseek-device-linux: device registration succeeded (${#DEVICE_ID} characters)" >&2
fi

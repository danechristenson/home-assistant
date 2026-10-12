#!/bin/sh
set -e

OPTIONS=/data/options.json
CONFIG=/tmp/livekit.yaml
SHARED=/share/matrix-group-calls/calls.conf
IP_CHECK_SECONDS=${IP_CHECK_SECONDS:-300}
HOSTNAME_PATTERN='^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$'

# Print an option's string value (simple values only; both options are
# validated below, so escape sequences never need handling)
option_text() {
    sed -nE "s/.*\"$1\": *\"([^\"]*)\".*/\1/p" "$OPTIONS" | head -n 1
}

# Only advertise the call service to Matrix while it is actually running
rm -f "$SHARED"

DOMAIN=$(option_text domain)
SERVER_NAME=$(option_text matrix_server_name)
for value in "domain=$DOMAIN" "matrix_server_name=$SERVER_NAME"; do
    if ! printf '%s' "${value#*=}" | grep -qE "$HOSTNAME_PATTERN"; then
        echo "[ERROR] Set '${value%%=*}' to a hostname like matrix.example.com," >&2
        echo "[ERROR] without https://, a port or a path. Got: '${value#*=}'" >&2
        exit 1
    fi
done

# LiveKit API key and secret, shared only between LiveKit and lk-jwt-service
if ! grep -qE '^[A-Za-z0-9]{20} [0-9a-f]{64}$' /data/keys 2>/dev/null; then
    (umask 077 && printf '%s %s\n' \
        "API$(head -c 512 /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 17)" \
        "$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')" > /data/keys)
fi
read -r API_KEY API_SECRET < /data/keys

# The public IP media is reachable on; looked up on public DNS so a local
# DNS override doesn't interfere
lookup_public_ip() {
    nslookup -type=a "$DOMAIN" 1.1.1.1 2>/dev/null \
        | awk '/^Name:/ { found = 1 } found && /^Address:/ { print $2; exit }'
}

write_config() {
    (umask 077 && cat > "$CONFIG" <<YAML
port: 7880
bind_addresses:
  - ""
rtc:
  tcp_port: 7881
  # All call media over one UDP port, so only it needs forwarding
  udp_port: 7882
  enable_loopback_candidate: false
YAML
)
    # Offer the home-network address too, so devices at home connect directly
    # instead of looping out through the router and back (NAT loopback drops
    # UDP and turns video into blocks). Only private LAN ranges: Docker and
    # Home Assistant's internal networks (172.16.0.0/12) aren't reachable.
    # Without such an address LiveKit refuses to start, so check first.
    if [ -n "$LAN_IPS" ]; then
        cat >> "$CONFIG" <<YAML
  advertise_internal_ip: true
  ips:
    includes:
      - 192.168.0.0/16
      - 10.0.0.0/8
YAML
    fi
    if [ -n "$PUBLIC_IP" ]; then
        printf '  use_external_ip: false\n  node_ip: %s\n' "$PUBLIC_IP" >> "$CONFIG"
        echo "[INFO] Group calls running at https://$DOMAIN (public IP $PUBLIC_IP)"
    else
        printf '  use_external_ip: true\n' >> "$CONFIG"
        echo "[WARN] Could not look up the public IP of '$DOMAIN'; asking a public STUN server instead." >&2
    fi
    cat >> "$CONFIG" <<YAML
keys:
  $API_KEY: $API_SECRET
room:
  # Only lk-jwt-service may create call rooms, and only for local users
  auto_create: false
logging:
  level: info
YAML
}

LIVEKIT_PID=
JWT_PID=
SLEEP_PID=

start_livekit() {
    /livekit-server --config "$CONFIG" &
    LIVEKIT_PID=$!
}

stop_livekit() {
    kill -TERM "$LIVEKIT_PID" 2>/dev/null || true
    wait "$LIVEKIT_PID" 2>/dev/null || true
}

shutdown() {
    rm -f "$SHARED"
    kill "$SLEEP_PID" "$JWT_PID" 2>/dev/null || true
    stop_livekit
    exit 0
}
trap shutdown TERM INT

LAN_IPS=$(ip -4 -o addr show 2>/dev/null \
    | awk '{ sub(/\/.*/, "", $4); print $4 }' | grep -E '^(192\.168|10)\.' | tr '\n' ' ')
if [ -n "$LAN_IPS" ]; then
    echo "[INFO] Offering home-network address(es) to devices at home: $LAN_IPS"
else
    echo "[WARN] No home-network address (192.168.x.x or 10.x.x.x) found; devices at home will connect through your public IP." >&2
fi

PUBLIC_IP=$(lookup_public_ip)
write_config
start_livekit

LIVEKIT_URL="wss://$DOMAIN" \
LIVEKIT_KEY="$API_KEY" \
LIVEKIT_SECRET="$API_SECRET" \
LIVEKIT_FULL_ACCESS_HOMESERVERS="$SERVER_NAME" \
LIVEKIT_JWT_BIND="0.0.0.0:7883" \
    lk-jwt-service &
JWT_PID=$!

# Tell the Matrix add-on where to send clients for group calls
mkdir -p "$(dirname "$SHARED")"
printf 'livekit_service_url=https://%s\n' "$DOMAIN" > "$SHARED"

# Follow home IP changes; stop if either service dies
set +e
while true; do
    sleep "$IP_CHECK_SECONDS" &
    SLEEP_PID=$!
    wait "$SLEEP_PID"

    for pid in "$LIVEKIT_PID" "$JWT_PID"; do
        if ! kill -0 "$pid" 2>/dev/null; then
            echo "[ERROR] A call service stopped unexpectedly." >&2
            rm -f "$SHARED"
            kill "$LIVEKIT_PID" "$JWT_PID" 2>/dev/null
            exit 1
        fi
    done

    NEW_IP=$(lookup_public_ip)
    if [ -n "$NEW_IP" ] && [ "$NEW_IP" != "$PUBLIC_IP" ]; then
        echo "[INFO] Public IP changed from ${PUBLIC_IP:-unknown} to $NEW_IP; restarting LiveKit"
        PUBLIC_IP=$NEW_IP
        write_config
        stop_livekit
        start_livekit
    fi
done

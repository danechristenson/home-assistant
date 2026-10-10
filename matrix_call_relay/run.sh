#!/bin/sh
set -e

OPTIONS=/data/options.json
CONFIG=/tmp/turnserver.conf
SHARED=/share/matrix-call-relay/relay.conf
IP_CHECK_SECONDS=${IP_CHECK_SECONDS:-300}

# Print an option's string value (simple values only; the address is
# validated below, so escape sequences never need handling)
option_text() {
    sed -nE "s/.*\"$1\": *\"([^\"]*)\".*/\1/p" "$OPTIONS" | head -n 1
}

# Only advertise the relay to Matrix while it is actually running
rm -f "$SHARED"

ADDRESS=$(option_text address)
if [ -z "$ADDRESS" ]; then
    echo "[ERROR] Set 'address' in the add-on configuration before starting." >&2
    echo "[ERROR] It is the public hostname phones use to reach the relay, e.g. matrix.example.com." >&2
    exit 1
fi
if ! printf '%s' "$ADDRESS" | grep -qE '^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$'; then
    echo "[ERROR] 'address' must be a hostname like matrix.example.com or an IPv4 address," >&2
    echo "[ERROR] without https://, a port or a path. Got: $ADDRESS" >&2
    exit 1
fi

# coturn must tell clients the public IP the relay ports are reachable on;
# look it up on public DNS so a local DNS override doesn't interfere
lookup_public_ip() {
    if printf '%s' "$ADDRESS" | grep -qE '^[0-9]+(\.[0-9]+){3}$'; then
        printf '%s' "$ADDRESS"
    else
        nslookup -type=a "$ADDRESS" 1.1.1.1 2>/dev/null \
            | awk '/^Name:/ { found = 1 } found && /^Address:/ { print $2; exit }'
    fi
}

CONTAINER_IP=$(hostname -i | awk '{ print $1 }')

if ! grep -qE '^[0-9a-f]{64}$' /data/secret 2>/dev/null; then
    (umask 077 && head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > /data/secret)
fi
SECRET=$(cat /data/secret)

write_config() {
    (umask 077 && cat > "$CONFIG" <<CONF
listening-port=3478
listening-ip=$CONTAINER_IP
min-port=49160
max-port=49179
realm=$ADDRESS
use-auth-secret
static-auth-secret=$SECRET
fingerprint
no-tls
no-dtls
no-cli
no-multicast-peers
no-software-attribute
user-quota=12
total-quota=1200
# Calls only need UDP relays; cap each session at ~4 Mbit/s (HD video) and
# answer nothing, not even basic STUN, without valid credentials
no-tcp-relay
max-bps=500000
secure-stun
# Drop root once the ports are open
proc-user=nobody
proc-group=nogroup
log-file=stdout
simple-log
pidfile=/tmp/turnserver.pid
# Never relay into private networks (protects the home network); allow the
# relay itself so calls where both sides use it still work
denied-peer-ip=0.0.0.0-0.255.255.255
denied-peer-ip=10.0.0.0-10.255.255.255
denied-peer-ip=100.64.0.0-100.127.255.255
denied-peer-ip=127.0.0.0-127.255.255.255
denied-peer-ip=169.254.0.0-169.254.255.255
denied-peer-ip=172.16.0.0-172.31.255.255
denied-peer-ip=192.0.0.0-192.0.0.255
denied-peer-ip=192.0.2.0-192.0.2.255
denied-peer-ip=192.88.99.0-192.88.99.255
denied-peer-ip=192.168.0.0-192.168.255.255
denied-peer-ip=198.18.0.0-198.19.255.255
denied-peer-ip=198.51.100.0-198.51.100.255
denied-peer-ip=203.0.113.0-203.0.113.255
denied-peer-ip=240.0.0.0-255.255.255.255
denied-peer-ip=::1
denied-peer-ip=64:ff9b::-64:ff9b::ffff:ffff
denied-peer-ip=::ffff:0.0.0.0-::ffff:255.255.255.255
denied-peer-ip=100::-100::ffff:ffff:ffff:ffff
denied-peer-ip=2001::-2001:1ff:ffff:ffff:ffff:ffff:ffff:ffff
denied-peer-ip=2002::-2002:ffff:ffff:ffff:ffff:ffff:ffff:ffff
denied-peer-ip=fc00::-fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff
denied-peer-ip=fe80::-febf:ffff:ffff:ffff:ffff:ffff:ffff:ffff
allowed-peer-ip=$CONTAINER_IP
CONF
)
    if [ -n "$PUBLIC_IP" ]; then
        echo "external-ip=$PUBLIC_IP/$CONTAINER_IP" >> "$CONFIG"
        echo "[INFO] Call relay running at $ADDRESS (public IP $PUBLIC_IP)"
    else
        echo "[WARN] Could not look up the public IP of '$ADDRESS'." >&2
        echo "[WARN] Calls will only be relayed on your home network. Check 'address'." >&2
    fi
}

RELAY_PID=
SLEEP_PID=

start_relay() {
    turnserver -c "$CONFIG" &
    RELAY_PID=$!
}

stop_relay() {
    kill -TERM "$RELAY_PID" 2>/dev/null || true
    wait "$RELAY_PID" 2>/dev/null || true
}

shutdown() {
    rm -f "$SHARED"
    kill "$SLEEP_PID" 2>/dev/null || true
    stop_relay
    exit 0
}
trap shutdown TERM INT

PUBLIC_IP=$(lookup_public_ip)
write_config
start_relay

# Hand the Matrix add-on the address and shared secret
mkdir -p "$(dirname "$SHARED")"
(umask 077 && printf 'address=%s\nsecret=%s\n' "$ADDRESS" "$SECRET" > "$SHARED")

# Home IPs change; follow them so relayed calls keep working
set +e
while true; do
    sleep "$IP_CHECK_SECONDS" &
    SLEEP_PID=$!
    wait "$SLEEP_PID"

    if ! kill -0 "$RELAY_PID" 2>/dev/null; then
        echo "[ERROR] The relay stopped unexpectedly." >&2
        rm -f "$SHARED"
        exit 1
    fi

    NEW_IP=$(lookup_public_ip)
    if [ -n "$NEW_IP" ] && [ "$NEW_IP" != "$PUBLIC_IP" ]; then
        echo "[INFO] Public IP changed from ${PUBLIC_IP:-unknown} to $NEW_IP; restarting the relay"
        PUBLIC_IP=$NEW_IP
        write_config
        stop_relay
        start_relay
    fi
done

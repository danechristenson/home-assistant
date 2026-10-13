# Shared by the sidebar CGI scripts (sourced, not run). Only the Home
# Assistant ingress proxy can reach them, and it sets the X-Remote-User-*
# headers from the logged-in Home Assistant session.

reply() {
    printf 'Content-Type: application/json\r\nCache-Control: no-store\r\n\r\n%s\n' "$1"
    exit 0
}

json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g' | tr -d '\000-\037'
}

. /tmp/autologin.env 2>/dev/null || reply '{"enabled":false}'
[ "$AUTOLOGIN" = "true" ] || reply '{"enabled":false}'

if [ -z "$HTTP_X_REMOTE_USER_NAME" ]; then
    reply '{"enabled":true,"error":"Home Assistant did not say which user is signed in."}'
fi

# The Matrix account for the Home Assistant user who opened the panel
LOCALPART=$(printf '%s' "$HTTP_X_REMOTE_USER_NAME" | tr 'A-Z' 'a-z' | sed 's/[^a-z0-9._=-]/_/g')
USER_ID="@$LOCALPART:$SERVER_NAME"

# Never hand out the server's own bot accounts
case "$LOCALPART" in
    conduit | _ha_autologin)
        reply "{\"enabled\":true,\"error\":\"The Home Assistant username '$LOCALPART' is reserved by the Matrix server.\"}"
        ;;
esac

# Call the client API through the autologin appservice: api PATH [POST-DATA]
api() {
    wget -q -O - \
        --header "Authorization: Bearer $AS_TOKEN" \
        --header "Content-Type: application/json" \
        ${2:+--post-data "$2"} \
        "$INTERNAL_URL$1" 2>/dev/null
}

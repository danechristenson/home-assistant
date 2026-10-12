# Changelog

## 1.13.9.1

- The call sign-in service now uses port 7883 instead of 8081, which other
  apps commonly use. Update the reverse proxy's `proxy_pass` to `:7883`.

## 1.13.9

- Initial release: LiveKit 1.13.9 and lk-jwt-service 0.8.0.

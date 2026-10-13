# Changelog

## 1.13.9.4

- Update lk-jwt-service 0.8.0 → 0.9.0.

## 1.13.9.3

- Call limits: at most 8 people per call, and no new people once all calls
  together pass 100 Mbit/s. Both are adjustable on the Configuration tab
  (0 turns a limit off).

## 1.13.9.2

- Fix blocky video and broken-up sound for callers at home: LiveKit now runs
  on the host network and also offers the home-network address, so devices
  at home connect directly instead of looping through the router. Media for
  outside callers no longer passes through Docker's port forwarding either.

## 1.13.9.1

- The call sign-in service now uses port 7883 instead of 8081, which other
  apps commonly use. Update the reverse proxy's `proxy_pass` to `:7883`.

## 1.13.9

- Initial release: LiveKit 1.13.9 and lk-jwt-service 0.8.0.

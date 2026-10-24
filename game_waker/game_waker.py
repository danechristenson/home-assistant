#!/usr/bin/env python3
"""Game Waker: starts a stopped game add-on when someone tries to join it.

For each game in the options, while it's stopped, watch its UDP ports and start
it on the first packet from an address allowed to wake it: the home networks
(home and VPN), addresses listed in the options, and addresses remembered from
the start page. The internet is full of server browsers and trackers sending
exactly what a joining game sends, so other addresses never wake a game. It
only watches the traffic, like tcpdump, and never takes the ports, so the game
can always start, from here or anywhere else.

The start page (web_port, for a reverse proxy) shows each game's state and a
Start button; pressing it starts the game and remembers the visitor's address,
so next time joining from there wakes it by itself."""

import html
import ipaddress
import json
import os
import re
import socket
import subprocess
import sys
import threading
import time
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

OPTIONS = "/data/options.json"
REMEMBERED = "/data/remembered.json"
# The smoke test points these at a stand-in
SUPERVISOR = os.environ.get("SUPERVISOR_URL", "http://supervisor")
TOKEN = os.environ.get("SUPERVISOR_TOKEN", "")
# How long to wait for a packet before checking the game's state (and the
# allowed addresses) again
LISTEN_SECONDS = int(os.environ.get("LISTEN_SECONDS", "60"))
# Home Assistant itself (e.g. a server-status check on 127.0.0.1) and the
# add-ons' own network: never players
INTERNAL = [ipaddress.ip_network(n) for n in ("127.0.0.0/8", "::1/128", "172.30.32.0/23")]
# Remembered addresses not seen for this long are forgotten (home connections
# change address now and then)
FORGET_DAYS = 60

lock = threading.Lock()
log_lock = threading.Lock()


def log(level, message):
    with log_lock:   # one line at a time from the threads
        sys.stdout.write(f"[{level}] {message}\n")
        sys.stdout.flush()


def supervisor(path, method="GET", body=None, timeout=30):
    request = urllib.request.Request(
        SUPERVISOR + path, method=method,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        data = response.read()
    return json.loads(data) if data else {}


def addon_state(slug):
    try:
        return supervisor(f"/addons/{slug}/info").get("data", {}).get("state") or ""
    except OSError:
        return ""


def networks(entries, what):
    result = []
    for entry in entries:
        try:
            result.append(ipaddress.ip_network(entry.strip(), strict=False))
        except ValueError:
            log("WARN", f"Ignoring {what} '{entry}': not an IP address or network")
    return result


# --- remembered addresses: {"203.0.113.5": last seen (unix time)} ---

def load_remembered():
    try:
        with open(REMEMBERED) as f:
            saved = json.load(f)
    except (OSError, ValueError):
        return {}
    cutoff = time.time() - FORGET_DAYS * 86400
    return {ip: seen for ip, seen in saved.items() if seen >= cutoff}


def remember(ip):
    with lock:
        saved = load_remembered()
        new = ip not in saved
        saved[ip] = time.time()
        with open(REMEMBERED + ".tmp", "w") as f:
            json.dump(saved, f)
        os.replace(REMEMBERED + ".tmp", REMEMBERED)
    return new


class Game:
    def __init__(self, options, home, extra):
        self.name = options.get("name") or options["addon"]
        self.addon = options["addon"]
        self.ports = options["udp_ports"]
        self.query_port = options.get("query_port")
        self.join_address = options.get("join_address") or ""
        self.home, self.extra = home, extra
        self.starting = threading.Lock()

    def allowed(self):
        """Networks and remembered addresses that may wake this game."""
        return self.home + self.extra + [ipaddress.ip_network(ip) for ip in load_remembered()]

    def may_wake(self, ip):
        address = ipaddress.ip_address(ip)
        return not any(address in n for n in INTERNAL) and any(address in n for n in self.allowed())

    def capture_filter(self):
        ports = f"portrange {self.ports}" if "-" in self.ports else f"port {self.ports}"
        sources = " or ".join(f"src net {n}" if n.num_addresses > 1 else f"src host {n.network_address}"
                              for n in self.allowed())
        internal = " and ".join(f"not src net {n}" for n in INTERNAL)
        return f"udp and dst {ports} and ({sources}) and {internal}" if sources else None

    def start(self, why, from_ip=""):
        """Start the game unless it's already starting; in the background, as games often update first."""
        if not self.starting.acquire(blocking=False):
            return
        log("INFO", f"{self.name}: {why}; starting it")
        try:
            supervisor("/core/api/events/game_waker_wake", "POST", {"addon": self.addon, "game": self.name, "from": from_ip})
        except OSError:
            log("WARN", f"{self.name}: couldn't tell Home Assistant (event game_waker_wake)")

        def run():
            try:
                supervisor(f"/addons/{self.addon}/start", "POST", timeout=900)
                log("INFO", f"{self.name}: started")
            except OSError as err:
                log("WARN", f"{self.name}: couldn't start it ({err}); trying again on the next join")
            finally:
                self.starting.release()
        threading.Thread(target=run, daemon=True).start()

    def ready(self):
        """Ask the game itself (Steam server query on 127.0.0.1): (answering, players)."""
        if not self.query_port:
            return None, None
        query = b"\xff\xff\xff\xffTSource Engine Query\x00"
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
            s.settimeout(1)
            try:
                s.sendto(query, ("127.0.0.1", self.query_port))
                data, _ = s.recvfrom(1400)
                if data[4:5] == b"A":   # a challenge: ask again with it
                    s.sendto(query + data[5:9], ("127.0.0.1", self.query_port))
                    data, _ = s.recvfrom(1400)
            except OSError:
                return False, None
        if data[4:5] != b"I":
            return False, None
        # header, protocol, then name, map, folder and game (each ending in 0), app id (2), players
        rest = data[6:]
        for _ in range(4):
            rest = rest[rest.index(b"\x00") + 1:]
        return True, rest[2] if len(rest) > 2 else None

    def watch(self):
        log("INFO", f"{self.name}: starts when someone at home, on the VPN or at a remembered address "
                    f"joins on UDP {self.ports}")
        while True:
            state = addon_state(self.addon)
            if state == "":
                log("WARN", f"{self.name}: no add-on '{self.addon}' (the slug is in its page's address); "
                            "checking again in 5 minutes")
                time.sleep(300)
                continue
            if state not in ("stopped", "error") or self.starting.locked():
                time.sleep(15)
                continue
            capture = self.capture_filter()
            if capture is None:
                time.sleep(LISTEN_SECONDS)
                continue
            try:
                out = subprocess.run(["tcpdump", "-c", "1", "-l", "-n", "-p", "-q", "-x", "-i", "any", capture],
                                     capture_output=True, text=True, timeout=LISTEN_SECONDS).stdout
            except subprocess.TimeoutExpired:
                continue
            if not out.strip():
                continue
            # "... IP 203.0.113.5.50000 > 192.168.8.112.2456: UDP, length 25", then the packet in hex
            match = re.search(r" IP6? ([0-9a-fA-F.:]+)\.\d+ > [0-9a-fA-F.:]+\.(\d+): .*length (\d+)", out)
            if not match or not self.may_wake(match.group(1)):
                continue
            payload = "".join(re.sub(r"^[^:]*:", "", line) for line in out.splitlines()[1:]).replace(" ", "")[56:72]
            if addon_state(self.addon) in ("stopped", "error"):
                self.start(f"{match.group(1)} is trying to join (UDP {match.group(2)}, {match.group(3)} bytes, "
                           f"starting {payload})", match.group(1))


# --- the start page ---

PAGE = """<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>
  :root {{ --bg: #f4f5f7; --card: #fff; --text: #16181d; --muted: #5d6470; --accent: #2e7d32; --line: #dde1e6; }}
  @media (prefers-color-scheme: dark) {{
    :root {{ --bg: #121417; --card: #1c1f24; --text: #eceef1; --muted: #9aa2ad; --accent: #66bb6a; --line: #2c3138; }}
  }}
  body {{ margin: 0; background: var(--bg); color: var(--text); font: 16px/1.5 system-ui, sans-serif; }}
  main {{ max-width: 34rem; margin: 0 auto; padding: 32px 16px; }}
  section {{ background: var(--card); border: 1px solid var(--line); border-radius: 12px; padding: 20px; margin-bottom: 16px; }}
  h1 {{ font-size: 1.4rem; margin: 0 0 4px; }}
  .state {{ color: var(--muted); margin: 0 0 16px; }}
  button {{ font: inherit; font-weight: 600; color: #fff; background: var(--accent); border: 0; border-radius: 8px;
            padding: 12px 20px; width: 100%; cursor: pointer; }}
  button:disabled {{ opacity: 0.45; cursor: default; }}
  code {{ font-size: 1.05em; }}
  .note {{ color: var(--muted); font-size: 0.9rem; margin: 12px 0 0; }}
</style>
<main>
{sections}
</main>
<script>
  async function refresh() {{
    const games = await (await fetch("status", {{ cache: "no-store" }})).json();
    games.forEach((g, i) => {{
      const state = document.getElementById("state-" + i), button = document.getElementById("start-" + i);
      const running = g.state === "started";
      state.textContent = g.starting ? "Starting… this usually takes a minute or two."
        : !running ? "Stopped. Press Start, then join once it's ready."
        : g.ready === false ? "Starting… almost there."
        : "Running" + (g.players != null ? ", " + g.players + (g.players === 1 ? " player" : " players") : "") + ": join now.";
      button.disabled = running || g.starting;
    }});
  }}
  async function start(i) {{
    document.getElementById("start-" + i).disabled = true;
    const r = await fetch("start/" + i, {{ method: "POST" }});
    document.getElementById("note-" + i).textContent = (await r.json()).message;
    refresh();
  }}
  refresh();
  setInterval(refresh, 5000);
</script>
</html>
"""

SECTION = """<section>
  <h1>{name}</h1>
  <p class="state" id="state-{i}">Checking…</p>
  <button id="start-{i}" onclick="start({i})" disabled>Start {name}</button>
  {join}
  <p class="note" id="note-{i}">Started from here, it remembers your address: next time, just join.</p>
</section>"""


def handler_for(games, trusted):
    class Handler(BaseHTTPRequestHandler):
        def visitor(self):
            """The visitor's address: from the reverse proxy's header when it comes through one."""
            peer = ipaddress.ip_address(self.client_address[0])
            if any(peer in n for n in trusted):
                forwarded = self.headers.get("X-Real-IP") or (self.headers.get("X-Forwarded-For") or "").split(",")[0]
                try:
                    return str(ipaddress.ip_address(forwarded.strip()))
                except ValueError:
                    pass
            return str(peer)

        def send(self, code, body, kind):
            data = body.encode()
            self.send_response(code)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(data)))
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(data)

        def do_GET(self):
            if self.path == "/status":
                rows = []
                for g in games:
                    ready, players = g.ready()
                    rows.append({"name": g.name, "state": addon_state(g.addon), "starting": g.starting.locked(),
                                 "ready": ready, "players": players})
                self.send(200, json.dumps(rows), "application/json")
            elif self.path == "/":
                title = games[0].name if len(games) == 1 else "Game servers"
                sections = "\n".join(SECTION.format(
                    i=i, name=html.escape(g.name),
                    join=f'<p class="note">Join at <code>{html.escape(g.join_address)}</code></p>' if g.join_address else "")
                    for i, g in enumerate(games))
                self.send(200, PAGE.format(title=html.escape(title), sections=sections), "text/html; charset=utf-8")
            else:
                self.send(404, "Not found", "text/plain")

        def do_POST(self):
            match = re.fullmatch(r"/start/(\d+)", self.path)
            if not match or int(match.group(1)) >= len(games):
                self.send(404, "Not found", "text/plain")
                return
            g, ip = games[int(match.group(1))], self.visitor()
            address = ipaddress.ip_address(ip)
            message = f"Starting {g.name}."
            if not any(address in n for n in INTERNAL) and not any(address in n for n in g.home + g.extra):
                if remember(ip):
                    log("INFO", f"Remembering {ip} (pressed Start): it can now wake the games by joining")
                message += " Your address is remembered: next time, just join."
            if addon_state(g.addon) in ("stopped", "error"):
                g.start(f"{ip} pressed Start", ip)
            else:
                message = f"{g.name} is already running."
            self.send(200, json.dumps({"message": message}), "application/json")

        def log_message(self, *args):
            pass
    return Handler


def main():
    with open(OPTIONS) as f:
        options = json.load(f)
    home = networks(options.get("home_networks") or [], "home network")
    extra = networks(options.get("wake_from") or [], "address")
    games = [Game(g, home, extra) for g in options.get("games") or []]
    if not games:
        log("WARN", "No games to watch: add one under 'Games' on the Configuration tab")
        while True:
            time.sleep(3600)
    for g in games:
        threading.Thread(target=g.watch, daemon=True).start()
    # The reverse proxy reaches the page from Home Assistant's own or a private network
    trusted = INTERNAL + [ipaddress.ip_network(n) for n in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")]
    port = int(options.get("web_port") or 8095)
    log("INFO", f"Start page on port {port} (put it behind your reverse proxy)")
    ThreadingHTTPServer(("0.0.0.0", port), handler_for(games, trusted)).serve_forever()


if __name__ == "__main__":
    main()

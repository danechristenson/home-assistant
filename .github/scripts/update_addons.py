#!/usr/bin/env python3
"""Bump the Matrix add-ons to the latest upstream releases.

Checks Continuwuity, Element Web, qrcode-generator and the Alpine and busybox
build images for Matrix, coturn, LiveKit, lk-jwt-service, mautrix-go, Go and
the Home Assistant base image for the helper bot, the itzg image, Mojang's
Bedrock server and Go for Minecraft, Node.js for Foundry VTT, and NPMplus. On Mondays
(or with UPDATE_GO_MODULES=1) it also updates all of the helper bot's Go
modules. For each add-on whose dependencies changed, updates its Dockerfile
(where every version is pinned), the add-on version and the changelog, then
prints the changed add-on folders (one per line) for the workflow to build and
test.

The daily run only updates within the current major version, straight to
main. A new major version can break things (qrcode-generator 2.x moved its
files, coturn majors can change config, each Foundry VTT generation needs a
particular Node.js major), so `--major --only <add-on>` applies just the
newer major versions for one add-on, and the workflow opens a pull request
with them. Continuwuity (v26.x), the itzg image (2026.x) and NPMplus
(2026-11-18-r1) number their versions by year or date, so they have no major
versions and always update daily.

Add-on versions are semantic (MAJOR.MINOR.PATCH) and the add-on's own: an
update within the current major version is a patch (26.9.2 -> 26.9.3), a new
major version a major one (4.18.0 -> 5.0.0); new features in an add-on bump
its minor version by hand. Upstream versions are in the changelog. Versions
must always increase for Home Assistant (AwesomeVersion), and never use "-N"
suffixes: those are pre-releases and sort as older.
"""

import json
import os
import re
import subprocess
import sys
import urllib.request
from datetime import date
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
# --major: apply only updates to a newer major version (for a pull request)
MAJOR = "--major" in sys.argv


def get_json(url, headers=None):
    request = urllib.request.Request(url, headers={"User-Agent": "addon-updater", **(headers or {})})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def version_key(version):
    return tuple(int(part) for part in re.findall(r"\d+", version))


def latest_continuwuity():
    releases = get_json("https://forgejo.ellis.link/api/v1/repos/continuwuation/continuwuity/releases?limit=20")
    tags = [r["tag_name"] for r in releases if not r["draft"] and not r["prerelease"]]
    tags = [t for t in tags if re.fullmatch(r"v\d+\.\d+\.\d+", t)]
    return max(tags, key=version_key)


def ghcr_tag_exists(repository, tag):
    token = get_json(f"https://ghcr.io/token?scope=repository:{repository}:pull")["token"]
    request = urllib.request.Request(
        f"https://ghcr.io/v2/{repository}/manifests/{tag}",
        method="HEAD",
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json",
        },
    )
    try:
        urllib.request.urlopen(request, timeout=30)
        return True
    except urllib.error.HTTPError:
        return False


def latest_element():
    return get_json("https://api.github.com/repos/element-hq/element-web/releases/latest")["tag_name"]


def pick(versions, current):
    """The version to move to: the newest in the current major version, or with
    --major the newest of a newer major version. Either way, current if none."""
    major = version_key(current)[0]
    wanted = (lambda m: m > major) if MAJOR else (lambda m: m == major)
    return max((v for v in versions if v and wanted(version_key(v)[0])), key=version_key, default=current)


def pick_calendar(latest, current):
    """Year-based versions have no major versions: always the daily run's."""
    return current if MAJOR else latest


def latest_qrcode(current):
    versions = get_json("https://registry.npmjs.org/qrcode-generator")["versions"]
    return pick([v for v in versions if re.fullmatch(r"\d+\.\d+\.\d+", v)], current)


def latest_alpine(current):
    tags = get_json("https://hub.docker.com/v2/repositories/library/alpine/tags?page_size=100&ordering=last_updated")
    return pick([t["name"] for t in tags["results"] if re.fullmatch(r"\d+\.\d+", t["name"])], current)


def latest_busybox(current):
    tags = get_json("https://hub.docker.com/v2/repositories/library/busybox/tags?page_size=100&ordering=last_updated")
    return pick([m.group(1) for t in tags["results"] if (m := re.fullmatch(r"(\d+\.\d+)-musl", t["name"]))], current)


def ghcr_architectures(repository, tag):
    """The architectures a multi-arch image on ghcr.io is published for."""
    token = get_json(f"https://ghcr.io/token?scope=repository:{repository}:pull")["token"]
    try:
        index = get_json(f"https://ghcr.io/v2/{repository}/manifests/{tag}", {
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json",
        })
    except urllib.error.HTTPError:
        return set()
    return {m["platform"]["architecture"] for m in index.get("manifests", [])}


def latest_ha_base(current):
    """The Home Assistant base image (multi-arch) follows Alpine's x.y; only
    once it is published for both architectures."""
    alpine = latest_alpine(current)
    if {"amd64", "arm64"} <= ghcr_architectures("home-assistant/base", alpine):
        return alpine
    return current


def latest_coturn(current):
    tags = get_json("https://hub.docker.com/v2/repositories/coturn/coturn/tags?page_size=100&ordering=last_updated")
    return pick([m.group(1) for t in tags["results"] if (m := re.fullmatch(r"(\d+\.\d+\.\d+)-alpine", t["name"]))], current)


def latest_livekit():
    tag = get_json("https://api.github.com/repos/livekit/livekit/releases/latest")["tag_name"]
    tags = get_json("https://hub.docker.com/v2/repositories/livekit/livekit-server/tags?page_size=50&ordering=last_updated")
    published = {t["name"] for t in tags["results"]}
    return tag if tag in published else None


def latest_lk_jwt():
    tag = get_json("https://api.github.com/repos/element-hq/lk-jwt-service/releases/latest")["tag_name"].lstrip("v")
    return tag if ghcr_tag_exists("element-hq/lk-jwt-service", tag) else None


def read_value(text, pattern):
    match = re.search(pattern, text, re.M)
    if not match:
        sys.exit(f"Could not find {pattern!r}")
    return match.group(1)


def add_changelog_entry(folder, version, lines):
    path = ROOT / folder / "CHANGELOG.md"
    text = path.read_text()
    if MAJOR:
        lines = lines + ["New major version: check the upstream release notes before merging."]
    entry = f"## {version}\n\n" + "".join(f"- {line}\n" for line in lines) + "\n"
    path.write_text(text.replace("# Changelog\n\n", f"# Changelog\n\n{entry}", 1))


def bump_version(folder):
    """The add-on's next version: a patch, or with --major a major version."""
    major, minor, patch = (int(p) for p in read_value((ROOT / folder / "config.yaml").read_text(),
                                                       r'^version: "(\d+\.\d+\.\d+)"$').split("."))
    return f"{major + 1}.0.0" if MAJOR else f"{major}.{minor}.{patch + 1}"


def set_addon_version(folder, version):
    path = ROOT / folder / "config.yaml"
    path.write_text(re.sub(r'^version: ".*"$', f'version: "{version}"', path.read_text(), count=1, flags=re.M))


def update_matrix():
    dockerfile = ROOT / "matrix" / "Dockerfile"
    docker_text = dockerfile.read_text()
    current = {
        "continuwuity": read_value(docker_text, r"^FROM ghcr\.io/continuwuity/continuwuity:(v[\d.]+)$"),
        "element": read_value(docker_text, r"^ARG ELEMENT_VERSION=(\S+)$"),
        "qrcode": read_value(docker_text, r"^ARG QRCODE_VERSION=(\S+)$"),
        "alpine": read_value(docker_text, r"^FROM alpine:([\d.]+) AS web$"),
        "busybox": read_value(docker_text, r"^FROM busybox:([\d.]+)-musl AS busybox$"),
    }
    latest = {
        "continuwuity": pick_calendar(latest_continuwuity(), current["continuwuity"]),
        "element": pick([latest_element()], current["element"]),
        "qrcode": latest_qrcode(current["qrcode"]),
        "alpine": latest_alpine(current["alpine"]),
        "busybox": latest_busybox(current["busybox"]),
    }
    if not ghcr_tag_exists("continuwuity/continuwuity", latest["continuwuity"]):
        print(f"Continuwuity {latest['continuwuity']} image not published yet; keeping {current['continuwuity']}", file=sys.stderr)
        latest["continuwuity"] = current["continuwuity"]

    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    docker_text = docker_text.replace(f"continuwuity:{current['continuwuity']}", f"continuwuity:{latest['continuwuity']}")
    docker_text = docker_text.replace(f"ELEMENT_VERSION={current['element']}", f"ELEMENT_VERSION={latest['element']}")
    docker_text = docker_text.replace(f"QRCODE_VERSION={current['qrcode']}", f"QRCODE_VERSION={latest['qrcode']}")
    docker_text = docker_text.replace(f"FROM alpine:{current['alpine']} AS", f"FROM alpine:{latest['alpine']} AS")
    docker_text = docker_text.replace(f"FROM busybox:{current['busybox']}-musl", f"FROM busybox:{latest['busybox']}-musl")
    dockerfile.write_text(docker_text)

    addon_version = read_value((ROOT / "matrix" / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("matrix")
    set_addon_version("matrix", new_version)

    names = {"continuwuity": "Continuwuity", "element": "Element Web", "qrcode": "qrcode-generator",
             "alpine": "Alpine (Element Web build stage)", "busybox": "busybox"}
    add_changelog_entry("matrix", new_version, [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()])
    print(f"matrix: {addon_version} -> {new_version} ({', '.join(changes)})", file=sys.stderr)
    return True


def update_relay():
    dockerfile = ROOT / "matrix_call_relay" / "Dockerfile"
    text = dockerfile.read_text()
    current = read_value(text, r"^FROM coturn/coturn:([\d.]+)-alpine$")
    latest = latest_coturn(current)
    if version_key(latest) <= version_key(current):
        return False
    dockerfile.write_text(text.replace(f"coturn/coturn:{current}-alpine", f"coturn/coturn:{latest}-alpine"))
    addon_version = read_value((ROOT / "matrix_call_relay" / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("matrix_call_relay")
    set_addon_version("matrix_call_relay", new_version)
    add_changelog_entry("matrix_call_relay", new_version, [f"Update coturn {current} → {latest}."])
    print(f"matrix_call_relay: {addon_version} -> {new_version} (coturn {latest})", file=sys.stderr)
    return True


def update_group_calls():
    dockerfile = ROOT / "matrix_group_calls" / "Dockerfile"
    text = dockerfile.read_text()
    current = {
        "livekit": read_value(text, r"^FROM livekit/livekit-server:(v[\d.]+)$"),
        "lk_jwt": read_value(text, r"^FROM ghcr\.io/element-hq/lk-jwt-service:(\S+) AS jwt$"),
    }
    latest = {"livekit": pick([latest_livekit()], current["livekit"]), "lk_jwt": pick([latest_lk_jwt()], current["lk_jwt"])}
    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    text = text.replace(f"livekit-server:{current['livekit']}", f"livekit-server:{latest['livekit']}")
    text = text.replace(f"lk-jwt-service:{current['lk_jwt']} AS", f"lk-jwt-service:{latest['lk_jwt']} AS")
    dockerfile.write_text(text)

    addon_version = read_value((ROOT / "matrix_group_calls" / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("matrix_group_calls")
    set_addon_version("matrix_group_calls", new_version)

    names = {"livekit": "LiveKit", "lk_jwt": "lk-jwt-service"}
    add_changelog_entry("matrix_group_calls", new_version, [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()])
    print(f"matrix_group_calls: {addon_version} -> {new_version} ({', '.join(changes)})", file=sys.stderr)
    return True


def latest_mautrix():
    return get_json("https://proxy.golang.org/maunium.net/go/mautrix/@latest")["Version"]


def latest_go():
    """The newest Go minor version with a published golang:<x.y>-alpine image."""
    releases = get_json("https://go.dev/dl/?mode=json")
    minors = sorted({".".join(r["version"].removeprefix("go").split(".")[:2]) for r in releases if r["stable"]}, key=version_key)
    for minor in reversed(minors):
        try:
            get_json(f"https://hub.docker.com/v2/repositories/library/golang/tags/{minor}-alpine")
            return minor
        except urllib.error.HTTPError:
            continue
    return None


def go_requirements(go_mod):
    """{module: version} for every requirement in a go.mod."""
    return dict(re.findall(r"^\s*(?:require\s+)?([\w.\-/]+\.[\w.\-/]+) (v\S+)", go_mod, re.M))


def update_helper_bot():
    """Monday's run also updates every Go module (golang.org/x/crypto, x/net…),
    not only mautrix-go, so fixes in them don't wait for a mautrix release."""
    folder = ROOT / "matrix_helper_bot"
    dockerfile = folder / "Dockerfile"
    text = dockerfile.read_text()
    go_mod = folder / "go.mod"
    modules_before = go_requirements(go_mod.read_text())
    current = {
        "mautrix": modules_before["maunium.net/go/mautrix"],
        "go": read_value(text, r"^FROM golang:([\d.]+)-alpine AS build$"),
        "base": read_value(text, r"^FROM ghcr\.io/home-assistant/base:([\d.]+)$"),
    }
    latest = {"mautrix": pick([latest_mautrix()], current["mautrix"]), "go": pick([latest_go()], current["go"]),
              "base": latest_ha_base(current["base"])}
    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    all_modules = not MAJOR and (date.today().weekday() == 0 or os.environ.get("UPDATE_GO_MODULES") == "1")

    if "go" in changes:
        text = text.replace(f"golang:{current['go']}-alpine", f"golang:{latest['go']}-alpine")
    if "base" in changes:
        text = text.replace(f"home-assistant/base:{current['base']}", f"home-assistant/base:{latest['base']}")
    dockerfile.write_text(text)

    # Update go.mod and go.sum with the Go toolchain the add-on builds with
    go_commands = []
    if "mautrix" in changes:
        go_commands.append(f"go get maunium.net/go/mautrix@{latest['mautrix']}")
    if all_modules:
        go_commands.append("go get -u ./...")
    if go_commands:
        subprocess.run(
            ["docker", "run", "--rm", "-v", f"{folder}:/src:z", "-w", "/src", f"golang:{latest['go']}-alpine",
             "sh", "-c", " && ".join(go_commands + ["go mod tidy", f"chown {os_ids()} go.mod go.sum"])],
            check=True,
        )
    modules_after = go_requirements(go_mod.read_text())
    module_lines = [f"Update {m} {modules_before[m]} → {v}." for m, v in sorted(modules_after.items())
                    if m in modules_before and v != modules_before[m] and m != "maunium.net/go/mautrix"]
    module_lines += [f"Add {m} {v}." for m, v in sorted(modules_after.items()) if m not in modules_before]
    if not changes and not module_lines:
        return False

    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("matrix_helper_bot")
    set_addon_version("matrix_helper_bot", new_version)

    names = {"mautrix": "mautrix-go", "go": "Go", "base": "Home Assistant base image"}
    lines = [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()] + module_lines
    add_changelog_entry("matrix_helper_bot", new_version, lines)
    print(f"matrix_helper_bot: {addon_version} -> {new_version} ({', '.join(list(changes) + (['go modules'] if module_lines else []))})", file=sys.stderr)
    return True


def latest_farmos(current):
    tags = get_json("https://hub.docker.com/v2/repositories/farmos/farmos/tags?page_size=100&ordering=last_updated")
    return pick([t["name"] for t in tags["results"] if re.fullmatch(r"\d+\.\d+\.\d+", t["name"])], current)


def update_farmos():
    """The add-on applies farmOS's database updates when it starts with a new
    version."""
    folder = ROOT / "farmos"
    dockerfile = folder / "Dockerfile"
    text = dockerfile.read_text()
    current = read_value(text, r"^FROM farmos/farmos:([\d.]+)$")
    latest = latest_farmos(current)
    if version_key(latest) <= version_key(current):
        return False
    dockerfile.write_text(text.replace(f"farmos/farmos:{current}\n", f"farmos/farmos:{latest}\n"))
    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("farmos")
    set_addon_version("farmos", new_version)
    add_changelog_entry("farmos", new_version, [f"Update farmOS {current} → {latest}."])
    print(f"farmos: {addon_version} -> {new_version} (farmOS {latest})", file=sys.stderr)
    return True


def latest_itzg_bedrock():
    tags = get_json("https://hub.docker.com/v2/repositories/itzg/minecraft-bedrock-server/tags?page_size=100&ordering=last_updated")
    versions = [t["name"] for t in tags["results"] if re.fullmatch(r"\d{4}\.\d+\.\d+", t["name"])]
    return max(versions, key=version_key)


def latest_bedrock():
    links = get_json("https://net.web.minecraft-services.net/api/v1.0/download/links",
                     {"User-Agent": "itzg/minecraft-bedrock-server"})
    url = next(l["downloadUrl"] for l in links["result"]["links"] if l["downloadType"] == "serverBedrockLinux")
    return re.search(r"bedrock-server-([\d.]+)\.zip", url).group(1)


def update_minecraft():
    """A new Bedrock release only needs a restart, which installing the bumped
    version gives (with Auto update on)."""
    folder = ROOT / "minecraft_bedrock"
    dockerfile = folder / "Dockerfile"
    text = dockerfile.read_text()
    current = {
        "itzg": read_value(text, r"^FROM itzg/minecraft-bedrock-server:([\d.]+)$"),
        "bedrock": read_value(text, r"^ENV VERSION=([\d.]+)$"),
        "go": read_value(text, r"^FROM golang:([\d.]+)-alpine AS botapi$"),
    }
    latest = {"itzg": pick_calendar(latest_itzg_bedrock(), current["itzg"]),
              "bedrock": pick([latest_bedrock()], current["bedrock"]), "go": pick([latest_go()], current["go"])}
    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    text = text.replace(f"itzg/minecraft-bedrock-server:{current['itzg']}", f"itzg/minecraft-bedrock-server:{latest['itzg']}")
    text = text.replace(f"ENV VERSION={current['bedrock']}", f"ENV VERSION={latest['bedrock']}")
    text = text.replace(f"golang:{current['go']}-alpine", f"golang:{latest['go']}-alpine")
    dockerfile.write_text(text)

    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("minecraft_bedrock")
    set_addon_version("minecraft_bedrock", new_version)

    names = {"itzg": "itzg/minecraft-bedrock-server", "bedrock": "Bedrock server", "go": "Go (bot API)"}
    add_changelog_entry("minecraft_bedrock", new_version, [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()])
    print(f"minecraft_bedrock: {addon_version} -> {new_version} ({', '.join(changes)})", file=sys.stderr)
    return True


def latest_node(current):
    """The newest node:<x.y.z>-trixie-slim image: in the current major version,
    or with --major in the next few."""
    major = version_key(current)[0]
    versions = []
    for m in range(major + 1, major + 5) if MAJOR else [major]:
        tags = get_json(f"https://hub.docker.com/v2/repositories/library/node/tags?page_size=100&name={m}.&ordering=last_updated")
        versions += [v.group(1) for t in tags["results"] if (v := re.fullmatch(r"(\d+\.\d+\.\d+)-trixie-slim", t["name"]))]
    return pick(versions, current)


def update_foundry():
    """Only Node.js is in the image (each user installs Foundry from their
    own license), so a Node.js update bumps the add-on's patch version."""
    folder = ROOT / "foundry_vtt"
    dockerfile = folder / "Dockerfile"
    text = dockerfile.read_text()
    current = read_value(text, r"^FROM node:([\d.]+)-trixie-slim$")
    latest = latest_node(current)
    if version_key(latest) <= version_key(current):
        return False
    dockerfile.write_text(text.replace(f"node:{current}-trixie-slim", f"node:{latest}-trixie-slim"))

    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("foundry_vtt")
    set_addon_version("foundry_vtt", new_version)
    add_changelog_entry("foundry_vtt", new_version, [f"Update Node.js {current} → {latest}."])
    print(f"foundry_vtt: {addon_version} -> {new_version} (node)", file=sys.stderr)
    return True


def latest_npmplus():
    """NPMplus's newest release (dated: 2026-11-18-r1; its betas are
    pre-releases) and its image's digest, once the image is on Docker Hub."""
    tag = get_json("https://api.github.com/repos/ZoeyVid/NPMplus/releases/latest")["tag_name"]
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}-r\d+", tag):
        return None
    try:
        image = get_json(f"https://hub.docker.com/v2/repositories/zoeyvid/npmplus/tags/{tag}")
    except urllib.error.HTTPError:
        return None
    return tag, image["digest"]


def update_npmplus():
    """The image runs as it is, pinned by tag and digest in the Dockerfile, so
    a new NPMplus release is the whole update. NPMplus rewrites every host's
    nginx files when it starts with a new version, as with its own compose
    file. A tag pushed again with a new digest is not followed."""
    folder = ROOT / "npmplus"
    dockerfile = folder / "Dockerfile"
    text = dockerfile.read_text()
    pinned = read_value(text, r"^FROM docker\.io/zoeyvid/npmplus:(\S+)$")
    current = pinned.split("@")[0]
    latest = latest_npmplus()
    if MAJOR or not latest or version_key(latest[0]) <= version_key(current):
        return False
    dockerfile.write_text(text.replace(f"zoeyvid/npmplus:{pinned}", f"zoeyvid/npmplus:{latest[0]}@{latest[1]}"))

    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    new_version = bump_version("npmplus")
    set_addon_version("npmplus", new_version)
    add_changelog_entry("npmplus", new_version, [f"Update NPMplus {current} → {latest[0]}."])
    print(f"npmplus: {addon_version} -> {new_version} (NPMplus {latest[0]})", file=sys.stderr)
    return True


def os_ids():
    import os
    return f"{os.getuid()}:{os.getgid()}"


UPDATERS = {
    "farmos": update_farmos,
    "foundry_vtt": update_foundry,
    "matrix": update_matrix,
    "matrix_call_relay": update_relay,
    "matrix_group_calls": update_group_calls,
    "matrix_helper_bot": update_helper_bot,
    "minecraft_bedrock": update_minecraft,
    "npmplus": update_npmplus,
}


def main():
    only = sys.argv[sys.argv.index("--only") + 1] if "--only" in sys.argv else None
    changed = [name for name, update in UPDATERS.items() if (only in (None, name)) and update()]
    print("\n".join(changed))
    kind = "major versions" if MAJOR else "updates"
    print(f"Checked {date.today()} for {kind}: {', '.join(changed) or 'nothing new'}", file=sys.stderr)


if __name__ == "__main__":
    main()

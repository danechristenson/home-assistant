#!/usr/bin/env python3
"""Bump the Matrix add-ons to the latest upstream releases.

Checks Continuwuity, Element Web, qrcode-generator, coturn, LiveKit,
lk-jwt-service, mautrix-go and Go for the helper bot, and the itzg image and
Mojang's Bedrock server for Minecraft. For each add-on
whose dependencies changed, updates build.yaml, the add-on version and the
changelog, then prints the changed add-on folders (one per line) for the
workflow to build and test.

qrcode-generator and coturn only update within their current major version:
qrcode-generator 2.x moved its files and coturn majors can change config, so
those need a deliberate manual upgrade.

Versions must always increase for Home Assistant (AwesomeVersion): a new
Continuwuity release becomes the Matrix version (26.10.0); any other change
bumps a fourth number (26.9.1.5). Never use "-N" suffixes: those are
pre-releases and sort as older.
"""

import json
import re
import subprocess
import sys
import urllib.request
from datetime import date
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


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


def same_major(versions, current):
    major = version_key(current)[0]
    return max((v for v in versions if version_key(v)[0] == major), key=version_key, default=current)


def latest_qrcode(current):
    versions = get_json("https://registry.npmjs.org/qrcode-generator")["versions"]
    return same_major([v for v in versions if re.fullmatch(r"\d+\.\d+\.\d+", v)], current)


def latest_coturn(current):
    tags = get_json("https://hub.docker.com/v2/repositories/coturn/coturn/tags?page_size=100&ordering=last_updated")
    versions = [m.group(1) for t in tags["results"] if (m := re.fullmatch(r"(\d+\.\d+\.\d+)-alpine", t["name"]))]
    return same_major(versions, current)


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
    entry = f"## {version}\n\n" + "".join(f"- {line}\n" for line in lines) + "\n"
    path.write_text(text.replace("# Changelog\n\n", f"# Changelog\n\n{entry}", 1))


def set_addon_version(folder, version):
    path = ROOT / folder / "config.yaml"
    path.write_text(re.sub(r'^version: ".*"$', f'version: "{version}"', path.read_text(), count=1, flags=re.M))


def update_matrix():
    build = ROOT / "matrix" / "build.yaml"
    text = build.read_text()
    current = {
        "continuwuity": read_value(text, r"continuwuity:(v[\d.]+)$"),
        "element": read_value(text, r"ELEMENT_VERSION: (\S+)$"),
        "qrcode": read_value(text, r"QRCODE_VERSION: (\S+)$"),
    }
    latest = {
        "continuwuity": latest_continuwuity(),
        "element": latest_element(),
        "qrcode": latest_qrcode(current["qrcode"]),
    }
    if not ghcr_tag_exists("continuwuity/continuwuity", latest["continuwuity"]):
        print(f"Continuwuity {latest['continuwuity']} image not published yet; keeping {current['continuwuity']}", file=sys.stderr)
        latest["continuwuity"] = current["continuwuity"]

    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    text = text.replace(f"continuwuity:{current['continuwuity']}", f"continuwuity:{latest['continuwuity']}")
    text = text.replace(f"ELEMENT_VERSION: {current['element']}", f"ELEMENT_VERSION: {latest['element']}")
    text = text.replace(f"QRCODE_VERSION: {current['qrcode']}", f"QRCODE_VERSION: {latest['qrcode']}")
    build.write_text(text)

    addon_version = read_value((ROOT / "matrix" / "config.yaml").read_text(), r'^version: "(.*)"$')
    if "continuwuity" in changes:
        new_version = latest["continuwuity"].lstrip("v")
    else:
        parts = addon_version.split(".")
        parts = parts + ["0"] if len(parts) == 3 else parts
        parts[3] = str(int(parts[3]) + 1)
        new_version = ".".join(parts)
    set_addon_version("matrix", new_version)

    names = {"continuwuity": "Continuwuity", "element": "Element Web", "qrcode": "qrcode-generator"}
    add_changelog_entry("matrix", new_version, [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()])
    print(f"matrix: {addon_version} -> {new_version} ({', '.join(changes)})", file=sys.stderr)
    return True


def update_relay():
    build = ROOT / "matrix_call_relay" / "build.yaml"
    text = build.read_text()
    current = read_value(text, r"coturn/coturn:([\d.]+)-alpine$")
    latest = latest_coturn(current)
    if version_key(latest) <= version_key(current):
        return False
    build.write_text(text.replace(f"coturn/coturn:{current}-alpine", f"coturn/coturn:{latest}-alpine"))
    set_addon_version("matrix_call_relay", latest)
    add_changelog_entry("matrix_call_relay", latest, [f"Update coturn {current} → {latest}."])
    print(f"matrix_call_relay: {current} -> {latest}", file=sys.stderr)
    return True


def update_group_calls():
    build = ROOT / "matrix_group_calls" / "build.yaml"
    text = build.read_text()
    current = {
        "livekit": read_value(text, r"livekit-server:(v[\d.]+)$"),
        "lk_jwt": read_value(text, r"LK_JWT_VERSION: (\S+)$"),
    }
    latest = {"livekit": latest_livekit() or current["livekit"], "lk_jwt": latest_lk_jwt() or current["lk_jwt"]}
    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    text = text.replace(f"livekit-server:{current['livekit']}", f"livekit-server:{latest['livekit']}")
    text = text.replace(f"LK_JWT_VERSION: {current['lk_jwt']}", f"LK_JWT_VERSION: {latest['lk_jwt']}")
    build.write_text(text)

    addon_version = read_value((ROOT / "matrix_group_calls" / "config.yaml").read_text(), r'^version: "(.*)"$')
    if "livekit" in changes:
        new_version = latest["livekit"].lstrip("v")
    else:
        parts = addon_version.split(".")
        parts = parts + ["0"] if len(parts) == 3 else parts
        parts[3] = str(int(parts[3]) + 1)
        new_version = ".".join(parts)
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


def update_helper_bot():
    folder = ROOT / "matrix_helper_bot"
    build = folder / "build.yaml"
    text = build.read_text()
    current = {
        "mautrix": read_value((folder / "go.mod").read_text(), r"maunium\.net/go/mautrix (v[\d.]+)$"),
        "go": read_value(text, r'GO_VERSION: "([\d.]+)"$'),
    }
    latest = {"mautrix": latest_mautrix(), "go": latest_go() or current["go"]}
    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    if "go" in changes:
        build.write_text(text.replace(f'GO_VERSION: "{current["go"]}"', f'GO_VERSION: "{latest["go"]}"'))
    if "mautrix" in changes:
        # Update go.mod and go.sum with the Go toolchain the add-on builds with
        subprocess.run(
            ["docker", "run", "--rm", "-v", f"{folder}:/src", "-w", "/src", f"golang:{latest['go']}-alpine",
             "sh", "-c", f"go get maunium.net/go/mautrix@{latest['mautrix']} && go mod tidy && chown {os_ids()} go.mod go.sum"],
            check=True,
        )

    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    parts = addon_version.split(".")
    parts[2] = str(int(parts[2]) + 1)
    new_version = ".".join(parts)
    set_addon_version("matrix_helper_bot", new_version)

    names = {"mautrix": "mautrix-go", "go": "Go"}
    add_changelog_entry("matrix_helper_bot", new_version, [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()])
    print(f"matrix_helper_bot: {addon_version} -> {new_version} ({', '.join(changes)})", file=sys.stderr)
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
    """The add-on version is the itzg image version plus a fourth number that
    counts Bedrock updates (2026.10.0.3). A new Bedrock release only needs a
    restart, which installing the bumped version gives (with Auto update on)."""
    folder = ROOT / "minecraft_bedrock"
    build = folder / "build.yaml"
    text = build.read_text()
    current = {
        "itzg": read_value(text, r"itzg/minecraft-bedrock-server:([\d.]+)$"),
        "bedrock": read_value(text, r"BEDROCK_VERSION: ([\d.]+)$"),
    }
    latest = {"itzg": latest_itzg_bedrock(), "bedrock": latest_bedrock()}
    changes = {k: (current[k], latest[k]) for k in current if version_key(latest[k]) > version_key(current[k])}
    if not changes:
        return False

    text = text.replace(f"itzg/minecraft-bedrock-server:{current['itzg']}", f"itzg/minecraft-bedrock-server:{latest['itzg']}")
    text = text.replace(f"BEDROCK_VERSION: {current['bedrock']}", f"BEDROCK_VERSION: {latest['bedrock']}")
    build.write_text(text)

    addon_version = read_value((folder / "config.yaml").read_text(), r'^version: "(.*)"$')
    if "itzg" in changes:
        new_version = f"{latest['itzg']}.1"
    else:
        parts = addon_version.split(".")
        parts = parts + ["0"] if len(parts) == 3 else parts
        parts[3] = str(int(parts[3]) + 1)
        new_version = ".".join(parts)
    set_addon_version("minecraft_bedrock", new_version)

    names = {"itzg": "itzg/minecraft-bedrock-server", "bedrock": "Bedrock server"}
    add_changelog_entry("minecraft_bedrock", new_version, [f"Update {names[k]} {old} → {new}." for k, (old, new) in changes.items()])
    print(f"minecraft_bedrock: {addon_version} -> {new_version} ({', '.join(changes)})", file=sys.stderr)
    return True


def os_ids():
    import os
    return f"{os.getuid()}:{os.getgid()}"


def main():
    changed = []
    if update_matrix():
        changed.append("matrix")
    if update_relay():
        changed.append("matrix_call_relay")
    if update_group_calls():
        changed.append("matrix_group_calls")
    if update_helper_bot():
        changed.append("matrix_helper_bot")
    if update_minecraft():
        changed.append("minecraft_bedrock")
    print("\n".join(changed))
    print(f"Checked {date.today()}: {', '.join(changed) or 'everything up to date'}", file=sys.stderr)


if __name__ == "__main__":
    main()

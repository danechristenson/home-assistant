#!/bin/sh
# For each add-on with a new major version upstream: apply it on a branch from
# main (major/<add-on>), build and smoke-test it, and open a pull request.
# Nothing is merged: a major version reaches Home Assistant only when the pull
# request is merged. A failed smoke test still opens one, as a draft, so the
# new version is visible; it needs manual work first.
#
# Closing a pull request without merging means "not this version": it isn't
# reopened, until an even newer major version comes out. An open one is
# rebuilt when main has moved on, so it stays mergeable.
#
# Usage: major_updates.sh            (in the workflow; needs GH_TOKEN)
#        DRY_RUN=1 major_updates.sh  (local: build and test, but don't push or open anything)
set -eu

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
DRY_RUN=${DRY_RUN:-}
BASE=${BASE:-origin/main}

run() {
    if [ -n "$DRY_RUN" ]; then echo "(dry run) $*"; else "$@"; fi
}

for addon in foundry_vtt matrix matrix_call_relay matrix_group_calls matrix_helper_bot minecraft_bedrock; do
    branch="major/$addon"
    git checkout -q -B "$branch" "$BASE"
    if [ -z "$(python3 .github/scripts/update_addons.py --major --only "$addon")" ]; then
        continue
    fi

    # The versions, from the changelog entry the script just wrote
    entry=$(awk '/^## /{n++} n==1' "$addon/CHANGELOG.md")
    updates=$(echo "$entry" | sed -n 's/^- Update \(.*\)\.$/\1/p' | paste -sd ';' - | sed 's/;/; /g')
    title="Major update for $addon: $updates"

    state=""
    if [ -z "$DRY_RUN" ]; then
        state=$(gh pr list --state all --head "$branch" --json title,state \
            | jq -r --arg t "$title" '[.[] | select(.title == $t) | .state][0] // ""')
    fi
    if [ "$state" = "CLOSED" ] || [ "$state" = "MERGED" ]; then
        echo "$title: already decided ($state)"
        continue
    fi
    if [ "$state" = "OPEN" ] && git merge-base --is-ancestor "$BASE" "origin/$branch" 2>/dev/null; then
        echo "$title: pull request open and up to date"
        continue
    fi

    # Twice: a download can fail now and then (GitHub has returned a 404 for a
    # release that exists), and a failed pull request would stay marked failed
    if sh .github/scripts/smoke_test.sh "$addon" || sh .github/scripts/smoke_test.sh "$addon"; then
        result="Built and smoke-tested: **passed**."
        draft=""
    else
        result="Built and smoke-tested: **failed** (see the workflow log). It needs changes before merging."
        draft="--draft"
    fi

    git add "$addon"
    git -c user.name="github-actions[bot]" -c user.email="41898282+github-actions[bot]@users.noreply.github.com" \
        commit -q -m "chore($addon): $updates"
    body=$(printf '%s\n\n%s\n\n%s\n' \
        "A new major version upstream, so the daily update left it for review." \
        "$entry" \
        "$result Merging deploys it: Home Assistant installs it automatically where Auto update is on.")

    run git push -f origin "$branch"
    if [ "$(gh pr list --state open --head "$branch" --json number --jq length 2>/dev/null || echo 0)" != "0" ] && [ -z "$DRY_RUN" ]; then
        gh pr edit "$branch" --title "$title" --body "$body"
        if [ -n "$draft" ]; then gh pr ready "$branch" --undo || true; else gh pr ready "$branch" || true; fi
    else
        # shellcheck disable=SC2086
        run gh pr create --base main --head "$branch" --title "$title" --body "$body" $draft
    fi
done
git checkout -q --detach "$BASE"

#!/usr/bin/env bash
# Runtime harness: make install-omarchy must leave a real plugin directory with
# copies of our files, replace an older symlink install without touching what it
# pointed at, and converge when run twice.
set -euo pipefail
SRC_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SRC_DIR/../.." && pwd)"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
PLUGIN="$TMP/.config/omarchy/plugins/io.github.ryong256.kanban"

install_omarchy() {
    make -s -C "$ROOT" install-omarchy HOME="$TMP" >/dev/null
}
fail() { echo "FAIL: $1" >&2; exit 1; }

assert_copy() {
    [[ -d "$PLUGIN" && ! -L "$PLUGIN" ]] || fail "$1: plugin is not a real directory"
    for f in Main.qml manifest.json; do
        [[ -f "$PLUGIN/$f" && ! -L "$PLUGIN/$f" ]] || fail "$1: $f is not a regular file"
        cmp -s "$SRC_DIR/$f" "$PLUGIN/$f" || fail "$1: $f differs from the source"
    done
}

# A fresh install copies the plugin into place.
install_omarchy
assert_copy "fresh install"

# An older symlink install is replaced, and the directory it pointed at survives.
rm -rf "$PLUGIN"
mkdir -p "$TMP/old-plugin"
echo "keep me" > "$TMP/old-plugin/Main.qml"
ln -s "$TMP/old-plugin" "$PLUGIN"
install_omarchy
assert_copy "symlink install"
left=$(shopt -s dotglob nullglob; cd "$TMP/old-plugin" && echo *)
[[ "$left" == "Main.qml" ]] || fail "symlink target gained or lost files"
[[ "$(<"$TMP/old-plugin/Main.qml")" == "keep me" ]] || fail "symlink target was overwritten"

# Running the target again converges on the same state.
install_omarchy
assert_copy "second run"

echo "OK"

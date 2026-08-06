#!/bin/bash
set -euo pipefail

# Re-fetches the vendored WIT dependencies under wit/deps from the
# WebAssembly package registry using wkg:
#   https://github.com/bytecodealliance/wasm-pkg-tools
#
# This uses `wkg get` with exact versions rather than `wkg wit fetch`
# because `wkg wit fetch` resolves at most one version per package name,
# and this library intentionally depends on two versions of several wasi
# packages (e.g. wasi:http@0.2.8 and wasi:http@0.3.0).
#
# To update a dependency: bump its version below and in wit/world.wit, run
# this script, then run ./regenerate_bindings.sh and commit the results.
#
# Uses the wkg binary from PATH by default; override with e.g.
#   WKG=~/source/bytecodealliance/wasm-pkg-tools/target/debug/wkg ./fetch_wit_deps.sh
WKG="${WKG:-wkg}"

PACKAGES=(
  "wasi:cli@0.2.8"
  "wasi:cli@0.3.0"
  "wasi:clocks@0.2.8"
  "wasi:clocks@0.3.0"
  "wasi:config@0.2.0-rc.1"
  "wasi:filesystem@0.2.8"
  "wasi:filesystem@0.3.0"
  "wasi:http@0.2.8"
  "wasi:http@0.3.0"
  "wasi:io@0.2.8"
  "wasi:logging@0.1.0-draft"
  "wasi:random@0.2.8"
  "wasi:random@0.3.0"
  "wasi:sockets@0.2.8"
  "wasi:sockets@0.3.0"
)

rm -rf wit/deps
mkdir -p wit/deps

for pkg in "${PACKAGES[@]}"; do
  # wasi:http@0.2.8 -> wit/deps/wasi-http-0.2.8/package.wit
  dir="wit/deps/$(echo "$pkg" | tr ':@' '--')"
  mkdir -p "$dir"
  "$WKG" get "$pkg" --format wit --overwrite -o "$dir/package.wit"
done

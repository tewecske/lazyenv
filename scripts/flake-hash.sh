#!/usr/bin/env bash
#
# Keeps flake.nix's vendorHash in step with go.mod/go.sum.
#
#   ./scripts/flake-hash.sh               # recompute vendorHash and its stamp (needs nix)
#   ./scripts/flake-hash.sh --check [REV] # fast check of one commit, default HEAD (the pre-push gate)
#   ./scripts/flake-hash.sh --install-hook
#
# vendorHash is a fixed-output hash: Nix evaluation accepts a stale one and only fails in the build
# that needs it, which for a flake user is `nix profile install` or `nixos-rebuild`. So next to it
# flake.nix records the sha256 of go.mod+go.sum it was computed from, and --check compares that
# stamp with the commit's files. That needs no build and no network, only git.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly REPO_ROOT
readonly HOOKS_DIR="scripts/githooks"
readonly STAMP_PREFIX="# vendorHash inputs (go.mod, go.sum): "

failures=0
ok() { printf '  \033[32mok\033[0m    %s\n' "$*"; }
bad() { printf '  \033[31mFAIL\033[0m  %s\n' "$*" >&2; failures=$((failures + 1)); }
warn() { printf '  \033[33mwarn\033[0m  %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

sha256() {
  if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi | cut -d' ' -f1
}

# Digest of go.mod+go.sum at REV, or in the working tree when REV is empty.
inputs_digest() {
  local rev="$1"
  if [ -n "$rev" ]; then
    git -C "$REPO_ROOT" show "$rev:go.mod" "$rev:go.sum" | sha256
  else
    cat "$REPO_ROOT/go.mod" "$REPO_ROOT/go.sum" | sha256
  fi
}

recorded_stamp() {
  grep -F "$STAMP_PREFIX" | head -1 | sed "s|.*$STAMP_PREFIX||" | tr -d '[:space:]'
}

check_mode() {
  local rev="${1:-HEAD}" sha flake
  sha="$(git -C "$REPO_ROOT" rev-parse --verify "$rev^{commit}")" || die "not a commit: $rev"

  # Branches from before the flake have nothing to check.
  flake="$(git -C "$REPO_ROOT" show "$sha:flake.nix" 2>/dev/null)" || return 0

  printf 'Flake check of %s (%s)\n' "$(git -C "$REPO_ROOT" rev-parse --short "$sha")" \
    "$(git -C "$REPO_ROOT" log -1 --format=%s "$sha")"

  if git -C "$REPO_ROOT" cat-file -e "$sha:flake.lock" 2>/dev/null; then
    ok "flake.lock is tracked"
  else
    bad "flake.lock is not tracked at this commit (git add it)"
  fi

  if grep -qE 'vendorHash[[:space:]]*=[[:space:]]*(lib\.fakeHash|"sha256-A{16})' <<<"$flake"; then
    bad "vendorHash is still a placeholder (run just flake-hash)"
  else
    ok "vendorHash is not a placeholder"
  fi

  local recorded digest
  recorded="$(recorded_stamp <<<"$flake")"
  digest="$(inputs_digest "$sha")"
  if [ -z "$recorded" ]; then
    bad "flake.nix has no vendorHash stamp (run just flake-hash)"
  elif [ "$recorded" = "$digest" ]; then
    ok "vendorHash matches go.mod and go.sum"
  else
    bad "vendorHash is stale: go.mod or go.sum changed since it was computed (run just flake-hash)"
  fi

  # Evaluating catches Nix errors and a lock that no longer satisfies flake.nix. Optional, so
  # contributors without Nix can still push.
  if command -v nix >/dev/null; then
    local system log
    system="$(nix eval --impure --raw --expr builtins.currentSystem)"
    log="$(mktemp)"
    if nix eval --no-update-lock-file --raw \
      "git+file://$REPO_ROOT?rev=$sha#packages.$system.lazyenv.drvPath" >/dev/null 2>"$log"; then
      ok "packages.$system.lazyenv evaluates"
    else
      bad "packages.$system.lazyenv does not evaluate:"
      sed 's/^/        /' "$log" >&2
    fi
    rm -f "$log"
  else
    warn "nix not found, skipped evaluating the flake"
  fi

  if [ "$failures" -gt 0 ]; then
    die "$failures check(s) failed. Fix, or push with --no-verify if you know better."
  fi
}

update_mode() {
  command -v nix >/dev/null || die "nix is not on PATH"
  # Globals, not locals: the EXIT trap runs after this function returns.
  flake="$REPO_ROOT/flake.nix"
  backup="$(mktemp)"
  log="$(mktemp)"
  cp "$flake" "$backup"
  trap 'cp "$backup" "$flake"; rm -f "$backup" "$log"' EXIT

  # Build the Go module derivation with a placeholder; the mismatch reports the real hash.
  sed -i.bak -E 's|vendorHash = "[^"]*";|vendorHash = lib.fakeHash;|' "$flake" && rm -f "$flake.bak"
  echo "Building the Go modules to compute vendorHash..."
  if nix build --no-link "$REPO_ROOT#lazyenv.goModules" >"$log" 2>&1; then
    die "the build accepted lib.fakeHash, which should be impossible"
  fi
  local got
  got="$(grep -oE 'got: +sha256-[A-Za-z0-9+/=]+' "$log" | awk '{print $2}' | head -1)"
  [ -n "$got" ] || { cat "$log" >&2; die "the build failed for another reason"; }

  cp "$backup" "$flake"
  sed -i.bak -E "s|vendorHash = \"[^\"]*\";|vendorHash = \"$got\";|" "$flake"
  # Basic regex, so the parentheses in the prefix match literally.
  sed -i.bak "s|$STAMP_PREFIX.*|$STAMP_PREFIX$(inputs_digest '')|" "$flake"
  rm -f "$flake.bak"
  trap 'rm -f "$backup" "$log"' EXIT

  grep -qF "$STAMP_PREFIX" "$flake" || die "no stamp line in flake.nix; add one above vendorHash"
  echo "vendorHash = $got"
}

case "${1:-}" in
  --check) check_mode "${2:-HEAD}" ;;
  --install-hook)
    git -C "$REPO_ROOT" config core.hooksPath "$HOOKS_DIR"
    echo "core.hooksPath = $HOOKS_DIR"
    ;;
  "") update_mode ;;
  *) die "unknown argument: $1" ;;
esac

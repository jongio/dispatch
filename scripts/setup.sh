#!/usr/bin/env bash
# scripts/setup.sh — Developer machine setup and verification for Dispatch.
#
# Usage:
#   ./scripts/setup.sh           # Full setup: install deps + build + verify
#   ./scripts/setup.sh --check   # Check-only: report what's installed/missing
#
# This script verifies (and optionally installs) all dependencies needed to
# build, test, and run Dispatch from source on Linux or macOS.

set -euo pipefail

CHECK_ONLY=false
if [[ "${1:-}" == "--check" ]]; then
    CHECK_ONLY=true
fi

ERRORS=()
WARNINGS=()

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
status()  { echo "  [*] $1"; }
ok()      { echo "  [OK] $1"; }
missing() { echo "  [!!] $1"; ERRORS+=("$1"); }
warn()    { echo "  [~] $1"; WARNINGS+=("$1"); }
section() { echo ""; echo "=== $1 ==="; }

has_cmd() { command -v "$1" &>/dev/null; }

get_go_version() {
    go version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+(\.[0-9]+)?' || echo ""
}

version_ge() {
    # Returns 0 if $1 >= $2 (dot-separated version comparison)
    printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1 | grep -qx "$2"
}

# ---------------------------------------------------------------------------
echo ""
echo "  Dispatch Developer Setup"
echo "  ========================"
echo ""

section "Required Dependencies"

# --- Go ---
if has_cmd go; then
    GO_VER=$(get_go_version)
    if [[ -n "$GO_VER" ]] && version_ge "$GO_VER" "1.26"; then
        ok "Go $GO_VER (>= 1.26 required)"
    else
        missing "Go $GO_VER found but >= 1.26 required. Download: https://go.dev/dl/"
    fi
else
    missing "Go not found. Install from https://go.dev/dl/ (>= 1.26 required)"
fi

# --- Git ---
if has_cmd git; then
    GIT_VER=$(git --version 2>/dev/null)
    ok "Git ($GIT_VER)"
else
    missing "Git not found. Install from https://git-scm.com/downloads"
fi

# --- Mage ---
if has_cmd mage; then
    ok "Mage ($(mage --version 2>/dev/null || echo 'installed'))"
else
    if [[ "$CHECK_ONLY" == true ]]; then
        missing "Mage not found. Install: go install github.com/magefile/mage@latest"
    else
        status "Installing Mage..."
        go install github.com/magefile/mage@latest 2>/dev/null || true
        if has_cmd mage; then
            ok "Mage installed"
        else
            missing "Mage install failed. Ensure \$GOPATH/bin is in PATH, then: go install github.com/magefile/mage@latest"
        fi
    fi
fi

# --- GitHub Copilot CLI ---
if has_cmd ghcs; then
    ok "GitHub Copilot CLI (ghcs)"
elif has_cmd copilot; then
    ok "GitHub Copilot CLI (copilot)"
else
    warn "GitHub Copilot CLI not found (ghcs/copilot). Required at runtime to have sessions to browse. Install: https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-in-the-command-line"
fi

# --- Session store ---
SESSION_STORE="$HOME/.copilot/session-store.db"
if [[ -f "$SESSION_STORE" ]]; then
    ok "Session store exists ($SESSION_STORE)"
else
    warn "Session store not found at $SESSION_STORE. Run 'ghcs' at least once to create it. Dispatch can still run in --demo mode without it."
fi

# ---------------------------------------------------------------------------
section "Optional Tools (used by 'mage preflight')"

declare -A TOOLS=(
    [golangci-lint]="go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest|Linter"
    [gofumpt]="go install mvdan.cc/gofumpt@latest|Strict formatter"
    [govulncheck]="go install golang.org/x/vuln/cmd/govulncheck@latest|Vulnerability scanner"
    [deadcode]="go install golang.org/x/tools/cmd/deadcode@latest|Dead code detector"
)

for tool in golangci-lint gofumpt govulncheck deadcode; do
    IFS='|' read -r install_cmd desc <<< "${TOOLS[$tool]}"
    if has_cmd "$tool"; then
        ok "$desc ($tool)"
    else
        if [[ "$CHECK_ONLY" == true ]]; then
            warn "$desc not found. Install: $install_cmd"
        else
            status "Installing $tool..."
            eval "$install_cmd" 2>/dev/null || true
            if has_cmd "$tool"; then
                ok "$desc ($tool) installed"
            else
                warn "$desc install failed. Ensure \$GOPATH/bin is in PATH, then: $install_cmd"
            fi
        fi
    fi
done

# --- GCC (for race detector) ---
if has_cmd gcc; then
    ok "GCC (enables race detector in tests)"
else
    warn "GCC not found. Race detector may be unavailable. Install via your package manager (apt install gcc, brew install gcc, etc.)"
fi

# ---------------------------------------------------------------------------
if [[ "$CHECK_ONLY" == false ]]; then
    section "Build & Verify"

    REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
    cd "$REPO_ROOT"

    status "Running 'go build ./...'..."
    if go build ./...; then
        ok "Build succeeded"
    else
        missing "Build failed"
    fi

    status "Running 'go test ./... -count=1'..."
    if go test ./... -count=1; then
        ok "All tests passed"
    else
        missing "Tests failed"
    fi

    if has_cmd mage; then
        status "Running 'mage install'..."
        if mage install; then
            ok "mage install succeeded - dispatch-dev is ready"
        else
            missing "mage install failed"
        fi
    fi
fi

# ---------------------------------------------------------------------------
section "Summary"

if [[ ${#ERRORS[@]} -eq 0 && ${#WARNINGS[@]} -eq 0 ]]; then
    echo ""
    echo "  All checks passed! Your machine is ready for Dispatch development."
    if [[ "$CHECK_ONLY" == false ]]; then
        echo "  Run 'dispatch-dev' or 'dispatch-dev --demo' to launch."
    fi
elif [[ ${#ERRORS[@]} -eq 0 ]]; then
    echo ""
    echo "  Setup complete with ${#WARNINGS[@]} warning(s) (non-blocking):"
    for w in "${WARNINGS[@]}"; do echo "    - $w"; done
    if [[ "$CHECK_ONLY" == false ]]; then
        echo ""
        echo "  Run 'dispatch-dev' or 'dispatch-dev --demo' to launch."
    fi
else
    echo ""
    echo "  Setup incomplete - ${#ERRORS[@]} error(s):"
    for e in "${ERRORS[@]}"; do echo "    - $e"; done
    if [[ ${#WARNINGS[@]} -gt 0 ]]; then
        echo "  ${#WARNINGS[@]} warning(s):"
        for w in "${WARNINGS[@]}"; do echo "    - $w"; done
    fi
    exit 1
fi

echo ""

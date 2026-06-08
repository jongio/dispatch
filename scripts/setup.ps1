# scripts/setup.ps1 — Developer machine setup and verification for Dispatch.
#
# Usage:
#   .\scripts\setup.ps1           # Full setup: install deps + build + verify
#   .\scripts\setup.ps1 -Check    # Check-only: report what's installed/missing
#
# This script verifies (and optionally installs) all dependencies needed to
# build, test, and run Dispatch from source on a Windows machine.

param(
    [switch]$Check  # When set, only report status without installing anything.
)

$ErrorActionPreference = 'Stop'
$Script:Errors = @()
$Script:Warnings = @()

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function Write-Status  { param([string]$M) Write-Host "  [*] $M" -ForegroundColor Cyan }
function Write-OK      { param([string]$M) Write-Host "  [OK] $M" -ForegroundColor Green }
function Write-Missing { param([string]$M) Write-Host "  [!!] $M" -ForegroundColor Red; $Script:Errors += $M }
function Write-Warn    { param([string]$M) Write-Host "  [~] $M" -ForegroundColor Yellow; $Script:Warnings += $M }
function Write-Section { param([string]$M) Write-Host "`n=== $M ===" -ForegroundColor White }

function Test-Command {
    param([string]$Name)
    $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Get-GoVersion {
    $out = & go version 2>$null
    if ($out -match 'go(\d+\.\d+(\.\d+)?)') { return $Matches[1] }
    return $null
}

# ---------------------------------------------------------------------------
# Required dependencies
# ---------------------------------------------------------------------------
Write-Host "`n  Dispatch Developer Setup" -ForegroundColor White
Write-Host "  ========================`n"

Write-Section "Required Dependencies"

# --- Go ---
if (Test-Command 'go') {
    $goVer = Get-GoVersion
    if ($goVer) {
        $parts = $goVer -split '\.'
        $major = [int]$parts[0]
        $minor = [int]$parts[1]
        if ($major -gt 1 -or ($major -eq 1 -and $minor -ge 26)) {
            Write-OK "Go $goVer (>= 1.26 required)"
        } else {
            Write-Missing "Go $goVer found but >= 1.26 required. Download: https://go.dev/dl/"
        }
    }
} else {
    Write-Missing "Go not found. Install from https://go.dev/dl/ (>= 1.26 required)"
}

# --- Git ---
if (Test-Command 'git') {
    $gitVer = & git --version 2>$null
    Write-OK "Git ($gitVer)"
} else {
    Write-Missing "Git not found. Install from https://git-scm.com/downloads"
}

# --- Mage ---
if (Test-Command 'mage') {
    $mageVer = & mage --version 2>$null
    Write-OK "Mage ($mageVer)"
} else {
    if ($Check) {
        Write-Missing "Mage not found. Install: go install github.com/magefile/mage@latest"
    } else {
        Write-Status "Installing Mage..."
        & go install github.com/magefile/mage@latest 2>&1 | Out-Null
        if (Test-Command 'mage') {
            Write-OK "Mage installed"
        } else {
            Write-Missing "Mage install failed. Run: go install github.com/magefile/mage@latest"
        }
    }
}

# --- GitHub Copilot CLI ---
$hasCopilot = (Test-Command 'ghcs') -or (Test-Command 'copilot')
if ($hasCopilot) {
    if (Test-Command 'ghcs') {
        Write-OK "GitHub Copilot CLI (ghcs)"
    } else {
        Write-OK "GitHub Copilot CLI (copilot)"
    }
} else {
    Write-Warn "GitHub Copilot CLI not found (ghcs/copilot). Required at runtime to have sessions to browse. Install: https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-in-the-command-line"
}

# --- Session store ---
$sessionStore = Join-Path $env:USERPROFILE '.copilot\session-store.db'
if (Test-Path $sessionStore) {
    Write-OK "Session store exists ($sessionStore)"
} else {
    Write-Warn "Session store not found at $sessionStore. Run 'ghcs' at least once to create it. Dispatch can still run in --demo mode without it."
}

# ---------------------------------------------------------------------------
# Optional tools (used by mage preflight)
# ---------------------------------------------------------------------------
Write-Section "Optional Tools (used by 'mage preflight')"

$optionalTools = @(
    @{ Name = 'golangci-lint'; Install = 'go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest'; Desc = 'Linter' },
    @{ Name = 'gofumpt';       Install = 'go install mvdan.cc/gofumpt@latest'; Desc = 'Strict formatter' },
    @{ Name = 'govulncheck';   Install = 'go install golang.org/x/vuln/cmd/govulncheck@latest'; Desc = 'Vulnerability scanner' },
    @{ Name = 'deadcode';      Install = 'go install golang.org/x/tools/cmd/deadcode@latest'; Desc = 'Dead code detector' }
)

foreach ($tool in $optionalTools) {
    if (Test-Command $tool.Name) {
        Write-OK "$($tool.Desc) ($($tool.Name))"
    } else {
        if ($Check) {
            Write-Warn "$($tool.Desc) not found. Install: $($tool.Install)"
        } else {
            Write-Status "Installing $($tool.Name)..."
            Invoke-Expression $tool.Install 2>&1 | Out-Null
            if (Test-Command $tool.Name) {
                Write-OK "$($tool.Desc) ($($tool.Name)) installed"
            } else {
                Write-Warn "$($tool.Desc) install failed. Run: $($tool.Install)"
            }
        }
    }
}

# --- GCC (for race detector) ---
if (Test-Command 'gcc') {
    Write-OK "GCC (enables race detector in tests)"
} else {
    Write-Warn "GCC not found. Race detector will be skipped. Install via MSYS2, MinGW, or TDM-GCC."
}

# ---------------------------------------------------------------------------
# Build & verify (skip in check-only mode)
# ---------------------------------------------------------------------------
if (-not $Check) {
    Write-Section "Build & Verify"

    $repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    if (-not (Test-Path (Join-Path $repoRoot 'go.mod'))) {
        # Try current directory
        $repoRoot = Get-Location
    }

    Push-Location $repoRoot
    try {
        Write-Status "Running 'go build ./...'..."
        & go build ./... 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-OK "Build succeeded"
        } else {
            Write-Missing "Build failed (exit code $LASTEXITCODE)"
        }

        Write-Status "Running 'go test ./... -count=1'..."
        & go test ./... -count=1 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-OK "All tests passed"
        } else {
            Write-Missing "Tests failed (exit code $LASTEXITCODE)"
        }

        if (Test-Command 'mage') {
            Write-Status "Running 'mage install'..."
            & mage install 2>&1
            if ($LASTEXITCODE -eq 0) {
                Write-OK "mage install succeeded - dispatch-dev is ready"
            } else {
                Write-Missing "mage install failed (exit code $LASTEXITCODE)"
            }
        }
    } finally {
        Pop-Location
    }
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
Write-Section "Summary"

if ($Script:Errors.Count -eq 0 -and $Script:Warnings.Count -eq 0) {
    Write-Host "`n  All checks passed! Your machine is ready for Dispatch development." -ForegroundColor Green
    if (-not $Check) {
        Write-Host "  Run 'dispatch-dev' or 'dispatch-dev --demo' to launch." -ForegroundColor Cyan
    }
} elseif ($Script:Errors.Count -eq 0) {
    Write-Host "`n  Setup complete with $($Script:Warnings.Count) warning(s) (non-blocking):" -ForegroundColor Yellow
    foreach ($w in $Script:Warnings) { Write-Host "    - $w" -ForegroundColor Yellow }
    if (-not $Check) {
        Write-Host "`n  Run 'dispatch-dev' or 'dispatch-dev --demo' to launch." -ForegroundColor Cyan
    }
} else {
    Write-Host "`n  Setup incomplete - $($Script:Errors.Count) error(s):" -ForegroundColor Red
    foreach ($e in $Script:Errors) { Write-Host "    - $e" -ForegroundColor Red }
    if ($Script:Warnings.Count -gt 0) {
        Write-Host "  $($Script:Warnings.Count) warning(s):" -ForegroundColor Yellow
        foreach ($w in $Script:Warnings) { Write-Host "    - $w" -ForegroundColor Yellow }
    }
    exit 1
}

Write-Host ""

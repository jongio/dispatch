<#
.SYNOPSIS
    Generate screenshot PNGs for the Dispatch website.

.DESCRIPTION
    Builds and runs the screenshot generator which drives the TUI through
    every visual state using --demo mode data, captures the ANSI output,
    converts it to styled HTML, then uses Playwright to render each HTML
    file as a PNG image.

    Output goes to web/public/screenshots/ by default.

.PARAMETER OutDir
    Override the output directory (default: web/public/screenshots).

.PARAMETER Check
    Verify the Go capture path against the fake session database without
    rendering PNGs. Intermediate HTML is removed before the script exits.

.EXAMPLE
    .\screenshots.ps1
    .\screenshots.ps1 -OutDir .\my-shots
    .\screenshots.ps1 -Check
#>

param(
    [string]$OutDir,
    [switch]$Check
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Push-Location "$PSScriptRoot\.."
try {
    # Verify we're in the repo root.
    if (-not (Test-Path "internal\data\testdata\fake_sessions.db")) {
        Write-Error "Run this script from the repository root."
        exit 1
    }

    $renderDir = if ($OutDir) { $OutDir } elseif ($Check) { ".screenshots-check" } else { "web\public\screenshots" }
    if ($Check -and (Test-Path $renderDir)) {
        Remove-Item $renderDir -Recurse -Force
    }

    $goArgs = @("-tags", "screenshots", "./cmd/screenshots", "--out", $renderDir)
    if ($Check) { $goArgs += "--check" }

    Write-Host "Capturing TUI states as HTML..." -ForegroundColor Cyan
    & go run @goArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Capture failed."
        exit 1
    }

    if ($Check) {
        Remove-Item $renderDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "Screenshot capture check passed." -ForegroundColor Green
        return
    }

    Write-Host "Rendering PNGs with Playwright..." -ForegroundColor Cyan
    node cmd/screenshots/render.mjs $renderDir
    if ($LASTEXITCODE -ne 0) {
        Write-Error "PNG rendering failed."
        exit 1
    }

    Write-Host "Done." -ForegroundColor Green
}
finally {
    Pop-Location
}

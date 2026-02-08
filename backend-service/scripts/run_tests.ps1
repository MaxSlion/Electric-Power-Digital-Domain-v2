<# 
.SYNOPSIS
    Backend Service Test Runner
.DESCRIPTION
    Unified test script for running unit, integration, and API tests
.PARAMETER Type
    Type of tests to run: unit, integration, api, or all
.PARAMETER Verbose
    Show verbose output
.PARAMETER Cover
    Generate coverage report
#>

param(
    [ValidateSet("unit", "integration", "api", "all")]
    [string]$Type = "all",
    
    [switch]$Verbose,
    
    [switch]$Cover
)

$ErrorActionPreference = "Stop"

# Change to backend-service directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptDir
Set-Location $projectRoot

Write-Host ""
Write-Host "=" * 60 -ForegroundColor Cyan
Write-Host "Running $Type tests..." -ForegroundColor Cyan
Write-Host "=" * 60 -ForegroundColor Cyan
Write-Host ""

$testArgs = @()

if ($Verbose) {
    $testArgs += "-v"
}

if ($Cover) {
    $testArgs += "-cover"
    $testArgs += "-coverprofile=coverage.out"
}

switch ($Type) {
    "unit" {
        # Run only unit tests (exclude integration)
        $testArgs += "./internal/..."
        $testArgs += "-run"
        $testArgs += "^Test[^I]|^TestI[^n]"
    }
    "integration" {
        # Run only integration tests
        $testArgs += "./tests/..."
    }
    "api" {
        # Run API/handler tests
        $testArgs += "./internal/http/..."
    }
    "all" {
        # Run all tests
        $testArgs += "./..."
    }
}

Write-Host "Command: go test $($testArgs -join ' ')" -ForegroundColor Gray
Write-Host ""

$startTime = Get-Date
$result = & go test @testArgs 2>&1
$endTime = Get-Date
$duration = $endTime - $startTime

$result | ForEach-Object { Write-Host $_ }

Write-Host ""
Write-Host "=" * 60 -ForegroundColor Cyan
Write-Host "Duration: $($duration.TotalSeconds.ToString('F2'))s" -ForegroundColor Gray

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ All tests passed!" -ForegroundColor Green
} else {
    Write-Host "❌ Tests failed with exit code: $LASTEXITCODE" -ForegroundColor Red
}
Write-Host "=" * 60 -ForegroundColor Cyan
Write-Host ""

if ($Cover -and (Test-Path "coverage.out")) {
    Write-Host "Generating HTML coverage report..." -ForegroundColor Yellow
    go tool cover -html=coverage.out -o coverage.html
    Write-Host "Coverage report: coverage.html" -ForegroundColor Green
}

exit $LASTEXITCODE

# Docker Compose stack smoke (S0 + health). S1-S7 run in CI via go test.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

Write-Host "==> Building and starting stack..."
docker compose up -d --build

Write-Host "==> Waiting for services..."
Start-Sleep -Seconds 15

Write-Host "==> S0: compose ps"
docker compose ps

$fail = 0
function Check-Url($name, $url) {
    try {
        Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 10 | Out-Null
        Write-Host "OK  $name $url"
    } catch {
        Write-Host "FAIL $name $url"
        $script:fail = 1
    }
}

Check-Url "ingest health" "http://localhost:8080/health"
Check-Url "ingest ready"  "http://localhost:8080/ready"
Check-Url "signal ready"  "http://localhost:8081/ready"
Check-Url "web"           "http://localhost/"

if ($fail -ne 0) {
    Write-Host "Smoke checks failed. Logs: docker compose logs"
    exit 1
}

Write-Host ""
Write-Host "Stack smoke passed (S0 + health)."
Write-Host "Next: open http://localhost for S8 (2 browsers), or run:"
Write-Host '  $env:RILLNET_REDIS_ADDRESS="localhost:6379"; go test ./tests/integration/... -run TestSmoke -count=1'

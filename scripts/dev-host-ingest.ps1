# Web UI + signal in Docker; ingest on the host (fixes WebRTC ICE on Docker Desktop Windows).
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

Write-Host "Starting redis, web, signal (host-stack, no ingest container)..."
docker compose -f docker-compose.host-stack.yml up -d --build

Write-Host "Waiting for Redis on 127.0.0.1:6379..."
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    try {
        $tcp = New-Object System.Net.Sockets.TcpClient
        $tcp.Connect("127.0.0.1", 6379)
        $tcp.Close()
        $ready = $true
        break
    } catch {
        Start-Sleep -Seconds 1
    }
}
if (-not $ready) {
    Write-Error "Redis is not reachable on 127.0.0.1:6379. Run: docker compose -f docker-compose.host-stack.yml ps"
}

$env:RILLNET_CONFIG_PATH = "configs/config.dev.host.yaml"
$env:RILLNET_REDIS_ADDRESS = "127.0.0.1:6379"
$env:RILLNET_SERVER_ADDRESS = ":8080"
$env:RILLNET_JWT_SECRET = "dev-docker-compose-secret-change-in-production"
$env:RILLNET_REDIS_ENABLED = "true"

Write-Host ""
Write-Host "Open http://localhost — ingest API uses this process on :8080"
Write-Host "Publisher tab F12: [RillNet Publisher] ice=connected"
Write-Host "Ingest logs: publisher started streaming track"
Write-Host ""

go run ./cmd/ingest

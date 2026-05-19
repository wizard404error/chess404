$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$goExe = "C:\Program Files\Go\bin\go.exe"
$webEnvExample = Join-Path $repoRoot "apps\web\.env.example"
$webEnvLocal = Join-Path $repoRoot "apps\web\.env.local"

if (-not (Test-Path $webEnvLocal) -and (Test-Path $webEnvExample)) {
    Copy-Item $webEnvExample $webEnvLocal
}

function Start-Chess404Window {
    param(
        [string]$Title,
        [string]$Workdir,
        [string[]]$EnvVars,
        [string]$Command
    )

    $envBlock = ($EnvVars | ForEach-Object { "`$env:$_" }) -join "`n"
    $script = @"
`$Host.UI.RawUI.WindowTitle = '$Title'
Set-Location '$Workdir'
$envBlock
$Command
"@

    Start-Process powershell -ArgumentList "-NoExit", "-ExecutionPolicy", "Bypass", "-Command", $script | Out-Null
}

$servicesDir = Join-Path $repoRoot "services\realtime"

# Platform Service (8083) - guest sessions, accounts, match claims, history
Start-Chess404Window `
    -Title "Chess404 Platform Service" `
    -Workdir $servicesDir `
    -EnvVars @(
        "PLATFORM_ADDR=':8083'",
        "GUEST_STORE_BACKEND='file'",
        "GUEST_STORE_PATH='data/guest-profiles.json'",
        "GUEST_STORE_SQLITE_PATH='data/guest-profiles.sqlite'",
        "MATCH_ARCHIVE_BACKEND='file'",
        "MATCH_ARCHIVE_PATH='data/match-archive.json'",
        "MATCH_ARCHIVE_SQLITE_PATH='data/match-archive.sqlite'",
        "ACCOUNT_STORE_BACKEND='file'",
        "ACCOUNT_STORE_PATH='data/accounts.json'",
        "ACCOUNT_STORE_SQLITE_PATH='data/accounts.sqlite'",
        "MATCH_CLAIM_STORE_BACKEND='memory'"
    ) `
    -Command "& '$goExe' run .\cmd\platform-service"

Start-Sleep -Seconds 1

# Match Service (8082) - WebSocket match rooms, move broadcasting
Start-Chess404Window `
    -Title "Chess404 Match Service" `
    -Workdir $servicesDir `
    -EnvVars @(
        "MATCH_SERVICE_ADDR=':8082'",
        "MATCH_ARCHIVE_BACKEND='file'",
        "MATCH_ARCHIVE_PATH='data/match-archive.json'",
        "MATCH_ARCHIVE_SQLITE_PATH='data/match-archive.sqlite'",
        "PLATFORM_SERVICE_INTERNAL_URL='http://127.0.0.1:8083'"
    ) `
    -Command "& '$goExe' run .\cmd\match-service"

Start-Sleep -Seconds 1

# Matchmaking Service (8084) - queue tickets, auto-match pairing
Start-Chess404Window `
    -Title "Chess404 Matchmaking Service" `
    -Workdir $servicesDir `
    -EnvVars @(
        "MATCHMAKING_ADDR=':8084'",
        "MATCHMAKING_TICKET_STORE_BACKEND='file'",
        "MATCHMAKING_TICKET_STORE_PATH='data/matchmaking-tickets.json'",
        "MATCHMAKING_TICKET_STORE_SQLITE_PATH='data/matchmaking-tickets.sqlite'",
        "MATCH_SERVICE_INTERNAL_URL='http://127.0.0.1:8082'",
        "PLATFORM_SERVICE_INTERNAL_URL='http://127.0.0.1:8083'"
    ) `
    -Command "& '$goExe' run .\cmd\matchmaking-service"

Start-Sleep -Seconds 1

# Gateway (8090) - external-facing reverse proxy / auth entrypoint
Start-Chess404Window `
    -Title "Chess404 Gateway" `
    -Workdir $servicesDir `
    -EnvVars @(
        "GATEWAY_ADDR=':8090'",
        "MATCH_SERVICE_INTERNAL_URL='http://127.0.0.1:8082'",
        "PLATFORM_SERVICE_INTERNAL_URL='http://127.0.0.1:8083'",
        "MATCHMAKING_SERVICE_INTERNAL_URL='http://127.0.0.1:8084'"
    ) `
    -Command "& '$goExe' run .\cmd\gateway"

Start-Sleep -Seconds 1

# Next.js web app
Start-Chess404Window `
    -Title "Chess404 Web" `
    -Workdir $repoRoot `
    -EnvVars @() `
    -Command "pnpm --filter @chess404/web dev"

Write-Host ""
Write-Host "Chess404 local stack is starting in separate PowerShell windows."
Write-Host "NOTE: Go services compile on first launch (~30s). Subsequent starts are instant."
Write-Host ""
Write-Host "Web app:             http://localhost:3000"
Write-Host "Platform service:    http://localhost:8083/healthz"
Write-Host "Match service:       http://localhost:8082/healthz"
Write-Host "Matchmaking service: http://localhost:8084/healthz"
Write-Host "Gateway:             http://localhost:8090/healthz"

$ErrorActionPreference = "Stop"

$repoRoot   = Split-Path -Parent $MyInvocation.MyCommand.Path
$goExe      = "C:\Program Files\Go\bin\go.exe"
$credsFile  = Join-Path $repoRoot ".pg-creds.env"

# ── Load Postgres credentials ─────────────────────────────────────────────────
if (-not (Test-Path $credsFile)) {
    Write-Host ""
    Write-Host "No .pg-creds.env found. Run .\setup-postgres.ps1 first to create the database and save credentials."
    Write-Host ""
    Write-Host "Alternatively, create .pg-creds.env manually:"
    Write-Host "  PG_HOST=127.0.0.1"
    Write-Host "  PG_PORT=5432"
    Write-Host "  PG_USER=postgres"
    Write-Host "  PG_PASS=yourpassword"
    Write-Host "  PG_DB=chess404"
    Write-Host "  PG_DSN=postgres://postgres:yourpassword@127.0.0.1:5432/chess404?sslmode=disable"
    exit 1
}

$pgCreds = @{}
Get-Content $credsFile | ForEach-Object {
    if ($_ -match "^([^=]+)=(.*)$") {
        $pgCreds[$matches[1].Trim()] = $matches[2].Trim()
    }
}

$pgDsn = $pgCreds["PG_DSN"]
if (-not $pgDsn) {
    Write-Error ".pg-creds.env is missing PG_DSN. Run .\setup-postgres.ps1 again."
    exit 1
}

Write-Host ""
Write-Host "Starting Chess404 with Postgres backends..."
Write-Host "DSN: $pgDsn"
Write-Host ""

$webEnvExample = Join-Path $repoRoot "apps\web\.env.example"
$webEnvLocal   = Join-Path $repoRoot "apps\web\.env.local"
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

# Platform Service (8083) – Postgres for guests, accounts, archive, and claims via memory
Start-Chess404Window `
    -Title "Chess404 Platform (Postgres)" `
    -Workdir $servicesDir `
    -EnvVars @(
        "PLATFORM_ADDR=':8083'",
        "GUEST_STORE_BACKEND='postgres'",
        "GUEST_STORE_POSTGRES_URL='$pgDsn'",
        "MATCH_ARCHIVE_BACKEND='postgres'",
        "MATCH_ARCHIVE_POSTGRES_URL='$pgDsn'",
        "ACCOUNT_STORE_BACKEND='postgres'",
        "ACCOUNT_STORE_POSTGRES_URL='$pgDsn'",
        "MATCH_CLAIM_STORE_BACKEND='memory'"
    ) `
    -Command "& '$goExe' run .\cmd\platform-service"

Start-Sleep -Seconds 2

# Match Service (8082) – Postgres for archive
Start-Chess404Window `
    -Title "Chess404 Match (Postgres)" `
    -Workdir $servicesDir `
    -EnvVars @(
        "MATCH_SERVICE_ADDR=':8082'",
        "MATCH_ARCHIVE_BACKEND='postgres'",
        "MATCH_ARCHIVE_POSTGRES_URL='$pgDsn'",
        "PLATFORM_SERVICE_INTERNAL_URL='http://127.0.0.1:8083'"
    ) `
    -Command "& '$goExe' run .\cmd\match-service"

Start-Sleep -Seconds 1

# Matchmaking Service (8084) – file backend (Redis not required locally)
Start-Chess404Window `
    -Title "Chess404 Matchmaking" `
    -Workdir $servicesDir `
    -EnvVars @(
        "MATCHMAKING_ADDR=':8084'",
        "MATCHMAKING_TICKET_STORE_BACKEND='file'",
        "MATCHMAKING_TICKET_STORE_PATH='data/matchmaking-tickets.json'",
        "MATCH_SERVICE_INTERNAL_URL='http://127.0.0.1:8082'",
        "PLATFORM_SERVICE_INTERNAL_URL='http://127.0.0.1:8083'"
    ) `
    -Command "& '$goExe' run .\cmd\matchmaking-service"

Start-Sleep -Seconds 1

# Gateway (8090)
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
Write-Host "Chess404 Postgres stack is starting in separate PowerShell windows."
Write-Host "NOTE: Go services compile on first launch (~30s). Subsequent starts are instant."
Write-Host ""
Write-Host "Web app:             http://localhost:3000"
Write-Host "Platform service:    http://localhost:8083/healthz"
Write-Host "Match service:       http://localhost:8082/healthz"
Write-Host "Matchmaking service: http://localhost:8084/healthz"
Write-Host "Gateway:             http://localhost:8090/healthz"
Write-Host ""
Write-Host "Backends: platform=postgres, match=postgres, matchmaking=file"

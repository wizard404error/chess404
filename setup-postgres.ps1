$ErrorActionPreference = "Stop"

$repoRoot   = Split-Path -Parent $MyInvocation.MyCommand.Path
$psqlCand   = @(
    "C:\Program Files\PostgreSQL\18\bin\psql.exe",
    "C:\Program Files\PostgreSQL\17\bin\psql.exe",
    "C:\Program Files\PostgreSQL\16\bin\psql.exe",
    "C:\Program Files\PostgreSQL\15\bin\psql.exe"
)
$psql = $psqlCand | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $psql) {
    Write-Error "psql.exe not found. Make sure PostgreSQL is installed."
    exit 1
}
Write-Host "Using: $psql"

# ── Collect credentials ────────────────────────────────────────────────────────
$pgHost = Read-Host "Postgres host [127.0.0.1]"
if (-not $pgHost) { $pgHost = "127.0.0.1" }

$pgPort = Read-Host "Postgres port [5432]"
if (-not $pgPort) { $pgPort = "5432" }

$pgUser = Read-Host "Postgres superuser [postgres]"
if (-not $pgUser) { $pgUser = "postgres" }

$pgPassRaw = Read-Host "Postgres password (input hidden)" -AsSecureString
$pgPass    = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto(
                 [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($pgPassRaw))

$pgDb = Read-Host "App database name [chess404]"
if (-not $pgDb) { $pgDb = "chess404" }

$env:PGPASSWORD = $pgPass

# ── Test connection ────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "Testing connection to postgres..."
$testResult = & $psql -h $pgHost -p $pgPort -U $pgUser -c "SELECT version();" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "Could not connect to Postgres: $testResult"
    exit 1
}
Write-Host "Connection OK."

# ── Create database ────────────────────────────────────────────────────────────
Write-Host "Creating database '$pgDb' (if it does not already exist)..."
& $psql -h $pgHost -p $pgPort -U $pgUser -c "SELECT 1 FROM pg_database WHERE datname='$pgDb'" | Out-Null
$exists = & $psql -h $pgHost -p $pgPort -U $pgUser -tAc "SELECT COUNT(*) FROM pg_database WHERE datname='$pgDb'"
if ($exists.Trim() -eq "1") {
    Write-Host "  database '$pgDb' already exists."
} else {
    & $psql -h $pgHost -p $pgPort -U $pgUser -c "CREATE DATABASE $pgDb;" | Out-Null
    Write-Host "  database '$pgDb' created."
}

# ── Save creds to .pg-creds.env (gitignored) ──────────────────────────────────
$credsFile  = Join-Path $repoRoot ".pg-creds.env"
$encodedPass = [System.Net.WebUtility]::UrlEncode($pgPass)
$dsn = "postgres://${pgUser}:${encodedPass}@${pgHost}:${pgPort}/${pgDb}?sslmode=disable"
@"
PG_HOST=$pgHost
PG_PORT=$pgPort
PG_USER=$pgUser
PG_PASS=$pgPass
PG_DB=$pgDb
PG_DSN=$dsn
"@ | Set-Content -Encoding UTF8 $credsFile
Write-Host ""
Write-Host "Credentials saved to .pg-creds.env (add to .gitignore if not already there)."
Write-Host "DSN: $dsn"
Write-Host ""
Write-Host "Setup complete. Run .\start-local-postgres.ps1 to start the postgres-backed stack."

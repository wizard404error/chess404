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
        [string]$Command
    )

    $script = @"
`$Host.UI.RawUI.WindowTitle = '$Title'
Set-Location '$Workdir'
$Command
"@

    Start-Process powershell -ArgumentList "-NoExit", "-ExecutionPolicy", "Bypass", "-Command", $script | Out-Null
}

$servicesDir = Join-Path $repoRoot "services\realtime"

Start-Chess404Window `
    -Title "Chess404 Gateway" `
    -Workdir $servicesDir `
    -Command "`$env:GATEWAY_ADDR=':8090'; & '$goExe' run .\cmd\gateway"

Start-Chess404Window `
    -Title "Chess404 Match Service" `
    -Workdir $servicesDir `
    -Command "`$env:MATCH_SERVICE_ADDR=':8082'; & '$goExe' run .\cmd\match-service"

Start-Chess404Window `
    -Title "Chess404 Platform Service" `
    -Workdir $servicesDir `
    -Command "`$env:PLATFORM_ADDR=':8083'; & '$goExe' run .\cmd\platform-service"

Start-Chess404Window `
    -Title "Chess404 Matchmaking Service" `
    -Workdir $servicesDir `
    -Command "`$env:MATCHMAKING_ADDR=':8084'; & '$goExe' run .\cmd\matchmaking-service"

Start-Chess404Window `
    -Title "Chess404 Web" `
    -Workdir $repoRoot `
    -Command "pnpm --filter @chess404/web dev"

Write-Host ""
Write-Host "Chess404 local stack is starting in separate PowerShell windows."
Write-Host "Web app: http://localhost:3000"
Write-Host "Gateway: http://localhost:8090/healthz"
Write-Host "Match service: http://localhost:8082/healthz"
Write-Host "Platform service: http://localhost:8083/healthz"
Write-Host "Matchmaking service: http://localhost:8084/healthz"

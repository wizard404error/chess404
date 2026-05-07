$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

& (Join-Path $repoRoot "stop-local.ps1")
Start-Sleep -Seconds 2
& (Join-Path $repoRoot "start-local.ps1")

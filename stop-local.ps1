$ErrorActionPreference = "Stop"

$ports = @(3000, 8082, 8083, 8084, 8090)
$launcherWindows = Get-Process powershell -ErrorAction SilentlyContinue |
    Where-Object {
        $_.MainWindowTitle -like "Chess404 *" -or
        $_.MainWindowTitle -like "next-server*"
    }

foreach ($window in $launcherWindows) {
    try {
        Write-Host "Stopping window $($window.MainWindowTitle) (PID $($window.Id))..."
        $null = $window.CloseMainWindow()
    } catch {
        Write-Warning ("Could not stop launcher window PID {0}: {1}" -f $window.Id, $_.Exception.Message)
    }
}

Start-Sleep -Seconds 3

$remainingConnections = Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
    Where-Object { $_.LocalPort -in $ports }

if (-not $remainingConnections) {
    Write-Host "Chess404 local processes stopped."
    return
}

$remainingByPort = $remainingConnections |
    Sort-Object LocalPort |
    Select-Object LocalPort, OwningProcess

$table = $remainingByPort | Format-Table -AutoSize | Out-String
throw "Some Chess404 ports are still in use after stop attempt.`n$table`nClose the remaining service windows manually, then run restart-local.ps1 again."

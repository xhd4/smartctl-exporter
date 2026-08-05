param(
    [string]$ServiceName = "smartctl-exporter",
    [string]$DistDir = "dist",
    [string]$ExeName = "smartctl-exporter.exe"
)

# Ensure the script is running as Administrator; if not, relaunch elevated
$currId = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($currId)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "[stop-service] Restarting with administrator privileges for service '$ServiceName'" -ForegroundColor Yellow
    $argList = @(
        '-NoProfile'
        '-ExecutionPolicy','Bypass'
        '-File',"`"$PSCommandPath`""
        '-ServiceName',"`"$ServiceName`""
        '-DistDir',"`"$DistDir`""
        '-ExeName',"`"$ExeName`""
    )

    Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -Verb RunAs
    exit 0
}

Write-Host "[stop-service] Trying to stop service '$ServiceName'" -ForegroundColor Cyan

# Stop service, process and release binary
$svc = Get-Service $ServiceName -ErrorAction SilentlyContinue
if ($svc) {
    if ($svc.Status -ne 'Stopped') {
        Write-Host "[stop-service] Service '$ServiceName' found with status '$($svc.Status)'. Stopping..." -ForegroundColor Cyan
        Stop-Service $svc -Force -ErrorAction SilentlyContinue
        try {
            $svc.WaitForStatus('Stopped','00:00:15')
            Write-Host "[stop-service] Service '$ServiceName' stopped." -ForegroundColor Green
        } catch {
            Write-Host "[stop-service] Failed to wait for service '$ServiceName' to stop (it may still have stopped)." -ForegroundColor Yellow
        }
    } else {
        Write-Host "[stop-service] Service '$ServiceName' is already stopped." -ForegroundColor DarkGray
    }
} else {
    Write-Host "[stop-service] Service '$ServiceName' not found (maybe not installed or renamed)." -ForegroundColor DarkGray
}

$procName = [System.IO.Path]::GetFileNameWithoutExtension($ExeName)
$p = Get-Process -Name $procName -ErrorAction SilentlyContinue
if ($p) {
    Write-Host "[stop-service] Process '$procName' found (PID: $($p.Id -join ', ')). Stopping..." -ForegroundColor Cyan
    $p | Stop-Process -Force -ErrorAction SilentlyContinue
} else {
    Write-Host "[stop-service] Process '$procName' not found." -ForegroundColor DarkGray
}

$exePath = Join-Path $DistDir $ExeName
Write-Host "[stop-service] Checking binary '$exePath'" -ForegroundColor Cyan
if (Test-Path $exePath) {
    try {
        Remove-Item -Force $exePath -ErrorAction Stop
        Write-Host "[stop-service] Old binary '$exePath' removed." -ForegroundColor Green
    } catch {
        Write-Host "[stop-service] Failed to remove '$exePath'. File may be locked: $($_.Exception.Message)" -ForegroundColor Red
    }
} else {
    Write-Host "[stop-service] Binary '$exePath' not found - nothing to delete." -ForegroundColor DarkGray
}

Write-Host "[stop-service] Done." -ForegroundColor Green

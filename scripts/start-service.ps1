param(
    [string]$ServiceName = "smartctl-exporter",
    [string]$DistDir = "dist",
    [string]$ExeName = "smartctl-exporter.exe",
    [string]$ConfigFile = ""
)

$currId = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($currId)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "[start-service] Restarting with administrator privileges for service '$ServiceName'" -ForegroundColor Yellow
    $argList = @(
        '-NoProfile'
        '-ExecutionPolicy','Bypass'
        '-File',"`"$PSCommandPath`""
        '-ServiceName',"`"$ServiceName`""
        '-DistDir',"`"$DistDir`""
        '-ExeName',"`"$ExeName`""
    )
    if ($ConfigFile -ne "") {
        $argList += @('-ConfigFile', "`"$ConfigFile`"")
    }
    Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -Verb RunAs
    exit 0
}

Write-Host "[start-service] Trying to start service or binary '$ServiceName'." -ForegroundColor Cyan

$svc = Get-Service $ServiceName -ErrorAction SilentlyContinue
if ($svc) {
    if ($svc.Status -ne 'Running') {
        Start-Service $svc -ErrorAction SilentlyContinue
        try {
            $svc.WaitForStatus('Running','00:00:15')
            Write-Host "[start-service] Service '$ServiceName' is running." -ForegroundColor Green
        } catch {
            Write-Host "[start-service] Failed to wait for service start." -ForegroundColor Yellow
        }
    } else {
        Write-Host "[start-service] Service already running." -ForegroundColor DarkGray
    }
    exit 0
}

$exePath = Join-Path $DistDir $ExeName
if (Test-Path $exePath) {
    $args = @()
    if ($ConfigFile -ne "") {
        $args = @("--config.file=$ConfigFile")
    }
    Write-Host "[start-service] Starting '$exePath'." -ForegroundColor Green
    Start-Process -FilePath $exePath -ArgumentList $args -WorkingDirectory $DistDir
} else {
    Write-Host "[start-service] Binary not found: $exePath" -ForegroundColor Red
}

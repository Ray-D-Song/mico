param(
    [string]$InstallDir = "$env:USERPROFILE\bin"
)

$ErrorActionPreference = "Stop"

$Repo = "Ray-D-Song/mico"
$Bin  = "mico"
$BaseUrl = "https://github.com/$Repo/releases/latest/download"

$OS = "windows"
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "amd64" }
$Target = "${Bin}_${OS}_${Arch}.exe"

$TmpDir = Join-Path $env:TEMP "mico_install_$(Get-Random)"
New-Item -ItemType Directory -Force -Path $TmpDir | Out-Null

try {
    Write-Host "--> Downloading mico windows/${Arch}..."
    $ProgressPreference = 'SilentlyContinue'

    Invoke-WebRequest -Uri "$BaseUrl/$Target" -OutFile "$TmpDir\mico.exe"
    Invoke-WebRequest -Uri "$BaseUrl/checksums.txt" -OutFile "$TmpDir\checksums.txt"

    Write-Host "--> Verifying checksum..."
    $Expected = (Select-String -Path "$TmpDir\checksums.txt" -Pattern $Target).Line.Split(" ")[0]
    $Actual   = (Get-FileHash -Path "$TmpDir\mico.exe" -Algorithm SHA256).Hash.ToLower()
    if ($Expected -ne $Actual) {
        throw "Checksum mismatch! Expected: $Expected, Got: $Actual"
    }

    if (!(Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    Move-Item -Force "$TmpDir\mico.exe" "$InstallDir\mico.exe"

    Write-Host "--> Installed to $InstallDir\mico.exe"

    if ($env:PATH -notlike "*$InstallDir*") {
        Write-Host ""
        Write-Host "[!] $InstallDir is not in your PATH." -ForegroundColor Yellow
        Write-Host "    Run this in an admin PowerShell to add it permanently:"
        Write-Host ""
        Write-Host "    [Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$InstallDir', 'User')"
        Write-Host ""
    }

    Write-Host "--> Done! Run 'mico --help' to get started."
} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}

#Requires -Version 5.1
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$Repo = 'bitomia/realm'
$InstallDir = if ($env:REALM_INSTALL_DIR) { $env:REALM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\realm' }

function Get-LatestTag {
    $url = "https://github.com/$Repo/releases/latest"
    $resp = Invoke-WebRequest -Uri $url -MaximumRedirection 0 -UseBasicParsing -ErrorAction SilentlyContinue
    if (-not $resp) {
        $resp = [System.Net.WebRequest]::Create($url)
        $resp.AllowAutoRedirect = $false
        $r = $resp.GetResponse()
        $location = $r.Headers['Location']
        $r.Close()
    } else {
        $location = $resp.Headers.Location
    }
    if ($location) {
        return ($location -split '/tag/')[-1]
    }
    return $null
}

function Verify-Checksum {
    param([string]$File, [string]$ChecksumFile)
    $expected = (Get-Content $ChecksumFile -Raw).Trim().Split()[0]
    $actual = (Get-FileHash -Algorithm SHA256 -Path $File).Hash.ToLower()
    if ($expected.ToLower() -ne $actual) {
        Write-Error "checksum mismatch`n  expected: $expected`n  actual:   $actual"
    }
}

if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    Write-Error "unsupported architecture: $env:PROCESSOR_ARCHITECTURE (only amd64 is supported)"
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$tag = Get-LatestTag
if (-not $tag) {
    Write-Error 'could not determine latest release'
}
Write-Host "Installing $tag..."

$assetName = 'realm-windows-amd64.zip'
$url = "https://github.com/$Repo/releases/download/$tag/$assetName"
$checksumUrl = "$url.sha256"

$tmpdir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmpdir | Out-Null

try {
    $assetPath = Join-Path $tmpdir $assetName
    $checksumPath = "$assetPath.sha256"

    Write-Host "Downloading $assetName..."
    Invoke-WebRequest -Uri $url -OutFile $assetPath -UseBasicParsing
    Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath -UseBasicParsing

    Write-Host 'Verifying checksum...'
    Verify-Checksum -File $assetPath -ChecksumFile $checksumPath

    Write-Host "Extracting to $InstallDir..."
    Expand-Archive -Path $assetPath -DestinationPath $InstallDir -Force

    $binary = Join-Path $InstallDir 'realm.exe'
    Write-Host "realm $tag installed to $binary"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $paths = if ($userPath) { $userPath -split ';' } else { @() }
    if ($paths -notcontains $InstallDir) {
        Write-Host ''
        Write-Host "Add $InstallDir to your PATH:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"`$env:Path;$InstallDir`", 'User')"
    }
} finally {
    Remove-Item -Recurse -Force $tmpdir -ErrorAction SilentlyContinue
}

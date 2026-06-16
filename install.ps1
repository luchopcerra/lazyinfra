$ErrorActionPreference = "Stop"

$Repo = "luchopcerra/lazyinfra"
$BinaryName = "lazyinfra"
$InstallDir = Join-Path $HOME ".lazyinfra\bin"

function Fail($Message) {
    Write-Error "lazyinfra install error: $Message"
    exit 1
}

function Get-AssetArchitecture {
    $arch = if ($env:PROCESSOR_ARCHITEW6432) {
        $env:PROCESSOR_ARCHITEW6432
    }
    else {
        $env:PROCESSOR_ARCHITECTURE
    }

    switch ($arch) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { Fail "unsupported architecture: $arch" }
    }
}

Write-Host "Detecting latest lazyinfra release..."
$releaseUri = "https://api.github.com/repos/$Repo/releases/latest"
$release = Invoke-RestMethod -Uri $releaseUri -Headers @{ "User-Agent" = "lazyinfra-installer" }

if (-not $release.tag_name) {
    Fail "could not determine the latest release tag from GitHub"
}

$tag = [string]$release.tag_name
$version = $tag.TrimStart("v")
$arch = Get-AssetArchitecture
$archive = "${BinaryName}_${version}_windows_${arch}.zip"
$asset = $release.assets | Where-Object { $_.name -eq $archive } | Select-Object -First 1

if (-not $asset) {
    Fail "release asset not found for windows/${arch}: $archive"
}

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("lazyinfra-install-" + [System.Guid]::NewGuid().ToString("N"))
$zipPath = Join-Path $tmpDir $archive
$extractDir = Join-Path $tmpDir "extract"

try {
    New-Item -ItemType Directory -Path $tmpDir, $extractDir -Force | Out-Null

    Write-Host "Downloading $archive..."
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath -Headers @{ "User-Agent" = "lazyinfra-installer" }

    Write-Host "Extracting archive..."
    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

    $binaryPath = Get-ChildItem -Path $extractDir -Recurse -Filter "$BinaryName.exe" |
        Select-Object -First 1 -ExpandProperty FullName

    if (-not $binaryPath) {
        Fail "archive did not contain $BinaryName.exe"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $target = Join-Path $InstallDir "$BinaryName.exe"

    Write-Host "Installing to $target..."
    Copy-Item -Path $binaryPath -Destination $target -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @()
    if ($userPath) {
        $pathEntries = $userPath -split ";" | Where-Object { $_ }
    }

    $alreadyOnPath = $pathEntries | Where-Object { $_.TrimEnd("\") -ieq $InstallDir.TrimEnd("\") }
    if (-not $alreadyOnPath) {
        $newUserPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        Write-Host "Added $InstallDir to your User PATH."
    }

    $currentPathEntries = $env:Path -split ";" | Where-Object { $_ }
    $alreadyOnCurrentPath = $currentPathEntries | Where-Object { $_.TrimEnd("\") -ieq $InstallDir.TrimEnd("\") }
    if (-not $alreadyOnCurrentPath) {
        $env:Path = "$env:Path;$InstallDir"
    }

    Write-Host "lazyinfra $tag installed successfully."
    Write-Host "Run 'lazyinfra --help' to get started. Open a new terminal if the command is not found."
}
finally {
    if (Test-Path $tmpDir) {
        Remove-Item -Path $tmpDir -Recurse -Force
    }
}

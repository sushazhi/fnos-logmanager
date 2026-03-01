param(
    [string]$Version,
    [switch]$ForceDownload,
    [switch]$SkipVueBuild
)

$ErrorActionPreference = "Stop"

$PROJECT_DIR = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }
$MANIFEST_FILE = Join-Path $PROJECT_DIR "manifest"

if ($Version) {
    $APP_VERSION = $Version.Trim()
    Write-Host "Using version: $APP_VERSION" -ForegroundColor Cyan
} else {
    $Version = ""
    $lines = Get-Content $MANIFEST_FILE
    foreach ($line in $lines) {
        if ($line -match "^version\s*=\s*(\S+)") {
            $Version = $matches[1].Trim()
            break
        }
    }
    if (-not $Version) {
        Write-Host "Error: Cannot read version from manifest" -ForegroundColor Red
        exit 1
    }
    $APP_VERSION = $Version
    Write-Host "Using manifest version: $APP_VERSION" -ForegroundColor Cyan
}

$BUILD_DIR = Join-Path $PROJECT_DIR ".local-build"
$VERSION_FILE = Join-Path $BUILD_DIR "versions.json"
$FNPACK_URL = "https://static2.fnnas.com/fnpack/fnpack-1.2.1-windows-amd64"

function Get-VersionInfo {
    if (Test-Path $VERSION_FILE) {
        try {
            return Get-Content $VERSION_FILE -Raw | ConvertFrom-Json
        } catch {
            return @{}
        }
    }
    return @{}
}

function Save-VersionInfo {
    param($Component, $Version)
    $versions = Get-VersionInfo
    if ($versions -is [System.Management.Automation.PSCustomObject]) {
        $hash = @{}
        $versions.PSObject.Properties | ForEach-Object { $hash[$_.Name] = $_.Value }
        $versions = $hash
    }
    $versions[$Component] = $Version
    $versions | ConvertTo-Json -Depth 10 | Set-Content $VERSION_FILE -Force
}

function Test-VersionMatch {
    param($Component, $ExpectedVersion)
    $versions = Get-VersionInfo
    return ($versions.$Component -eq $ExpectedVersion)
}

function Get-FileDirect {
    param($Url, $OutFile, $Description, $Component, $Version)

    if ((-not $ForceDownload) -and (Test-Path $OutFile) -and ((Get-Item $OutFile).Length -gt 0)) {
        if (Test-VersionMatch -Component $Component -ExpectedVersion $Version) {
            Write-Host "  Using cached $Description (version $Version)" -ForegroundColor Green
            return $true
        }
    }

    Write-Host "  Downloading $Description..." -ForegroundColor Yellow

    try {
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing
        if ((Test-Path $OutFile) -and (Get-Item $OutFile).Length -gt 0) {
            Write-Host "  Download $Description success" -ForegroundColor Green
            Save-VersionInfo -Component $Component -Version $Version
            return $true
        }
    } catch {
        Write-Host "  Error: Download $Description failed - $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }

    return $false
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  fnOS LogManager - Build" -ForegroundColor Cyan
Write-Host "  Version: $APP_VERSION" -ForegroundColor Gray
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "[1/5] Setup build directory..." -ForegroundColor Yellow
$dirsToClean = @("app\ui\assets", "app\ui\images", "app\server", "cmd", "config", "wizard")
foreach ($dir in $dirsToClean) {
    $cleanPath = Join-Path $BUILD_DIR $dir
    if (Test-Path $cleanPath) {
        Remove-Item -Path "$cleanPath\*" -Recurse -Force -ErrorAction SilentlyContinue
    }
}
@("app\server", "app\ui", "cmd", "config", "wizard") | ForEach-Object {
    New-Item -ItemType Directory -Force -Path (Join-Path $BUILD_DIR $_) | Out-Null
}
Write-Host "  Build directory ready" -ForegroundColor Green

Write-Host "[2/5] Build Vue frontend..." -ForegroundColor Yellow
$UI_DIR = Join-Path $PROJECT_DIR "app\ui"
$VERSION_PLACEHOLDER = "__APP_VERSION__"
$appHeaderPath = Join-Path $UI_DIR "src\components\AppHeader.vue"
$useUpdatePath = Join-Path $UI_DIR "src\composables\useUpdate.js"
$originalAppHeaderContent = $null
$originalUseUpdateContent = $null

if (-not $SkipVueBuild) {
    if (Test-Path "$UI_DIR\package.json") {
        # Save original content and inject version into files BEFORE build
        if (Test-Path $appHeaderPath) {
            Write-Host "  Injecting version into AppHeader.vue..." -ForegroundColor Yellow
            $originalAppHeaderContent = Get-Content $appHeaderPath -Raw -Encoding UTF8
            $appHeaderContent = $originalAppHeaderContent -replace [regex]::Escape($VERSION_PLACEHOLDER), $APP_VERSION
            [System.IO.File]::WriteAllText($appHeaderPath, $appHeaderContent, [System.Text.Encoding]::UTF8)
            Write-Host "  Version $APP_VERSION injected into AppHeader.vue" -ForegroundColor Green
        }
        
        if (Test-Path $useUpdatePath) {
            Write-Host "  Injecting version into useUpdate.js..." -ForegroundColor Yellow
            $originalUseUpdateContent = Get-Content $useUpdatePath -Raw -Encoding UTF8
            $useUpdateContent = $originalUseUpdateContent -replace [regex]::Escape($VERSION_PLACEHOLDER), $APP_VERSION
            [System.IO.File]::WriteAllText($useUpdatePath, $useUpdateContent, [System.Text.Encoding]::UTF8)
            Write-Host "  Version $APP_VERSION injected into useUpdate.js" -ForegroundColor Green
        }
        
        Push-Location $UI_DIR
        
        if (-not (Test-Path "node_modules")) {
            Write-Host "  Installing npm dependencies..." -ForegroundColor Yellow
            $npmInstall = npm install 2>&1
            if ($LASTEXITCODE -ne 0) {
                Write-Host "  Error: npm install failed" -ForegroundColor Red
                Write-Host $npmInstall
                Pop-Location
                if ($null -ne $originalAppHeaderContent) {
                    [System.IO.File]::WriteAllText($appHeaderPath, $originalAppHeaderContent, [System.Text.Encoding]::UTF8)
                }
                if ($null -ne $originalUseUpdateContent) {
                    [System.IO.File]::WriteAllText($useUpdatePath, $originalUseUpdateContent, [System.Text.Encoding]::UTF8)
                }
                exit 1
            }
        }
        
        Write-Host "  Building Vue app..." -ForegroundColor Yellow
        $npmBuild = npm run build 2>&1
        $buildExitCode = $LASTEXITCODE
        
        Pop-Location
        
        # Restore original file contents
        if ($null -ne $originalAppHeaderContent) {
            [System.IO.File]::WriteAllText($appHeaderPath, $originalAppHeaderContent, [System.Text.Encoding]::UTF8)
            Write-Host "  Restored original AppHeader.vue" -ForegroundColor Green
        }
        if ($null -ne $originalUseUpdateContent) {
            [System.IO.File]::WriteAllText($useUpdatePath, $originalUseUpdateContent, [System.Text.Encoding]::UTF8)
            Write-Host "  Restored original useUpdate.js" -ForegroundColor Green
        }
        
        if ($buildExitCode -ne 0) {
            Write-Host "  Error: Vue build failed" -ForegroundColor Red
            Write-Host $npmBuild
            exit 1
        }
        
        if (Test-Path "$UI_DIR\dist") {
            Write-Host "  Vue build complete" -ForegroundColor Green
        } else {
            Write-Host "  Error: Vue build output not found" -ForegroundColor Red
            exit 1
        }
    }
} else {
    Write-Host "  Skipping Vue build" -ForegroundColor Yellow
}

Write-Host "[3/5] Copy project files..." -ForegroundColor Yellow
Copy-Item "$PROJECT_DIR\cmd\*" "$BUILD_DIR\cmd\" -Recurse -Force
Copy-Item "$PROJECT_DIR\config\*" "$BUILD_DIR\config\" -Recurse -Force
Copy-Item "$PROJECT_DIR\wizard\*" "$BUILD_DIR\wizard\" -Recurse -Force

$manifestContent = Get-Content $MANIFEST_FILE -Raw -Encoding UTF8
$manifestContent = $manifestContent -replace "(?m)^version\s*=.*", "version = $APP_VERSION"
[System.IO.File]::WriteAllText("$BUILD_DIR\manifest", $manifestContent, [System.Text.Encoding]::UTF8)

@("LICENSE", "ICON.PNG", "ICON_256.PNG") | ForEach-Object {
    if (Test-Path "$PROJECT_DIR\$_") { Copy-Item "$PROJECT_DIR\$_" "$BUILD_DIR\" -Force }
}

$UI_DIST = Join-Path $UI_DIR "dist"
if (Test-Path $UI_DIST) {
    Copy-Item "$UI_DIST\*" "$BUILD_DIR\app\ui\" -Recurse -Force
    if (Test-Path "$UI_DIR\config") { Copy-Item "$UI_DIR\config" "$BUILD_DIR\app\ui\" -Force }
    if (Test-Path "$UI_DIR\images") { Copy-Item "$UI_DIR\images" "$BUILD_DIR\app\ui\" -Recurse -Force }
    Write-Host "  Vue dist files copied" -ForegroundColor Green
} else {
    if (Test-Path "$PROJECT_DIR\app\ui\config") { Copy-Item "$PROJECT_DIR\app\ui\config" "$BUILD_DIR\app\ui\" -Force }
    if (Test-Path "$PROJECT_DIR\app\ui\images") { Copy-Item "$PROJECT_DIR\app\ui\images" "$BUILD_DIR\app\ui\" -Recurse -Force }
    if (Test-Path "$PROJECT_DIR\app\ui\index.html") { Copy-Item "$PROJECT_DIR\app\ui\index.html" "$BUILD_DIR\app\ui\" -Force }
    Write-Host "  Static UI files copied" -ForegroundColor Green
}

Write-Host "[4/5] Prepare server files..." -ForegroundColor Yellow

$serverDir = Join-Path $BUILD_DIR "app\server"
New-Item -ItemType Directory -Force -Path $serverDir | Out-Null

if (Test-Path "$PROJECT_DIR\app\server\server.js") {
    Copy-Item "$PROJECT_DIR\app\server\server.js" "$serverDir\server.js" -Force
    Copy-Item "$PROJECT_DIR\app\server\package.json" "$serverDir\package.json" -Force
    
    $subdirs = @("utils", "middleware", "services", "routes")
    foreach ($subdir in $subdirs) {
        $srcPath = Join-Path "$PROJECT_DIR\app\server" $subdir
        $dstPath = Join-Path $serverDir $subdir
        if (Test-Path $srcPath) {
            New-Item -ItemType Directory -Force -Path $dstPath | Out-Null
            Copy-Item "$srcPath\*.js" $dstPath -Force
            Write-Host "  Copied $subdir/" -ForegroundColor Green
        }
    }
    
    Write-Host "  Server files copied" -ForegroundColor Green
}

Write-Host "[5/5] Build package..." -ForegroundColor Yellow
$FNPACK_VER = "1.2.1"
$FNPACK_FILE = $FNPACK_URL.Substring($FNPACK_URL.LastIndexOf('/') + 1)
$fnpackPath = Join-Path $BUILD_DIR $FNPACK_FILE
if ((-not $ForceDownload) -and (Test-Path $fnpackPath) -and (Test-VersionMatch -Component "fnpack" -ExpectedVersion $FNPACK_VER)) {
    Write-Host "  Using cached fnpack $FNPACK_VER" -ForegroundColor Green
} else {
    if (-not (Get-FileDirect -Url $FNPACK_URL -OutFile $fnpackPath -Description "fnpack" -Component "fnpack" -Version $FNPACK_VER)) { exit 1 }
}

Remove-Item "$BUILD_DIR\logmanager.fpk" -Force -ErrorAction SilentlyContinue
Push-Location $BUILD_DIR
Start-Process -FilePath $fnpackPath -ArgumentList "build" -Wait -NoNewWindow
$ok = Test-Path "logmanager.fpk"
Pop-Location

if ($ok) {
    Move-Item "$BUILD_DIR\logmanager.fpk" "$PROJECT_DIR\logmanager-$APP_VERSION.fpk" -Force
    Write-Host "  Build success!" -ForegroundColor Green
} else {
    Write-Host "  Error: Build failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Build complete!" -ForegroundColor Green
Write-Host "  Output: logmanager-$APP_VERSION.fpk" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

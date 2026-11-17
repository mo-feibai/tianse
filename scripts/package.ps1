# 天色 —— 一键打包
#
#   pwsh -File scripts/package.ps1            # 绿色版 + 安装包
#   pwsh -File scripts/package.ps1 -SkipSetup # 只出绿色版
#
# 产物：
#   .build/bin/tianse.exe            绿色版（单文件，带图标与版本资源）
#   bin/tianse-<版本>-setup.exe      安装包（per-user，不弹 UAC）
#
# 为什么用 Inno Setup 而不是 Wails 模板自带的 NSIS：
#   Inno 装一次就有，且自带简体中文语言包；NSIS 要另外下 2.3 MB 便携版，
#   而模板里的架构检测（wails_tools.nsh 的 wails.checkArchitecture）在无桌面
#   会话下会把 x64 机器误判成非原生 AMD64，静默安装直接以退出码 65 中止。
#   Inno 的产物在本机同样无法运行验证，原因见 README「已知限制」。

param(
    [switch]$SkipSetup,
    [string]$Version = '0.1.0'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# 沙箱/受限环境下默认的 GOPATH 不可写，模块缓存统一放工作区内。
$env:GOMODCACHE = Join-Path $root '.build/gomod'
$env:GOCACHE = Join-Path $root '.build/gocache'
$env:GOBIN = Join-Path $root '.build/bin'
$env:GOSUMDB = 'off'
$env:GOFLAGS = '-mod=mod'

$wails3 = Join-Path $env:GOBIN 'wails3.exe'
$exe = Join-Path $root '.build/bin/tianse.exe'
$setup = Join-Path $root "bin/tianse-$Version-setup.exe"

function Step($text) { Write-Host "==> $text" -ForegroundColor Cyan }

function Find-Iscc {
    $pf86 = [Environment]::GetEnvironmentVariable('ProgramFiles(x86)')
    $candidates = @(
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 7\ISCC.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe'),
        (Join-Path $env:ProgramFiles 'Inno Setup 7\ISCC.exe'),
        (Join-Path $pf86 'Inno Setup 6\ISCC.exe')
    )
    foreach ($c in $candidates) { if (Test-Path $c) { return $c } }
    $cmd = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    throw '找不到 ISCC.exe。装 Inno Setup 6/7（https://jrsoftware.org/isdl.php），或把它加进 PATH。'
}

# --- 1. 前端 ---------------------------------------------------------------
Step '构建前端'
Push-Location (Join-Path $root 'frontend')
try {
    npm run build --cache (Join-Path $root '.build/npm-cache')
    if ($LASTEXITCODE -ne 0) { throw 'npm run build 失败' }
} finally { Pop-Location }

# --- 2. 图标与版本资源 ------------------------------------------------------
Step '生成 syso（图标 + 版本信息）'
if (-not (Test-Path $wails3)) { throw "找不到 $wails3，先跑 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0" }
Push-Location (Join-Path $root 'build')
try {
    & $wails3 generate syso -arch amd64 -icon windows/icon.ico -manifest windows/wails.exe.manifest -info windows/info.json -out ../wails_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw 'generate syso 失败' }
} finally { Pop-Location }

# --- 3. 绿色版 -------------------------------------------------------------
Step '编译绿色版'
# -H windowsgui 让程序不挂控制台窗口；没有它双击会先闪一个黑框。
& go build -tags production -trimpath -buildvcs=false -ldflags="-w -s -H windowsgui" -o $exe .
if ($LASTEXITCODE -ne 0) { throw 'go build 失败' }
Remove-Item -Force (Join-Path $root 'wails_windows_amd64.syso') -ErrorAction SilentlyContinue

Write-Host ("    {0}  ({1} MB)" -f $exe, [math]::Round((Get-Item $exe).Length / 1MB, 2))

if ($SkipSetup) { return }

# --- 4. 安装包 -------------------------------------------------------------
Step '打安装包（Inno Setup）'
$iscc = Find-Iscc
New-Item -ItemType Directory -Force -Path (Split-Path $setup) | Out-Null
Push-Location $root
try {
    & $iscc "/DAppVersion=$Version" "/DSourceExe=$exe" 'build\windows\inno\tianse.iss'
    if ($LASTEXITCODE -ne 0) { throw 'ISCC 编译失败' }
} finally { Pop-Location }

Write-Host ("    {0}  ({1} MB)" -f $setup, [math]::Round((Get-Item $setup).Length / 1MB, 2))
Write-Host ''
Write-Host '完成。' -ForegroundColor Green

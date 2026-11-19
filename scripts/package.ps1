# 天色 —— 一键打包
#
#   pwsh -File scripts/package.ps1                 # 用 VERSION 文件里的版本打包
#   pwsh -File scripts/package.ps1 -Bump patch    # 先升版本（0.2.0 → 0.2.1）再打包
#   pwsh -File scripts/package.ps1 -Bump minor    # 0.2.0 → 0.3.0
#   pwsh -File scripts/package.ps1 -Bump major    # 0.2.0 → 1.0.0
#   pwsh -File scripts/package.ps1 -Version 0.3.0 # 直接指定
#   pwsh -File scripts/package.ps1 -SkipSetup     # 只出绿色版
#   pwsh -File scripts/package.ps1 -OutDir D:\pub # 换发布目录（默认 ../tianse-dist）
#
# 版本号只有一处：仓库根目录的 VERSION 文件。它同时喂给
#   1. go build 的 -ldflags（internal/version.Number，--version 与诊断报告读它）
#   2. build/windows/info.json（PE 版本资源，资源管理器属性页显示的值）
#   3. Inno 的 /DAppVersion（安装包文件名与卸载项）
#   4. 前端 vite define（设置页「关于 · 版本」）
# 所以不要在别处再写死一个 0.x.y。
#
# 产物：
#   .build/bin/tianse.exe            绿色版（单文件，带图标与版本资源）
#   bin/tianse-<版本>-setup.exe      安装包（per-user，不弹 UAC）
#   ../tianse-dist/                  发布目录（默认在仓库外，见下）
#
# 产物最后会被重标为 Medium 完整性级别。原因：在受限环境下工作区目录可能带着
# Low 标签（Mandatory Label\Low），新建文件会继承它，而安装器是 asInvoker——
# 以 Low 启动后连 %TEMP% 都写不了，安装时报「无法创建目录 ...\Temp\is-XXXX.tmp，
# 错误 5：拒绝访问」。这个标签不跟着文件走内容，只跟着路径继承，所以「复制到别的
# 目录就能装」正是它的指纹。用 /setintegritylevel Medium 显式落一个标签即可。
# 该步骤需要 Medium 及以上权限运行本脚本；在 Low 上下文里会失败并给出提示。
#
# 但 Medium 只解决了装不上，没解决「打开文件 - 安全警告 / 无法验证发布者」：Windows
# 的附件逻辑会把带 Low 标签的路径当成 Internet 区域，该树下每个 exe/lnk 双击都弹。
# 这个判断看的是路径标签，不是签名——本机未签名的 magpie.exe 放在普通目录就不弹。
# 所以最后一步把两个产物复制到仓库外的发布目录（默认 ../tianse-dist），新文件不继承
# 任何标签，从那里双击安装包既装得上也不弹警告。
#
# 为什么用 Inno Setup 而不是 Wails 模板自带的 NSIS：
#   Inno 装一次就有，且自带简体中文语言包；NSIS 要另外下 2.3 MB 便携版，
#   而模板里的架构检测（wails_tools.nsh 的 wails.checkArchitecture）在无桌面
#   会话下会把 x64 机器误判成非原生 AMD64，静默安装直接以退出码 65 中止。
#   Inno 的产物在本机同样无法运行验证，原因见 README「已知限制」。

param(
    [switch]$SkipSetup,
    [string]$Version = '',
    [ValidateSet('', 'patch', 'minor', 'major')]
    [string]$Bump = '',
    [string]$OutDir = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$versionFile = Join-Path $root 'VERSION'

function Read-Version {
    if (-not (Test-Path $versionFile)) { throw "找不到 $versionFile" }
    $v = (Get-Content -Raw $versionFile).Trim()
    if ($v -notmatch '^\d+\.\d+\.\d+$') { throw "VERSION 文件内容不是 x.y.z：'$v'" }
    return $v
}

function Bump-Version([string]$current, [string]$part) {
    $n = $current.Split('.') | ForEach-Object { [int]$_ }
    switch ($part) {
        'major' { return ('{0}.0.0' -f ($n[0] + 1)) }
        'minor' { return ('{0}.{1}.0' -f $n[0], ($n[1] + 1)) }
        'patch' { return ('{0}.{1}.{2}' -f $n[0], $n[1], ($n[2] + 1)) }
    }
}

# 版本解析优先级：-Version > -Bump（基于 VERSION 当前值）> VERSION 原值。
# -Bump 会把结果写回 VERSION，-Version 也写回：这样打包过的版本就是仓库里的版本，
# 不会出现「打出来的包叫 0.3.0、VERSION 还写着 0.2.0」。
$current = Read-Version
if ($Bump) {
    if ($Version) { throw '-Version 与 -Bump 只能选一个' }
    $Version = Bump-Version $current $Bump
}
if (-not $Version) { $Version = $current }
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "版本号必须是 x.y.z：'$Version'" }
if ($Version -ne $current) {
    # 用 WriteAllText 而不是 Set-Content：后者写 CRLF，会让 VERSION 每次打包多出假改动。
    [System.IO.File]::WriteAllText($versionFile, $Version)
    Write-Host "==> 版本 $current → $Version（已写回 VERSION）" -ForegroundColor Yellow
}

# 沙箱/受限环境下默认的 GOPATH 不可写，模块缓存统一放工作区内。
$env:GOMODCACHE = Join-Path $root '.build/gomod'
$env:GOCACHE = Join-Path $root '.build/gocache'
$env:GOBIN = Join-Path $root '.build/bin'
$env:GOSUMDB = 'off'
$env:GOFLAGS = '-mod=mod'

$wails3 = Join-Path $env:GOBIN 'wails3.exe'
$exe = Join-Path $root '.build/bin/tianse.exe'
$setup = Join-Path $root "bin/tianse-$Version-setup.exe"
$infoJson = Join-Path $root 'build/windows/info.json'

function Step($text) { Write-Host "==> $text" -ForegroundColor Cyan }

# 产物默认继承工作区目录的完整性标签。带 Low 标签的 exe 双击后以 Low IL 运行，
# 连 %TEMP% 都写不了（安装器报「错误 5：拒绝访问」），所以显式落一个 Medium 标签。
# 失败不中止打包：绿色版本身能跑，只是安装包在受限目录下会装不上。
function Set-ArtifactLabel([string]$path) {
    if (-not (Test-Path $path)) { return }
    $out = & icacls $path /setintegritylevel Medium 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "    警告：无法给 $(Split-Path $path -Leaf) 设置完整性级别（$out）" -ForegroundColor Yellow
        Write-Host "          在 Low 权限的终端里跑本脚本会出现这个提示；用普通终端重跑一次即可。" -ForegroundColor Yellow
    }
}

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

# --- 2. 版本资源 -----------------------------------------------------------
# info.json 是 syso 的输入，写死版本就会让 exe 属性页停在旧号。这里按 VERSION 改写，
# 只动四个版本字段，其余（公司名、描述、版权）保持手写内容。
Step "写版本资源 $Version"
$info = Get-Content -Raw $infoJson | ConvertFrom-Json
$info.fixed.file_version = $Version
$info.fixed.product_version = $Version
foreach ($lang in $info.info.PSObject.Properties) {
    $lang.Value.FileVersion = $Version
    $lang.Value.ProductVersion = $Version
}
# 同上：转成 LF 再写，否则每次打包 info.json 整文件重写。
$json = ($info | ConvertTo-Json -Depth 8) -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($infoJson, $json + "`n")

# --- 3. 图标与版本资源 ------------------------------------------------------
Step '生成 syso（图标 + 版本信息）'
if (-not (Test-Path $wails3)) { throw "找不到 $wails3，先跑 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0" }
Push-Location (Join-Path $root 'build')
try {
    & $wails3 generate syso -arch amd64 -icon windows/icon.ico -manifest windows/wails.exe.manifest -info windows/info.json -out ../wails_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw 'generate syso 失败' }
} finally { Pop-Location }

# --- 4. 绿色版 -------------------------------------------------------------
Step '编译绿色版'
# -H windowsgui 让程序不挂控制台窗口；没有它双击会先闪一个黑框。
# -X 把版本号编进 internal/version.Number，--version 与诊断报告都读它。
$ldflags = "-w -s -H windowsgui -X tianse/internal/version.Number=$Version"
& go build -tags production -trimpath -buildvcs=false -ldflags="$ldflags" -o $exe .
if ($LASTEXITCODE -ne 0) { throw 'go build 失败' }
Remove-Item -Force (Join-Path $root 'wails_windows_amd64.syso') -ErrorAction SilentlyContinue

Set-ArtifactLabel $exe
Write-Host ("    {0}  ({1} MB)" -f $exe, [math]::Round((Get-Item $exe).Length / 1MB, 2))

if ($SkipSetup) { return }

# --- 5. 安装包 -------------------------------------------------------------
Step '打安装包（Inno Setup）'
$iscc = Find-Iscc
New-Item -ItemType Directory -Force -Path (Split-Path $setup) | Out-Null
Push-Location $root
try {
    & $iscc "/DAppVersion=$Version" "/DSourceExe=$exe" 'build\windows\inno\tianse.iss'
    if ($LASTEXITCODE -ne 0) { throw 'ISCC 编译失败' }
} finally { Pop-Location }

Set-ArtifactLabel $setup
Write-Host ("    {0}  ({1} MB)" -f $setup, [math]::Round((Get-Item $setup).Length / 1MB, 2))

# --- 7. 发布目录 -----------------------------------------------------------
# 把两个产物复制到工作区外。Windows 把带 Low 标签的路径当 Internet 区域，
# 该树下的 exe 双击就弹「打开文件 - 安全警告 / 无法验证发布者」——这跟有没有签名
# 无关，未签名但放在普通目录的 exe 不弹。副本在新目录里重新创建，不继承任何标签，
# 从发布目录双击安装包既装得上也不弹警告。
Step '复制到发布目录'
if (-not $OutDir) { $OutDir = Join-Path (Split-Path -Parent $root) 'tianse-dist' }
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$published = @()
foreach ($f in @($exe, $setup)) {
    if (-not (Test-Path $f)) { continue }
    $dest = Join-Path $OutDir (Split-Path $f -Leaf)
    Copy-Item $f $dest -Force
    $published += $dest
    Write-Host ("    {0}" -f $dest)
}
if ($published.Count -eq 0) {
    Write-Host '    警告：没有可复制的产物' -ForegroundColor Yellow
}
Write-Host ''
Write-Host "完成。版本 $Version" -ForegroundColor Green
Write-Host "安装包：$(Join-Path $OutDir (Split-Path $setup -Leaf))" -ForegroundColor Green


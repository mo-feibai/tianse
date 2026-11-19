# 天色

随天光而变的护眼工具。屏幕色温、亮度与明暗跟随本地日出日落自动变化，白天清透，入夜温润。

## 它做什么

三件事，不做第四件：

1. **色温**——按 Tanner Helland 近似生成 Gamma Ramp，日落前 2 小时从 6500 K 余弦过渡到 3400 K，日出前 2 小时回到白天档。
2. **亮度**——DDC/CI 调显示器硬件背光，以启动时读到的值为基线乘系数，30% 为安全下限，变化不足 3% 不写。
3. **明暗**——改 Windows 应用/系统主题并广播，色温降到 4500 K 转深色、回到 5200 K 转浅色（滞回，避免临界闪烁）。

不做：20-20-20 提醒、休息倒计时、蓝光眼镜广告。

## 状态

| 里程碑 | 内容 | 状态 |
|---|---|---|
| 1 | Gamma 色温闭环 | 已完成 |
| 2 | DDC/CI 亮度 | 已完成 |
| 3 | 明暗切换 | 已完成 |
| 4 | 设置界面 | 已完成 |
| 5 | 打包 | 已完成（安装包未签名，安装时会提示「未知发布者」） |

全屏挂起、开机自启、夜间模式冲突检测已并入 1–4。

设计规范见 [docs/DESIGN.md](docs/DESIGN.md)，产品与技术决策见 [docs/SPEC.md](docs/SPEC.md)。

## 开发

环境要求：Go 1.26+、Node 24+、WebView2 运行时。

```bash
# 前端
cd frontend && npm install --cache ../.build/npm-cache && npm run build

# 生成图标（改了配色或几何后跑一次）
go run ./scripts/gen-icon .

# 编译
go build -o .build/bin/tianse.exe .

# 测试（只覆盖纯逻辑：色温映射、调色板、日出日落、曲线）
go test ./internal/...
```

### 环境诊断

换一台机器后先跑这个，它会打印一份可以直接贴回来的报告：

```bash
.build/bin/tianse.exe --diagnose
```

报告内容：Windows 版本 / 当前会话与交互式桌面 / 配置目录可写性 / 每块显示器的 device name·分辨率·DDC 亮度三元组·Gamma 读写探针 / 夜间模式状态 / WebView2 版本 / 结论行（色温能否生效）。

本产品绝大多数失败是环境问题而不是逻辑问题，报告把每一项变成具名的「是/否」。

报告本身抓到过三个真 bug，修好后才有现在的读数：

1. **DDC/CI 永远报不可用。** `EnumDisplayMonitors` 的回调找到目标显示器后返回 0（语义是「停止枚举」），系统因此把整次枚举判为失败，错误信息还是一句没用的「操作成功完成」。改成回调恒定返回 1、找到后靠标志位提前短路。
2. **夜间模式永远报未配置。** CloudStore 每条记录是「容器键 + 同名子键」两层，值 `Data` 在第二层；只打开第一层能成功但读不到任何值。同时值名固定是 `Data`，不是键路径的最后一段。
3. **内层 payload 永远找不到。** Bond 信封是 struct→struct→list<int8> 三层，原来只在顶层扫 list，永远只看到 struct。改成递归下降。

修好后本机读数：DDC/CI 可用（0/100/100）、Gamma 读写均成功、夜间模式关闭。

### 没有桌面会话时看界面

本机若没有 DWM 合成（远程/服务态），WebView2 起不来，窗口无法运行。此时用预览模式在浏览器里看布局：

```bash
node .build/serve.mjs          # 静态服务，端口 25380
# 打开 http://127.0.0.1:25380/?preview=1
```

`?preview=1` 时前端读 `frontend/src/mock.ts` 的假数据，不连后端。

## 打包

一条命令出两个产物：

```powershell
pwsh -File scripts/package.ps1                 # 用 VERSION 里的版本打包
pwsh -File scripts/package.ps1 -Bump patch     # 0.2.0 → 0.2.1 再打包
pwsh -File scripts/package.ps1 -Bump minor     # 0.2.0 → 0.3.0
pwsh -File scripts/package.ps1 -Version 1.0.0  # 直接指定
pwsh -File scripts/package.ps1 -SkipSetup      # 只出绿色版
```

| 产物 | 大小 | 说明 |
|---|---|---|
| `.build/bin/tianse.exe` | 11.5 MB | 绿色版，单文件，带图标与版本资源，双击即用 |
| `bin/tianse-<版本>-setup.exe` | 6.3 MB | Inno Setup 安装包，per-user 不弹 UAC，中英双语 |
| `../tianse-dist/` | 同上 | 发布目录（仓库外），放同一份绿色版与安装包，**从这里双击安装包不会弹安全警告** |

**版本号只有一处**：仓库根目录的 `VERSION` 文件。脚本读它，然后写进四个地方——`go build` 的 `-ldflags -X tianse/internal/version.Number`（`tianse.exe --version` 与诊断报告读它）、`build/windows/info.json`（资源管理器属性页的 FileVersion/ProductVersion）、Inno 的 `/DAppVersion`（安装包文件名与卸载项）、前端 vite define（设置页「关于 · 版本」）。`-Bump` 与 `-Version` 都会把结果写回 `VERSION`，所以打包过的版本就是仓库里的版本。

脚本做七件事：构建前端 → 按 VERSION 改写 info.json → `wails3 generate syso` 把图标和版本资源编进 PE → `go build -tags production -ldflags="-w -s -H windowsgui -X tianse/internal/version.Number=<版本>"` 出绿色版 → 调 `ISCC.exe` 打安装包 → 给两个产物落 Medium 完整性标签 → 复制到仓库外的发布目录（原因见下两节）。不带 `-X` 的普通 `go build` 报 `dev`。发布目录默认是 `../tianse-dist`，用 `-OutDir` 改。

**为什么是 Inno 而不是 Wails 模板自带的 NSIS**：Inno 装一次就有，且自带简体中文语言包；NSIS 要另下 2.3 MB 便携版，而模板里的架构检测（`wails_tools.nsh` 的 `wails.checkArchitecture`）在无桌面会话下会把 x64 机器误判成非原生 AMD64，静默安装直接以退出码 65 中止。

### 安装包在 bin 目录里装不上：完整性标签

症状：双击 `bin\tianse-<版本>-setup.exe` 报「安装程序无法创建目录 `C:\Users\...\Temp\is-XXXX.tmp`。错误 5：拒绝访问。」；把同一个 exe 复制到别的目录（比如桌面）就能装。

原因不是安装脚本，是**产物继承了工作区目录的完整性标签**。受限环境下工作区被标成 `Mandatory Label\Low`，新建文件继承它：

```
icacls bin\tianse-0.3.0-setup.exe
  Mandatory Label\Low Mandatory Level:(I)(NW)   ← 继承来的
```

安装器 manifest 是 `asInvoker`，于是它以 Low 完整性级别运行，而 `%TEMP%` 是 Medium——连临时目录都创建不了。复制到桌面之所以能装，是因为**标签跟着路径继承，不跟着文件内容走**，副本在新位置没有 Low 标签。

修法是显式落一个 Medium 标签，`scripts/package.ps1` 现在每次打包都会对两个产物做这一步：

```powershell
icacls bin\tianse-0.3.0-setup.exe /setintegritylevel Medium
```

注意这一步需要 Medium 及以上权限；在 Low 权限的终端里跑脚本会打印警告但继续（绿色版本身能跑，只是安装包在受限目录下装不上）。`/setintegritylevel` 在 Low 上下文里自身也会被拒，所以别指望脚本自救。

### 双击弹出「打开文件 - 安全警告 / 无法验证发布者」

症状：双击工作区里的 `bin\tianse-<版本>-setup.exe` 或任意 exe，先弹一个带红盾的「无法验证发布者」；把同一个文件复制到别处就不弹。

**不是签名问题。** 判断依据是路径标签，不是签名：本机未签名的 `magpie.exe` 放在普通目录就不弹。

真正的机制在上一节：DSH 的 Windows 沙箱给工作区目录打了一条**可继承的 Low 完整性标签**（SACL 里一条 ACE，不是 DACL，所以 `icacls /reset` 去不掉）。Windows 的附件逻辑把带 Low 标签的路径当作 Internet 区域下载物，于是该树下每个 `.exe` / `.lnk` / `.bat` 打开时都走一遍「从网上下载的文件」流程——安全警告、未知发布者、快捷方式图标退化成通用白页。

所以「以前打包不弹」的原因很朴素：以前产物落在没打标签的目录，或者是从别处复制/下载来的（属 MyComputer 区域）。跟有没有签名无关。

现在 `scripts/package.ps1` 第七步会把两个产物复制到仓库外的 `../tianse-dist`，副本在新目录里重新创建、不继承任何标签：

```
icacls ..\tianse-dist\tianse-0.3.0-setup.exe
  （无 Mandatory Label 行）
```

**从那里双击安装包，既不弹安全警告，也不会报「错误 5」**——安装包自身已经是 Medium，不再需要复制到桌面。

如果一定要让工作区里的文件也不弹，可以整棵树去标签（需要 Medium 以上权限）：

```powershell
icacls E:\TSCODE\project\tianse /setintegritylevel "(OI)(CI)M" /T /C
```

代价是**Low 沙箱里的 shell 从此不能再写这棵树**（Low 不能写 Medium），日常改代码会失败；而且 DSH 重新授权工作区时可能再打回标签。所以推荐用发布目录，别动工作区。

真要消除 SmartScreen 的「未知发布者」字样，只有受信任的代码签名证书（EV 立刻建立信誉，OV 要累积下载量），再用 `signtool sign /f cert.pfx /fd sha256` 签两个产物。本项目没有采购证书。

## 字体

界面用两款中文字体，**子集产物已进仓库**，日常开发不需要重跑：

| 用途 | 字体 | 子集后 |
|---|---|---|
| 展示字 | 京华老宋体 KingHwa_OldSong v2.002 | 293 KB（377 字） |
| 正文字 | 霞鹜文楷 Screen LXGW WenKai Screen v1.530 | 1003 KB（3899 字） |

源文件（62 MB）在 `assets/fonts/src/`，被 git 忽略；下载地址与重新生成方式见 [assets/fonts/README.md](assets/fonts/README.md)。改了界面文案里出现的汉字后，跑一次：

```bash
python scripts/subset-fonts.py
```

展示字只收界面源码 + 城市表里出现过的字，所以**新增标题字必须重跑**，否则那两个字会回落到系统宋体。

## 配置

`%APPDATA%\Tianse\config.yaml`，日志在 `%APPDATA%\Tianse\logs\tianse.log`（1 MB 轮转，保留 3 份）。开发时可用环境变量 `TIANSE_CONFIG_DIR` 改目录。

## 授权

代码 MIT。随包字体的授权见 `assets/fonts/` 下的 LICENSE 文件。

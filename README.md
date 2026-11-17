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
| 5 | 打包 | 已完成（安装器需在有桌面的机器上复验） |

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
pwsh -File scripts/package.ps1              # 绿色版 + 安装包
pwsh -File scripts/package.ps1 -SkipSetup   # 只出绿色版
```

| 产物 | 大小 | 说明 |
|---|---|---|
| `.build/bin/tianse.exe` | 11.5 MB | 绿色版，单文件，带图标与版本资源，双击即用 |
| `bin/tianse-0.1.0-setup.exe` | 6.3 MB | Inno Setup 安装包，per-user 不弹 UAC，中英双语 |

脚本做四件事：构建前端 → `wails3 generate syso` 把图标和版本资源编进 PE → `go build -tags production -ldflags="-w -s -H windowsgui"` 出绿色版 → 调 `ISCC.exe` 打安装包。版本号用 `-Version` 改，会同时进 exe 的版本资源和安装包文件名。

**为什么是 Inno 而不是 Wails 模板自带的 NSIS**：Inno 装一次就有，且自带简体中文语言包；NSIS 要另下 2.3 MB 便携版，而模板里的架构检测（`wails_tools.nsh` 的 `wails.checkArchitecture`）在无桌面会话下会把 x64 机器误判成非原生 AMD64，静默安装直接以退出码 65 中止。

### 已知限制：安装器还没在真机上跑过

本项目开发环境是一台没有桌面合成的机器（`dwm` 与 `explorer` 都没运行），安装器在那里**启动即退**：`setup.exe` 0.5 秒后以退出码 1 结束，`/LOG` 指定的日志文件不落盘，目标目录不创建，注册表里不留卸载项。用最小化的空白 Inno 工程（`Compression=none`、`Uninstallable=no`）能同样复现，所以与本项目的脚本无关。

已排除的原因：32 位进程跑不起来（`SysWOW64\cmd.exe` 正常）、GUI 进程活不了（notepad 正常）、不能创建孙进程（`cmd /c` 正常）、沙箱禁止落 exe（写工作区和 `%TEMP%` 都成功）、请求提权被拒（manifest 是 `asInvoker`）、事件日志有崩溃记录（近 25 分钟无 1000/1001）。剩下的线索是 loader 确实解出了临时目录 `is-XXXX.tmp`，说明它跑到「解包」之后才死。

**所以安装包需要在一台正常桌面机器上装一次确认**。绿色版没有这个问题——它只是个 exe，`--diagnose` 在受限会话里也能跑完。

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

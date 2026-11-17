; 天色 —— Inno Setup 安装脚本
;
; 为什么用 Inno 而不是 Wails 模板里的 NSIS：本机已装 Inno Setup 7 且自带简体中文
; 语言包，NSIS 还要另外下载。两者产物等价，选现成的那条路。
;
; 编译（由 scripts\package.ps1 调用）：
;   ISCC.exe /DAppVersion=0.1.0 /DSourceExe=... build\windows\inno\tianse.iss

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#ifndef SourceExe
  #define SourceExe "..\..\..\.build\bin\tianse.exe"
#endif

[Setup]
; AppId 一旦发布就不能改：它是升级与卸载的识别键。
AppId={{8F3C2A54-6B71-4E9D-A2C8-1D5B7E4F9A30}
AppName=天色
AppVersion={#AppVersion}
AppVerName=天色 {#AppVersion}
AppPublisher=Tianse
AppCopyright=© 2026 Tianse
; 不写这几行，setup.exe 的「文件版本」属性是空白，描述也只有「天色 Setup」。
VersionInfoVersion={#AppVersion}
VersionInfoProductName=天色
VersionInfoDescription=天色 安装程序
VersionInfoCompany=Tianse

; 装到 %LOCALAPPDATA%\Programs，全程不需要管理员。护眼工具没有理由弹 UAC。
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
DefaultDirName={autopf}\天色
DisableProgramGroupPage=yes
DisableDirPage=auto

DefaultGroupName=天色
UninstallDisplayName=天色
UninstallDisplayIcon={app}\tianse.exe

; 一个 12 MB 的单文件程序，压缩收益有限；lzma2/max 让安装包从 11.5 MB 降到约 6 MB。
Compression=lzma2/max
SolidCompression=yes
InternalCompressLevel=max

ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0

WizardStyle=modern
SetupIconFile=..\icon.ico

; 产物落在仓库根的 bin/，与 NSIS 那条路同一个位置。
OutputDir=..\..\..\bin
OutputBaseFilename=tianse-{#AppVersion}-setup

[Languages]
Name: "zh"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

[CustomMessages]
zh.LaunchApp=立即运行 天色
zh.WebView2Missing=天色 需要 Microsoft Edge WebView2 运行时才能显示界面。现在下载安装吗？（约 2 MB，需要联网）
zh.WebView2Title=缺少 WebView2 运行时
en.LaunchApp=Launch 天色
en.WebView2Missing=天色 needs the Microsoft Edge WebView2 Runtime to show its window. Download and install it now? (about 2 MB, needs internet)
en.WebView2Title=WebView2 Runtime missing

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "tianse.exe"; Flags: ignoreversion
Source: "..\..\..\assets\fonts\LICENSE-*.txt"; DestDir: "{app}\licenses"; Flags: ignoreversion
Source: "..\..\..\LICENSE"; DestDir: "{app}\licenses"; DestName: "LICENSE-Tianse.txt"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\天色"; Filename: "{app}\tianse.exe"
Name: "{autodesktop}\天色"; Filename: "{app}\tianse.exe"; Tasks: desktopicon

[Run]
; 装 WebView2 是唯一必须联网的一步，且只在目标机器没有运行时才发生。
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "正在安装 WebView2 运行时…"; Flags: waituntilterminated; Check: PrepareBootstrapper
Filename: "{app}\tianse.exe"; Description: "{cm:LaunchApp}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; 只删程序目录。%APPDATA%\Tianse 里的配置和日志留给用户：
; 重装后颜色还是原来那套，不需要重新选城市。
Type: filesandordirs; Name: "{app}"

[Code]
function WebView2Version(): String;
var
  v: String;
begin
  Result := '';
  if RegQueryStringValue(HKLM, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', v) then
    Result := v
  else if RegQueryStringValue(HKCU, 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', v) then
    Result := v;
end;

function NeedsWebView2(): Boolean;
begin
  Result := WebView2Version() = '';
end;

// 引导程序不打进安装包（多 1.9 MB，而绝大多数机器已经有 WebView2），
// 改成运行时按需下载。下载失败不阻断安装：程序仍能启动并提示用户。
function PrepareBootstrapper(): Boolean;
var
  Url: String;
begin
  Result := False;
  if not NeedsWebView2() then
    exit;
  if MsgBox(CustomMessage('WebView2Missing'), mbConfirmation, MB_YESNO) <> IDYES then
    exit;
  Url := 'https://go.microsoft.com/fwlink/p/?LinkId=2124703';
  try
    DownloadTemporaryFile(Url, 'MicrosoftEdgeWebview2Setup.exe', '', nil);
    Result := True;
  except
    MsgBox('下载失败：' + GetExceptionMessage, mbError, MB_OK);
  end;
end;

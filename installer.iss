; Inno Setup Script for SmartPrint Desktop Agent
; Produces a Windows installer that installs to C:\Program Files\SmartPrint Agent

#define MyAppName "SmartPrint Agent"
#define MyAppVersion "1.1.0"
#define MyAppPublisher "SmartPrint"
#define MyAppExeName "SmartPrintAgent.exe"

[Setup]
AppId={{C8E11D9A-4E3F-44F1-8AC8-A78C53A37280}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\SmartPrint Agent
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
OutputDir=build\bin
OutputBaseFilename=SmartPrintAgent-Setup
SetupIconFile=build\windows\icon.ico
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64
CloseApplications=yes
CloseApplicationsFilter=*.exe
RestartApplications=no
UninstallDisplayIcon={app}\{#MyAppExeName}
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=dialog

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "Launch automatically on Windows startup (minimized in system tray)"; GroupDescription: "Startup Options:"; Flags: checkedonce

[Files]
Source: "build\bin\SmartPrintAgent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "build\bin\SmartPrintAgent.exe.manifest"; DestDir: "{app}"; Flags: ignoreversion
Source: "build\windows\icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "build\windows\icon.png"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon; IconFilename: "{app}\icon.ico"

[Registry]
; Register HKCU Run entry if autostart task is checked during setup
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "SmartPrintAgent"; ValueData: """{app}\{#MyAppExeName}"" -minimized"; Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

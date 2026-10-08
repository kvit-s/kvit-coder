; Inno Setup script for the kvit-coder Windows installer, built by
; scripts/package-installer.sh, which passes the version, the architecture,
; the folder of files to install and the output folder:
;   ISCC.exe /DKvitVersion=0.6.0 /DKvitArch=amd64 /DStageDir=<stage> /DOutputDir=<out> windows-installer.iss
;
; It installs for the current user only, without asking for administrator
; rights, into %LOCALAPPDATA%\Programs\kvit-coder. That folder stays writable
; by the user, which kvit-coder-ui's :update needs to replace the programs in
; place. Besides copying the files in the stage folder (the two programs,
; config.example.yaml, README.md and LICENSE), it:
;   - makes kc.exe and kcu.exe, the short names scripts/install.sh gives its
;     symlinks on unix, as hard links to kvit-coder.exe and kvit-coder-ui.exe
;     (copies where the disk cannot hold a hard link);
;   - adds the folder to the user's PATH, and takes it out again on uninstall;
;   - copies config.example.yaml to %USERPROFILE%\.kvit-coder\config.yaml when
;     there is no configuration yet. Uninstalling leaves %USERPROFILE%\.kvit-coder,
;     which also holds the sessions, alone.
;
; Every kvit-coder installer, for either architecture, has the same AppId, so
; a newer one upgrades an installed kvit-coder in place. Relative paths here
; are resolved against this script's folder.

#ifndef KvitVersion
  #error KvitVersion must be defined (pass /DKvitVersion=...)
#endif
#ifndef KvitArch
  #error KvitArch must be defined (pass /DKvitArch=amd64 or arm64)
#endif
#ifndef StageDir
  #error StageDir must be defined (pass /DStageDir=... the folder to install)
#endif
#ifndef OutputDir
  #define OutputDir "."
#endif
; The version as numbers only for the installer's own file version, which
; cannot hold a pre-release suffix: 0.6.0 for 0.6.0-rc1.
#ifndef KvitVersionNumeric
  #define KvitVersionNumeric KvitVersion
#endif
; The product's identity, the same in every kvit-coder installer; never change
; it. A leading "{{" in AppId is Inno's way of writing one "{".
#ifndef KvitAppId
  #define KvitAppId "{{2DEEC128-6BD2-4F6D-8C8E-3A4EFED181C0}"
#endif
#ifndef KvitAppName
  #define KvitAppName "kvit-coder"
#endif
#ifndef KvitOutputBase
  #define KvitOutputBase "kvit-coder_" + KvitVersion + "_windows_" + KvitArch + "_setup"
#endif

[Setup]
AppId={#KvitAppId}
AppName={#KvitAppName}
AppVersion={#KvitVersion}
AppPublisher=kvit-s
AppPublisherURL=https://github.com/kvit-s/kvit-coder
AppSupportURL=https://github.com/kvit-s/kvit-coder/issues
AppUpdatesURL=https://github.com/kvit-s/kvit-coder/releases
; Per-user install: no administrator prompt, and {autopf} resolves to the
; per-user programs folder, %LOCALAPPDATA%\Programs. The PATH entry is
; written to HKCU to match.
PrivilegesRequired=lowest
DefaultDirName={autopf}\{#KvitAppName}
; A terminal program has nothing to put in the Start menu.
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\kvit-coder-ui.exe
; Tells running programs to reload the environment after install and
; uninstall, so a terminal opened afterwards sees the PATH change.
ChangesEnvironment=yes
#if KvitArch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#elif KvitArch == "amd64"
; x64compatible includes Windows 11 on arm64, which runs x64 programs.
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#else
  #error KvitArch must be amd64 or arm64
#endif
; Go programs need Windows 10.
MinVersion=10.0
WizardStyle=modern
Compression=lzma2/max
SolidCompression=yes
SetupLogging=yes
OutputDir={#OutputDir}
OutputBaseFilename={#KvitOutputBase}
VersionInfoVersion={#KvitVersionNumeric}
VersionInfoTextVersion={#KvitVersion}
VersionInfoProductName={#KvitAppName}
VersionInfoProductTextVersion={#KvitVersion}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Messages]
FinishedLabelNoIcons=Setup has finished installing [name].%n%nOpen a new terminal, go to the folder you want to work in, and run kcu for the interactive front end, or kc -p "<prompt>" for one headless turn.%n%nThe first time, kcu opens :setup to choose a model provider and the models to use.

[Files]
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[UninstallDelete]
; The short names the [Code] section makes, and what :update leaves: the
; replaced programs as .old until the next start, and staged .new files it
; could not move into place.
Type: files; Name: "{app}\kc.exe"
Type: files; Name: "{app}\kcu.exe"
Type: files; Name: "{app}\*.old"
Type: files; Name: "{app}\*.new"

[Code]
const
  EnvironmentKey = 'Environment';

function CreateHardLink(lpFileName, lpExistingFileName: String; lpSecurityAttributes: Integer): Boolean;
  external 'CreateHardLinkW@kernel32.dll stdcall';

{ Makes Name in the install folder another name for Target: a hard link, or
  a copy where the disk cannot hold one. kvit-coder-ui's :update replaces
  either kind along with the program. A running kc.exe cannot be deleted but
  can be renamed, so an old one is moved aside to .old, which the next start
  of kvit-coder-ui removes. }
procedure MakeShortName(const Name, Target: String);
var
  Link, Existing: String;
begin
  Link := ExpandConstant('{app}\' + Name);
  Existing := ExpandConstant('{app}\' + Target);
  if FileExists(Link) and not DeleteFile(Link) then
  begin
    DeleteFile(Link + '.old');
    RenameFile(Link, Link + '.old');
  end;
  if CreateHardLink(Link, Existing, 0) then
    Log('Linked ' + Link + ' to ' + Existing)
  else if FileCopy(Existing, Link, False) then
    Log('Copied ' + Existing + ' to ' + Link + ' (no hard link on this disk)')
  else
    SuppressibleMsgBox('Could not create ' + Link + '. Start the programs as kvit-coder and kvit-coder-ui instead.',
      mbError, MB_OK, IDOK);
end;

function SameDir(const A, B: String): Boolean;
begin
  Result := CompareText(RemoveBackslashUnlessRoot(Trim(A)), RemoveBackslashUnlessRoot(B)) = 0;
end;

{ Returns Path without the entries that name Dir, and sets Found when there
  was one. The other entries, empty ones included, are kept as they were. }
function PathWithout(const Path, Dir: String; var Found: Boolean): String;
var
  Rest, Entry: String;
  I: Integer;
  First: Boolean;
begin
  Result := '';
  Found := False;
  First := True;
  { The added separator ends the last entry, so a trailing ";" in Path comes
    back as an empty last entry and is kept. }
  Rest := Path + ';';
  while Rest <> '' do
  begin
    I := Pos(';', Rest);
    Entry := Copy(Rest, 1, I - 1);
    Rest := Copy(Rest, I + 1, Length(Rest));
    if (Entry <> '') and SameDir(Entry, Dir) then
      Found := True
    else
    begin
      if not First then
        Result := Result + ';';
      Result := Result + Entry;
      First := False;
    end;
  end;
end;

{ The user's PATH is read and written unexpanded, so entries such as
  %USERPROFILE%\bin stay as they are. }
procedure AddToPath(const Dir: String);
var
  Path: String;
  Found: Boolean;
begin
  if not RegQueryStringValue(HKCU, EnvironmentKey, 'Path', Path) then
    Path := '';
  PathWithout(Path, Dir, Found);
  if Found then
  begin
    Log(Dir + ' is already on the user PATH');
    exit;
  end;
  { The PATH keeps its own style: one that ends in ";" still does, so
    RemoveFromPath gives back exactly the PATH this started from. }
  if Path = '' then
    Path := Dir
  else if Path[Length(Path)] = ';' then
    Path := Path + Dir + ';'
  else
    Path := Path + ';' + Dir;
  if RegWriteExpandStringValue(HKCU, EnvironmentKey, 'Path', Path) then
    Log('Added ' + Dir + ' to the user PATH')
  else
    SuppressibleMsgBox('Could not add ' + Dir + ' to your PATH. Add it by hand to run kc and kcu from any folder.',
      mbError, MB_OK, IDOK);
end;

procedure RemoveFromPath(const Dir: String);
var
  Path, Kept: String;
  Found: Boolean;
begin
  if not RegQueryStringValue(HKCU, EnvironmentKey, 'Path', Path) then
    exit;
  Kept := PathWithout(Path, Dir, Found);
  if Found and RegWriteExpandStringValue(HKCU, EnvironmentKey, 'Path', Kept) then
    Log('Removed ' + Dir + ' from the user PATH');
end;

{ A first install has nothing to run against, so it leaves a configuration
  to edit rather than an error about not finding one. Never overwrites. }
procedure SeedConfig;
var
  Dir, Config: String;
begin
  Dir := ExpandConstant('{%USERPROFILE}\.kvit-coder');
  Config := Dir + '\config.yaml';
  if FileExists(Config) then
    exit;
  if ForceDirectories(Dir) and FileCopy(ExpandConstant('{app}\config.example.yaml'), Config, True) then
    Log('Wrote ' + Config);
end;

{ The agent runs its shell commands with Git for Windows' sh.exe. This looks
  where a Git install registers itself and on the PATH; the agent itself
  looks in more places, so a miss here is only a warning. }
function GitForWindowsFound: Boolean;
var
  Root: String;
begin
  Result :=
    (RegQueryStringValue(HKLM, 'SOFTWARE\GitForWindows', 'InstallPath', Root) and FileExists(Root + '\usr\bin\sh.exe')) or
    (RegQueryStringValue(HKCU, 'SOFTWARE\GitForWindows', 'InstallPath', Root) and FileExists(Root + '\usr\bin\sh.exe')) or
    (FileSearch('sh.exe', GetEnv('PATH')) <> '') or
    (FileSearch('git.exe', GetEnv('PATH')) <> '');
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    MakeShortName('kc.exe', 'kvit-coder.exe');
    MakeShortName('kcu.exe', 'kvit-coder-ui.exe');
    AddToPath(ExpandConstant('{app}'));
    SeedConfig;
    if not GitForWindowsFound then
      SuppressibleMsgBox('Git for Windows was not found. kvit-coder runs its shell commands with Git''s sh.exe; '
        + 'install Git from https://git-scm.com/download/win and keep its default PATH option.',
        mbInformation, MB_OK, IDOK);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    RemoveFromPath(ExpandConstant('{app}'));
end;

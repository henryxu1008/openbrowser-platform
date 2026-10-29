; Ant Browser NSIS Installer Script
; Usage: makensis /DVERSION=1.4.0 /DSTAGINGDIR=C:\path\to\staging installer.nsi

Unicode True

!ifndef VERSION
  !define VERSION "1.4.0"
!endif
!ifndef STAGINGDIR
  !define STAGINGDIR "..\publish\staging"
!endif

!define PRODUCT_NAME    "硕米指纹浏览器"
!define LEGACY_PRODUCT_NAME "Ant Browser"
; Keep backup credentials at the path used by existing installations and the backend.
!define LEGACY_DATA_DIR_NAME "Ant Browser"
!define PRODUCT_EXE     "ant-chrome.exe"
!define PRODUCT_ICON    "AntBrowser.ico"
!define UNINSTALL_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\AntBrowser"
!define INSTALL_DIR     "$PROGRAMFILES64\Ant Browser"
!define POWERSHELL_EXE  "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"

!include "MUI2.nsh"
!include "LogicLib.nsh"

Var ShortcutDesktopOption

!macro MaintainShortcuts MODE
retry_shortcuts:
  ClearErrors
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\shortcut-migration.ps1" -Mode ${MODE} -InstallDir "$INSTDIR" -ProductName "${PRODUCT_NAME}" -LegacyProductName "${LEGACY_PRODUCT_NAME}" $ShortcutDesktopOption' $0
  IfErrors shortcuts_failed
  ${If} $0 == 0
    Return
  ${EndIf}
shortcuts_failed:
  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "无法完成快捷方式迁移或清理。请检查快捷方式目录权限后重试；取消将停止本次操作。" IDRETRY retry_shortcuts
  Abort "快捷方式维护未完成，操作已停止。"
!macroend

Function InstallShortcuts
  !insertmacro MaintainShortcuts Install
FunctionEnd

Function un.RemoveShortcuts
  !insertmacro MaintainShortcuts Uninstall
FunctionEnd

!macro WriteCloseProcessScript HANDLE
  FileWrite ${HANDLE} "param([string]$$InstallDir, [string]$$ExcludePath)$\r$\n"
  FileWrite ${HANDLE} "$$ErrorActionPreference = 'SilentlyContinue'$\r$\n"
  FileWrite ${HANDLE} "if ([string]::IsNullOrWhiteSpace($$InstallDir) -or -not (Test-Path -LiteralPath $$InstallDir)) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$root = [System.IO.Path]::GetFullPath($$InstallDir).TrimEnd('\') + '\'$\r$\n"
  FileWrite ${HANDLE} "$$exclude = ''$\r$\n"
  FileWrite ${HANDLE} "if (-not [string]::IsNullOrWhiteSpace($$ExcludePath)) { $$exclude = [System.IO.Path]::GetFullPath($$ExcludePath) }$\r$\n"
  FileWrite ${HANDLE} "function Get-AntBrowserProcesses {$\r$\n"
  FileWrite ${HANDLE} "  @(Get-CimInstance Win32_Process | Where-Object {$\r$\n"
  FileWrite ${HANDLE} "    $$_.ExecutablePath -and $$_.ExecutablePath.StartsWith($$root, [System.StringComparison]::OrdinalIgnoreCase) -and ($$exclude -eq '' -or -not $$_.ExecutablePath.Equals($$exclude, [System.StringComparison]::OrdinalIgnoreCase))$\r$\n"
  FileWrite ${HANDLE} "  })$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$procs = @(Get-AntBrowserProcesses)$\r$\n"
  FileWrite ${HANDLE} "if (-not $$procs -or $$procs.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$names = ($$procs | ForEach-Object { $$_.Name + '#' + $$_.ProcessId }) -join ', '$\r$\n"
  FileWrite ${HANDLE} "Write-Host ('running: ' + $$names)$\r$\n"
  FileWrite ${HANDLE} "exit 1$\r$\n"
  FileWrite ${HANDLE} "exit 0$\r$\n"
!macroend

Function CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "检查安装目录中的运行进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath ""' $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到安装目录中仍有运行进程。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n请先从窗口或托盘菜单正常退出应用，再点击“重试”重新检查；点击“取消”将终止本次安装。" IDRETRY retry_powershell IDCANCEL install_abort

install_abort:
  Abort "安装已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  MessageBox MB_OK|MB_ICONSTOP "无法安全检查安装目录中的运行进程。为避免强制中断，安装已取消。"
  Abort "安装已取消：无法安全检查安装目录中的运行进程。"

done:
FunctionEnd

Function un.CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "检查安装目录中的运行进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "$INSTDIR\Uninstall.exe"' $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到安装目录中仍有运行进程。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n请先从窗口或托盘菜单正常退出应用，再点击“重试”重新检查；点击“取消”将终止本次卸载。" IDRETRY retry_powershell IDCANCEL uninstall_abort

uninstall_abort:
  Abort "卸载已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  MessageBox MB_OK|MB_ICONSTOP "无法安全检查安装目录中的运行进程。为避免强制中断，卸载已取消。"
  Abort "卸载已取消：无法安全检查安装目录中的运行进程。"

done:
FunctionEnd

Function RegisterDiagnosticsWatcher
  IfFileExists "$INSTDIR\data\diagnostics\install-ant-chrome-diagnostics-watcher.ps1" 0 done
  DetailPrint "Registering read-only diagnostics watcher"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\data\diagnostics\install-ant-chrome-diagnostics-watcher.ps1" -InstallDir "$INSTDIR"' $0
  ${If} $0 == 0
    DetailPrint "Diagnostics watcher registered"
  ${Else}
    DetailPrint "Warning: diagnostics watcher registration failed"
  ${EndIf}
done:
FunctionEnd

Function un.UnregisterDiagnosticsWatcher
  IfFileExists "$INSTDIR\data\diagnostics\install-ant-chrome-diagnostics-watcher.ps1" 0 done
  DetailPrint "Removing read-only diagnostics watcher"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\data\diagnostics\install-ant-chrome-diagnostics-watcher.ps1" -InstallDir "$INSTDIR" -Uninstall' $0
done:
FunctionEnd

Function PreserveLegacyBackupLocalConfig
  StrCpy $4 "$INSTDIR"
  ReadRegStr $3 HKLM "${UNINSTALL_KEY}" "InstallLocation"
  ${If} "$3" != ""
    IfFileExists "$3\backup.local.yaml" 0 legacy_use_current_install
    StrCpy $4 "$3"
  ${EndIf}

legacy_use_current_install:
  IfFileExists "$APPDATA\${LEGACY_DATA_DIR_NAME}\backup.local.yaml" legacy_done
  IfFileExists "$4\backup.local.yaml" 0 legacy_done

  ClearErrors
  CreateDirectory "$APPDATA\${LEGACY_DATA_DIR_NAME}"
  IfErrors legacy_failed
  DetailPrint "Preserving legacy backup credentials"
  ClearErrors
  CopyFiles /SILENT "$4\backup.local.yaml" "$APPDATA\${LEGACY_DATA_DIR_NAME}"
  IfErrors legacy_failed
  Goto legacy_done

legacy_failed:
  MessageBox MB_ICONSTOP|MB_OK "Could not preserve the legacy backup credentials. Installation is cancelled to prevent OpenList / S3 Token loss."
  Abort "Installation cancelled: legacy backup credential migration failed."

legacy_done:
FunctionEnd

Function un.PreserveLegacyBackupLocalConfig
  IfFileExists "$INSTDIR\backup.local.yaml" 0 done
  IfFileExists "$APPDATA\${LEGACY_DATA_DIR_NAME}\backup.local.yaml" done

  ClearErrors
  CreateDirectory "$APPDATA\${LEGACY_DATA_DIR_NAME}"
  IfErrors preserve_failed

  DetailPrint "迁移旧版本地备份凭据到 $APPDATA\${LEGACY_DATA_DIR_NAME}"
  ClearErrors
  CopyFiles "$INSTDIR\backup.local.yaml" "$APPDATA\${LEGACY_DATA_DIR_NAME}\"
  IfErrors preserve_failed
  Goto done

preserve_failed:
  MessageBox MB_ICONSTOP|MB_OK "无法保存旧版本地备份凭据：$APPDATA\${LEGACY_DATA_DIR_NAME}\backup.local.yaml。为避免 OpenList / S3 Token 丢失，卸载已取消。请检查该目录权限后重试。"
  Abort "卸载已取消：旧版本地备份凭据迁移失败。"

done:
FunctionEnd

Name "${PRODUCT_NAME} ${VERSION}"
!ifndef OUTPUTFILE
  !define OUTPUTFILE "..\publish\output\AntBrowser-Setup-${VERSION}.exe"
!endif
OutFile "${OUTPUTFILE}"
InstallDir "${INSTALL_DIR}"
InstallDirRegKey HKLM "${UNINSTALL_KEY}" "InstallLocation"
RequestExecutionLevel admin
!ifdef BESTCOMPRESSION
  SetCompressor /SOLID lzma
!else
  SetCompressor lzma
!endif

!define MUI_ICON "..\build\windows\icon.ico"
!define MUI_UNICON "..\build\windows\icon.ico"

!insertmacro MUI_PAGE_WELCOME
!define MUI_DIRECTORYPAGE_TEXT_TOP "请选择${PRODUCT_NAME}的安装目录。配置、数据库和浏览器档案保存在当前 Windows 用户的本地应用数据目录，升级不会重置已有用户数据。"
!define MUI_DIRECTORYPAGE_TEXT_DESTINATION "程序安装目录"
!insertmacro MUI_PAGE_DIRECTORY
!define MUI_COMPONENTSPAGE_SMALLDESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
; Launch from the normal desktop/start-menu shortcut, never as the elevated installer user.
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

Section "${PRODUCT_NAME}（必选）" SecMain
  SectionIn RO
  Call CloseInstalledProcesses
  Call PreserveLegacyBackupLocalConfig
  SetOutPath "$INSTDIR"
  File "${STAGINGDIR}\${PRODUCT_EXE}"
  File "${__FILEDIR__}\shortcut-migration.ps1"
  File "${STAGINGDIR}\oidc-public.json"
  File "${STAGINGDIR}\shuomi-public.json"
  File /oname=config.defaults.yaml "${STAGINGDIR}\config.yaml"
  File /oname=${PRODUCT_ICON} "..\build\windows\icon.ico"
!if /FileExists "${STAGINGDIR}\config.yaml"
  IfFileExists "$INSTDIR\config.yaml" +2 0
    File "${STAGINGDIR}\config.yaml"
!else
  !echo "Warning: ${STAGINGDIR}\\config.yaml not found, installer will use runtime defaults."
!endif
!if /FileExists "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR\chrome"
  File /r "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR"
!endif
  CreateDirectory "$INSTDIR\data"
  SetOutPath "$INSTDIR\data\diagnostics"
  File /r "${STAGINGDIR}\data\diagnostics\*"
  SetOutPath "$INSTDIR"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName"     "${PRODUCT_NAME}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "Publisher"       "Ant Chrome Team"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayIcon"     "$INSTDIR\${PRODUCT_ICON}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "NoModify"        "1"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "NoRepair"        "1"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  Call RegisterDiagnosticsWatcher
SectionEnd

Section "Proxy Runtime (xray / sing-box)" SecRuntime
  SectionIn RO
  SetOutPath "$INSTDIR\bin"
  File "${STAGINGDIR}\bin\xray.exe"
  File "${STAGINGDIR}\bin\sing-box.exe"
SectionEnd

Section /o "Desktop Shortcut" SecDesktop
  StrCpy $ShortcutDesktopOption "-CreateDesktopShortcut"
SectionEnd

Section -FinalizeShortcuts
  Call InstallShortcuts
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain}    "${PRODUCT_NAME}主程序及默认配置（必选）"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRuntime} "xray and sing-box proxy tools (vless/vmess/hysteria2)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a shortcut on the desktop"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  Call un.CloseInstalledProcesses
  Call un.UnregisterDiagnosticsWatcher
  Call un.PreserveLegacyBackupLocalConfig
  Call un.RemoveShortcuts

  Delete /REBOOTOK "$INSTDIR\${PRODUCT_EXE}"
  Delete /REBOOTOK "$INSTDIR\oidc-public.json"
  Delete /REBOOTOK "$INSTDIR\shuomi-public.json"
  Delete /REBOOTOK "$INSTDIR\config.defaults.yaml"
  Delete /REBOOTOK "$INSTDIR\${PRODUCT_ICON}"
  Delete /REBOOTOK "$INSTDIR\Uninstall.exe"
  Delete /REBOOTOK "$INSTDIR\shortcut-migration.ps1"
  RMDir /r /REBOOTOK "$INSTDIR\bin"
  RMDir /r /REBOOTOK "$INSTDIR\chrome"
  DeleteRegKey HKLM "${UNINSTALL_KEY}"
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "是否清理旧版安装目录中的遗留数据？$\r$\n$\r$\n选择“是”将删除安装目录残留的旧版 data、日志和配置，操作不可恢复。选择“否”保留它们。$\r$\n$\r$\n各 Windows 用户本地应用数据目录中的数据均会保留，重新安装后可继续使用。" IDYES un_remove_all_data IDNO un_keep_user_data

un_remove_all_data:
  RMDir /r /REBOOTOK "$INSTDIR\data"
  RMDir /r /REBOOTOK "$INSTDIR\logs"
  RMDir /r /REBOOTOK "$INSTDIR"
  Goto un_done

un_keep_user_data:
  RMDir /REBOOTOK "$INSTDIR"
  Goto un_done

un_done:
  IfFileExists "$INSTDIR\." 0 +2
    MessageBox MB_ICONEXCLAMATION|MB_OK "检测到部分文件仍被占用，已标记为重启后自动删除。"
SectionEnd

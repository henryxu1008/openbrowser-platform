# Compatible with the Windows PowerShell 5.1 used by the installer.
[CmdletBinding()]
param(
    [ValidateSet('Install', 'Uninstall')][string]$Mode = 'Install',
    [string]$InstallDir,
    [string]$ProductName,
    [string]$LegacyProductName,
    [switch]$CreateDesktopShortcut
)

if (-not ('Shuomi.ShortcutFile' -as [type])) {
    Add-Type -Path (Join-Path $PSScriptRoot 'shell-link.cs')
}

function Invoke-BrowserShortcutMaintenance {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][ValidateSet('Install', 'Uninstall')][string]$Mode,
        [Parameter(Mandatory)][string]$InstallDir,
        [Parameter(Mandatory)][string]$ProductName,
        [Parameter(Mandatory)][string]$LegacyProductName,
        [Parameter(Mandatory)][string[]]$ProgramsRoots,
        [Parameter(Mandatory)][string[]]$DesktopRoots,
        [switch]$CreateDesktopShortcut
    )
    $ErrorActionPreference = 'Stop'
    foreach ($name in @($ProductName, $LegacyProductName)) {
        if ([string]::IsNullOrWhiteSpace($name) -or $name -in @('.', '..') -or
            $name.IndexOfAny([IO.Path]::GetInvalidFileNameChars()) -ge 0) {
            throw 'Shortcut product names must be single directory names.'
        }
    }
    if (-not [IO.Path]::IsPathRooted($InstallDir)) { throw 'An absolute installation directory is required.' }
    $installRoot = [IO.Path]::GetFullPath($InstallDir)
    $appTarget = Join-Path $installRoot 'ant-chrome.exe'
    $uninstallTarget = Join-Path $installRoot 'Uninstall.exe'
    $programs = @($ProgramsRoots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)
    $desktops = @($DesktopRoots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)
    if (-not $programs.Count -or -not $desktops.Count) { throw 'Shortcut directories are required.' }

    function Test-NormalShortcutPath([string]$Path) {
        foreach ($entry in @($Path, [IO.Path]::GetDirectoryName($Path))) {
            $item = Get-Item -LiteralPath $entry -Force -ErrorAction SilentlyContinue
            if ($item -and ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { return $false }
        }
        return $true
    }

    function Get-OwnedShortcut([string]$Path, [string]$Target) {
        if (-not [IO.File]::Exists($Path) -or -not (Test-NormalShortcutPath $Path)) { return $null }
        try {
            $link = [Shuomi.ShortcutFile]::Read($Path)
            if ([string]::IsNullOrWhiteSpace($link.TargetPath)) { return $null }
            $resolvedTarget = [IO.Path]::GetFullPath([Environment]::ExpandEnvironmentVariables($link.TargetPath))
            if (-not $resolvedTarget.Equals($Target, [StringComparison]::OrdinalIgnoreCase)) { return $null }
            return [pscustomobject]@{ Arguments = [string]$link.Arguments }
        } catch {
            Write-Warning "Keeping unreadable shortcut: $Path"
            return $null
        }
    }

    function New-OwnedShortcut([string]$Path, [string]$Target) {
        if (-not (Test-NormalShortcutPath $Path)) { Write-Warning "Keeping redirected shortcut path: $Path"; return }
        if (Test-Path -LiteralPath $Path) {
            if (-not (Get-OwnedShortcut $Path $Target)) { Write-Warning "Keeping another installation's shortcut: $Path" }
            return
        }
        [void][IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path))
        [Shuomi.ShortcutFile]::Write($Path, $Target, '', $installRoot, (Join-Path $installRoot 'AntBrowser.ico'))
        if (-not (Get-OwnedShortcut $Path $Target)) { throw "Could not create shortcut: $Path" }
    }

    function Move-OwnedShortcut([string]$Source, [string]$Destination, [string]$Target) {
        if ($Source.Equals($Destination, [StringComparison]::OrdinalIgnoreCase)) { return }
        $sourceLink = Get-OwnedShortcut $Source $Target
        if (-not $sourceLink) { return }
        if (-not (Test-NormalShortcutPath $Destination)) { Write-Warning "Keeping redirected shortcut destination: $Destination"; return }
        if (Test-Path -LiteralPath $Destination) {
            $destinationLink = Get-OwnedShortcut $Destination $Target
            if (-not $destinationLink -or $destinationLink.Arguments -cne $sourceLink.Arguments) {
                Write-Warning "Keeping legacy shortcut because its destination is in use: $Source"
                return
            }
        } else {
            [void][IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Destination))
            # Copy the Shell link bytes to retain arguments, icons and other user properties.
            [IO.File]::Copy($Source, $Destination, $false)
        }
        if (-not (Get-OwnedShortcut $Destination $Target)) { throw "Shortcut migration verification failed: $Destination" }
        Remove-Item -LiteralPath $Source -Force
    }

    function Remove-OwnedShortcut([string]$Path, [string]$Target) {
        if (Get-OwnedShortcut $Path $Target) { Remove-Item -LiteralPath $Path -Force }
    }

    function Remove-EmptyShortcutDirectory([string]$Path) {
        $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
        if ($item -and $item.PSIsContainer -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            if (@(Get-ChildItem -LiteralPath $Path -Force).Count -eq 0) { [IO.Directory]::Delete($Path, $false) }
        }
    }

    foreach ($root in $programs) {
        $legacyGroup = Join-Path $root $LegacyProductName
        $newGroup = Join-Path $root $ProductName
        if ($Mode -eq 'Install') {
            Move-OwnedShortcut (Join-Path $legacyGroup "$LegacyProductName.lnk") (Join-Path $newGroup "$ProductName.lnk") $appTarget
            Move-OwnedShortcut (Join-Path $legacyGroup 'Uninstall.lnk') (Join-Path $newGroup 'Uninstall.lnk') $uninstallTarget
        } else {
            foreach ($name in @($ProductName, $LegacyProductName) | Select-Object -Unique) {
                $group = Join-Path $root $name
                Remove-OwnedShortcut (Join-Path $group "$name.lnk") $appTarget
                Remove-OwnedShortcut (Join-Path $group 'Uninstall.lnk') $uninstallTarget
                Remove-EmptyShortcutDirectory $group
            }
        }
        Remove-EmptyShortcutDirectory $legacyGroup
    }
    foreach ($root in $desktops) {
        if ($Mode -eq 'Install') {
            Move-OwnedShortcut (Join-Path $root "$LegacyProductName.lnk") (Join-Path $root "$ProductName.lnk") $appTarget
        } else {
            foreach ($name in @($ProductName, $LegacyProductName) | Select-Object -Unique) {
                Remove-OwnedShortcut (Join-Path $root "$name.lnk") $appTarget
            }
        }
    }
    if ($Mode -eq 'Install') {
        $ownedGroups = @($programs | Where-Object { Get-OwnedShortcut (Join-Path (Join-Path $_ $ProductName) "$ProductName.lnk") $appTarget })
        if (-not $ownedGroups.Count) { $ownedGroups = @($programs[0]) }
        foreach ($root in $ownedGroups) {
            $group = Join-Path $root $ProductName
            New-OwnedShortcut (Join-Path $group "$ProductName.lnk") $appTarget
            New-OwnedShortcut (Join-Path $group 'Uninstall.lnk') $uninstallTarget
        }
        if ($CreateDesktopShortcut) {
            $ownedDesktops = @($desktops | Where-Object { Get-OwnedShortcut (Join-Path $_ "$ProductName.lnk") $appTarget })
            if (-not $ownedDesktops.Count) { New-OwnedShortcut (Join-Path $desktops[0] "$ProductName.lnk") $appTarget }
        }
    }
}

if ($MyInvocation.InvocationName -ne '.') {
    try {
        Invoke-BrowserShortcutMaintenance -Mode $Mode -InstallDir $InstallDir -ProductName $ProductName -LegacyProductName $LegacyProductName -CreateDesktopShortcut:$CreateDesktopShortcut -ProgramsRoots @(
            [Environment]::GetFolderPath('CommonPrograms'), [Environment]::GetFolderPath('Programs')
        ) -DesktopRoots @(
            [Environment]::GetFolderPath('CommonDesktopDirectory'), [Environment]::GetFolderPath('DesktopDirectory')
        )
        exit 0
    } catch {
        [Console]::Error.WriteLine($_.Exception.Message)
        exit 1
    }
}

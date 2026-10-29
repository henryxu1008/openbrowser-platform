#requires -Version 5.1
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../../publish/shortcut-migration.ps1')
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('ant-shortcut-tests-' + [Guid]::NewGuid().ToString('N'))
$fixtureShell = New-Object -ComObject WScript.Shell
$script:checks = 0

function Assert-ShortcutCase([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
    $script:checks++
}
function New-ShortcutCase([string]$Name) {
    $root = Join-Path $testRoot $Name
    $params = @{
        InstallDir = Join-Path $root 'installed browser'
        ProductName = '硕米指纹浏览器'
        LegacyProductName = 'Ant Browser'
        ProgramsRoots = @((Join-Path $root 'common programs'), (Join-Path $root 'user programs'))
        DesktopRoots = @((Join-Path $root 'common desktop'), (Join-Path $root 'user desktop'))
    }
    foreach ($directory in @($params.InstallDir) + $params.ProgramsRoots + $params.DesktopRoots) {
        [void][IO.Directory]::CreateDirectory($directory)
    }
    foreach ($name in @('ant-chrome.exe', 'Uninstall.exe')) {
        [IO.File]::WriteAllText((Join-Path $params.InstallDir $name), 'non-executable fixture')
    }
    return $params
}
function Write-TestShortcut([string]$Path, [string]$Target, [string]$Arguments = '') {
    [void][IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path))
    $link = $fixtureShell.CreateShortcut($Path)
    try {
        $link.TargetPath = $Target
        $link.Arguments = $Arguments
        $link.WorkingDirectory = [IO.Path]::GetDirectoryName($Target)
        $link.Description = 'preserve customized shortcut metadata'
        $link.Save()
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($link) }
}
function Get-TestShortcutTarget([string]$Path) {
    $link = $fixtureShell.CreateShortcut($Path)
    try { return $link.TargetPath } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($link) }
}

try {
    $case = New-ShortcutCase 'fresh'
    $group = Join-Path $case.ProgramsRoots[0] $case.ProductName
    Invoke-BrowserShortcutMaintenance @case -Mode Install
    Assert-ShortcutCase ((Get-TestShortcutTarget (Join-Path $group ($case.ProductName + '.lnk'))) -ieq (Join-Path $case.InstallDir 'ant-chrome.exe')) 'Fresh installation lacks its application shortcut.'
    Assert-ShortcutCase ((Get-TestShortcutTarget (Join-Path $group 'Uninstall.lnk')) -ieq (Join-Path $case.InstallDir 'Uninstall.exe')) 'Fresh installation lacks its uninstall shortcut.'
    Assert-ShortcutCase (-not (Test-Path (Join-Path $case.DesktopRoots[0] ($case.ProductName + '.lnk')))) 'Fresh installation created an unselected desktop shortcut.'
    Invoke-BrowserShortcutMaintenance @case -Mode Install -CreateDesktopShortcut
    Assert-ShortcutCase (Test-Path (Join-Path $case.DesktopRoots[0] ($case.ProductName + '.lnk'))) 'Selected desktop shortcut was not created.'
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    Assert-ShortcutCase (-not (Test-Path $group)) 'Uninstall left its empty program group.'
    Assert-ShortcutCase (-not (Test-Path (Join-Path $case.DesktopRoots[0] ($case.ProductName + '.lnk')))) 'Uninstall left its desktop shortcut.'

    foreach ($scope in @(0, 1)) {
        $case = New-ShortcutCase "upgrade-$scope"
        $oldGroup = Join-Path $case.ProgramsRoots[$scope] $case.LegacyProductName
        $newGroup = Join-Path $case.ProgramsRoots[$scope] $case.ProductName
        $oldApp = Join-Path $oldGroup ($case.LegacyProductName + '.lnk')
        $newApp = Join-Path $newGroup ($case.ProductName + '.lnk')
        Write-TestShortcut $oldApp (Join-Path $case.InstallDir 'ant-chrome.exe') '--profile-directory="retained profile"'
        Write-TestShortcut (Join-Path $oldGroup 'Uninstall.lnk') (Join-Path $case.InstallDir 'Uninstall.exe')
        Write-TestShortcut (Join-Path $case.DesktopRoots[$scope] ($case.LegacyProductName + '.lnk')) (Join-Path $case.InstallDir 'ant-chrome.exe')
        $before = (Get-FileHash -LiteralPath $oldApp -Algorithm SHA256).Hash
        Invoke-BrowserShortcutMaintenance @case -Mode Install
        Assert-ShortcutCase (-not (Test-Path $oldGroup)) "Legacy program group remains for scope $scope."
        Assert-ShortcutCase ((Get-FileHash -LiteralPath $newApp -Algorithm SHA256).Hash -eq $before) 'Migration lost shortcut arguments or metadata.'
        Assert-ShortcutCase (-not (Test-Path (Join-Path $case.DesktopRoots[$scope] ($case.LegacyProductName + '.lnk')))) 'Legacy desktop shortcut remains.'
        Assert-ShortcutCase (Test-Path (Join-Path $case.DesktopRoots[$scope] ($case.ProductName + '.lnk'))) 'Upgrade dropped the existing desktop shortcut when the desktop option was unselected.'
        Invoke-BrowserShortcutMaintenance @case -Mode Install -CreateDesktopShortcut
        $desktopLinks = @($case.DesktopRoots | ForEach-Object { Get-ChildItem -LiteralPath $_ -Filter '*.lnk' })
        Assert-ShortcutCase ($desktopLinks.Count -eq 1) 'Repeated installation duplicated the desktop shortcut across scopes.'
        Assert-ShortcutCase ((Get-FileHash -LiteralPath $newApp -Algorithm SHA256).Hash -eq $before) 'Repeated installation changed a customized shortcut.'
        Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
        Assert-ShortcutCase (-not (Test-Path $newGroup)) 'Uninstall left the migrated program group.'
    }

    $case = New-ShortcutCase 'mixed-uninstall'
    foreach ($scope in @(0, 1)) {
        foreach ($name in @($case.ProductName, $case.LegacyProductName)) {
            $group = Join-Path $case.ProgramsRoots[$scope] $name
            Write-TestShortcut (Join-Path $group "$name.lnk") (Join-Path $case.InstallDir 'ant-chrome.exe')
            Write-TestShortcut (Join-Path $group 'Uninstall.lnk') (Join-Path $case.InstallDir 'Uninstall.exe')
            Write-TestShortcut (Join-Path $case.DesktopRoots[$scope] "$name.lnk") (Join-Path $case.InstallDir 'ant-chrome.exe')
        }
    }
    $retainedGroup = Join-Path $case.ProgramsRoots[0] $case.LegacyProductName
    [IO.File]::WriteAllText((Join-Path $retainedGroup 'user-notes.txt'), 'retain unrelated files')
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    $remainingLinks = @(Get-ChildItem -LiteralPath ([IO.Path]::GetDirectoryName($case.InstallDir)) -Recurse -Filter '*.lnk')
    Assert-ShortcutCase ($remainingLinks.Count -eq 0) 'Uninstall left an owned old or new shortcut in one of the scopes.'
    Assert-ShortcutCase (Test-Path (Join-Path $retainedGroup 'user-notes.txt')) 'Uninstall removed unrelated program-group contents.'

    $case = New-ShortcutCase 'other-installation'
    $otherTarget = Join-Path ($case.InstallDir + '-other') 'ant-chrome.exe'
    foreach ($scope in @(0, 1)) {
        foreach ($name in @($case.ProductName, $case.LegacyProductName)) {
            $group = Join-Path $case.ProgramsRoots[$scope] $name
            Write-TestShortcut (Join-Path $group "$name.lnk") $otherTarget
            Write-TestShortcut (Join-Path $group 'Uninstall.lnk') (Join-Path ([IO.Path]::GetDirectoryName($otherTarget)) 'Uninstall.exe')
            Write-TestShortcut (Join-Path $case.DesktopRoots[$scope] "$name.lnk") $otherTarget
        }
    }
    $foreignLinks = @(Get-ChildItem -LiteralPath ([IO.Path]::GetDirectoryName($case.InstallDir)) -Recurse -Filter '*.lnk')
    $before = @($foreignLinks | ForEach-Object { (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }) -join ','
    Invoke-BrowserShortcutMaintenance @case -Mode Install -CreateDesktopShortcut
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    $after = @($foreignLinks | ForEach-Object { (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }) -join ','
    Assert-ShortcutCase ($before -ceq $after) 'Another installation sharing the old or new display name was changed.'

    $case = New-ShortcutCase 'destination-conflict'
    $oldPath = Join-Path $case.DesktopRoots[0] ($case.LegacyProductName + '.lnk')
    $newPath = Join-Path $case.DesktopRoots[0] ($case.ProductName + '.lnk')
    Write-TestShortcut $oldPath (Join-Path $case.InstallDir 'ant-chrome.exe')
    Write-TestShortcut $newPath $otherTarget
    $before = (Get-FileHash -LiteralPath $newPath -Algorithm SHA256).Hash
    Invoke-BrowserShortcutMaintenance @case -Mode Install
    Assert-ShortcutCase (Test-Path $oldPath) 'Conflicting destination caused the original entry to be lost.'
    Assert-ShortcutCase ((Get-FileHash -LiteralPath $newPath -Algorithm SHA256).Hash -eq $before) 'Migration overwrote a conflicting shortcut.'
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    Assert-ShortcutCase (-not (Test-Path $oldPath)) 'Uninstall did not remove its legacy shortcut after a migration conflict.'
    Assert-ShortcutCase ((Get-FileHash -LiteralPath $newPath -Algorithm SHA256).Hash -eq $before) 'Uninstall removed a conflicting shortcut.'

    $case = New-ShortcutCase 'existing-owned-destination'
    $oldPath = Join-Path $case.DesktopRoots[0] ($case.LegacyProductName + '.lnk')
    $newPath = Join-Path $case.DesktopRoots[0] ($case.ProductName + '.lnk')
    Write-TestShortcut $oldPath (Join-Path $case.InstallDir 'ant-chrome.exe')
    Write-TestShortcut $newPath (Join-Path $case.InstallDir 'ant-chrome.exe')
    $before = (Get-FileHash -LiteralPath $newPath -Algorithm SHA256).Hash
    Invoke-BrowserShortcutMaintenance @case -Mode Install
    Assert-ShortcutCase (-not (Test-Path $oldPath)) 'Matching existing new shortcut did not replace the legacy entry.'
    Assert-ShortcutCase ((Get-FileHash -LiteralPath $newPath -Algorithm SHA256).Hash -eq $before) 'Existing new shortcut was unnecessarily rewritten.'
    Write-TestShortcut $oldPath (Join-Path $case.InstallDir 'ant-chrome.exe') '--retained-custom-argument'
    Invoke-BrowserShortcutMaintenance @case -Mode Install
    Assert-ShortcutCase (Test-Path $oldPath) 'Migration discarded conflicting custom arguments.'
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    Assert-ShortcutCase (-not (Test-Path $oldPath)) 'Uninstall left the customized legacy entry.'
    Assert-ShortcutCase (-not (Test-Path $newPath)) 'Uninstall left the existing new entry.'

    $case = New-ShortcutCase 'unreadable'
    $badLink = Join-Path $case.DesktopRoots[0] ($case.LegacyProductName + '.lnk')
    [IO.File]::WriteAllText($badLink, 'not a shell link')
    Invoke-BrowserShortcutMaintenance @case -Mode Install
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    Assert-ShortcutCase ([IO.File]::ReadAllText($badLink) -ceq 'not a shell link') 'Unreadable shortcut was changed.'

    $case = New-ShortcutCase 'redirected-group'
    $outsideGroup = Join-Path $testRoot 'junction-target'
    [void][IO.Directory]::CreateDirectory($outsideGroup)
    $redirectedGroup = Join-Path $case.ProgramsRoots[0] $case.LegacyProductName
    [void](New-Item -ItemType Junction -Path $redirectedGroup -Target $outsideGroup)
    $redirectedLink = Join-Path $outsideGroup ($case.LegacyProductName + '.lnk')
    Write-TestShortcut $redirectedLink (Join-Path $case.InstallDir 'ant-chrome.exe')
    Invoke-BrowserShortcutMaintenance @case -Mode Install
    Invoke-BrowserShortcutMaintenance @case -Mode Uninstall
    Assert-ShortcutCase (Test-Path $redirectedLink) 'Maintenance traversed a redirected program group.'
    [IO.Directory]::Delete($redirectedGroup, $false)

    Write-Output "Shortcut migration: $script:checks real Shell-link assertions PASS (PowerShell $($PSVersionTable.PSVersion)); all fixtures isolated; product executions 0."
} finally {
    [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($fixtureShell)
    $resolved = [IO.Path]::GetFullPath($testRoot)
    $temporary = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (-not $resolved.StartsWith($temporary, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolved) -notmatch '^ant-shortcut-tests-[a-f0-9]{32}$') { throw 'Refusing unexpected shortcut-fixture cleanup path.' }
    if (Test-Path -LiteralPath $resolved) { Remove-Item -LiteralPath $resolved -Recurse -Force }
}

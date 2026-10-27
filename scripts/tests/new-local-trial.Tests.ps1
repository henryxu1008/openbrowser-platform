#requires -Version 7.0
$ErrorActionPreference = 'Stop'
$launcher = Join-Path $PSScriptRoot '..\new-local-trial.ps1'
$opener = Join-Path $PSScriptRoot '..\open-local-trial.ps1'
foreach ($entry in @($launcher, $opener)) {
    $parseErrors = $null
    $tokens = $null
    [void][Management.Automation.Language.Parser]::ParseFile($entry, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count) { throw 'Launcher syntax errors.' }
}
# Dot sourcing either public entry loads contracts without opening a process.
. $opener

# The only process-start boundary is replaced before any launcher invocation.
$script:StartCalls = 0
$script:CapturedStartInfo = $null
$script:PassedCases = 0
function Invoke-AntTrialProcess {
    param([Diagnostics.ProcessStartInfo]$StartInfo)
    $script:StartCalls++
    $script:CapturedStartInfo = $StartInfo
    return [pscustomobject]@{ Id = 123456 }
}
function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}
function Assert-Rejected {
    param([scriptblock]$Action, [string]$Root)
    $before = $script:StartCalls
    $rejected = $false
    try { & $Action | Out-Null } catch { $rejected = $true }
    Assert-True $rejected 'Expected rejection.'
    Assert-True ($script:StartCalls -eq $before) 'Rejected input reached the process boundary.'
    if ($Root) { Assert-True (-not (Test-Path -LiteralPath $Root)) 'Rejected input created a trial directory.' }
    $script:PassedCases++
}

$testParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = Join-Path $testParent ('ant-local-trial-tests-' + [Guid]::NewGuid().ToString('N'))
$oldUnrelated = [Environment]::GetEnvironmentVariable('ANT_TRIAL_UNRELATED', 'Process')
try {
    New-Item -ItemType Directory -Path $testRoot | Out-Null
    $sourceDirectory = Join-Path $testRoot 'source'
    New-Item -ItemType Directory -Path $sourceDirectory | Out-Null
    $fixtureExe = Join-Path $sourceDirectory 'ant-chrome.exe'
    # Plain text deliberately cannot be a real product. It is never executed.
    [IO.File]::WriteAllText($fixtureExe, 'offline launcher fixture')
    [IO.File]::WriteAllText((Join-Path $sourceDirectory 'app.db'), 'old scene must remain')
    $fixtureHash = (Get-FileHash -LiteralPath $fixtureExe -Algorithm SHA256).Hash
    $publicFile = Join-Path $sourceDirectory 'public.json'
    [IO.File]::WriteAllText($publicFile, '{"issuer":"https://identity.example.test","clientId":"public-native-fixture"}')
    $arguments = @{ ExePath = $fixtureExe; ExpectedExeSha256 = $fixtureHash; PublicConfigPath = $publicFile }
    [Environment]::SetEnvironmentVariable('ANT_TRIAL_UNRELATED', 'do-not-inherit', 'Process')

    $newRoot = Join-Path $testRoot 'fresh'
    $result = New-AntLocalTrial @arguments -TrialRoot $newRoot
    Assert-True ($script:StartCalls -eq 1 -and $result.ProcessId -eq 123456) 'Success must call the mocked process boundary once.'
    $info = $script:CapturedStartInfo
    Assert-True ($info.FileName -eq (Join-Path $newRoot 'ant-chrome.exe') -and $info.WorkingDirectory -eq $newRoot) 'EXE/appRoot and working directory must be the fresh root.'
    Assert-True (-not $info.UseShellExecute -and $info.CreateNoWindow) 'Unexpected shell/console launch mode.'
    Assert-True ((Get-FileHash -LiteralPath $info.FileName -Algorithm SHA256).Hash -eq $fixtureHash) 'Copied EXE bytes changed.'
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $newRoot 'app.db')) -and -not (Test-Path -LiteralPath (Join-Path $newRoot 'data'))) 'Old source scene was adopted.'
    Assert-True ([IO.File]::ReadAllText((Join-Path $sourceDirectory 'app.db')) -eq 'old scene must remain') 'Source scene changed.'
    Assert-True ((Get-FileHash -LiteralPath $fixtureExe -Algorithm SHA256).Hash -eq $fixtureHash) 'Source EXE changed.'
    $expectedPaths = @{ USERPROFILE = 'user'; HOME = 'user'; APPDATA = 'user\Roaming'; LOCALAPPDATA = 'user\Local'; TEMP = 'temp'; TMP = 'temp' }
    foreach ($name in $expectedPaths.Keys) {
        Assert-True ($info.Environment[$name] -eq (Join-Path $newRoot $expectedPaths[$name])) "Child path binding failed: $name"
    }
    Assert-True ($info.Environment['ABFREE_OIDC_ISSUER'] -eq 'https://identity.example.test' -and $info.Environment['ABFREE_OIDC_CLIENT_ID'] -eq 'public-native-fixture') 'Public OIDC values were not passed.'
    $allowed = @('SystemRoot','WINDIR','SystemDrive','ComSpec','PATH','PATHEXT','USERPROFILE','HOME','APPDATA','LOCALAPPDATA','TEMP','TMP','ABFREE_OIDC_ISSUER','ABFREE_OIDC_CLIENT_ID')
    Assert-True (@($info.Environment.Keys | Where-Object { $_ -notin $allowed }).Count -eq 0) 'Extra inherited environment key.'
    Assert-True (-not $info.Environment.ContainsKey('ANT_TRIAL_UNRELATED')) 'Unrelated parent environment leaked.'
    Assert-True ([Environment]::GetEnvironmentVariable('ANT_TRIAL_UNRELATED','Process') -eq 'do-not-inherit') 'Launcher changed parent environment.'
    Assert-True ($result.PSObject.Properties.Name -notcontains 'clientId') 'Launcher output disclosed public configuration.'
    $script:PassedCases++

    New-Item -ItemType Directory -Path (Join-Path $newRoot 'data') | Out-Null
    $keptFiles = @('config.yaml', 'data\app.db', 'user\Local\kept-cache')
    $keptHashes = @{}
    foreach ($relative in $keptFiles) {
        $file = Join-Path $newRoot $relative
        [IO.File]::WriteAllText($file, 'existing trial fixture: ' + $relative)
        $keptHashes[$relative] = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash
    }
    $resumeArguments = @{ ExpectedExeSha256 = $fixtureHash; PublicConfigPath = $publicFile; TrialRoot = $newRoot }
    $lockedExe = [IO.File]::Open((Join-Path $newRoot 'ant-chrome.exe'), [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    try { $reopened = Open-AntLocalTrial @resumeArguments } finally { $lockedExe.Dispose() }
    Assert-True ($script:StartCalls -eq 2 -and $reopened.TrialRoot -eq $newRoot) 'Existing trial was not opened once in the same root.'
    $reopenedInfo = $script:CapturedStartInfo
    Assert-True ($reopenedInfo.FileName -eq $info.FileName -and $reopenedInfo.WorkingDirectory -eq $info.WorkingDirectory) 'Reopen drifted to a different EXE/data root.'
    foreach ($name in $info.Environment.Keys) {
        Assert-True ($reopenedInfo.Environment[$name] -eq $info.Environment[$name]) "Reopen child environment differs: $name"
    }
    Assert-True ($reopenedInfo.Environment.Count -eq $info.Environment.Count) 'Reopen inherited extra environment.'
    foreach ($relative in $keptFiles) {
        Assert-True ((Get-FileHash -LiteralPath (Join-Path $newRoot $relative) -Algorithm SHA256).Hash -eq $keptHashes[$relative]) 'Reopen changed existing configuration/database/cache.'
    }
    $script:PassedCases++
    $missingExisting = Join-Path $testRoot 'missing-existing'
    $resourcePublic=Join-Path $sourceDirectory 'resource-public.json'
    [IO.File]::WriteAllText($resourcePublic,'{"issuer":"https://identity.example.test","clientId":"public-native-fixture","apiProjectId":"existing-api-project"}')
    $resourceRoot=Join-Path $testRoot 'resource-trial'
    $beforeResource=$script:StartCalls
    New-AntLocalTrial -ExePath $fixtureExe -ExpectedExeSha256 $fixtureHash -PublicConfigPath $resourcePublic -TrialRoot $resourceRoot | Out-Null
    $copiedResource=Get-Content -LiteralPath (Join-Path $resourceRoot 'oidc-public.json') -Raw | ConvertFrom-Json
    Assert-True ($copiedResource.apiProjectId -ceq 'existing-api-project') 'Trial dropped the installed API resource configuration.'
    Open-AntLocalTrial -ExpectedExeSha256 $fixtureHash -PublicConfigPath $resourcePublic -TrialRoot $resourceRoot | Out-Null
    Assert-True ($script:StartCalls -eq $beforeResource+2) 'Resource trial did not use the original mocked launch seam.'
    Assert-Rejected {Open-AntLocalTrial -ExpectedExeSha256 $fixtureHash -PublicConfigPath $resourcePublic -TrialRoot $newRoot}
    $script:PassedCases++
    Assert-Rejected { Open-AntLocalTrial -ExpectedExeSha256 $fixtureHash -PublicConfigPath $publicFile -TrialRoot $missingExisting } $missingExisting
    Assert-Rejected { Open-AntLocalTrial -ExpectedExeSha256 ('0' * 64) -PublicConfigPath $publicFile -TrialRoot $newRoot }
    $invalidOpenFile = Join-Path $sourceDirectory 'invalid-open.json'
    [IO.File]::WriteAllText($invalidOpenFile, '{"issuer":"http://identity.example.test","clientId":"public"}')
    Assert-Rejected { Open-AntLocalTrial -ExpectedExeSha256 $fixtureHash -PublicConfigPath $invalidOpenFile -TrialRoot $newRoot }
    $incomplete = Join-Path $testRoot 'incomplete-existing'
    New-Item -ItemType Directory -Path $incomplete | Out-Null
    [IO.File]::Copy($fixtureExe, (Join-Path $incomplete 'ant-chrome.exe'), $false)
    Assert-Rejected { Open-AntLocalTrial -ExpectedExeSha256 $fixtureHash -PublicConfigPath $publicFile -TrialRoot $incomplete }
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $incomplete 'user'))) 'Reopen silently repaired missing layout.'

    $marker = Join-Path $newRoot 'keep.txt'
    [IO.File]::WriteAllText($marker, 'keep')
    Assert-Rejected { New-AntLocalTrial @arguments -TrialRoot $newRoot }
    Assert-True ([IO.File]::ReadAllText($marker) -eq 'keep') 'Existing trial data changed.'
    $badHashRoot = Join-Path $testRoot 'bad-hash'
    Assert-Rejected { New-AntLocalTrial -ExePath $fixtureExe -ExpectedExeSha256 ('0' * 64) -PublicConfigPath $publicFile -TrialRoot $badHashRoot } $badHashRoot
    Assert-Rejected { New-AntLocalTrial @arguments -TrialRoot 'relative-trial' }
    Assert-Rejected { New-AntLocalTrial @arguments -TrialRoot (Join-Path $sourceDirectory 'must-not-touch-source') }
    Assert-Rejected { New-AntLocalTrial @arguments -TrialRoot (Join-Path $testRoot 'missing-parent\trial') }

    $invalidPublic = @(
        '{"issuer":"http://identity.example.test","clientId":"public"}',
        '{"issuer":"https://user@identity.example.test","clientId":"public"}',
        '{"issuer":"https://identity.example.test?query=value","clientId":"public"}',
        '{"issuer":"https://identity.example.test","clientId":""}',
        '{"issuer":"https://identity.example.test","clientId":"public","unexpected":"value"}',
        '{"issuer":"https://identity.example.test","issuer":"https://other.example.test","clientId":"public"}',
        '{"issuer":123,"clientId":"public"}',
        'not-json'
    )
    for ($i = 0; $i -lt $invalidPublic.Count; $i++) {
        $invalidFile = Join-Path $sourceDirectory "invalid-$i.json"
        [IO.File]::WriteAllText($invalidFile, $invalidPublic[$i])
        $invalidRoot = Join-Path $testRoot "invalid-$i"
        Assert-Rejected { New-AntLocalTrial -ExePath $fixtureExe -ExpectedExeSha256 $fixtureHash -PublicConfigPath $invalidFile -TrialRoot $invalidRoot } $invalidRoot
    }
    $junction = Join-Path $testRoot 'junction'
    New-Item -ItemType Junction -Path $junction -Target $sourceDirectory | Out-Null
    $junctionRoot = Join-Path $testRoot 'junction-trial'
    Assert-Rejected { New-AntLocalTrial -ExePath (Join-Path $junction 'ant-chrome.exe') -ExpectedExeSha256 $fixtureHash -PublicConfigPath $publicFile -TrialRoot $junctionRoot } $junctionRoot
    Remove-Item -LiteralPath $junction -Force
    New-Item -ItemType Junction -Path $junction -Target $newRoot | Out-Null
    Assert-Rejected { Open-AntLocalTrial -ExpectedExeSha256 $fixtureHash -PublicConfigPath $publicFile -TrialRoot $junction }
    Remove-Item -LiteralPath $junction -Force
    # Make the source uncopyable only after its real hash was read.
    # This exercises an actual IO exception with a caller using Continue.
    $script:CopyHashCalls = 0
    $script:CopyLock = $null
    function Get-FileHash {
        [CmdletBinding()]
        param([string]$LiteralPath, [string]$Algorithm)
        $script:CopyHashCalls++
        $hash = Microsoft.PowerShell.Utility\Get-FileHash @PSBoundParameters
        if ($script:CopyHashCalls -eq 1) {
            $script:CopyLock = [IO.File]::Open($LiteralPath, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::None)
        }
        return $hash
    }
    $copyFailureRoot = Join-Path $testRoot 'copy-failure'
    $beforeCopyCalls = $script:StartCalls
    $copyRejected = $false
    $ErrorActionPreference = 'Continue'
    try {
        try { New-AntLocalTrial @arguments -TrialRoot $copyFailureRoot | Out-Null } catch { $copyRejected = $true }
        Assert-True $copyRejected 'Copy failure did not terminate.'
        Assert-True ($script:CopyHashCalls -eq 1 -and $script:StartCalls -eq $beforeCopyCalls) 'Copy exception continued to another operation.'
        Assert-True ($ErrorActionPreference -eq 'Continue') 'Launcher changed caller error policy.'
        Assert-True (Test-Path -LiteralPath $copyFailureRoot -PathType Container) 'Partial trial was silently deleted.'
        $script:PassedCases++
    } finally {
        $ErrorActionPreference = 'Stop'
        if ($null -ne $script:CopyLock) { $script:CopyLock.Dispose() }
        Remove-Item Function:Get-FileHash
    }
    function Invoke-AntTrialProcess {
        param([Diagnostics.ProcessStartInfo]$StartInfo)
        $script:StartCalls++
        throw 'Controlled mocked process-start failure.'
    }
    $startFailureRoot = Join-Path $testRoot 'start-failure'
    $beforeStartCalls = $script:StartCalls
    $startRejected = $false
    try { New-AntLocalTrial @arguments -TrialRoot $startFailureRoot | Out-Null } catch { $startRejected = $true }
    Assert-True ($startRejected -and $script:StartCalls -eq $beforeStartCalls + 1) 'Process-start failure was retried or hidden.'
    Assert-True (Test-Path -LiteralPath (Join-Path $startFailureRoot 'ant-chrome.exe')) 'Failed start deleted the partial trial.'
    $script:PassedCases++
    $beforeReopenFailure = $script:StartCalls
    $reopenRejected = $false
    try { Open-AntLocalTrial @resumeArguments | Out-Null } catch { $reopenRejected = $true }
    Assert-True ($reopenRejected -and $script:StartCalls -eq $beforeReopenFailure + 1) 'Existing-root process failure was hidden or retried.'
    foreach ($relative in $keptFiles) {
        Assert-True ((Get-FileHash -LiteralPath (Join-Path $newRoot $relative) -Algorithm SHA256).Hash -eq $keptHashes[$relative]) 'Failed reopen changed existing state.'
    }
    $script:PassedCases++
    Write-Output "Local trial launcher: syntax and $script:PassedCases behavior cases PASS; process boundary mocked; product executions 0."
} finally {
    [Environment]::SetEnvironmentVariable('ANT_TRIAL_UNRELATED', $oldUnrelated, 'Process')
    # No recursive deletion unless the exact task-owned temporary root is qualified.
    $resolved = [IO.Path]::GetFullPath($testRoot)
    if (-not $resolved.StartsWith($testParent.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase) -or
        [IO.Path]::GetFileName($resolved) -notmatch '^ant-local-trial-tests-[a-f0-9]{32}$') {
        throw 'Refusing cleanup outside the task-owned temporary root.'
    }
    if (Test-Path -LiteralPath $resolved) {
        if (((Get-Item -LiteralPath $resolved).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw 'Refusing recursive cleanup of a reparse root.'
        }
        $link = Join-Path $resolved 'junction'
        if (Test-Path -LiteralPath $link) { Remove-Item -LiteralPath $link -Force }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}

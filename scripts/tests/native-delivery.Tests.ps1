#requires -Version 7.0
param([switch]$CompileInstaller)
$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
foreach ($scriptFile in @('publish/identity-config.ps1', 'bat/publish.ps1')) {
    $tokens = $null
    $errors = $null
    [void][Management.Automation.Language.Parser]::ParseFile((Join-Path $repoRoot $scriptFile), [ref]$tokens, [ref]$errors)
    if ($errors.Count) { throw 'Publisher syntax errors.' }
}
. (Join-Path $repoRoot 'publish/identity-config.ps1')
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('ant-native-delivery-' + [Guid]::NewGuid().ToString('N'))
$source = Join-Path $testRoot 'public.json'
$staging = Join-Path $testRoot 'staging'
$cases = 0
try {
    New-Item -ItemType Directory -Path $staging | Out-Null
    [IO.File]::WriteAllText($source, '{"issuer":"https://identity.example.test","clientId":"existing-native"}', [Text.UTF8Encoding]::new($true))
    $before = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash
    Copy-PublicIdentityConfig -Path $source -StagingDir $staging
    $bytes = [IO.File]::ReadAllBytes((Join-Path $staging 'oidc-public.json'))
    $public = [Text.Encoding]::UTF8.GetString($bytes) | ConvertFrom-Json
    if ($public.issuer -cne 'https://identity.example.test' -or $public.clientId -cne 'existing-native' -or $bytes[0] -eq 239) { throw 'Native public metadata copy/BOM handling failed.' }
    if ((Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ne $before) { throw 'Publisher changed its input.' }
    $cases++
    foreach ($invalid in @(
        '{"issuer":"https://localhost:35443","clientId":"native"}',
        '{"issuer":"https://127.0.0.1","clientId":"native"}',
        '{"issuer":"http://identity.example.test","clientId":"native"}',
        '{"issuer":"https://identity.example.test","clientId":"native","secret":"fixture"}',
        '{"issuer":"https://identity.example.test","issuer":"https://other.example.test","clientId":"native"}',
        '{"Issuer":"https://identity.example.test","clientId":"native"}',
        '{"issuer":"https://identity.example.test","clientId":123}',
        '{"issuer":"https://identity.example.test","clientId":""}',
        (' ' * 4097)
    )) {
        [IO.File]::WriteAllText($source, $invalid)
        $rejected = $false
        try { Copy-PublicIdentityConfig -Path $source -StagingDir $staging } catch { $rejected = $true }
        if (-not $rejected) { throw 'Invalid identity metadata was accepted.' }
        if ([Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes((Join-Path $staging 'oidc-public.json'))) -cne [Text.Encoding]::UTF8.GetString($bytes)) { throw 'Rejected input replaced the accepted metadata.' }
        $cases++
    }
    $rejected = $false
    try { Read-PublicIdentityConfig -Path (Join-Path $testRoot 'missing.json') | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw 'Missing publisher configuration accepted.' }
    $cases++
    if ($CompileInstaller) {
        $compiler = Get-Command makensis.exe -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source
        if (-not $compiler) {
            foreach ($candidate in @('C:\Program Files (x86)\NSIS\makensis.exe', 'C:\Program Files\NSIS\makensis.exe')) {
                if (Test-Path -LiteralPath $candidate -PathType Leaf) { $compiler = $candidate; break }
            }
        }
        if (-not $compiler) { throw 'NSIS compiler required for the installer compile check.' }
        foreach ($directory in @('bin', 'data/diagnostics')) { New-Item -ItemType Directory -Path (Join-Path $staging $directory) -Force | Out-Null }
        foreach ($name in @('ant-chrome.exe', 'config.yaml', 'bin/xray.exe', 'bin/sing-box.exe', 'data/diagnostics/fixture.ps1')) {
            [IO.File]::WriteAllText((Join-Path $staging $name), 'non-executable packaging fixture')
        }
        $installer = Join-Path $testRoot 'installer-fixture.exe'
        & $compiler /NOCONFIG /V2 /DVERSION=0.0.0-ci "/DSTAGINGDIR=$staging" "/DOUTPUTFILE=$installer" (Join-Path $repoRoot 'publish/installer.nsi')
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $installer -PathType Leaf)) { throw 'Installer fixture compilation failed.' }
        $cases++
    }
    Write-Output "Native delivery: syntax and $cases publisher cases PASS; installer compile requested=$CompileInstaller; product/installer executions 0."
} finally {
    $resolved = [IO.Path]::GetFullPath($testRoot)
    $temporary = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (-not $resolved.StartsWith($temporary, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolved) -notmatch '^ant-native-delivery-[a-f0-9]{32}$') { throw 'Refusing unexpected fixture cleanup path.' }
    if (Test-Path -LiteralPath $resolved) { Remove-Item -LiteralPath $resolved -Recurse -Force }
}

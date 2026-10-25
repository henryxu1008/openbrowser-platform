#requires -Version 7.0
<#
.SYNOPSIS
Prepare and launch a fresh local trial. Existing roots are preserved and refused.
#>
[CmdletBinding()]
param(
    [string]$ExePath,
    [string]$ExpectedExeSha256,
    [string]$PublicConfigPath,
    [string]$TrialRoot
)

function Get-AntTrialLocalPath {
    param([Parameter(Mandatory)][string]$Path)
    if ($Path -notmatch '^[A-Za-z]:[\\/]' -or $Path.Substring(2).Contains(':')) {
        throw 'Use an absolute local drive path, without alternate streams.'
    }
    $full = [IO.Path]::GetFullPath($Path)
    if ($full.Length -gt 3) { $full = $full.TrimEnd('\') }
    $cursor = $full
    while ($cursor) {
        if (Test-Path -LiteralPath $cursor) {
            $item = Get-Item -LiteralPath $cursor -Force -ErrorAction Stop
            if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
                throw 'Trial inputs and destination must not traverse reparse points.'
            }
        }
        $cursor = [IO.Path]::GetDirectoryName($cursor)
    }
    return $full
}

function Invoke-AntTrialProcess {
    param([Diagnostics.ProcessStartInfo]$StartInfo)
    return [Diagnostics.Process]::Start($StartInfo)
}

function New-AntLocalTrial {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$ExePath,
        [Parameter(Mandatory)][ValidatePattern('^[A-Fa-f0-9]{64}$')][string]$ExpectedExeSha256,
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$PublicConfigPath,
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$TrialRoot
    )
    $ErrorActionPreference = 'Stop'
    if (-not $IsWindows) { throw 'This local trial entry requires Windows PowerShell 7.' }
    $source = Get-AntTrialLocalPath $ExePath
    $publicFile = Get-AntTrialLocalPath $PublicConfigPath
    $root = Get-AntTrialLocalPath $TrialRoot
    if (-not (Test-Path -LiteralPath $source -PathType Leaf) -or
        [IO.Path]::GetExtension($source) -ine '.exe' -or
        -not (Test-Path -LiteralPath $publicFile -PathType Leaf)) {
        throw 'Provide an existing EXE and a public configuration file.'
    }
    if (Test-Path -LiteralPath $root) { throw 'TrialRoot already exists; nothing will be adopted or overwritten.' }
    $parent = [IO.Path]::GetDirectoryName($root)
    if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
        throw 'The new trial directory must have an existing parent.'
    }
    foreach ($inputDirectory in @([IO.Path]::GetDirectoryName($source), [IO.Path]::GetDirectoryName($publicFile))) {
        if ($root.StartsWith($inputDirectory.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Choose a new trial root outside the source/configuration directories.'
        }
    }
    $expected = $ExpectedExeSha256.ToLowerInvariant()
    if ((Get-FileHash -LiteralPath $source -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant() -ne $expected) {
        throw 'EXE SHA256 mismatch; no trial directory or process was created.'
    }
    $document = $null
    try {
        $document = [System.Text.Json.JsonDocument]::Parse([IO.File]::ReadAllText($publicFile))
        if ($document.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { throw 'shape' }
        $properties = @($document.RootElement.EnumerateObject())
        if ($properties.Count -ne 2 -or
            @($properties | Where-Object Name -CEQ 'issuer').Count -ne 1 -or
            @($properties | Where-Object Name -CEQ 'clientId').Count -ne 1 -or
            @($properties | Where-Object { $_.Value.ValueKind -ne [System.Text.Json.JsonValueKind]::String }).Count -ne 0) {
            throw 'shape'
        }
        $issuer = $document.RootElement.GetProperty('issuer').GetString().Trim()
        $clientId = $document.RootElement.GetProperty('clientId').GetString().Trim()
    } catch {
        throw 'Public JSON must contain only one string issuer and one string clientId.'
    } finally {
        if ($null -ne $document) { $document.Dispose() }
    }
    $issuerUri = $null
    if (-not [Uri]::TryCreate($issuer, [UriKind]::Absolute, [ref]$issuerUri) -or
        $issuerUri.Scheme -cne 'https' -or $issuerUri.UserInfo -or $issuerUri.Query -or $issuerUri.Fragment -or
        [string]::IsNullOrWhiteSpace($clientId) -or $clientId -match '[\x00-\x1f]') {
        throw 'Use a normal HTTPS issuer without credentials/query/fragment and a nonempty public clientId.'
    }

    # Production main.go uses the EXE directory as appRoot, regardless of CWD.
    New-Item -ItemType Directory -Path $root -ErrorAction Stop | Out-Null
    [void](Get-AntTrialLocalPath $root)
    $copiedExe = Join-Path $root 'ant-chrome.exe'
    [IO.File]::Copy($source, $copiedExe, $false)
    if ((Get-FileHash -LiteralPath $copiedExe -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant() -ne $expected) {
        throw 'Copied EXE SHA256 mismatch; partial trial directory is retained, with no process started.'
    }
    foreach ($relative in @('user', 'user\Roaming', 'user\Local', 'temp')) {
        New-Item -ItemType Directory -Path (Join-Path $root $relative) -ErrorAction Stop | Out-Null
    }
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $copiedExe
    $startInfo.WorkingDirectory = $root
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.Environment.Clear()
    foreach ($systemName in @('SystemRoot', 'WINDIR', 'SystemDrive', 'ComSpec', 'PATH', 'PATHEXT')) {
        $systemValue = [Environment]::GetEnvironmentVariable($systemName, 'Process')
        if (-not [string]::IsNullOrEmpty($systemValue)) { $startInfo.Environment[$systemName] = $systemValue }
    }
    $paths = @{ USERPROFILE = 'user'; HOME = 'user'; APPDATA = 'user\Roaming'; LOCALAPPDATA = 'user\Local'; TEMP = 'temp'; TMP = 'temp' }
    foreach ($name in $paths.Keys) { $startInfo.Environment[$name] = Join-Path $root $paths[$name] }
    $startInfo.Environment['ABFREE_OIDC_ISSUER'] = $issuer
    $startInfo.Environment['ABFREE_OIDC_CLIENT_ID'] = $clientId
    $process = Invoke-AntTrialProcess $startInfo
    if ($null -eq $process) { throw 'Process start returned no handle; no retry was attempted.' }
    return [pscustomobject]@{ ProcessId = $process.Id; TrialRoot = $root; ExePath = $copiedExe; ExeSha256 = $expected }
}

if ($MyInvocation.InvocationName -ne '.') {
    foreach ($required in @('ExePath', 'ExpectedExeSha256', 'PublicConfigPath', 'TrialRoot')) {
        if (-not $PSBoundParameters.ContainsKey($required)) { throw "Required parameter: $required" }
    }
    New-AntLocalTrial @PSBoundParameters
}

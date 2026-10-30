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
    [string]$ApiConfigPath,
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

function Read-AntTrialPublicConfig {
    param([string]$Path)
    $file = Get-AntTrialLocalPath $Path
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw 'Public configuration file is missing.' }
    $document = $null
    try {
        $document = [System.Text.Json.JsonDocument]::Parse([IO.File]::ReadAllText($file))
        if ($document.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { throw 'shape' }
        $properties = @($document.RootElement.EnumerateObject())
        $hasProject = @($properties | Where-Object Name -CEQ 'apiProjectId').Count -eq 1
        if (($properties.Count -ne 2 -and -not ($properties.Count -eq 3 -and $hasProject)) -or
            @($properties | Where-Object Name -CEQ 'issuer').Count -ne 1 -or
            @($properties | Where-Object Name -CEQ 'clientId').Count -ne 1 -or
            @($properties | Where-Object { $_.Value.ValueKind -ne [System.Text.Json.JsonValueKind]::String }).Count -ne 0) {
            throw 'shape'
        }
        $issuer = $document.RootElement.GetProperty('issuer').GetString().Trim()
        $clientId = $document.RootElement.GetProperty('clientId').GetString().Trim()
        $apiProjectId = $null
        if ($hasProject) {
            $apiProjectId = $document.RootElement.GetProperty('apiProjectId').GetString()
            if ($apiProjectId -cnotmatch '\A[A-Za-z0-9_-]{1,128}\z') { throw 'shape' }
        }
    } catch {
        throw 'Public JSON requires issuer/clientId and permits only a bounded public apiProjectId.'
    } finally {
        if ($null -ne $document) { $document.Dispose() }
    }
    $issuerUri = $null
    if (-not [Uri]::TryCreate($issuer, [UriKind]::Absolute, [ref]$issuerUri) -or
        $issuerUri.Scheme -cne 'https' -or $issuerUri.UserInfo -or $issuerUri.Query -or $issuerUri.Fragment -or
        [string]::IsNullOrWhiteSpace($clientId) -or $clientId -match '[\x00-\x1f]') {
        throw 'Use a normal HTTPS issuer without credentials/query/fragment and a nonempty public clientId.'
    }
    return [pscustomobject]@{ Issuer = $issuer; ClientId = $clientId; APIProjectId = $apiProjectId }
}

# Same public contract as internal/apiconfig: one bounded JSON apiOrigin,
# HTTPS origin only; never credentials, a path, query, fragment or a default service.
function Read-AntTrialApiConfig {
    param([string]$Path)
    $file = Get-AntTrialLocalPath $Path
    if (-not (Test-Path -LiteralPath $file -PathType Leaf) -or (Get-Item -LiteralPath $file).Length -gt 4096) {
        throw 'Public API configuration is missing or exceeds 4096 bytes.'
    }
    $document = $null
    try {
        $document = [System.Text.Json.JsonDocument]::Parse([IO.File]::ReadAllText($file))
        if ($document.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { throw 'shape' }
        $properties = @($document.RootElement.EnumerateObject())
        if ($properties.Count -ne 1 -or $properties[0].Name -cne 'apiOrigin' -or
            $properties[0].Value.ValueKind -ne [System.Text.Json.JsonValueKind]::String) { throw 'shape' }
        $origin = $properties[0].Value.GetString()
        $uri = $null
        if ($origin -cnotmatch '^https://[^@/?#\\\s]+$' -or
            -not [Uri]::TryCreate($origin, [UriKind]::Absolute, [ref]$uri) -or
            -not $uri.Host -or $uri.UserInfo -or $uri.Query -or $uri.Fragment) { throw 'origin' }
        return [pscustomobject]@{ APIOrigin = $origin }
    } catch {
        throw 'Public API JSON requires only apiOrigin: an HTTPS origin without credentials, path, query or fragment.'
    } finally {
        if ($null -ne $document) { $document.Dispose() }
    }
}

function Start-AntTrialAtRoot {
    param([string]$Root, [string]$ExpectedHash, [object]$PublicConfig, [object]$ApiConfig)
    $installedIdentity = Join-Path $Root 'oidc-public.json'
    $installed = if (Test-Path -LiteralPath $installedIdentity -PathType Leaf) { Read-AntTrialPublicConfig -Path $installedIdentity } else { $null }
    if ($PublicConfig.APIProjectId -or $installed.APIProjectId) {
        if ($installed.Issuer -cne $PublicConfig.Issuer -or $installed.ClientId -cne $PublicConfig.ClientId -or $installed.APIProjectId -cne $PublicConfig.APIProjectId) {
            throw 'Existing trial identity/resource differs; do not adopt or overwrite it.'
        }
    }
    $installedApiPath = Join-Path $Root 'shuomi-public.json'
    if ($PublicConfig.APIProjectId -or $ApiConfig -or (Test-Path -LiteralPath $installedApiPath)) {
        if (-not $ApiConfig) { throw 'Explicit API configuration is required for this Store trial; nothing was changed.' }
        $installedApi = Read-AntTrialApiConfig $installedApiPath
        if ($installedApi.APIOrigin -cne $ApiConfig.APIOrigin) { throw 'Existing trial API origin differs; do not adopt or overwrite it.' }
    }
    $copiedExe = Join-Path $Root 'ant-chrome.exe'
    [void](Get-AntTrialLocalPath $copiedExe)
    if (-not (Test-Path -LiteralPath $copiedExe -PathType Leaf) -or
        (Get-FileHash -LiteralPath $copiedExe -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant() -ne $ExpectedHash) {
        throw 'Trial EXE SHA256 mismatch or file missing; no process was started.'
    }
    foreach ($relative in @('user', 'user\Roaming', 'user\Local', 'temp')) {
        $directory = Join-Path $Root $relative
        [void](Get-AntTrialLocalPath $directory)
        if (-not (Test-Path -LiteralPath $directory -PathType Container)) {
            throw 'Required trial user/temp directory is missing; existing data will not be repaired or reset.'
        }
    }
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $copiedExe
    $startInfo.WorkingDirectory = $Root
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.Environment.Clear()
    foreach ($systemName in @('SystemRoot', 'WINDIR', 'SystemDrive', 'ComSpec', 'PATH', 'PATHEXT')) {
        $systemValue = [Environment]::GetEnvironmentVariable($systemName, 'Process')
        if (-not [string]::IsNullOrEmpty($systemValue)) { $startInfo.Environment[$systemName] = $systemValue }
    }
    $paths = @{ USERPROFILE = 'user'; HOME = 'user'; APPDATA = 'user\Roaming'; LOCALAPPDATA = 'user\Local'; TEMP = 'temp'; TMP = 'temp' }
    foreach ($name in $paths.Keys) { $startInfo.Environment[$name] = Join-Path $Root $paths[$name] }
    $startInfo.Environment['ABFREE_OIDC_ISSUER'] = $PublicConfig.Issuer
    $startInfo.Environment['ABFREE_OIDC_CLIENT_ID'] = $PublicConfig.ClientId
    $process = Invoke-AntTrialProcess $startInfo
    if ($null -eq $process) { throw 'Process start returned no handle; no retry was attempted.' }
    return [pscustomobject]@{ ProcessId = $process.Id; TrialRoot = $Root; ExePath = $copiedExe; ExeSha256 = $ExpectedHash }
}

function Open-AntLocalTrial {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][ValidatePattern('^[A-Fa-f0-9]{64}$')][string]$ExpectedExeSha256,
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$PublicConfigPath,
        [string]$ApiConfigPath,
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$TrialRoot
    )
    $ErrorActionPreference = 'Stop'
    if (-not $IsWindows) { throw 'This local trial entry requires Windows PowerShell 7.' }
    $root = Get-AntTrialLocalPath $TrialRoot
    if (-not (Test-Path -LiteralPath $root -PathType Container)) { throw 'Select an existing trial root explicitly.' }
    $public = Read-AntTrialPublicConfig $PublicConfigPath
    if ($public.APIProjectId -and -not $ApiConfigPath) { throw 'Store trial requires explicit ApiConfigPath.' }
    $api = if ($ApiConfigPath) { Read-AntTrialApiConfig $ApiConfigPath } else { $null }
    Start-AntTrialAtRoot -Root $root -ExpectedHash $ExpectedExeSha256.ToLowerInvariant() -PublicConfig $public -ApiConfig $api
}


function New-AntLocalTrial {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$ExePath,
        [Parameter(Mandatory)][ValidatePattern('^[A-Fa-f0-9]{64}$')][string]$ExpectedExeSha256,
        [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$PublicConfigPath,
        [string]$ApiConfigPath,
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
    $public = Read-AntTrialPublicConfig $publicFile
    if ($public.APIProjectId -and -not $ApiConfigPath) { throw 'Store trial requires explicit ApiConfigPath.' }
    $api = if ($ApiConfigPath) { Read-AntTrialApiConfig $ApiConfigPath } else { $null }
    if ($ApiConfigPath) {
        $apiDirectory = [IO.Path]::GetDirectoryName((Get-AntTrialLocalPath $ApiConfigPath))
        if ($root.StartsWith($apiDirectory.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Choose a new trial root outside the API configuration directory.'
        }
    }
    # Production main.go uses the EXE directory as appRoot, regardless of CWD.
    New-Item -ItemType Directory -Path $root -ErrorAction Stop | Out-Null
    [void](Get-AntTrialLocalPath $root)
    [IO.File]::Copy($source, (Join-Path $root 'ant-chrome.exe'), $false)
    $metadata = [ordered]@{ issuer = $public.Issuer; clientId = $public.ClientId }
    if ($public.APIProjectId) { $metadata['apiProjectId'] = $public.APIProjectId }
    [IO.File]::WriteAllText((Join-Path $root 'oidc-public.json'), ($metadata | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
    if ($api) {
        [IO.File]::WriteAllText((Join-Path $root 'shuomi-public.json'), (@{ apiOrigin = $api.APIOrigin } | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
    }
    foreach ($relative in @('user', 'user\Roaming', 'user\Local', 'temp')) {
        New-Item -ItemType Directory -Path (Join-Path $root $relative) -ErrorAction Stop | Out-Null
    }
    Start-AntTrialAtRoot -Root $root -ExpectedHash $expected -PublicConfig $public -ApiConfig $api
}

if ($MyInvocation.InvocationName -ne '.') {
    foreach ($required in @('ExePath', 'ExpectedExeSha256', 'PublicConfigPath', 'TrialRoot')) {
        if (-not $PSBoundParameters.ContainsKey($required)) { throw "Required parameter: $required" }
    }
    New-AntLocalTrial @PSBoundParameters
}

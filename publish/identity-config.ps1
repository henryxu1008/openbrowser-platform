#requires -Version 7.0
# Publisher-only helpers. End users launch the installed application shortcut.
function Read-PublicIdentityConfig {
    param([Parameter(Mandatory)][string]$Path)
    $ErrorActionPreference = 'Stop'
    $file = Get-Item -LiteralPath $Path -ErrorAction Stop
    if ($file.PSIsContainer -or $file.Length -gt 4096) { throw 'Provide a public identity JSON file of at most 4096 bytes.' }
    $document = $null
    try {
        $document = [System.Text.Json.JsonDocument]::Parse([IO.File]::ReadAllText($file.FullName))
        if ($document.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { throw 'shape' }
        $properties = @($document.RootElement.EnumerateObject())
        if ($properties.Count -ne 2 -or
            @($properties | Where-Object Name -CEQ 'issuer').Count -ne 1 -or
            @($properties | Where-Object Name -CEQ 'clientId').Count -ne 1 -or
            @($properties | Where-Object { $_.Value.ValueKind -ne [System.Text.Json.JsonValueKind]::String }).Count) { throw 'shape' }
        $issuer = $document.RootElement.GetProperty('issuer').GetString()
        $clientId = $document.RootElement.GetProperty('clientId').GetString()
    } catch {
        throw 'Identity JSON must contain only one string issuer and one string clientId, without credentials.'
    } finally {
        if ($null -ne $document) { $document.Dispose() }
    }
    $uri = $null
    if (-not [Uri]::TryCreate($issuer, [UriKind]::Absolute, [ref]$uri) -or
        $uri.Scheme -cne 'https' -or $uri.UserInfo -or $uri.Query -or $uri.Fragment -or
        $issuer -match '[\x00-\x20\x7f-\x9f]' -or $uri.IsLoopback -or
        $uri.Host -in @('0.0.0.0', '[::]') -or $uri.Host.EndsWith('.localhost') -or
        [string]::IsNullOrWhiteSpace($clientId) -or $clientId.Trim() -cne $clientId -or
        [Text.Encoding]::UTF8.GetByteCount($clientId) -gt 256 -or $clientId -match '[\x00-\x1f\x7f-\x9f]') {
        throw 'Use the verified distribution instance HTTPS issuer and public Native clientId; local trial endpoints are not distributable.'
    }
    return [pscustomobject]@{ issuer = $issuer; clientId = $clientId }
}

function Copy-PublicIdentityConfig {
    param([Parameter(Mandatory)][string]$Path, [Parameter(Mandatory)][string]$StagingDir)
    $ErrorActionPreference = 'Stop'
    $public = Read-PublicIdentityConfig -Path $Path
    # Canonical UTF-8 without BOM is accepted by the native strict JSON reader.
    $json = [ordered]@{ issuer = $public.issuer; clientId = $public.clientId } | ConvertTo-Json
    if ([Text.Encoding]::UTF8.GetByteCount($json) -gt 4096) { throw 'Canonical public configuration exceeds the native size limit.' }
    [IO.File]::WriteAllText((Join-Path $StagingDir 'oidc-public.json'), $json, [Text.UTF8Encoding]::new($false))
}

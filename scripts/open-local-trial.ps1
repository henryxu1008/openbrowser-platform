#requires -Version 7.0
<#
.SYNOPSIS
Open the user's explicitly selected existing trial without copying or resetting data.
#>
[CmdletBinding()]
param(
    [string]$ExpectedExeSha256,
    [string]$PublicConfigPath,
    [string]$ApiConfigPath,
    [string]$TrialRoot
)
$openArguments = @{} + $PSBoundParameters
$invokeOpen = $MyInvocation.InvocationName -ne '.'
. (Join-Path $PSScriptRoot 'new-local-trial.ps1')
if ($invokeOpen) {
    foreach ($required in @('ExpectedExeSha256', 'PublicConfigPath', 'TrialRoot')) {
        if (-not $openArguments.ContainsKey($required)) { throw "Required parameter: $required" }
    }
    Open-AntLocalTrial @openArguments
}

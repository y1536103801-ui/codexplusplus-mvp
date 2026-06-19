param(
    [string]$Root,
    [string]$ProfileRoot,
    [string]$CodexHome,
    [string]$ConfigPath,
    [string]$SettingsPath,
    [string]$OutputPath,
    [string]$Label = "snapshot",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

function Resolve-OptionalPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return [System.IO.Path]::GetFullPath($Path)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $Root $Path))
}

if ([string]::IsNullOrWhiteSpace($ProfileRoot)) {
    $ProfileRoot = $env:USERPROFILE
}
$ProfileRoot = Resolve-OptionalPath $ProfileRoot

if ([string]::IsNullOrWhiteSpace($CodexHome)) {
    if (-not [string]::IsNullOrWhiteSpace($env:CODEX_HOME)) {
        $CodexHome = $env:CODEX_HOME
    } elseif (-not [string]::IsNullOrWhiteSpace($ProfileRoot)) {
        $CodexHome = Join-Path $ProfileRoot ".codex"
    }
}
$CodexHome = Resolve-OptionalPath $CodexHome

if ([string]::IsNullOrWhiteSpace($ConfigPath) -and -not [string]::IsNullOrWhiteSpace($CodexHome)) {
    $ConfigPath = Join-Path $CodexHome "config.toml"
}
$ConfigPath = Resolve-OptionalPath $ConfigPath

if ([string]::IsNullOrWhiteSpace($SettingsPath) -and -not [string]::IsNullOrWhiteSpace($ProfileRoot)) {
    $SettingsPath = Join-Path (Join-Path $ProfileRoot ".codex-session-delete") "settings.json"
}
$SettingsPath = Resolve-OptionalPath $SettingsPath

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    throw "OutputPath is required."
}
if (-not [System.IO.Path]::IsPathRooted($OutputPath)) {
    $OutputPath = Join-Path $Root $OutputPath
}
$OutputPath = [System.IO.Path]::GetFullPath($OutputPath)

if ((Test-Path -LiteralPath $OutputPath) -and -not $Force) {
    throw "Snapshot already exists: $OutputPath. Use -Force to replace it."
}

function Get-SafeHash {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($Value)
        $hashBytes = $sha.ComputeHash($bytes)
        return ([System.BitConverter]::ToString($hashBytes) -replace "-", "").ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function ConvertTo-SafeEvidenceText {
    param([string]$Text)
    if ($null -eq $Text) {
        return ""
    }
    $safe = $Text
    $safe = $safe -replace "(?i)\bsk-proj-[A-Za-z0-9_-]{8,}\b", "[redacted-openai-project-key]"
    $safe = $safe -replace "(?i)\bsk-ant-[A-Za-z0-9_-]{8,}\b", "[redacted-anthropic-key]"
    $safe = $safe -replace "(?i)\bsk-[A-Za-z0-9_-]{8,}\b", "[redacted-api-key]"
    $safe = $safe -replace "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b", "[redacted-jwt]"
    $safe = $safe -replace "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}", "Authorization: [redacted]"
    return $safe
}

function Get-RelativeSafePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $base = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath($Path)
    if ($full.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
        return ConvertTo-SafeEvidenceText ($full.Substring($base.Length).TrimStart('\', '/'))
    }
    return ConvertTo-SafeEvidenceText $full
}

function Get-AuthSecretFingerprintInput {
    param([string]$AuthContents)
    if ([string]::IsNullOrWhiteSpace($AuthContents)) {
        return ""
    }

    $trimmed = $AuthContents.Trim()
    if ($trimmed.StartsWith("{")) {
        try {
            $authObject = $trimmed | ConvertFrom-Json -ErrorAction Stop
            foreach ($key in @("OPENAI_API_KEY", "openai_api_key", "apiKey", "api_key", "upstreamKey", "upstream_key", "key")) {
                if ($null -ne $authObject.PSObject.Properties[$key] -and -not [string]::IsNullOrWhiteSpace([string]$authObject.$key)) {
                    return [string]$authObject.$key
                }
            }
        } catch {
            # Fall through to hashing opaque auth contents without printing them.
        }
    }

    return $AuthContents
}

function New-SanitizedProvider {
    param(
        [string]$Id,
        [string]$Name,
        [hashtable]$Fields,
        [string]$Source
    )

    if ([string]::IsNullOrWhiteSpace($Id)) {
        $Id = $Name
    }
    if ([string]::IsNullOrWhiteSpace($Name)) {
        $Name = $Id
    }
    if ([string]::IsNullOrWhiteSpace($Id)) {
        return $null
    }

    $baseUrl = ""
    foreach ($key in @("base_url", "baseUrl", "upstream_base_url", "upstreamBaseUrl", "url", "endpoint", "endpointUrl", "api_base", "apiBase", "relayBaseUrl")) {
        if ($Fields.ContainsKey($key) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$key])) {
            $baseUrl = [string]$Fields[$key]
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($baseUrl)) {
        foreach ($key in @("configContents", "config_contents", "relayCommonConfigContents")) {
            if ($Fields.ContainsKey($key) -and ([string]$Fields[$key]) -match "(?im)^\s*(base_url|upstream_base_url|api_base)\s*=\s*[""']([^""']+)[""']") {
                $baseUrl = $Matches[2]
                break
            }
        }
    }

    $apiKey = ""
    foreach ($key in @("experimental_bearer_token", "api_key", "apiKey", "upstream_key", "upstreamKey", "openai_api_key", "openaiApiKey", "key", "relayApiKey")) {
        if ($Fields.ContainsKey($key) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$key])) {
            $apiKey = [string]$Fields[$key]
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($apiKey)) {
        foreach ($key in @("authContents", "auth_contents", "auth")) {
            if ($Fields.ContainsKey($key) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$key])) {
                $apiKey = Get-AuthSecretFingerprintInput ([string]$Fields[$key])
                break
            }
        }
    }
    if ([string]::IsNullOrWhiteSpace($apiKey)) {
        foreach ($key in @("configContents", "config_contents", "relayCommonConfigContents")) {
            if ($Fields.ContainsKey($key) -and ([string]$Fields[$key]) -match "(?im)^\s*(experimental_bearer_token|api_key|openai_api_key|upstream_key)\s*=\s*[""']([^""']*)[""']") {
                $apiKey = $Matches[2]
                break
            }
        }
    }

    $isManaged = $false
    if ($Name -match "(?i)Codex\+\+\s*Cloud|CodexPlusPlus|managed") {
        $isManaged = $true
    }
    foreach ($flag in @("managed", "is_managed", "codexplus_managed", "codexplus_cloud")) {
        if ($Fields.ContainsKey($flag) -and ([string]$Fields[$flag]) -match "(?i)true|1|yes") {
            $isManaged = $true
        }
    }

    $fieldNames = @($Fields.Keys | ForEach-Object { [string]$_ } | Sort-Object)
    return [ordered]@{
        id = ConvertTo-SafeEvidenceText $Id
        name = ConvertTo-SafeEvidenceText $Name
        managed = $isManaged
        source = $Source
        base_url_hash = Get-SafeHash $baseUrl
        api_key_hash = Get-SafeHash $apiKey
        has_base_url = -not [string]::IsNullOrWhiteSpace($baseUrl)
        has_api_key = -not [string]::IsNullOrWhiteSpace($apiKey)
        field_names = $fieldNames
    }
}

function Read-TomlProviders {
    param([string]$Path)

    $providers = [ordered]@{}
    $defaultProvider = ""
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return [pscustomobject]@{ DefaultProvider = ""; Providers = $providers }
    }

    $currentId = ""
    $currentFields = @{}
    function Flush-TomlProvider {
        if ([string]::IsNullOrWhiteSpace($script:currentId)) {
            return
        }
        $provider = New-SanitizedProvider -Id $script:currentId -Name $script:currentId -Fields $script:currentFields -Source "config.toml/model_providers"
        if ($null -ne $provider) {
            $providers[$script:currentId] = $provider
        }
        $script:currentId = ""
        $script:currentFields = @{}
    }

    $script:currentId = ""
    $script:currentFields = @{}
    $insideProvider = $false
    foreach ($line in Get-Content -LiteralPath $Path -Encoding UTF8) {
        $trimmed = $line.Trim()
        if ($trimmed -match '^\s*model_provider\s*=\s*["'']([^"'']+)["'']') {
            $defaultProvider = $Matches[1]
        }
        if ($trimmed -match '^\s*\[model_providers\.("?)([^"\]]+)\1\]\s*$') {
            Flush-TomlProvider
            $script:currentId = $Matches[2]
            $script:currentFields = @{}
            $insideProvider = $true
            continue
        }
        if ($trimmed -match '^\s*\[') {
            Flush-TomlProvider
            $insideProvider = $false
            continue
        }
        if ($insideProvider -and $trimmed -match '^\s*([A-Za-z0-9_-]+)\s*=\s*(.+?)\s*$') {
            $key = $Matches[1]
            $value = $Matches[2].Trim().Trim('"').Trim("'")
            $script:currentFields[$key] = $value
        }
    }
    Flush-TomlProvider

    return [pscustomobject]@{ DefaultProvider = $defaultProvider; Providers = $providers }
}

function Convert-ObjectToMap {
    param([object]$Value)
    $map = @{}
    if ($null -eq $Value) {
        return $map
    }
    foreach ($property in $Value.PSObject.Properties) {
        $map[$property.Name] = $property.Value
    }
    return $map
}

function Get-ProviderNameFromMap {
    param([hashtable]$Fields)
    foreach ($key in @("name", "id", "provider", "provider_name", "providerName", "relay_id", "relayId", "profile_id", "profileId")) {
        if ($Fields.ContainsKey($key) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$key])) {
            return [string]$Fields[$key]
        }
    }
    return ""
}

function Read-JsonProviderNode {
    param(
        [object]$Node,
        [string]$Source
    )
    $items = New-Object System.Collections.Generic.List[object]
    if ($null -eq $Node) {
        return @()
    }
    if ($Node -is [System.Array]) {
        foreach ($entry in $Node) {
            $fields = Convert-ObjectToMap $entry
            $name = Get-ProviderNameFromMap $fields
            $provider = New-SanitizedProvider -Id $name -Name $name -Fields $fields -Source $Source
            if ($null -ne $provider) {
                $items.Add($provider)
            }
        }
        return @($items.ToArray())
    }

    $nodeMap = Convert-ObjectToMap $Node
    foreach ($property in $nodeMap.GetEnumerator()) {
        $fields = Convert-ObjectToMap $property.Value
        if (-not $fields.ContainsKey("id")) {
            $fields["id"] = $property.Key
        }
        if (-not $fields.ContainsKey("name")) {
            $fields["name"] = $property.Key
        }
        $provider = New-SanitizedProvider -Id $property.Key -Name (Get-ProviderNameFromMap $fields) -Fields $fields -Source $Source
        if ($null -ne $provider) {
            $items.Add($provider)
        }
    }
    return @($items.ToArray())
}

function Read-SettingsProviders {
    param([string]$Path)
    $providers = New-Object System.Collections.Generic.List[object]
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return @()
    }

    $json = Get-Content -Raw -LiteralPath $Path -Encoding UTF8 | ConvertFrom-Json
    foreach ($root in @($json, $json.settings, $json.provider_settings, $json.providerSettings, $json.legacy_settings, $json.legacySettings)) {
        if ($null -eq $root) {
            continue
        }
        $map = Convert-ObjectToMap $root
        foreach ($key in @("relayProfiles", "relay_profiles", "providers", "model_providers", "modelProviders")) {
            if ($map.ContainsKey($key)) {
                foreach ($provider in @(Read-JsonProviderNode $map[$key] "settings.json/$key")) {
                    $providers.Add($provider)
                }
            }
        }
    }
    return @($providers.ToArray())
}

$toml = Read-TomlProviders $ConfigPath
$settingsProviders = @(Read-SettingsProviders $SettingsPath)

$snapshot = [ordered]@{
    snapshot_schema = "codexplus-07-sanitized-provider-snapshot-v1"
    label = ConvertTo-SafeEvidenceText $Label
    captured_at_utc = [DateTime]::UtcNow.ToString("o")
    source_paths = [ordered]@{
        profile_root = Get-RelativeSafePath $ProfileRoot
        codex_home = Get-RelativeSafePath $CodexHome
        config = Get-RelativeSafePath $ConfigPath
        settings = Get-RelativeSafePath $SettingsPath
    }
    model_provider = ConvertTo-SafeEvidenceText $toml.DefaultProvider
    settings = [ordered]@{
        relayProfiles = @($settingsProviders)
    }
    model_providers = $toml.Providers
}

$parent = Split-Path -Parent $OutputPath
if (-not [string]::IsNullOrWhiteSpace($parent)) {
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
}
$jsonText = $snapshot | ConvertTo-Json -Depth 20
Set-Content -LiteralPath $OutputPath -Encoding UTF8 -Value $jsonText

Write-Host "Wrote sanitized provider snapshot: $OutputPath"
Write-Host "Snapshot stores provider names and URL/key hashes only; raw secrets are not written."

param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$Timestamp,
    [string]$EvidenceDir,
    [string]$PreUpgradeSnapshot,
    [string]$PostUpgradeSnapshot,
    [string]$LogoutSnapshot,
    [string]$RollbackSnapshot,
    [switch]$FixtureMode,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$PlanRoot = Join-Path $Root "codex-plus-dev-plan"
$CompatibilityGenerator = Join-Path $PlanRoot "tools\new-07-compatibility-evidence.ps1"

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $PlanRoot "test-runs"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

if ([string]::IsNullOrWhiteSpace($Timestamp)) {
    $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
}

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    if ($Timestamp -match "^\d{8}-\d{4}-compatibility$") {
        $runName = $Timestamp
    } elseif ($Timestamp -match "^\d{8}-\d{4}$") {
        $runName = "$Timestamp-compatibility"
    } else {
        throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-compatibility."
    }
    $EvidencePath = Join-Path $OutputRoot $runName
} elseif ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidencePath = $EvidenceDir
} else {
    $EvidencePath = Join-Path $Root $EvidenceDir
}

$EvidencePath = [System.IO.Path]::GetFullPath($EvidencePath)
$runName = Split-Path -Leaf $EvidencePath
if ($runName -notmatch "^\d{8}-\d{4}-compatibility$") {
    throw "Compatibility evidence directory must be named YYYYMMDD-HHMM-compatibility; got $runName."
}

function Invoke-CompatibilityScaffoldGenerator {
    $parent = Split-Path -Parent $EvidencePath
    $stamp = $runName -replace "-compatibility$", ""
    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $CompatibilityGenerator,
        "-Root", $Root,
        "-OutputRoot", $parent,
        "-Timestamp", $stamp
    )
    if ($Force) {
        $args += "-Force"
    }

    & powershell @args
    if ($LASTEXITCODE -ne 0) {
        throw "new-07-compatibility-evidence.ps1 failed with exit code $LASTEXITCODE."
    }
}

$requiredEvidenceFiles = @(
    "00-test-context.md",
    "01-pre-upgrade-snapshot.md",
    "02-post-upgrade-cloud.md",
    "03-cloud-logout-boundary.md",
    "04-manual-provider-switch.md",
    "05-provider-sync.md",
    "06-rollback-rehearsal.md",
    "07-compatibility-gate-report.md"
)

$needsScaffold = -not (Test-Path -LiteralPath $EvidencePath -PathType Container)
if (-not $needsScaffold) {
    foreach ($file in $requiredEvidenceFiles) {
        if (-not (Test-Path -LiteralPath (Join-Path $EvidencePath $file) -PathType Leaf)) {
            $needsScaffold = $true
            break
        }
    }
}

if ($needsScaffold) {
    Invoke-CompatibilityScaffoldGenerator
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
    $safe = $safe -replace "(?i)[""']?(api_key|apiKey|upstream_key|upstreamKey|access_token|accessToken|refresh_token|refreshToken|id_token|idToken|poll_token|pollToken|session_token|sessionToken|desktopAccessToken|desktopRefreshToken|bearerToken|authorization)[""']?\s*[:=]\s*[""']?[^,`r`n""'}\s]+", '$1=[redacted]'
    $safe = $safe -replace "(?im)^([A-Z0-9_]*(KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*)\s*=\s*(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b).{8,}$", '$1=[redacted]'
    return $safe
}

function Resolve-InputPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return [System.IO.Path]::GetFullPath($Path)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $Root $Path))
}

function Get-RelativePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $base = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath($Path)
    if ($full.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $full.Substring($base.Length).TrimStart('\', '/')
    }
    return $full
}

function Write-FixtureSnapshots {
    param([string]$Directory)
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null

    $pre = @"
{
  "desktop_version": "0.9.0-fixture",
  "settings": {
    "activeRelayId": "legacy-openai",
    "relayProfiles": [
      {
        "id": "legacy-openai",
        "name": "legacy-openai",
        "baseUrl": "https://legacy.example/v1",
        "apiKey": "[redacted]"
      },
      {
        "id": "local-dev",
        "name": "local-dev",
        "baseUrl": "http://127.0.0.1:11434/v1",
        "apiKey": "[redacted]"
      }
    ]
  }
}
"@
    $post = @"
{
  "desktop_version": "1.0.0-fixture",
  "model_provider": "Codex++ Cloud",
  "model_providers": {
    "legacy-openai": {
      "name": "legacy-openai",
      "base_url": "https://legacy.example/v1",
      "api_key": "[redacted]"
    },
    "local-dev": {
      "name": "local-dev",
      "base_url": "http://127.0.0.1:11434/v1",
      "api_key": "[redacted]"
    },
    "Codex++ Cloud": {
      "name": "Codex++ Cloud",
      "base_url": "http://127.0.0.1:19091/v1",
      "upstream_base_url": "https://gateway.example/v1",
      "api_key": "[redacted]",
      "managed": true,
      "default_model": "allowed-model-fixture"
    }
  }
}
"@
    $logout = @"
{
  "desktop_version": "1.0.0-fixture",
  "model_provider": "legacy-openai",
  "cloud_session_state": "logged_out",
  "model_providers": {
    "legacy-openai": {
      "name": "legacy-openai",
      "base_url": "https://legacy.example/v1",
      "api_key": "[redacted]"
    },
    "local-dev": {
      "name": "local-dev",
      "base_url": "http://127.0.0.1:11434/v1",
      "api_key": "[redacted]"
    },
    "Codex++ Cloud": {
      "name": "Codex++ Cloud",
      "base_url": "http://127.0.0.1:19091/v1",
      "upstream_base_url": "https://gateway.example/v1",
      "api_key": "[redacted]",
      "managed": true,
      "default_model": "allowed-model-fixture"
    }
  }
}
"@
    $rollback = @"
{
  "desktop_version": "0.9.0-fixture",
  "model_provider": "legacy-openai",
  "model_providers": {
    "legacy-openai": {
      "name": "legacy-openai",
      "base_url": "https://legacy.example/v1",
      "api_key": "[redacted]"
    },
    "local-dev": {
      "name": "local-dev",
      "base_url": "http://127.0.0.1:11434/v1",
      "api_key": "[redacted]"
    }
  }
}
"@

    Set-Content -LiteralPath (Join-Path $Directory "pre-upgrade.json") -Encoding UTF8 -Value $pre
    Set-Content -LiteralPath (Join-Path $Directory "post-upgrade.json") -Encoding UTF8 -Value $post
    Set-Content -LiteralPath (Join-Path $Directory "logout.json") -Encoding UTF8 -Value $logout
    Set-Content -LiteralPath (Join-Path $Directory "rollback.json") -Encoding UTF8 -Value $rollback
}

if ($FixtureMode) {
    $fixtureDir = Join-Path $EvidencePath "_fixture-compatibility-snapshots"
    if (Test-Path -LiteralPath $fixtureDir) {
        Remove-Item -LiteralPath $fixtureDir -Recurse -Force
    }
    Write-FixtureSnapshots $fixtureDir
    $PreUpgradeSnapshot = Join-Path $fixtureDir "pre-upgrade.json"
    $PostUpgradeSnapshot = Join-Path $fixtureDir "post-upgrade.json"
    $LogoutSnapshot = Join-Path $fixtureDir "logout.json"
    $RollbackSnapshot = Join-Path $fixtureDir "rollback.json"
}

function Add-ProviderRecord {
    param(
        [System.Collections.Generic.List[object]]$Providers,
        [string]$Name,
        [hashtable]$Fields,
        [string]$Source
    )
    if ([string]::IsNullOrWhiteSpace($Name)) {
        return
    }
    $fieldNames = @($Fields.Keys | ForEach-Object { [string]$_ })
    $isManaged = $false
    if ($Name -match "(?i)Codex\+\+\s*Cloud|CodexPlusPlus|managed") {
        $isManaged = $true
    }
    foreach ($flag in @("managed", "is_managed", "codexplus_managed", "codexplus_cloud")) {
        if ($Fields.ContainsKey($flag) -and ([string]$Fields[$flag]) -match "(?i)true|1|yes") {
            $isManaged = $true
        }
    }

    $apiKeyValue = Get-ProviderSecretFingerprintInput $Fields
    $baseUrlValue = Get-ProviderBaseUrlFingerprintInput $Fields
    $stableKey = Get-ProviderStableKey $Name $Fields

    $Providers.Add([pscustomobject]@{
        Name = $Name
        StableKey = $stableKey
        IsManaged = $isManaged
        HasApiKey = -not [string]::IsNullOrWhiteSpace($apiKeyValue)
        HasBaseUrl = -not [string]::IsNullOrWhiteSpace($baseUrlValue)
        ApiKeyHash = Get-SafeHash $apiKeyValue
        BaseUrlHash = Get-SafeHash $baseUrlValue
        FieldNames = $fieldNames
        Source = $Source
    })
}

function Get-ProviderFieldValue {
    param(
        [hashtable]$Fields,
        [string[]]$Aliases
    )
    foreach ($alias in $Aliases) {
        if ($Fields.ContainsKey($alias) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$alias])) {
            return [string]$Fields[$alias]
        }
    }
    return ""
}

function Get-ProviderBaseUrlFingerprintInput {
    param([hashtable]$Fields)
    $hashed = Get-ProviderFieldValue $Fields @("base_url_hash", "baseUrlHash", "upstream_base_url_hash", "upstreamBaseUrlHash", "url_hash", "endpoint_hash", "relayBaseUrlHash")
    if (-not [string]::IsNullOrWhiteSpace($hashed)) {
        return "hash:$hashed"
    }
    $direct = Get-ProviderFieldValue $Fields @("base_url", "baseUrl", "upstream_base_url", "upstreamBaseUrl", "url", "endpoint", "endpointUrl", "api_base", "apiBase", "relayBaseUrl")
    if (-not [string]::IsNullOrWhiteSpace($direct)) {
        return $direct
    }
    $configContents = Get-ProviderFieldValue $Fields @("configContents", "config_contents", "relayCommonConfigContents")
    if ($configContents -match "(?im)^\s*(base_url|upstream_base_url|api_base)\s*=\s*[""']([^""']+)[""']") {
        return $Matches[2]
    }
    return ""
}

function Get-ProviderSecretFingerprintInput {
    param([hashtable]$Fields)
    $hashed = Get-ProviderFieldValue $Fields @("api_key_hash", "apiKeyHash", "upstream_key_hash", "upstreamKeyHash", "openai_api_key_hash", "openaiApiKeyHash", "key_hash", "relayApiKeyHash", "experimental_bearer_token_hash")
    if (-not [string]::IsNullOrWhiteSpace($hashed)) {
        return "hash:$hashed"
    }
    $direct = Get-ProviderFieldValue $Fields @("api_key", "apiKey", "upstream_key", "upstreamKey", "openai_api_key", "openaiApiKey", "key", "relayApiKey")
    if (-not [string]::IsNullOrWhiteSpace($direct)) {
        return $direct
    }
    $authContents = Get-ProviderFieldValue $Fields @("authContents", "auth_contents", "auth")
    if (-not [string]::IsNullOrWhiteSpace($authContents)) {
        return $authContents
    }
    $configContents = Get-ProviderFieldValue $Fields @("configContents", "config_contents", "relayCommonConfigContents")
    if ($configContents -match "(?im)^\s*(experimental_bearer_token|api_key|openai_api_key|upstream_key)\s*=\s*[""']([^""']*)[""']") {
        return $Matches[2]
    }
    return ""
}

function Get-ProviderStableKey {
    param(
        [string]$Name,
        [hashtable]$Fields
    )
    foreach ($alias in @("id", "provider_id", "providerId", "relay_id", "relayId", "profile_id", "profileId", "name")) {
        if ($Fields.ContainsKey($alias) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$alias])) {
            return ([string]$Fields[$alias]).Trim().ToLowerInvariant()
        }
    }
    return $Name.Trim().ToLowerInvariant()
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

function Convert-ObjectToFieldMap {
    param([object]$Value)
    $map = @{}
    if ($null -eq $Value) {
        return $map
    }
    if ($Value -is [System.Collections.IDictionary]) {
        foreach ($key in $Value.Keys) {
            $map[[string]$key] = $Value[$key]
        }
        return $map
    }
    foreach ($property in $Value.PSObject.Properties) {
        $map[$property.Name] = $property.Value
    }
    return $map
}

function Add-JsonProviders {
    param(
        [System.Collections.Generic.List[object]]$Providers,
        [object]$Json,
        [string]$Source
    )
    $rootMap = Convert-ObjectToFieldMap $Json
    $foundProviderNode = $false
    foreach ($key in @("model_providers", "modelProviders", "providers", "relayProfiles", "relay_profiles")) {
        if ($rootMap.ContainsKey($key)) {
            Add-JsonProviderNode $Providers $rootMap[$key] "$Source/$key"
            $foundProviderNode = $true
        }
    }

    foreach ($key in @("settings", "provider_settings", "providerSettings", "legacy_settings", "legacySettings")) {
        if ($rootMap.ContainsKey($key)) {
            Add-JsonProviders $Providers $rootMap[$key] "$Source/$key"
        }
    }

    if ($rootMap.ContainsKey("relayBaseUrl") -or $rootMap.ContainsKey("relayApiKey")) {
        $fallbackFields = @{}
        foreach ($key in @("activeRelayId", "relayBaseUrl", "relayApiKey", "relayCommonConfigContents")) {
            if ($rootMap.ContainsKey($key)) {
                $fallbackFields[$key] = $rootMap[$key]
            }
        }
        $fallbackName = "legacy-relay"
        if ($rootMap.ContainsKey("activeRelayId") -and -not [string]::IsNullOrWhiteSpace([string]$rootMap["activeRelayId"])) {
            $fallbackName = [string]$rootMap["activeRelayId"]
        }
        if (-not $fallbackFields.ContainsKey("name")) {
            $fallbackFields["name"] = $fallbackName
        }
        if (-not $fallbackFields.ContainsKey("id")) {
            $fallbackFields["id"] = $fallbackName
        }
        Add-ProviderRecord $Providers $fallbackName $fallbackFields "$Source/root-relay"
        $foundProviderNode = $true
    }

    if (-not $foundProviderNode -and $rootMap.ContainsKey("model_provider")) {
        Add-ProviderRecord $Providers ([string]$rootMap["model_provider"]) @{} $Source
    }
}

function Add-JsonProviderNode {
    param(
        [System.Collections.Generic.List[object]]$Providers,
        [object]$ProvidersNode,
        [string]$Source
    )
    if ($null -eq $ProvidersNode) {
        return
    }
    if ($ProvidersNode -is [System.Array]) {
        foreach ($item in $ProvidersNode) {
            $fields = Convert-ObjectToFieldMap $item
            $name = Get-ProviderNameFromFields $fields
            Add-ProviderRecord $Providers $name $fields $Source
        }
        return
    }

    $providerMap = Convert-ObjectToFieldMap $ProvidersNode
    foreach ($entry in $providerMap.GetEnumerator()) {
        $fields = Convert-ObjectToFieldMap $entry.Value
        if (-not $fields.ContainsKey("name")) {
            $fields["name"] = $entry.Key
        }
        if (-not $fields.ContainsKey("id")) {
            $fields["id"] = $entry.Key
        }
        Add-ProviderRecord $Providers (Get-ProviderNameFromFields $fields) $fields $Source
    }
}

function Get-ProviderNameFromFields {
    param([hashtable]$Fields)
    foreach ($key in @("name", "id", "provider", "provider_name", "providerName", "relay_id", "relayId", "profile_id", "profileId")) {
        if ($Fields.ContainsKey($key) -and -not [string]::IsNullOrWhiteSpace([string]$Fields[$key])) {
            return [string]$Fields[$key]
        }
    }
    return ""
}

function Add-TomlProviders {
    param(
        [System.Collections.Generic.List[object]]$Providers,
        [string[]]$Lines,
        [string]$Source
    )
    $currentName = ""
    $currentFields = @{}
    $defaultProvider = ""

    function Flush-CurrentProvider {
        $name = $script:currentName
        if ([string]::IsNullOrWhiteSpace($name)) {
            $name = Get-ProviderNameFromFields $script:currentFields
        }
        if (-not [string]::IsNullOrWhiteSpace($name)) {
            Add-ProviderRecord $Providers $name $script:currentFields $Source
        }
        $script:currentName = ""
        $script:currentFields = @{}
    }

    $script:currentName = ""
    $script:currentFields = @{}
    $script:insideProviderTable = $false

    foreach ($line in $Lines) {
        $trimmed = $line.Trim()
        if ($trimmed -match '^\s*model_provider\s*=\s*["'']([^"'']+)["'']') {
            $defaultProvider = $Matches[1]
        }
        if ($trimmed -match '^\s*\[(model_providers|modelProviders|providers|relayProfiles|relay_profiles)\.("?)([^"\]]+)\2\]\s*$') {
            Flush-CurrentProvider
            $script:currentName = $Matches[3]
            $script:currentFields = @{}
            $script:insideProviderTable = $true
            continue
        }
        if ($trimmed -match '^\s*\[\[(model_providers|modelProviders|providers|relayProfiles|relay_profiles)\]\]\s*$') {
            Flush-CurrentProvider
            $script:currentName = ""
            $script:currentFields = @{}
            $script:insideProviderTable = $true
            continue
        }
        if ($trimmed -match '^\s*\[') {
            Flush-CurrentProvider
            $script:insideProviderTable = $false
            continue
        }
        if ($script:insideProviderTable -and $trimmed -match '^\s*([A-Za-z0-9_-]+)\s*=\s*(.+?)\s*$') {
            $key = $Matches[1]
            $value = $Matches[2].Trim().Trim('"').Trim("'")
            $script:currentFields[$key] = $value
        }
    }

    Flush-CurrentProvider

    if ($Providers.Count -eq 0 -and -not [string]::IsNullOrWhiteSpace($defaultProvider)) {
        Add-ProviderRecord $Providers $defaultProvider @{} $Source
    }
}

function Read-ProviderSnapshot {
    param(
        [string]$Label,
        [string]$Path
    )
    $resolved = Resolve-InputPath $Path
    $providers = New-Object System.Collections.Generic.List[object]
    if ([string]::IsNullOrWhiteSpace($resolved) -or -not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
        return [pscustomobject]@{
            Label = $Label
            Path = $resolved
            RelativePath = ConvertTo-SafeEvidenceText (Get-RelativePath $resolved)
            Missing = $true
            ParseError = ""
            RawText = ""
            Providers = @()
            ManualProviders = @()
            ManagedProviders = @()
        }
    }

    $raw = Get-Content -Raw -LiteralPath $resolved
    $parseError = ""
    try {
        if ([System.IO.Path]::GetExtension($resolved).ToLowerInvariant() -eq ".json") {
            $json = $raw | ConvertFrom-Json
            Add-JsonProviders $providers $json $Label
        } else {
            Add-TomlProviders $providers ($raw -split "`r?`n") $Label
        }
    } catch {
        $parseError = $_.Exception.Message
    }

    $providerArray = @($providers.ToArray())
    $manual = @($providerArray | Where-Object { -not $_.IsManaged } | ForEach-Object { $_.Name } | Sort-Object -Unique)
    $managed = @($providerArray | Where-Object { $_.IsManaged } | ForEach-Object { $_.Name } | Sort-Object -Unique)

    return [pscustomobject]@{
        Label = $Label
        Path = $resolved
        RelativePath = ConvertTo-SafeEvidenceText (Get-RelativePath $resolved)
        Missing = $false
        ParseError = $parseError
        RawText = $raw
        Providers = $providerArray
        ManualProviders = $manual
        ManagedProviders = $managed
    }
}

function Get-MissingNames {
    param(
        [string[]]$Expected,
        [string[]]$Actual
    )
    $missing = @()
    foreach ($name in $Expected) {
        if ($Actual -notcontains $name) {
            $missing += $name
        }
    }
    return $missing
}

function Format-Names {
    param([string[]]$Names)
    if ($Names.Count -eq 0) {
        return "none"
    }
    return ($Names | ForEach-Object { ConvertTo-SafeEvidenceText $_ }) -join ", "
}

function Get-ManualProviderComparisons {
    param(
        [object]$ExpectedSnapshot,
        [object]$ActualSnapshot
    )
    $comparisons = New-Object System.Collections.Generic.List[object]
    $expectedManual = @($ExpectedSnapshot.Providers | Where-Object { -not $_.IsManaged })
    $actualManual = @($ActualSnapshot.Providers | Where-Object { -not $_.IsManaged })
    foreach ($expected in $expectedManual) {
        $actualMatches = @($actualManual | Where-Object { $_.StableKey -eq $expected.StableKey })
        $present = ($actualMatches.Count -gt 0)
        $contentUnchanged = $false
        if ($present) {
            foreach ($actual in $actualMatches) {
                if (
                    ($actual.HasBaseUrl -eq $expected.HasBaseUrl) -and
                    ($actual.HasApiKey -eq $expected.HasApiKey) -and
                    ($actual.BaseUrlHash -eq $expected.BaseUrlHash) -and
                    ($actual.ApiKeyHash -eq $expected.ApiKeyHash)
                ) {
                    $contentUnchanged = $true
                    break
                }
            }
        }
        $comparisons.Add([pscustomobject]@{
            Name = $expected.Name
            StableKey = $expected.StableKey
            Present = $present
            ContentUnchanged = $contentUnchanged
        })
    }
    return @($comparisons.ToArray())
}

function Get-ChangedManualProviderNames {
    param([object[]]$Comparisons)
    return @($Comparisons | Where-Object { $_.Present -and -not $_.ContentUnchanged } | ForEach-Object { $_.Name } | Sort-Object -Unique)
}

function Test-ManualProviderContentUnchanged {
    param([object[]]$Comparisons)
    if ($Comparisons.Count -eq 0) {
        return $false
    }
    return (@($Comparisons | Where-Object { -not $_.Present -or -not $_.ContentUnchanged }).Count -eq 0)
}

function Test-LegacyProviderShapeParsed {
    param([object]$Snapshot)
    if ($Snapshot.Missing) {
        return $false
    }
    return (@($Snapshot.Providers | Where-Object { $_.Source -match "(?i)(settings|relayProfiles|relay_profiles)" }).Count -gt 0)
}

function Test-CommercialPolicyAbsent {
    param([string]$Text)
    return ($Text -notmatch "(?i)\b(price|price_amount|price_amount_minor|plan_id|plan_catalog|usage_policy|usage_policy_id|entitlement|multiplier|quota|purchase_url|renew_url)\b")
}

function Test-LogoutTokenCleared {
    param([string]$Text)
    $activeTokenPattern = "(?i)[""']?(access_token|accessToken|refresh_token|refreshToken|id_token|idToken|poll_token|pollToken|session_token|sessionToken|desktopAccessToken|desktopRefreshToken|bearerToken|authorization)[""']?\s*[:=]\s*[""']?(?!\[?redacted|<redacted|REDACTED|null\b|none\b|false\b|0\b|[""']?\s*[,}`r`n])[A-Za-z0-9._~+/=-]{8,}"
    return ($Text -notmatch $activeTokenPattern)
}

$pre = Read-ProviderSnapshot "pre-upgrade" $PreUpgradeSnapshot
$post = Read-ProviderSnapshot "post-upgrade" $PostUpgradeSnapshot
$logout = Read-ProviderSnapshot "logout" $LogoutSnapshot
$rollback = Read-ProviderSnapshot "rollback" $RollbackSnapshot

$allSnapshots = @($pre, $post, $logout, $rollback)
$missingSnapshots = @($allSnapshots | Where-Object { $_.Missing })
$parseFailures = @($allSnapshots | Where-Object { -not $_.Missing -and -not [string]::IsNullOrWhiteSpace($_.ParseError) })

$preManual = @($pre.ManualProviders)
$postManual = @($post.ManualProviders)
$logoutManual = @($logout.ManualProviders)
$rollbackManual = @($rollback.ManualProviders)

$postMissingManual = @(Get-MissingNames $preManual $postManual)
$logoutMissingManual = @(Get-MissingNames $preManual $logoutManual)
$rollbackMissingManual = @(Get-MissingNames $preManual $rollbackManual)
$postManualComparisons = @(Get-ManualProviderComparisons $pre $post)
$logoutManualComparisons = @(Get-ManualProviderComparisons $pre $logout)
$rollbackManualComparisons = @(Get-ManualProviderComparisons $pre $rollback)
$postChangedManual = @(Get-ChangedManualProviderNames $postManualComparisons)
$logoutChangedManual = @(Get-ChangedManualProviderNames $logoutManualComparisons)
$rollbackChangedManual = @(Get-ChangedManualProviderNames $rollbackManualComparisons)

$preHasManual = (-not $pre.Missing) -and ($preManual.Count -gt 0)
$postPreservesManual = $preHasManual -and ($postMissingManual.Count -eq 0)
$postManualContentUnchanged = $preHasManual -and (Test-ManualProviderContentUnchanged $postManualComparisons)
$postHasManagedCloud = (-not $post.Missing) -and (@($post.ManagedProviders).Count -gt 0)
$postHasNoCommercialPolicy = (-not $post.Missing) -and (Test-CommercialPolicyAbsent $post.RawText)
$postManagedRuntimeOnlyConfig = $postHasManagedCloud -and $postHasNoCommercialPolicy
$logoutPreservesManual = $preHasManual -and ($logoutMissingManual.Count -eq 0)
$logoutManualContentUnchanged = $preHasManual -and (Test-ManualProviderContentUnchanged $logoutManualComparisons)
$logoutClearsTokens = (-not $logout.Missing) -and (Test-LogoutTokenCleared $logout.RawText)
$snapshotsWithCommercialPolicy = @($allSnapshots | Where-Object { -not $_.Missing -and -not (Test-CommercialPolicyAbsent $_.RawText) })
$snapshotsWithTokenFields = @($allSnapshots | Where-Object { -not $_.Missing -and -not (Test-LogoutTokenCleared $_.RawText) })
$allSnapshotsHaveNoCommercialPolicy = ($snapshotsWithCommercialPolicy.Count -eq 0)
$allSnapshotsClearTokenFields = ($snapshotsWithTokenFields.Count -eq 0)
$preLegacyProviderShapeParsed = Test-LegacyProviderShapeParsed $pre
$manualSwitchEvidence = $postPreservesManual -and $postManualContentUnchanged -and ($postManual.Count -gt 0)
$providerSyncEvidence = $postPreservesManual -and $postManualContentUnchanged -and $logoutPreservesManual -and $logoutManualContentUnchanged
$rollbackPreservesManual = $preHasManual -and ($rollbackMissingManual.Count -eq 0)
$rollbackManualContentUnchanged = $preHasManual -and (Test-ManualProviderContentUnchanged $rollbackManualComparisons)

$allPassed = (
    ($missingSnapshots.Count -eq 0) -and
    ($parseFailures.Count -eq 0) -and
    $preLegacyProviderShapeParsed -and
    $preHasManual -and
    $postPreservesManual -and
    $postManualContentUnchanged -and
    $postHasManagedCloud -and
    $postHasNoCommercialPolicy -and
    $allSnapshotsHaveNoCommercialPolicy -and
    $allSnapshotsClearTokenFields -and
    $logoutPreservesManual -and
    $logoutManualContentUnchanged -and
    $logoutClearsTokens -and
    $manualSwitchEvidence -and
    $providerSyncEvidence -and
    $rollbackPreservesManual -and
    $rollbackManualContentUnchanged
)

$snapshotResultText = if ($allPassed) { "pass" } else { "fail" }
$resultText = $snapshotResultText

$snapshotList = ($allSnapshots | ForEach-Object {
    $state = if ($_.Missing) { "missing" } elseif (-not [string]::IsNullOrWhiteSpace($_.ParseError)) { "parse-failed" } else { "parsed" }
    "- $($_.Label): $state; path=$(ConvertTo-SafeEvidenceText $_.RelativePath); manual=$(Format-Names $_.ManualProviders); managed=$(Format-Names $_.ManagedProviders); legacy relayProfiles/settings parsed=$(Test-LegacyProviderShapeParsed $_)"
}) -join [Environment]::NewLine

$missingList = if ($missingSnapshots.Count -gt 0) {
    ($missingSnapshots | ForEach-Object { "- $($_.Label): input snapshot missing" }) -join [Environment]::NewLine
} else {
    "- none"
}

$parseFailureList = if ($parseFailures.Count -gt 0) {
    ($parseFailures | ForEach-Object { "- $($_.Label): parse failed; detail intentionally brief: $(ConvertTo-SafeEvidenceText $_.ParseError)" }) -join [Environment]::NewLine
} else {
    "- none"
}

$context = @"
# 00 Test Context

Run folder: $runName
Status: executed
Result: $resultText

Desktop version before upgrade: supplied by snapshot source when available; this runner records provider compatibility only.
Desktop version after upgrade: supplied by snapshot source when available; this runner records provider compatibility only.
Legacy settings source and sanitized snapshot path recorded below.
Environment owner and rollback owner: not supplied to the snapshot inspection runner.

## Snapshot Inputs

$snapshotList

## Snapshot Hygiene

- All snapshots token-field scan clear: $allSnapshotsClearTokenFields.
- All snapshots commercial-policy scan clear: $allSnapshotsHaveNoCommercialPolicy.
- Legacy relayProfiles/settings parsed: $preLegacyProviderShapeParsed.
- Manual provider content comparison uses nonprinted base URL/API key hashes: True.
- Snapshots with token fields: $(Format-Names @($snapshotsWithTokenFields | ForEach-Object { $_.Label })).
- Snapshots with commercial policy fields: $(Format-Names @($snapshotsWithCommercialPolicy | ForEach-Object { $_.Label })).

## Missing Inputs

$missingList

## Parse Failures

$parseFailureList

## Redaction

Snapshot contents are not copied into evidence. Provider names, booleans and sanitized paths are recorded. API keys redacted; token values are intentionally not printed.
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "00-test-context.md") -Encoding UTF8 -Value $context

$preResult = if ($preHasManual -and $parseFailures.Count -eq 0 -and -not $pre.Missing) { "pass" } else { "fail" }
$preEvidence = @"
# 01 Pre-Upgrade Snapshot

Result: $preResult

Manual provider names and types before upgrade recorded: $(Format-Names $preManual).
Base URLs redacted.
API keys redacted.
Default provider before upgrade recorded when present in the sanitized source.
Settings snapshot path, screenshot path, and redacted log path: $(ConvertTo-SafeEvidenceText $pre.RelativePath).

## Snapshot Inspection

- Pre-upgrade snapshot parsed: $(-not $pre.Missing -and [string]::IsNullOrWhiteSpace($pre.ParseError)).
- Legacy relayProfiles/settings parsed: $preLegacyProviderShapeParsed.
- Manual provider count: $($preManual.Count).
- Managed provider count before upgrade: $(@($pre.ManagedProviders).Count).
- Snapshot contents were not copied into this evidence folder.
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "01-pre-upgrade-snapshot.md") -Encoding UTF8 -Value $preEvidence

$postResult = if ($postPreservesManual -and $postHasManagedCloud -and $postHasNoCommercialPolicy) { "pass" } else { "fail" }
$postEvidence = @"
# 02 Post-Upgrade Managed Cloud

Result: $postResult

Manual providers preserved after upgrade: $postPreservesManual.
Manual provider content unchanged after upgrade: $postManualContentUnchanged.
Codex++ Cloud provider written or refreshed without overwriting manual providers: $postHasManagedCloud.
Advanced provider configuration remains reachable: snapshot evidence shows manual providers are still present; UI navigation remains separate runtime evidence.
No plan, price, multiplier, entitlement, or usage policy data was written by migration: $postHasNoCommercialPolicy.
Managed provider runtime-only config: $postManagedRuntimeOnlyConfig.
Manual provider content comparison used nonprinted base URL/API key hashes.

## Snapshot Inspection

- Manual providers before upgrade: $(Format-Names $preManual).
- Manual providers after upgrade: $(Format-Names $postManual).
- Missing manual providers after upgrade: $(Format-Names $postMissingManual).
- Manual providers with changed content after upgrade: $(Format-Names $postChangedManual).
- Managed providers after upgrade: $(Format-Names $post.ManagedProviders).
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "02-post-upgrade-cloud.md") -Encoding UTF8 -Value $postEvidence

$logoutSnapshotResult = if ($logoutPreservesManual -and $logoutManualContentUnchanged -and $logoutClearsTokens) { "pass" } else { "fail" }
$logoutEvidence = @"
# 03 Cloud Logout Boundary

Result: fail
Snapshot subset result: $logoutSnapshotResult

Cloud login creates only expected cloud/session state: not evaluated by snapshot inspection; runtime login evidence remains required.
Cloud logout clears cloud session state: $logoutClearsTokens.
Manual providers remain unchanged after logout: $logoutPreservesManual.
Manual provider content unchanged after logout: $logoutManualContentUnchanged.
Redacted before and after provider snapshots are compared by provider names only.

## Snapshot Inspection

- Manual providers before upgrade: $(Format-Names $preManual).
- Manual providers after logout: $(Format-Names $logoutManual).
- Missing manual providers after logout: $(Format-Names $logoutMissingManual).
- Manual providers with changed content after logout: $(Format-Names $logoutChangedManual).
- Logout token-field scan clear: $logoutClearsTokens.
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "03-cloud-logout-boundary.md") -Encoding UTF8 -Value $logoutEvidence

$manualSwitchSnapshotResult = if ($manualSwitchEvidence) { "pass" } else { "fail" }
$manualSwitchEvidenceText = @"
# 04 Manual Provider Switch

Result: fail
Snapshot subset result: $manualSwitchSnapshotResult

Manual provider can still be selected after upgrade: snapshot evidence shows manual providers still exist after upgrade.
Manual provider can still be used after managed cloud refresh: separate runtime request evidence remains required.
Manual provider content unchanged after managed cloud refresh: $postManualContentUnchanged.
Default user path still shows managed cloud entry point: snapshot evidence shows managed Cloud provider exists after upgrade.
Advanced users can reach provider configuration: snapshot evidence supports provider presence only; UI navigation remains separate runtime evidence.

## Snapshot Inspection

- Manual providers after upgrade: $(Format-Names $postManual).
- Managed providers after upgrade: $(Format-Names $post.ManagedProviders).
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "04-manual-provider-switch.md") -Encoding UTF8 -Value $manualSwitchEvidenceText

$providerSyncSnapshotResult = if ($providerSyncEvidence) { "pass" } else { "fail" }
$providerSyncEvidenceText = @"
# 05 Provider Sync

Result: fail
Snapshot subset result: $providerSyncSnapshotResult

Provider sync recognizes legacy profiles: manual provider names from the pre-upgrade snapshot are compared against post-upgrade and logout snapshots.
Provider sync does not corrupt manual provider entries: $providerSyncEvidence.
Provider sync does not log full API keys, JWTs, Authorization headers, upstream credentials, or .env secrets: this runner does not copy raw snapshot contents and writes only redacted evidence; runtime logs remain separate evidence.
Redacted sync logs and snapshot diff are represented by provider-name comparison plus nonprinted base URL/API key hash comparison.

## Snapshot Inspection

- Missing after upgrade: $(Format-Names $postMissingManual).
- Missing after logout: $(Format-Names $logoutMissingManual).
- Changed content after upgrade: $(Format-Names $postChangedManual).
- Changed content after logout: $(Format-Names $logoutChangedManual).
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "05-provider-sync.md") -Encoding UTF8 -Value $providerSyncEvidenceText

$rollbackSnapshotResult = if ($rollbackPreservesManual -and $rollbackManualContentUnchanged) { "pass" } else { "fail" }
$rollbackEvidence = @"
# 06 Rollback Rehearsal

Result: fail
Snapshot subset result: $rollbackSnapshotResult

Config rollback preserves or recovers manual providers: $rollbackPreservesManual.
Manual provider content unchanged after rollback: $rollbackManualContentUnchanged.
Desktop rollback keeps advanced provider settings reachable: snapshot evidence shows manual providers remain after rollback; runtime UI evidence remains required.
Backend/gateway rollback does not force managed-provider-only assumptions: snapshot evidence shows rollback state is not managed-provider-only.
Failed provider write recovery from last settings snapshot was recorded by provider-name comparison.
User-side key exposure response, if applicable, is redacted and owned by the release process.

## Snapshot Inspection

- Manual providers before upgrade: $(Format-Names $preManual).
- Manual providers after rollback: $(Format-Names $rollbackManual).
- Missing manual providers after rollback: $(Format-Names $rollbackMissingManual).
- Manual providers with changed content after rollback: $(Format-Names $rollbackChangedManual).
"@
Set-Content -LiteralPath (Join-Path $EvidencePath "06-rollback-rehearsal.md") -Encoding UTF8 -Value $rollbackEvidence

$commands = "- powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1 -PreUpgradeSnapshot <path> -PostUpgradeSnapshot <path> -LogoutSnapshot <path> -RollbackSnapshot <path> -EvidenceDir <compatibility-evidence-dir>"

$riskText = if ($allPassed) {
    "- Snapshot inspection passed. Runtime launch, UI navigation, real provider switching and gateway request evidence remain required for release."
} else {
    "- Snapshot inspection failed or was incomplete. Missing/parse failures or provider preservation failures must be fixed before Module J can treat compatibility evidence as ready."
}

$gate = @(
    "# 07 Compatibility Gate Report",
    "",
    "Compatibility evidence result: fail",
    "Compatibility snapshot subset result: $snapshotResultText",
    "",
    "## Commands Executed",
    "",
    $commands,
    "",
    "## Evidence Links",
    "",
    "- Sanitized snapshot comparison evidence: this compatibility evidence folder.",
    "- Snapshot paths are sanitized in 00-test-context.md.",
    "",
    "## Remaining Risks",
    "",
    $riskText,
    "",
    "## Release Boundary",
    "",
    "This snapshot runner is ready for Module J hygiene review only when it passes together with runtime compatibility evidence. It proves legacy relayProfiles/settings parsing, provider-list preservation and nonprinted manual-provider content comparison only. It does not override E2E, package, or release go/no-go."
) -join [Environment]::NewLine
Set-Content -LiteralPath (Join-Path $EvidencePath "07-compatibility-gate-report.md") -Encoding UTF8 -Value $gate

Write-Host "07 compatibility snapshot inspection evidence: $EvidencePath"
Write-Host "Compatibility snapshot inspection result: $snapshotResultText"
Write-Host "Provider names and sanitized paths recorded; token values intentionally not printed."

if (-not $allPassed) {
    Write-Host "07 compatibility snapshot inspection failed. Missing snapshots: $($missingSnapshots.Count); parse failures: $($parseFailures.Count)." -ForegroundColor Red
    exit 1
}

Write-Host "07 compatibility snapshot inspection passed."
exit 0

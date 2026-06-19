param(
    [string]$Root,
    [string]$EvidenceDir
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    throw "EvidenceDir is required. Example: -EvidenceDir codex-plus-dev-plan/test-runs/20260618-1500-compatibility"
}

if ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidencePath = $EvidenceDir
} else {
    $EvidencePath = Join-Path $Root $EvidenceDir
}

$results = New-Object System.Collections.Generic.List[object]

function Add-Check {
    param(
        [string]$Name,
        [bool]$Passed,
        [string]$Detail
    )
    $script:results.Add([pscustomobject]@{
        Check = $Name
        Result = if ($Passed) { "PASS" } else { "FAIL" }
        Detail = $Detail
    })
}

function Read-TextFile {
    param([string]$Path)
    return Get-Content -Raw -LiteralPath $Path
}

function Add-Text-Check {
    param(
        [string]$RelativeFile,
        [string]$Pattern,
        [string]$Name
    )
    $path = Join-Path $EvidencePath $RelativeFile
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        Add-Check $Name $false "$RelativeFile is missing."
        return
    }
    $text = Read-TextFile $path
    Add-Check $Name ([bool]($text -match $Pattern)) "$Pattern in $RelativeFile"
}

function Add-Result-Pass-Check {
    param(
        [string]$RelativeFile,
        [string]$Name
    )
    $path = Join-Path $EvidencePath $RelativeFile
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        Add-Check $Name $false "$RelativeFile is missing."
        return
    }
    $text = Read-TextFile $path
    $match = [regex]::Match($text, "(?im)^\s*Result\s*:\s*(pass|fail)\s*$")
    Add-Check $Name ($match.Success -and $match.Groups[1].Value.ToLowerInvariant() -eq "pass") "Result must be pass in $RelativeFile"
}

function Add-Run-Folder-Consistency-Check {
    param(
        [string]$RelativeFile,
        [string]$ExpectedLeaf,
        [string]$Name
    )
    $path = Join-Path $EvidencePath $RelativeFile
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        return
    }
    $text = Read-TextFile $path
    $matches = [regex]::Matches($text, "(?im)^\s*Run folder\s*:\s*([^\r\n]+?)\s*$")
    if ($matches.Count -eq 0) {
        Add-Check $Name $true "$RelativeFile does not declare Run folder."
        return
    }
    $values = @($matches | ForEach-Object { $_.Groups[1].Value.Trim().Trim([char]0x60) })
    $mismatched = @($values | Where-Object { $_ -ne $ExpectedLeaf })
    Add-Check $Name ($mismatched.Count -eq 0) "Run folder declaration(s): $($values -join ', '); expected $ExpectedLeaf"
}

function Get-Relative-Path {
    param([string]$Path)
    $base = [System.IO.Path]::GetFullPath($EvidencePath).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath($Path)
    if ($full.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $full.Substring($base.Length).TrimStart('\', '/')
    }
    return $full
}

$requiredFiles = @(
    "00-test-context.md",
    "01-pre-upgrade-snapshot.md",
    "02-post-upgrade-cloud.md",
    "03-cloud-logout-boundary.md",
    "04-manual-provider-switch.md",
    "05-provider-sync.md",
    "06-rollback-rehearsal.md",
    "07-compatibility-gate-report.md"
)

Add-Check "compatibility-evidence-dir-exists" (Test-Path -LiteralPath $EvidencePath -PathType Container) $EvidencePath

if (Test-Path -LiteralPath $EvidencePath -PathType Container) {
    $leaf = Split-Path -Leaf $EvidencePath
    Add-Check "compatibility-evidence-dir-name" ($leaf -match "^\d{8}-\d{4}-compatibility$") "Expected YYYYMMDD-HHMM-compatibility; got $leaf."

    foreach ($file in $requiredFiles) {
        $path = Join-Path $EvidencePath $file
        $exists = Test-Path -LiteralPath $path -PathType Leaf
        Add-Check "file-exists:$file" $exists $file
        if ($exists) {
            $item = Get-Item -LiteralPath $path
            Add-Check "file-nonempty:$file" ($item.Length -gt 0) "$file length=$($item.Length)"
        }
    }

    foreach ($file in $requiredFiles) {
        Add-Run-Folder-Consistency-Check $file $leaf "run-folder:$file"
    }

    Add-Text-Check "00-test-context.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "context:result-final"
    Add-Result-Pass-Check "00-test-context.md" "context:result-pass"
    Add-Text-Check "00-test-context.md" "(?i)desktop version before upgrade" "context:before-version"
    Add-Text-Check "00-test-context.md" "(?i)desktop version after upgrade" "context:after-version"
    Add-Text-Check "00-test-context.md" "(?i)Snapshot Inputs" "context:snapshot-inputs"
    Add-Text-Check "00-test-context.md" "(?im)^\s*-\s*pre-upgrade\s*:\s*parsed\b" "context:pre-upgrade-parsed"
    Add-Text-Check "00-test-context.md" "(?im)^\s*-\s*post-upgrade\s*:\s*parsed\b" "context:post-upgrade-parsed"
    Add-Text-Check "00-test-context.md" "(?im)^\s*-\s*logout\s*:\s*parsed\b" "context:logout-parsed"
    Add-Text-Check "00-test-context.md" "(?im)^\s*-\s*rollback\s*:\s*parsed\b" "context:rollback-parsed"
    Add-Text-Check "00-test-context.md" "(?is)Missing Inputs\s*\r?\n\s*- none" "context:no-missing-inputs"
    Add-Text-Check "00-test-context.md" "(?is)Parse Failures\s*\r?\n\s*- none" "context:no-parse-failures"
    Add-Text-Check "00-test-context.md" "(?i)All snapshots token-field scan clear\s*:\s*True" "context:all-token-scans-clear"
    Add-Text-Check "00-test-context.md" "(?i)All snapshots commercial-policy scan clear\s*:\s*True" "context:all-commercial-policy-scans-clear"
    Add-Text-Check "00-test-context.md" "(?i)Legacy relayProfiles/settings parsed\s*:\s*True" "context:legacy-relay-settings-parsed"
    Add-Text-Check "00-test-context.md" "(?i)Manual provider content comparison uses nonprinted base URL/API key hashes\s*:\s*True" "context:manual-content-hash-comparison"
    Add-Text-Check "01-pre-upgrade-snapshot.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "pre-upgrade:result-final"
    Add-Result-Pass-Check "01-pre-upgrade-snapshot.md" "pre-upgrade:result-pass"
    Add-Text-Check "01-pre-upgrade-snapshot.md" "(?i)Manual provider" "pre-upgrade:manual-provider"
    Add-Text-Check "01-pre-upgrade-snapshot.md" "(?i)API keys? redacted" "pre-upgrade:redacted-keys"
    Add-Text-Check "01-pre-upgrade-snapshot.md" "(?i)Snapshot Inspection" "pre-upgrade:snapshot-inspection"
    Add-Text-Check "01-pre-upgrade-snapshot.md" "(?i)Legacy relayProfiles/settings parsed\s*:\s*True" "pre-upgrade:legacy-relay-settings-parsed"
    Add-Text-Check "01-pre-upgrade-snapshot.md" "(?im)^\s*-\s*Manual provider count\s*:\s*[1-9]\d*\s*\." "pre-upgrade:manual-provider-count"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "post-upgrade:result-final"
    Add-Result-Pass-Check "02-post-upgrade-cloud.md" "post-upgrade:result-pass"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?i)Manual providers preserved after upgrade\s*:\s*True" "post-upgrade:manual-preserved"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?i)Manual provider content unchanged after upgrade\s*:\s*True" "post-upgrade:manual-content-unchanged"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?i)Codex\+\+ Cloud provider written or refreshed without overwriting manual providers\s*:\s*True" "post-upgrade:managed-cloud"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?i)No plan, price, multiplier, entitlement, or usage policy data was written by migration\s*:\s*True" "post-upgrade:no-commercial-policy"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?i)Managed provider runtime-only config\s*:\s*True|Codex\+\+ Cloud provider stores only runtime connection/auth configuration\s*:\s*True" "post-upgrade:managed-runtime-only-config"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?i)Advanced provider configuration remains reachable\s*:\s*True|Advanced provider configuration remains reachable" "post-upgrade:advanced-configuration-reachable"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?im)^\s*-\s*Missing manual providers after upgrade\s*:\s*none\s*\." "post-upgrade:no-missing-manual"
    Add-Text-Check "02-post-upgrade-cloud.md" "(?im)^\s*-\s*Manual providers with changed content after upgrade\s*:\s*none\s*\." "post-upgrade:no-changed-manual-content"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "logout:result-final"
    Add-Result-Pass-Check "03-cloud-logout-boundary.md" "logout:result-pass"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?i)Runtime cloud login/logout evidence result\s*:\s*pass" "logout:runtime-login-logout-pass"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?i)Manual providers remain unchanged after logout\s*:\s*True" "logout:manual-preserved"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?i)Manual provider content unchanged after logout\s*:\s*True" "logout:manual-content-unchanged"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?im)^\s*-\s*Missing manual providers after logout\s*:\s*none\s*\." "logout:no-missing-manual"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?im)^\s*-\s*Manual providers with changed content after logout\s*:\s*none\s*\." "logout:no-changed-manual-content"
    Add-Text-Check "03-cloud-logout-boundary.md" "(?im)^\s*-\s*Logout token-field scan clear\s*:\s*True\s*\." "logout:token-scan-clear"
    Add-Text-Check "04-manual-provider-switch.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "manual-switch:result-final"
    Add-Result-Pass-Check "04-manual-provider-switch.md" "manual-switch:result-pass"
    Add-Text-Check "04-manual-provider-switch.md" "(?i)Runtime manual provider selection result\s*:\s*pass" "manual-switch:runtime-selection-pass"
    Add-Text-Check "04-manual-provider-switch.md" "(?i)Runtime manual provider request result\s*:\s*pass" "manual-switch:runtime-request-pass"
    Add-Text-Check "04-manual-provider-switch.md" "(?i)Manual provider content unchanged after managed cloud refresh\s*:\s*True" "manual-switch:manual-content-unchanged"
    Add-Text-Check "04-manual-provider-switch.md" "(?i)Default user path still shows managed cloud entry point\s*:\s*True|Default user path still shows managed cloud entry point" "manual-switch:default-managed-entry-point"
    Add-Text-Check "04-manual-provider-switch.md" "(?i)Manual provider can still be selected" "manual-switch:selected"
    Add-Text-Check "05-provider-sync.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "provider-sync:result-final"
    Add-Result-Pass-Check "05-provider-sync.md" "provider-sync:result-pass"
    Add-Text-Check "05-provider-sync.md" "(?i)Runtime provider sync log review result\s*:\s*pass" "provider-sync:runtime-log-review-pass"
    Add-Text-Check "05-provider-sync.md" "(?i)Provider sync log secret scan clear\s*:\s*True|Provider sync logs? .*redacted" "provider-sync:log-secret-scan-clear"
    Add-Text-Check "05-provider-sync.md" "(?i)Provider sync recognizes legacy profiles" "provider-sync:legacy"
    Add-Text-Check "05-provider-sync.md" "(?i)does not corrupt manual provider entries" "provider-sync:no-corruption"
    Add-Text-Check "05-provider-sync.md" "(?im)^\s*-\s*Changed content after upgrade\s*:\s*none\s*\." "provider-sync:no-changed-content-after-upgrade"
    Add-Text-Check "05-provider-sync.md" "(?im)^\s*-\s*Changed content after logout\s*:\s*none\s*\." "provider-sync:no-changed-content-after-logout"
    Add-Text-Check "06-rollback-rehearsal.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "rollback:result-final"
    Add-Result-Pass-Check "06-rollback-rehearsal.md" "rollback:result-pass"
    Add-Text-Check "06-rollback-rehearsal.md" "(?i)Runtime rollback rehearsal result\s*:\s*pass" "rollback:runtime-rehearsal-pass"
    Add-Text-Check "06-rollback-rehearsal.md" "(?i)Manual provider content unchanged after rollback\s*:\s*True" "rollback:manual-content-unchanged"
    Add-Text-Check "06-rollback-rehearsal.md" "(?i)Failed provider write recovery .*recorded|Failed provider write recovery result\s*:\s*pass" "rollback:failed-provider-write-recovery"
    Add-Text-Check "06-rollback-rehearsal.md" "(?i)Config rollback" "rollback:config"
    Add-Text-Check "06-rollback-rehearsal.md" "(?i)Desktop rollback" "rollback:desktop"
    Add-Text-Check "06-rollback-rehearsal.md" "(?i)Backend/gateway rollback" "rollback:backend-gateway"
    Add-Text-Check "06-rollback-rehearsal.md" "(?im)^\s*-\s*Missing manual providers after rollback\s*:\s*none\s*\." "rollback:no-missing-manual"
    Add-Text-Check "06-rollback-rehearsal.md" "(?im)^\s*-\s*Manual providers with changed content after rollback\s*:\s*none\s*\." "rollback:no-changed-manual-content"
    Add-Text-Check "07-compatibility-gate-report.md" "(?im)^\s*Compatibility evidence result\s*:\s*(pass|fail)\s*$" "compat-report:result"
    $compatReportPath = Join-Path $EvidencePath "07-compatibility-gate-report.md"
    if (Test-Path -LiteralPath $compatReportPath -PathType Leaf) {
        $compatReportText = Read-TextFile $compatReportPath
        $compatResultMatch = [regex]::Match($compatReportText, "(?im)^\s*Compatibility evidence result\s*:\s*(pass|fail)\s*$")
        Add-Check "compat-report:result-pass" ($compatResultMatch.Success -and $compatResultMatch.Groups[1].Value.ToLowerInvariant() -eq "pass") "Compatibility evidence result must be pass."
    } else {
        Add-Check "compat-report:result-pass" $false "07-compatibility-gate-report.md is missing."
    }
    Add-Text-Check "07-compatibility-gate-report.md" "(?im)^\s*Compatibility snapshot subset result\s*:\s*pass\s*$" "compat-report:snapshot-subset-pass"
    Add-Text-Check "07-compatibility-gate-report.md" "(?im)^\s*Runtime compatibility result\s*:\s*pass\s*$" "compat-report:runtime-result-pass"
    Add-Text-Check "07-compatibility-gate-report.md" "(?i)Commands Executed" "compat-report:commands"
    Add-Text-Check "07-compatibility-gate-report.md" "inspect-07-compatibility-snapshots\.ps1" "compat-report:snapshot-inspector-command"
    Add-Text-Check "07-compatibility-gate-report.md" "(?i)Evidence Links" "compat-report:evidence-links"
    Add-Text-Check "07-compatibility-gate-report.md" "(?i)Remaining Risks" "compat-report:remaining-risks"
    Add-Text-Check "07-compatibility-gate-report.md" "(?i)Release Boundary" "compat-report:release-boundary"

    $textExtensions = @(".csv", ".html", ".json", ".log", ".md", ".txt", ".toml", ".yaml", ".yml")
    $textFiles = Get-ChildItem -LiteralPath $EvidencePath -Recurse -File |
        Where-Object { $textExtensions -contains $_.Extension.ToLowerInvariant() }

    $leakRules = @(
        @{ Name = "api-key"; Pattern = "(?i)\b(sk-[a-z0-9][a-z0-9_-]{18,}|sk-proj-[a-z0-9][a-z0-9_-]{18,}|sk-ant-[a-z0-9][a-z0-9_-]{18,})\b" },
        @{ Name = "authorization-header"; Pattern = "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}" },
        @{ Name = "jwt"; Pattern = "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b" },
        @{ Name = "token-field"; Pattern = "(?i)[""']?(access_token|accessToken|refresh_token|refreshToken|id_token|idToken|poll_token|pollToken|session_token|sessionToken|desktopAccessToken|desktopRefreshToken|bearerToken|authorization|api_key|apiKey|upstream_key|upstreamKey)[""']?\s*[:=]\s*[""']?(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b)[A-Za-z0-9._~+/=-]{12,}" },
        @{ Name = "env-secret"; Pattern = "(?i)^[A-Z0-9_]*(KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*\s*=\s*(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b).{8,}" },
        @{ Name = "unfinished-placeholder"; Pattern = "(?i)\b(TODO|TBD|PLACEHOLDER|NOT_EXECUTED|FILL_ME|pending)\b" }
    )

    foreach ($file in $textFiles) {
        $relative = Get-Relative-Path $file.FullName
        $lines = Get-Content -LiteralPath $file.FullName
        for ($index = 0; $index -lt $lines.Count; $index++) {
            $lineNumber = $index + 1
            foreach ($rule in $leakRules) {
                if ($lines[$index] -match $rule.Pattern) {
                    Add-Check ("scan:{0}:{1}:{2}" -f $rule.Name, $relative, $lineNumber) $false "Potential $($rule.Name) match; value intentionally not printed."
                }
            }
        }
    }

    if ($textFiles.Count -gt 0) {
        Add-Check "scan:text-files-covered" $true "$($textFiles.Count) compatibility evidence text file(s) scanned."
    } else {
        Add-Check "scan:text-files-covered" $false "No text files found under compatibility evidence directory."
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 compatibility evidence verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 compatibility evidence verification passed."
exit 0

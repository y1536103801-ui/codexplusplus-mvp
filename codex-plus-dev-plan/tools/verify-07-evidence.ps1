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
    throw "EvidenceDir is required. Example: -EvidenceDir codex-plus-dev-plan/test-runs/20260618-1230-e2e"
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
    Add-Check $Name ($text -match $Pattern) "$Pattern in $RelativeFile"
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
    "00-environment.md",
    "01-test-accounts.md",
    "02-contract-checks.md",
    "03-admin-setup.md",
    "04-client-api-e2e.md",
    "05-gateway-policy-e2e.md",
    "06-desktop-manager-e2e.md",
    "07-package-install-check.md",
    "08-compatibility-migration.md",
    "09-usage-events-audit.md",
    "10-rollback-notes.md",
    "11-defects.md",
    "12-release-gate-report.md"
)

Add-Check "evidence-dir-exists" (Test-Path -LiteralPath $EvidencePath -PathType Container) $EvidencePath

if (Test-Path -LiteralPath $EvidencePath -PathType Container) {
    $leaf = Split-Path -Leaf $EvidencePath
    Add-Check "evidence-dir-name" ($leaf -match "^\d{8}-\d{4}-e2e$") "Expected YYYYMMDD-HHMM-e2e; got $leaf."

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

    Add-Text-Check "12-release-gate-report.md" "(?im)^\s*Final recommendation\s*:\s*(go|go with accepted risks|no-go)\s*$" "release-report:final-recommendation"
    $releaseReportPath = Join-Path $EvidencePath "12-release-gate-report.md"
    if (Test-Path -LiteralPath $releaseReportPath -PathType Leaf) {
        $releaseReportText = Read-TextFile $releaseReportPath
        $recommendationMatch = [regex]::Match($releaseReportText, "(?im)^\s*Final recommendation\s*:\s*(go|go with accepted risks|no-go)\s*$")
        $level3Match = [regex]::Match($releaseReportText, "(?im)^\s*Level 3 result\s*:\s*(pass|fail)\s*$")
        Add-Check "release-report:final-recommendation-pass" ($recommendationMatch.Success -and $recommendationMatch.Groups[1].Value.ToLowerInvariant() -ne "no-go") "Final recommendation must be go or go with accepted risks."
        Add-Check "level3:pass-recorded" ($level3Match.Success -and $level3Match.Groups[1].Value.ToLowerInvariant() -eq "pass") "Level 3 result must be pass before Phase 1 go."
    } else {
        Add-Check "release-report:final-recommendation-pass" $false "12-release-gate-report.md is missing."
        Add-Check "level3:pass-recorded" $false "12-release-gate-report.md is missing."
    }
    Add-Text-Check "12-release-gate-report.md" "(?im)^\s*Level 3 result\s*:\s*(pass|fail)\s*$" "release-report:level3-result"
    Add-Text-Check "12-release-gate-report.md" "(?i)commands executed" "release-report:commands"
    Add-Text-Check "12-release-gate-report.md" "(?i)evidence links" "release-report:evidence-links"
    Add-Text-Check "12-release-gate-report.md" "(?i)remaining risks" "release-report:remaining-risks"
    Add-Text-Check "12-release-gate-report.md" "(?i)rollback" "release-report:rollback"

    $resultFiles = @(
        @{ File = "02-contract-checks.md"; Name = "contract-checks" },
        @{ File = "03-admin-setup.md"; Name = "admin-setup" },
        @{ File = "04-client-api-e2e.md"; Name = "client-api" },
        @{ File = "05-gateway-policy-e2e.md"; Name = "gateway-policy" },
        @{ File = "06-desktop-manager-e2e.md"; Name = "desktop-manager" },
        @{ File = "07-package-install-check.md"; Name = "package-install" },
        @{ File = "08-compatibility-migration.md"; Name = "compatibility-migration" },
        @{ File = "09-usage-events-audit.md"; Name = "usage-events-audit" },
        @{ File = "10-rollback-notes.md"; Name = "rollback-notes" }
    )
    foreach ($item in $resultFiles) {
        Add-Text-Check $item.File "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "$($item.Name):result-final"
        Add-Result-Pass-Check $item.File "$($item.Name):result-pass"
    }

    Add-Text-Check "00-environment.md" "(?i)Turnstile.*enabled" "environment:turnstile-enabled"
    Add-Text-Check "04-client-api-e2e.md" "(?i)browser handoff" "client-api:browser-handoff"
    Add-Text-Check "04-client-api-e2e.md" "/api/v1/auth/desktop/start" "client-api:browser-handoff-start-route"
    Add-Text-Check "04-client-api-e2e.md" "/api/v1/auth/desktop/complete" "client-api:browser-handoff-complete-route"
    Add-Text-Check "04-client-api-e2e.md" "/api/v1/auth/desktop/poll" "client-api:browser-handoff-poll-route"
    Add-Text-Check "04-client-api-e2e.md" "(?is)poll_token.*not.*authorize_url|authorize_url.*not.*poll_token" "client-api:poll-token-not-in-authorize-url"
    Add-Text-Check "04-client-api-e2e.md" "(?i)6 digit|six digit|six-digit" "client-api:verification-code-six-digit"
    Add-Text-Check "04-client-api-e2e.md" "(?i)bootstrap" "client-api:bootstrap"
    Add-Text-Check "04-client-api-e2e.md" "(?i)device" "client-api:device"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)user_active|active-user|active user" "gateway-policy:active-user"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)no entitlement|not_purchased|not purchased" "gateway-policy:no-entitlement"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)expired" "gateway-policy:expired"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)insufficient|low balance" "gateway-policy:insufficient-balance"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)revoked" "gateway-policy:revoked-device"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)unauthorized model|model_denied|model denied" "gateway-policy:unauthorized-model"
    Add-Text-Check "05-gateway-policy-e2e.md" "GATEWAY_POLICY_" "gateway-policy:structured-error-code"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)request[_ ]?id" "gateway-policy:request-id"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)service[_ ]?status" "gateway-policy:service-status"
    Add-Text-Check "05-gateway-policy-e2e.md" "(?i)ready-for-admin-audit|audit correlation" "gateway-policy:audit-correlation-ready"
    Add-Text-Check "06-desktop-manager-e2e.md" "(?im)^\s*Manager login result\s*:\s*pass\s*$|(?m)\|\s*Manager login\s*\|[^\r\n|]*\|\s*pass\s*\|" "desktop-manager:login"
    Add-Text-Check "06-desktop-manager-e2e.md" "(?im)^\s*Codex\+\+ Cloud provider result\s*:\s*pass\s*$|(?m)\|\s*Codex\+\+ Cloud[^\r\n|]*\|[^\r\n|]*\|\s*pass\s*\|" "desktop-manager:cloud-provider"
    Add-Text-Check "06-desktop-manager-e2e.md" "(?im)^\s*Manual provider preservation result\s*:\s*pass\s*$|(?m)\|\s*Manual providers?[^\r\n|]*\|[^\r\n|]*\|\s*pass\s*\|" "desktop-manager:manual-provider-preservation"
    Add-Text-Check "06-desktop-manager-e2e.md" "(?im)^\s*Codex launch result\s*:\s*pass\s*$|(?m)\|\s*Codex launch\s*\|[^\r\n|]*\|\s*pass\s*\|" "desktop-manager:codex-launch"
    Add-Text-Check "07-package-install-check.md" "(?i)Windows" "package-install:windows"
    Add-Text-Check "07-package-install-check.md" "(?i)macOS x64|macOS.*x64|x64.*DMG" "package-install:macos-x64"
    Add-Text-Check "07-package-install-check.md" "(?i)macOS arm64|arm64.*DMG" "package-install:macos-arm64"
    Add-Text-Check "07-package-install-check.md" "(?i)missing Codex|first-run|first run" "package-install:first-run"
    Add-Text-Check "08-compatibility-migration.md" "(?i)manual providers?" "compatibility-migration:manual-providers"
    Add-Text-Check "08-compatibility-migration.md" "(?i)logout" "compatibility-migration:logout"
    Add-Text-Check "08-compatibility-migration.md" "(?i)provider sync|rollback" "compatibility-migration:sync-or-rollback"
    Add-Text-Check "09-usage-events-audit.md" "(?i)usage" "audit:usage"
    Add-Text-Check "09-usage-events-audit.md" "(?i)admin audit" "audit:admin-audit"
    Add-Text-Check "09-usage-events-audit.md" "(?i)success" "audit:success-path"
    Add-Text-Check "09-usage-events-audit.md" "(?i)rejection|reject" "audit:rejection"
    Add-Text-Check "09-usage-events-audit.md" "(?i)gateway_policy_rejected" "audit:gateway-policy-rejected-event"
    Add-Text-Check "09-usage-events-audit.md" "GATEWAY_POLICY_" "audit:structured-error-code"
    Add-Text-Check "09-usage-events-audit.md" "(?i)request[_ ]?id" "audit:request-id"
    Add-Text-Check "09-usage-events-audit.md" "(?im)^\s*Gateway request[_ ]?id correlation\s*:\s*pass\s*$|(?m)\|\s*matched\s*\|" "audit:gateway-request-id-correlation"
    Add-Text-Check "09-usage-events-audit.md" "(?i)config[_ ]?version" "audit:config-version"
    Add-Text-Check "09-usage-events-audit.md" "(?i)redaction_applied|redact|redaction|redacted" "audit:redaction-note"
    Add-Text-Check "10-rollback-notes.md" "(?i)config rollback" "rollback:config"
    Add-Text-Check "10-rollback-notes.md" "(?i)backend rollback" "rollback:backend"
    Add-Text-Check "10-rollback-notes.md" "(?i)desktop rollback" "rollback:desktop"
    Add-Text-Check "10-rollback-notes.md" "(?i)entitlement" "rollback:entitlement"
    Add-Text-Check "10-rollback-notes.md" "(?i)provider write" "rollback:provider-write"
    Add-Text-Check "11-defects.md" "(?i)\bP0\b|\bP1\b|\bP2\b|\bP3\b" "defects:severity"

    $textExtensions = @(".csv", ".html", ".json", ".log", ".md", ".txt", ".yaml", ".yml")
    $textFiles = Get-ChildItem -LiteralPath $EvidencePath -Recurse -File |
        Where-Object { $textExtensions -contains $_.Extension.ToLowerInvariant() }

    $leakRules = @(
        @{ Name = "api-key"; Pattern = "(?i)\b(sk-[a-z0-9][a-z0-9_-]{18,}|sk-proj-[a-z0-9][a-z0-9_-]{18,}|sk-ant-[a-z0-9][a-z0-9_-]{18,})\b" },
        @{ Name = "authorization-header"; Pattern = "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}" },
        @{ Name = "jwt"; Pattern = "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b" },
        @{ Name = "token-field"; Pattern = "(?i)(access_token|refresh_token|id_token|poll_token|session_token|api_key|upstream_key)\s*[:=]\s*[""']?(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b)[A-Za-z0-9._~+/=-]{12,}" },
        @{ Name = "token-query"; Pattern = "(?i)(access_token|refresh_token|poll_token|session_token)=((?!redacted)[A-Za-z0-9._~%+-]{8,})" },
        @{ Name = "unfinished-placeholder"; Pattern = "(?i)\b(TODO|TBD|PLACEHOLDER|NOT_EXECUTED|FILL_ME|pending)\b" },
        @{ Name = "pending-scaffold"; Pattern = "(?i)(scaffold only|evidence pending until executed|No E2E run has been executed)" }
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
        Add-Check "scan:text-files-covered" $true "$($textFiles.Count) text evidence file(s) scanned."
    } else {
        Add-Check "scan:text-files-covered" $false "No text files found under evidence directory."
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 evidence verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 evidence verification passed."
exit 0

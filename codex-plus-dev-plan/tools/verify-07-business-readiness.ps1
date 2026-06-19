param(
    [string]$Root,
    [string]$EvidenceDir,
    [string]$SourceDocsRoot
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    throw "EvidenceDir is required. Example: -EvidenceDir codex-plus-dev-plan/test-runs/20260618-1911-business"
}

if ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidencePath = [System.IO.Path]::GetFullPath($EvidenceDir)
} else {
    $EvidencePath = [System.IO.Path]::GetFullPath((Join-Path $Root $EvidenceDir))
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

function Get-Relative-Path {
    param([string]$Path)
    $base = [System.IO.Path]::GetFullPath($EvidencePath).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath($Path)
    if ($full.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $full.Substring($base.Length).TrimStart('\', '/')
    }
    return $full
}

function Add-Text-Check {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Name,
        [string]$Detail
    )
    Add-Check $Name ([bool]($Text -match $Pattern)) $Detail
}

function Resolve-SourceDocsRoot {
    param([string]$CandidateRoot)

    if ([string]::IsNullOrWhiteSpace($CandidateRoot)) {
        $planCandidate = Join-Path $Root "codex-plus-dev-plan"
        if (Test-Path -LiteralPath $planCandidate -PathType Container) {
            return [System.IO.Path]::GetFullPath($planCandidate)
        }
        return [System.IO.Path]::GetFullPath($Root)
    }

    if ([System.IO.Path]::IsPathRooted($CandidateRoot)) {
        return [System.IO.Path]::GetFullPath($CandidateRoot)
    }

    return [System.IO.Path]::GetFullPath((Join-Path $Root $CandidateRoot))
}

function Test-OptionalSourceLine {
    param([string]$Line)
    return [bool]($Line -match "(?i)\|\s*optional\s*\|" -or $Line -match "(?i)\|\s*can defer\s*\|")
}

function Add-SourceDoc-UnresolvedScan {
    param(
        [string]$SourceRoot,
        [string]$DocumentName,
        [object[]]$Rules
    )

    $sourcePath = Join-Path $SourceRoot $DocumentName
    $existsCheckName = "source-doc-exists:{0}" -f $DocumentName
    Add-Check $existsCheckName (Test-Path -LiteralPath $sourcePath -PathType Leaf) $DocumentName
    if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
        return
    }

    $lines = Get-Content -LiteralPath $sourcePath
    $inFence = $false
    $matchCount = 0
    for ($index = 0; $index -lt $lines.Count; $index++) {
        $line = $lines[$index]
        $lineNumber = $index + 1
        if ($line -match '^\s*```') {
            $inFence = -not $inFence
            continue
        }
        $optionalSourceLine = Test-OptionalSourceLine $line
        if ($inFence -or $optionalSourceLine) {
            continue
        }
        foreach ($rule in $Rules) {
            if ($line -match $rule.Pattern) {
                $matchCount += 1
                $checkName = "source-unresolved:{0}:{1}:{2}" -f $rule.Name, $DocumentName, $lineNumber
                $detail = "Unresolved source-doc marker in {0} line {1} ({2}); value intentionally not printed." -f $DocumentName, $lineNumber, $rule.Name
                Add-Check $checkName $false $detail
            }
        }
    }

    $scanCheckName = "source-doc-scan:{0}" -f $DocumentName
    Add-Check $scanCheckName ($matchCount -eq 0) "$DocumentName required launch fields must not contain unresolved markers."
}

Add-Check "business-evidence-dir-exists" (Test-Path -LiteralPath $EvidencePath -PathType Container) $EvidencePath

if (Test-Path -LiteralPath $EvidencePath -PathType Container) {
    $leaf = Split-Path -Leaf $EvidencePath
    Add-Check "business-evidence-dir-name" ($leaf -match "^\d{8}-\d{4}-business$") "Expected YYYYMMDD-HHMM-business; got $leaf."

    $readinessPath = Join-Path $EvidencePath "11-business-readiness.md"
    Add-Check "file-exists:11-business-readiness.md" (Test-Path -LiteralPath $readinessPath -PathType Leaf) "11-business-readiness.md"

    if (Test-Path -LiteralPath $readinessPath -PathType Leaf) {
        $item = Get-Item -LiteralPath $readinessPath
        Add-Check "file-nonempty:11-business-readiness.md" ($item.Length -gt 0) "11-business-readiness.md length=$($item.Length)"
        $text = Get-Content -Raw -LiteralPath $readinessPath

        Add-Text-Check $text "(?im)^\s*Business readiness result\s*:\s*(pass|fail)\s*$" "business-result:final" "Business readiness result must be pass or fail."
        $businessResultMatch = [regex]::Match($text, "(?im)^\s*Business readiness result\s*:\s*(pass|fail)\s*$")
        Add-Check "business-result:pass" ($businessResultMatch.Success -and $businessResultMatch.Groups[1].Value.ToLowerInvariant() -eq "pass") "Business readiness result must be pass."
        Add-Text-Check $text "(?i)PRODUCTION-ENVIRONMENT-MATRIX\.md" "source:production-environment" "Production environment source must be referenced."
        Add-Text-Check $text "(?i)BUSINESS-CONFIG-DECISION-TABLE\.md" "source:business-config" "Business config source must be referenced."
        Add-Text-Check $text "(?i)SERVER-SIZING-AND-SCALING-GUIDE\.md" "source:server-sizing" "Server sizing source must be referenced."
        Add-Text-Check $text "(?i)DEPLOYMENT-AUTOMATION-RUNBOOK\.md" "source:deployment" "Deployment automation source must be referenced."
        Add-Text-Check $text "(?i)SECURITY-REVIEW-PLAN\.md" "source:security" "Security review source must be referenced."
        Add-Text-Check $text "(?i)COMPLIANCE-PRIVACY-LEGAL-CHECKLIST\.md" "source:compliance" "Compliance/privacy/legal source must be referenced."
        Add-Text-Check $text "(?i)OBSERVABILITY-SLO-ALERTING-PLAN\.md" "source:observability" "Observability/SLO source must be referenced."
        Add-Text-Check $text "(?i)COST-CONTROL-AND-ABUSE-RUNBOOK\.md" "source:cost-abuse" "Cost control and abuse source must be referenced."
        Add-Text-Check $text "(?i)SUPPORT-OPERATIONS-RUNBOOK\.md" "source:support" "Support operations source must be referenced."

        $requiredGates = @(
            "production environment values",
            "business config decisions",
            "server sizing and scaling",
            "deployment automation backup rollback healthcheck",
            "security review P0/P1/P2",
            "compliance privacy legal payment provider terms refund policy",
            "observability SLO dashboards alert routing",
            "cost control abuse spend caps emergency shutoff",
            "support operations paid-user support refund compensation admin recovery",
            "human business or legal decisions"
        )

        foreach ($gate in $requiredGates) {
            $escapedGate = [regex]::Escape($gate)
            Add-Text-Check $text ("(?im)^\|\s*" + $escapedGate + "\s*\|\s*(pass|fail|deferred)\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|") "gate:$gate" "$gate gate row must include status, owner, evidence, risk, mitigation and latest decision date."
            $gateMatch = [regex]::Match($text, ("(?im)^\|\s*" + $escapedGate + "\s*\|\s*(pass|fail|deferred)\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|\s*[^|\r\n]+\s*\|"))
            Add-Check "gate:${gate}:pass" ($gateMatch.Success -and $gateMatch.Groups[1].Value.ToLowerInvariant() -eq "pass") "$gate gate status must be pass."
        }

        Add-Text-Check $text "(?i)Open P0/P1 security items\s*:\s*none" "no-go-scan:no-open-p0-p1" "Open P0/P1 security items must be none."
        Add-Text-Check $text "(?i)Missing production value\s*:\s*none" "no-go-scan:production-values" "Missing production values must be none."
        Add-Text-Check $text "(?i)Missing first-launch package/model/quota/payment/cost decision\s*:\s*none" "no-go-scan:business-decisions" "First-launch business decisions must be complete."
        Add-Text-Check $text "(?i)Privacy policy, terms, refund policy and provider terms\s*:\s*defined" "no-go-scan:legal-terms" "Legal, privacy, refund and provider terms must be defined."
        Add-Text-Check $text "(?i)Dashboard, SLO and alert routing\s*:\s*defined" "no-go-scan:observability" "Dashboard, SLO and alert routing must be defined."
        Add-Text-Check $text "(?i)Cost cap and emergency stop\s*:\s*defined" "no-go-scan:cost-emergency" "Cost cap and emergency stop must be defined."
        Add-Text-Check $text "(?i)Paid-user support and entitlement correction\s*:\s*defined" "no-go-scan:support" "Paid-user support and entitlement correction must be defined."
        Add-Text-Check $text "(?i)Release Boundary" "release-boundary" "Release boundary must be present."

        $sourceDocsPath = Resolve-SourceDocsRoot $SourceDocsRoot
        Add-Check "source-doc-root-exists" (Test-Path -LiteralPath $sourceDocsPath -PathType Container) $sourceDocsPath
        if (Test-Path -LiteralPath $sourceDocsPath -PathType Container) {
            Add-SourceDoc-UnresolvedScan $sourceDocsPath "PRODUCTION-ENVIRONMENT-MATRIX.md" @(
                @{ Name = "state-not-final"; Pattern = "(?i)^\s*-\s*State:\s*(draft|planning)\s*$" },
                @{ Name = "placeholder"; Pattern = "(?i)\b(TBD|TODO|PLACEHOLDER|NOT_EXECUTED|FILL_ME)\b" },
                @{ Name = "unresolved-status"; Pattern = "(?i)\b(pending|waiting|unknown|not yet|planned)\b" }
            )
            Add-SourceDoc-UnresolvedScan $sourceDocsPath "BUSINESS-CONFIG-DECISION-TABLE.md" @(
                @{ Name = "state-not-final"; Pattern = "(?i)^\s*-\s*State:\s*(draft|planning)\s*$" },
                @{ Name = "placeholder"; Pattern = "(?i)\b(TBD|TODO|PLACEHOLDER|NOT_EXECUTED|FILL_ME)\b" },
                @{ Name = "unresolved-decision"; Pattern = "(?i)\b(decision needed|domain needed|user decision needed)\b" }
            )
        }

        $leakRules = @(
            @{ Name = "api-key"; Pattern = "(?i)\b(sk-[a-z0-9][a-z0-9_-]{18,}|sk-proj-[a-z0-9][a-z0-9_-]{18,}|sk-ant-[a-z0-9][a-z0-9_-]{18,})\b" },
            @{ Name = "authorization-header"; Pattern = "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}" },
            @{ Name = "jwt"; Pattern = "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b" },
            @{ Name = "unfinished-placeholder"; Pattern = "(?i)\b(TODO|TBD|PLACEHOLDER|NOT_EXECUTED|FILL_ME|pending)\b" },
            @{ Name = "scaffold-only"; Pattern = "(?i)scaffold only|not release evidence" }
        )

        $lines = Get-Content -LiteralPath $readinessPath
        for ($index = 0; $index -lt $lines.Count; $index++) {
            $lineNumber = $index + 1
            foreach ($rule in $leakRules) {
                if ($lines[$index] -match $rule.Pattern) {
                    Add-Check ("scan:{0}:{1}:{2}" -f $rule.Name, (Get-Relative-Path $readinessPath), $lineNumber) $false "Potential $($rule.Name) match; value intentionally not printed."
                }
            }
        }
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 business readiness verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 business readiness verification passed."
exit 0

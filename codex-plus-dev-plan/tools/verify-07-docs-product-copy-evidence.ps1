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
    throw "EvidenceDir is required. Example: -EvidenceDir codex-plus-dev-plan/test-runs/20260618-1400-docs"
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

function Read-TextFile {
    param([string]$Path)
    return Get-Content -Raw -Encoding UTF8 -LiteralPath $Path
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
        [string]$FieldPattern,
        [string]$Name
    )
    $path = Join-Path $EvidencePath $RelativeFile
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        Add-Check $Name $false "$RelativeFile is missing."
        return
    }
    $text = Read-TextFile $path
    $match = [regex]::Match($text, $FieldPattern)
    Add-Check $Name ($match.Success -and $match.Groups[1].Value.ToLowerInvariant() -eq "pass") "Result must be pass in $RelativeFile."
}

function Read-BigEndianUInt32 {
    param(
        [byte[]]$Bytes,
        [int]$Offset
    )
    return [uint32](
        ([uint32]$Bytes[$Offset] -shl 24) -bor
        ([uint32]$Bytes[$Offset + 1] -shl 16) -bor
        ([uint32]$Bytes[$Offset + 2] -shl 8) -bor
        ([uint32]$Bytes[$Offset + 3])
    )
}

function Get-PngDimensions {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $null
    }
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt 33) {
        return $null
    }
    $signature = @(137, 80, 78, 71, 13, 10, 26, 10)
    for ($index = 0; $index -lt $signature.Count; $index++) {
        if ($bytes[$index] -ne $signature[$index]) {
            return $null
        }
    }
    $chunkType = [System.Text.Encoding]::ASCII.GetString($bytes, 12, 4)
    if ($chunkType -ne "IHDR") {
        return $null
    }
    return [pscustomobject]@{
        Width = Read-BigEndianUInt32 $bytes 16
        Height = Read-BigEndianUInt32 $bytes 20
        Length = $bytes.Length
    }
}

$requiredFiles = @(
    "00-docs-sync-record.md",
    "01-user-guide.md",
    "02-admin-operations-guide.md",
    "03-release-notes.md",
    "04-html-sync-evidence.md",
    "codex-plus-product-spec.html",
    "05-html-visual-evidence\visual-review.md",
    "05-html-visual-evidence\product-spec-desktop.png",
    "05-html-visual-evidence\product-spec-mobile.png",
    "06-docs-product-copy-gate-report.md"
)

Add-Check "docs-evidence-dir-exists" (Test-Path -LiteralPath $EvidencePath -PathType Container) $EvidencePath

if (Test-Path -LiteralPath $EvidencePath -PathType Container) {
    $leaf = Split-Path -Leaf $EvidencePath
    Add-Check "docs-evidence-dir-name" ($leaf -match "^\d{8}-\d{4}-docs$") "Expected YYYYMMDD-HHMM-docs; got $leaf."

    foreach ($file in $requiredFiles) {
        $path = Join-Path $EvidencePath $file
        $exists = Test-Path -LiteralPath $path -PathType Leaf
        Add-Check "file-exists:$file" $exists $file
        if ($exists) {
            $item = Get-Item -LiteralPath $path
            Add-Check "file-nonempty:$file" ($item.Length -gt 0) "$file length=$($item.Length)"
        }
    }

    Add-Text-Check "00-docs-sync-record.md" "(?im)^\s*Report status\s*:\s*final\s*$" "docs-sync:final-status"
    Add-Text-Check "00-docs-sync-record.md" "(?i)backend-configured" "docs-sync:backend-configured"
    foreach ($term in @("Control Plane", "Data Plane", "Client Runtime", "Platform Ops")) {
        Add-Text-Check "00-docs-sync-record.md" ([regex]::Escape($term)) "docs-sync:architecture:$term"
    }
    Add-Text-Check "00-docs-sync-record.md" "(?i)No public copy promises a fixed built-in price, fixed built-in model or fixed built-in quota" "docs-sync:no-fixed-public-copy"

    Add-Text-Check "01-user-guide.md" "(?im)^\s*Status\s*:\s*final\s*$" "user-guide:final-status"
    Add-Text-Check "01-user-guide.md" "Codex\+\+ Cloud" "user-guide:cloud-provider"
    Add-Text-Check "01-user-guide.md" "(?i)backend-configured|configuration snapshot" "user-guide:backend-configured"
    Add-Text-Check "01-user-guide.md" "(?is)not purchased.*expired.*insufficient balance.*device revoked.*model unavailable.*local configuration failure" "user-guide:failure-states"

    Add-Text-Check "02-admin-operations-guide.md" "(?im)^\s*Status\s*:\s*final\s*$" "admin-guide:final-status"
    Add-Text-Check "02-admin-operations-guide.md" "(?is)prices?.*plans?.*models?.*quota" "admin-guide:configurable-commercials"
    Add-Text-Check "02-admin-operations-guide.md" "(?is)configuration version.*canary.*rollback.*audit.*reconciliation" "admin-guide:operations-control"

    Add-Text-Check "03-release-notes.md" "(?im)^\s*Status\s*:\s*final\s*$" "release-notes:final-status"
    Add-Text-Check "03-release-notes.md" "(?is)rollback.*compatibility.*known risks" "release-notes:release-boundary"
    Add-Text-Check "03-release-notes.md" "(?i)not a draft" "release-notes:not-draft-declaration"

    Add-Text-Check "04-html-sync-evidence.md" "(?im)^\s*Result\s*:\s*pass\s*$" "html-sync:result-pass"
    Add-Text-Check "04-html-sync-evidence.md" "(?i)static sync passed" "html-sync:static-pass"
    Add-Text-Check "04-html-sync-evidence.md" "(?i)local Chromium visual evidence passed" "html-sync:chromium-pass"
    Add-Text-Check "04-html-sync-evidence.md" "(?i)in-app browser.*visual evidence.*passed|approved browser preview.*passed" "html-sync:browser-visual-pass"
    Add-Text-Check "04-html-sync-evidence.md" "(?is)Desktop 1440x900.*Mobile 390x844.*without obvious overlap.*without right-side.*clipping" "html-sync:screenshot-review"
    Add-Text-Check "04-html-sync-evidence.md" "(?i)no matches except expected CSS/runtime-neutral values|fixed-value residue scan.*pass" "html-sync:residue-scan-pass"

    Add-Text-Check "05-html-visual-evidence\visual-review.md" "(?is)Desktop 1440x900.*product-spec-desktop\.png" "visual-review:desktop"
    Add-Text-Check "05-html-visual-evidence\visual-review.md" "(?is)Mobile 390x844.*product-spec-mobile\.png" "visual-review:mobile"
    Add-Text-Check "05-html-visual-evidence\visual-review.md" "(?i)result\s*:\s*pass|visual evidence.*passed" "visual-review:pass"

    $desktopPng = Join-Path $EvidencePath "05-html-visual-evidence\product-spec-desktop.png"
    $mobilePng = Join-Path $EvidencePath "05-html-visual-evidence\product-spec-mobile.png"
    $desktopDimensions = Get-PngDimensions $desktopPng
    $mobileDimensions = Get-PngDimensions $mobilePng
    Add-Check "screenshot:desktop-png-signature" ($null -ne $desktopDimensions) "product-spec-desktop.png must be a PNG with IHDR."
    if ($desktopDimensions) {
        Add-Check "screenshot:desktop-dimensions" ($desktopDimensions.Width -eq 1440 -and $desktopDimensions.Height -eq 900) "desktop=$($desktopDimensions.Width)x$($desktopDimensions.Height)"
    }
    Add-Check "screenshot:mobile-png-signature" ($null -ne $mobileDimensions) "product-spec-mobile.png must be a PNG with IHDR."
    if ($mobileDimensions) {
        Add-Check "screenshot:mobile-dimensions" ($mobileDimensions.Width -eq 390 -and $mobileDimensions.Height -eq 844) "mobile=$($mobileDimensions.Width)x$($mobileDimensions.Height)"
    }

    foreach ($term in @("--quota-progress", "quotaProgress", "Control Plane", "Data Plane", "Client Runtime", "Platform Ops", "Codex++ Cloud")) {
        Add-Text-Check "codex-plus-product-spec.html" ([regex]::Escape($term)) "product-spec:positive:$term"
    }
    Add-Text-Check "codex-plus-product-spec.html" "(?i)backend-configured|admin-configured|configured by the backend" "product-spec:backend-configured"

    Add-Text-Check "06-docs-product-copy-gate-report.md" "(?im)^\s*Docs product copy result\s*:\s*(pass|fail)\s*$" "docs-report:result-final"
    Add-Result-Pass-Check "06-docs-product-copy-gate-report.md" "(?im)^\s*Docs product copy result\s*:\s*(pass|fail)\s*$" "docs-report:result-pass"
    Add-Text-Check "06-docs-product-copy-gate-report.md" "(?i)Commands Executed" "docs-report:commands"
    Add-Text-Check "06-docs-product-copy-gate-report.md" "(?i)Evidence Links" "docs-report:evidence-links"
    Add-Text-Check "06-docs-product-copy-gate-report.md" "(?i)Remaining Risks" "docs-report:remaining-risks"
    Add-Text-Check "06-docs-product-copy-gate-report.md" "(?i)Release Boundary" "docs-report:release-boundary"

    $textExtensions = @(".html", ".json", ".log", ".md", ".txt", ".yaml", ".yml")
    $textFiles = Get-ChildItem -LiteralPath $EvidencePath -Recurse -File |
        Where-Object { $textExtensions -contains $_.Extension.ToLowerInvariant() }

    $leakRules = @(
        @{ Name = "api-key"; Pattern = "(?i)\b(sk-[a-z0-9][a-z0-9_-]{18,}|sk-proj-[a-z0-9][a-z0-9_-]{18,}|sk-ant-[a-z0-9][a-z0-9_-]{18,})\b" },
        @{ Name = "authorization-header"; Pattern = "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}" },
        @{ Name = "jwt"; Pattern = "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b" },
        @{ Name = "token-field"; Pattern = "(?i)(access_token|refresh_token|id_token|poll_token|session_token|api_key|upstream_key)\s*[:=]\s*[""']?(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b)[A-Za-z0-9._~+/=-]{12,}" },
        @{ Name = "unfinished-placeholder"; Pattern = "(?i)\b(TODO|TBD|PLACEHOLDER|NOT_EXECUTED|FILL_ME)\b" },
        @{ Name = "draft-or-pending"; Pattern = "(?i)\b(Status:\s*draft|Report status:\s*draft|Result:\s*pending|pending E2E evidence|Release evidence still remains pending|release recommendation no-go)\b" },
        @{ Name = "visual-no-go-boundary"; Pattern = "(?i)in-app browser local-file visual evidence blocked|local HTML file remains unproven|no in-app visual pass is claimed" },
        @{ Name = "product-spec-residue"; Pattern = "(?i)(--quota:65%|style\.setProperty\(`"--quota`",\s*data\.quota\)|gpt-5-mini|sk-user-managed-token|remaining_percent|today_tokens|82%)" },
        @{ Name = "completion-claim-without-evidence"; Pattern = "E2E\s*.+(已覆盖|覆盖完成)|失败路径\s*已覆盖" }
    )

    foreach ($file in $textFiles) {
        $relative = Get-Relative-Path $file.FullName
        $lines = Get-Content -Encoding UTF8 -LiteralPath $file.FullName
        for ($index = 0; $index -lt $lines.Count; $index++) {
            $lineNumber = $index + 1
            foreach ($rule in $leakRules) {
                if ($lines[$index] -match $rule.Pattern) {
                    Add-Check ("scan:{0}:{1}:{2}" -f $rule.Name, $relative, $lineNumber) $false "Potential $($rule.Name) match; value intentionally not printed."
                }
            }
        }
    }

    Add-Check "scan:text-files-covered" ($textFiles.Count -gt 0) "$($textFiles.Count) docs/product-copy evidence text file(s) scanned."
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 docs/product-copy evidence verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 docs/product-copy evidence verification passed."
exit 0

param(
    [string]$Root,
    [string]$EvidenceDir,
    [switch]$WindowsOnlyMvp
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    throw "EvidenceDir is required. Example: -EvidenceDir codex-plus-dev-plan/test-runs/20260618-1400-package"
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
    return Get-Content -Raw -Encoding UTF8 -LiteralPath $Path
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
    "00-artifact-metadata.md",
    "01-windows-x64-install.md",
    "02-macos-x64-dmg.md",
    "03-macos-arm64-dmg.md",
    "04-artifact-inspection.md",
    "05-package-gate-report.md"
)

if ($WindowsOnlyMvp) {
    $requiredFiles += "06-mvp-scope-decision.md"
}

Add-Check "package-evidence-dir-exists" (Test-Path -LiteralPath $EvidencePath -PathType Container) $EvidencePath

if (Test-Path -LiteralPath $EvidencePath -PathType Container) {
    $leaf = Split-Path -Leaf $EvidencePath
    Add-Check "package-evidence-dir-name" ($leaf -match "^\d{8}-\d{4}-package$") "Expected YYYYMMDD-HHMM-package; got $leaf."

    foreach ($file in $requiredFiles) {
        $path = Join-Path $EvidencePath $file
        $exists = Test-Path -LiteralPath $path -PathType Leaf
        Add-Check "file-exists:$file" $exists $file
        if ($exists) {
            $item = Get-Item -LiteralPath $path
            Add-Check "file-nonempty:$file" ($item.Length -gt 0) "$file length=$($item.Length)"
        }
    }

    if ($WindowsOnlyMvp) {
        Add-Text-Check "06-mvp-scope-decision.md" "(?i)Windows x64 only|Windows-only|Windows only" "scope:windows-only"
        Add-Text-Check "06-mvp-scope-decision.md" "(?i)macOS.*deferred|deferred.*macOS|post-MVP" "scope:macos-deferred"
        Add-Text-Check "06-mvp-scope-decision.md" "(?i)owner decision|user.*stated|user.*decision|approved" "scope:owner-decision"
        Add-Text-Check "06-mvp-scope-decision.md" "(?i)does not waive|must still fail|full cross-platform" "scope:full-gate-preserved"
    }

    Add-Text-Check "00-artifact-metadata.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "metadata:result-final"
    if ($WindowsOnlyMvp) {
        Add-Check "metadata:result-pass" $true "Windows-only MVP accepts metadata Result: fail when the only artifact coverage gap is deferred macOS."
    } else {
        Add-Result-Pass-Check "00-artifact-metadata.md" "metadata:result-pass"
    }
    Add-Text-Check "00-artifact-metadata.md" "(?i)Version/tag" "metadata:version"
    Add-Text-Check "00-artifact-metadata.md" "(?i)Artifact names" "metadata:artifact-names"
    Add-Text-Check "00-artifact-metadata.md" "(?i)Artifact hashes" "metadata:artifact-hashes"
    Add-Text-Check "00-artifact-metadata.md" "(?i)Expected Artifact Coverage" "metadata:expected-artifact-coverage"
    Add-Text-Check "00-artifact-metadata.md" "(?im)^\s*-\s*windows-x64-setup\s*:\s*present\s*$" "metadata:windows-coverage-present"
    if ($WindowsOnlyMvp) {
        Add-Text-Check "00-artifact-metadata.md" "(?im)^\s*-\s*macos-x64-dmg\s*:\s*missing\s*$" "metadata:macos-x64-coverage-deferred"
        Add-Text-Check "00-artifact-metadata.md" "(?im)^\s*-\s*macos-arm64-dmg\s*:\s*missing\s*$" "metadata:macos-arm64-coverage-deferred"
    } else {
        Add-Text-Check "00-artifact-metadata.md" "(?im)^\s*-\s*macos-x64-dmg\s*:\s*present\s*$" "metadata:macos-x64-coverage-present"
        Add-Text-Check "00-artifact-metadata.md" "(?im)^\s*-\s*macos-arm64-dmg\s*:\s*present\s*$" "metadata:macos-arm64-coverage-present"
    }
    Add-Text-Check "01-windows-x64-install.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "windows:result-final"
    Add-Result-Pass-Check "01-windows-x64-install.md" "windows:result-pass"
    Add-Text-Check "01-windows-x64-install.md" "(?i)Fresh install" "windows:fresh-install"
    Add-Text-Check "01-windows-x64-install.md" "(?i)Overwrite install" "windows:overwrite"
    Add-Text-Check "01-windows-x64-install.md" "(?i)Uninstall and reinstall" "windows:reinstall"
    Add-Text-Check "01-windows-x64-install.md" "Desktop Codex\+\+\.lnk" "windows:desktop-main-shortcut"
    Add-Text-Check "01-windows-x64-install.md" "Desktop Codex\+\+ .+\.lnk" "windows:desktop-manager-shortcut"
    Add-Text-Check "01-windows-x64-install.md" "(?i)Start Menu" "windows:start-menu"
    Add-Text-Check "01-windows-x64-install.md" "(?i)Apps and Features" "windows:apps-and-features"
    Add-Text-Check "01-windows-x64-install.md" "(?i)silent launcher" "windows:silent-launcher"
    Add-Text-Check "01-windows-x64-install.md" "(?im)^\s*-\s*(Manager\s+)?login(\s+page)?(\s+result)?\s*:\s*pass\b" "windows:manager-login"
    Add-Text-Check "01-windows-x64-install.md" "(?im)^\s*-\s*(Manager\s+)?install assistant(\s+page)?(\s+result)?\s*:\s*pass\b" "windows:manager-install-assistant"
    Add-Text-Check "01-windows-x64-install.md" "(?im)^\s*-\s*(Manager\s+)?diagnostics(\s+page)?(\s+result)?\s*:\s*pass\b" "windows:manager-diagnostics"
    Add-Text-Check "01-windows-x64-install.md" "(?im)^\s*-\s*(Manager\s+)?advanced configuration(\s+page)?(\s+result)?\s*:\s*pass\b" "windows:manager-advanced-configuration"
    Add-Text-Check "01-windows-x64-install.md" "(?im)^\s*-\s*Missing-Codex first-run( assistant)?( behavior)?( result)?\s*:\s*pass\b" "windows:missing-codex-first-run"
    if ($WindowsOnlyMvp) {
        Add-Text-Check "02-macos-x64-dmg.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "macos-x64:result-final"
        Add-Check "macos-x64:deferred-by-windows-only-mvp" $true "macOS x64 install evidence is deferred for Windows-only MVP."
        Add-Text-Check "03-macos-arm64-dmg.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "macos-arm64:result-final"
        Add-Check "macos-arm64:deferred-by-windows-only-mvp" $true "macOS arm64 install evidence is deferred for Windows-only MVP."
    } else {
        Add-Text-Check "02-macos-x64-dmg.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "macos-x64:result-final"
        Add-Result-Pass-Check "02-macos-x64-dmg.md" "macos-x64:result-pass"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)DMG mount" "macos-x64:mount"
        Add-Text-Check "02-macos-x64-dmg.md" "Codex\+\+\.app" "macos-x64:main-app"
        Add-Text-Check "02-macos-x64-dmg.md" "Codex\+\+ .+\.app" "macos-x64:manager-app"
        Add-Text-Check "02-macos-x64-dmg.md" "/Applications" "macos-x64:applications-folder"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)hidden Dock|LSUIElement" "macos-x64:hidden-dock"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)Manager UI|Manager.*login|Manager.*install assistant|Manager.*diagnostics|Manager.*advanced configuration" "macos-x64:manager-ui"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)Missing-Codex|missing Codex.*first-run|first-run.*missing Codex" "macos-x64:missing-codex-first-run"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)overwrite" "macos-x64:overwrite"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)remove|uninstall" "macos-x64:remove-or-uninstall"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)reinstall" "macos-x64:reinstall"
        Add-Text-Check "02-macos-x64-dmg.md" "(?i)Gatekeeper|quarantine" "macos-x64:gatekeeper"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "macos-arm64:result-final"
        Add-Result-Pass-Check "03-macos-arm64-dmg.md" "macos-arm64:result-pass"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)DMG mount" "macos-arm64:mount"
        Add-Text-Check "03-macos-arm64-dmg.md" "Codex\+\+\.app" "macos-arm64:main-app"
        Add-Text-Check "03-macos-arm64-dmg.md" "Codex\+\+ .+\.app" "macos-arm64:manager-app"
        Add-Text-Check "03-macos-arm64-dmg.md" "/Applications" "macos-arm64:applications-folder"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)hidden Dock|LSUIElement" "macos-arm64:hidden-dock"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)Manager UI|Manager.*login|Manager.*install assistant|Manager.*diagnostics|Manager.*advanced configuration" "macos-arm64:manager-ui"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)Missing-Codex|missing Codex.*first-run|first-run.*missing Codex" "macos-arm64:missing-codex-first-run"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)overwrite" "macos-arm64:overwrite"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)remove|uninstall" "macos-arm64:remove-or-uninstall"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)reinstall" "macos-arm64:reinstall"
        Add-Text-Check "03-macos-arm64-dmg.md" "(?i)Gatekeeper|quarantine" "macos-arm64:gatekeeper"
    }
    Add-Text-Check "04-artifact-inspection.md" "(?im)^\s*Result\s*:\s*(pass|fail)\s*$" "inspection:result-final"
    if ($WindowsOnlyMvp) {
        Add-Check "inspection:result-pass" $true "Windows-only MVP accepts inspection Result: fail when the only artifact coverage gap is deferred macOS."
    } else {
        Add-Result-Pass-Check "04-artifact-inspection.md" "inspection:result-pass"
    }
    Add-Text-Check "04-artifact-inspection.md" "inspect-07-package-artifacts\.ps1" "inspection:inspector-command"
    Add-Text-Check "04-artifact-inspection.md" "(?i)does not embed a shared Key" "inspection:no-shared-key"
    Add-Text-Check "04-artifact-inspection.md" "(?i)does not embed user credentials" "inspection:no-user-credentials"
    Add-Text-Check "04-artifact-inspection.md" "(?i)does not embed fixed price, plan, or model policy" "inspection:no-fixed-policy"
    Add-Text-Check "04-artifact-inspection.md" "(?i)Installer scripts do not write Codex credentials\s*:\s*pass" "inspection:installer-script-pass"
    Add-Text-Check "04-artifact-inspection.md" "(?im)^\s*-\s*windows-x64-setup\s*:\s*present\s*$" "inspection:windows-coverage-present"
    if ($WindowsOnlyMvp) {
        Add-Text-Check "04-artifact-inspection.md" "(?im)^\s*-\s*macos-x64-dmg\s*:\s*missing\s*$" "inspection:macos-x64-coverage-deferred"
        Add-Text-Check "04-artifact-inspection.md" "(?im)^\s*-\s*macos-arm64-dmg\s*:\s*missing\s*$" "inspection:macos-arm64-coverage-deferred"
    } else {
        Add-Text-Check "04-artifact-inspection.md" "(?im)^\s*-\s*macos-x64-dmg\s*:\s*present\s*$" "inspection:macos-x64-coverage-present"
        Add-Text-Check "04-artifact-inspection.md" "(?im)^\s*-\s*macos-arm64-dmg\s*:\s*present\s*$" "inspection:macos-arm64-coverage-present"
    }
    Add-Text-Check "04-artifact-inspection.md" "(?is)High Confidence Scanner Findings\s*\r?\n\s*- none" "inspection:no-scanner-findings"
    Add-Text-Check "04-artifact-inspection.md" "(?i)Package install does not overwrite existing manual provider configuration" "inspection:manual-provider-boundary"
    Add-Text-Check "05-package-gate-report.md" "(?im)^\s*Package evidence result\s*:\s*(pass|fail)\s*$" "package-report:result"
    $packageReportPath = Join-Path $EvidencePath "05-package-gate-report.md"
    if (Test-Path -LiteralPath $packageReportPath -PathType Leaf) {
        $packageReportText = Read-TextFile $packageReportPath
        $packageResultMatch = [regex]::Match($packageReportText, "(?im)^\s*Package evidence result\s*:\s*(pass|fail)\s*$")
        Add-Check "package-report:result-pass" ($packageResultMatch.Success -and $packageResultMatch.Groups[1].Value.ToLowerInvariant() -eq "pass") "Package evidence result must be pass."
    } else {
        Add-Check "package-report:result-pass" $false "05-package-gate-report.md is missing."
    }
    Add-Text-Check "05-package-gate-report.md" "(?i)Commands Executed" "package-report:commands"
    Add-Text-Check "05-package-gate-report.md" "(?i)Evidence Links" "package-report:evidence-links"
    Add-Text-Check "05-package-gate-report.md" "(?i)Remaining Risks" "package-report:remaining-risks"
    Add-Text-Check "05-package-gate-report.md" "(?i)Release Boundary" "package-report:release-boundary"

    $textExtensions = @(".csv", ".html", ".json", ".log", ".md", ".txt", ".yaml", ".yml")
    $textFiles = Get-ChildItem -LiteralPath $EvidencePath -Recurse -File |
        Where-Object { $textExtensions -contains $_.Extension.ToLowerInvariant() }

    $leakRules = @(
        @{ Name = "api-key"; Pattern = "(?i)\b(sk-[a-z0-9][a-z0-9_-]{18,}|sk-proj-[a-z0-9][a-z0-9_-]{18,}|sk-ant-[a-z0-9][a-z0-9_-]{18,})\b" },
        @{ Name = "authorization-header"; Pattern = "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}" },
        @{ Name = "jwt"; Pattern = "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b" },
        @{ Name = "token-field"; Pattern = "(?i)(access_token|refresh_token|id_token|poll_token|session_token|api_key|upstream_key)\s*[:=]\s*[""']?(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b)[A-Za-z0-9._~+/=-]{12,}" },
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
        Add-Check "scan:text-files-covered" $true "$($textFiles.Count) package evidence text file(s) scanned."
    } else {
        Add-Check "scan:text-files-covered" $false "No text files found under package evidence directory."
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 package evidence verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 package evidence verification passed."
exit 0

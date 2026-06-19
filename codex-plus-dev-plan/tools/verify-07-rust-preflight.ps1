param(
    [string]$Root,
    [int]$MinimumFreeGB = 20
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
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

function Get-CommandPath {
    param([string]$Command)
    $cmd = Get-Command $Command -ErrorAction SilentlyContinue
    if ($null -eq $cmd) {
        return $null
    }
    return $cmd.Source
}

$desktopRoot = Join-Path $Root "CodexPlusPlus-main"
$workspaceCargo = Join-Path $Root "work\rust-toolchain\bin\cargo.exe"
$workspaceRustc = Join-Path $Root "work\rust-toolchain\bin\rustc.exe"
$workspaceRustfmt = Join-Path $Root "work\rust-toolchain\bin\rustfmt.exe"
$workspaceDlltool = Join-Path $Root "work\w64devkit\bin\dlltool.exe"

Add-Check "desktop-root-exists" (Test-Path -LiteralPath $desktopRoot -PathType Container) $desktopRoot

$pathCargo = Get-CommandPath "cargo"
$pathRustc = Get-CommandPath "rustc"
$pathRustup = Get-CommandPath "rustup"
$pathLink = Get-CommandPath "link.exe"
$pathDlltool = Get-CommandPath "dlltool.exe"

Add-Check "path:cargo" (-not [string]::IsNullOrWhiteSpace($pathCargo)) $(if ($pathCargo) { $pathCargo } else { "cargo not found on PATH" })
Add-Check "path:rustc" (-not [string]::IsNullOrWhiteSpace($pathRustc)) $(if ($pathRustc) { $pathRustc } else { "rustc not found on PATH" })
Add-Check "path:rustup" (-not [string]::IsNullOrWhiteSpace($pathRustup)) $(if ($pathRustup) { $pathRustup } else { "rustup not found on PATH" })
Add-Check "path:msvc-link" (-not [string]::IsNullOrWhiteSpace($pathLink)) $(if ($pathLink) { $pathLink } else { "link.exe not found on PATH" })
Add-Check "path:gnu-dlltool" (-not [string]::IsNullOrWhiteSpace($pathDlltool)) $(if ($pathDlltool) { $pathDlltool } else { "dlltool.exe not found on PATH" })

Add-Check "workspace-toolchain:cargo" (Test-Path -LiteralPath $workspaceCargo -PathType Leaf) $workspaceCargo
Add-Check "workspace-toolchain:rustc" (Test-Path -LiteralPath $workspaceRustc -PathType Leaf) $workspaceRustc
Add-Check "workspace-toolchain:rustfmt" (Test-Path -LiteralPath $workspaceRustfmt -PathType Leaf) $workspaceRustfmt
Add-Check "workspace-toolchain:dlltool" (Test-Path -LiteralPath $workspaceDlltool -PathType Leaf) $workspaceDlltool

$driveRoot = [System.IO.Path]::GetPathRoot([System.IO.Path]::GetFullPath($Root))
$drive = Get-PSDrive -Name $driveRoot.TrimEnd(":\") -ErrorAction SilentlyContinue
if ($null -eq $drive) {
    Add-Check "disk:drive-found" $false "Drive not found for $driveRoot"
} else {
    $freeGB = [math]::Round($drive.Free / 1GB, 2)
    Add-Check "disk:minimum-free-gb" ($freeGB -ge $MinimumFreeGB) "freeGB=$freeGB; requiredGB=$MinimumFreeGB; drive=$driveRoot"
}

$targetPath = Join-Path $desktopRoot "target"
Add-Check "target-dir-clean-or-absent" (-not (Test-Path -LiteralPath $targetPath)) $targetPath

$hasPathRust = -not [string]::IsNullOrWhiteSpace($pathCargo) -and -not [string]::IsNullOrWhiteSpace($pathRustc)
$hasWorkspaceRust = (Test-Path -LiteralPath $workspaceCargo -PathType Leaf) -and (Test-Path -LiteralPath $workspaceRustc -PathType Leaf)
$hasRust = $hasPathRust -or $hasWorkspaceRust
$hasMsvcLink = -not [string]::IsNullOrWhiteSpace($pathLink)
$hasGnuLink = (-not [string]::IsNullOrWhiteSpace($pathDlltool)) -or (Test-Path -LiteralPath $workspaceDlltool -PathType Leaf)

Add-Check "rust-toolchain-available" $hasRust "requires cargo and rustc on PATH or under work/rust-toolchain"
Add-Check "rust-linker-available" ($hasMsvcLink -or $hasGnuLink) "requires MSVC link.exe or GNU dlltool.exe"

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 Rust preflight failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "This is an environment readiness failure, not a Rust test failure."
    exit 1
}

Write-Host ""
Write-Host "07 Rust preflight passed."
exit 0

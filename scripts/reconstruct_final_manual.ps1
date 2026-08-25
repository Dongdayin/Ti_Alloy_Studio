$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$PartsDir = Join-Path $Root 'docs\manual_b64'
$Output = Join-Path $Root 'docs\TiAlloyStudio-Manual.docx'
$Expected = '0389d2d967023c2a77fafc1e15ec0dfde28e0725b354672388ae2892a78edfda'

$parts = @(Get-ChildItem -LiteralPath $PartsDir -Filter 'part*.txt' -File | Sort-Object Name)
if ($parts.Count -ne 9) {
  throw "Expected 9 final-manual Base64 parts, found $($parts.Count)"
}

$b64 = ($parts | ForEach-Object { Get-Content -LiteralPath $_.FullName -Raw }) -join ''
try {
  $bytes = [Convert]::FromBase64String($b64)
} catch {
  throw "Final manual Base64 reconstruction failed: $($_.Exception.Message)"
}

[IO.File]::WriteAllBytes($Output, $bytes)
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Output).Hash.ToLowerInvariant()
if ($hash -ne $Expected) {
  throw "Final manual SHA256 mismatch: got $hash expected $Expected"
}
if ((Get-Item -LiteralPath $Output).Length -lt 60000) {
  throw 'Final manual is suspiciously small after reconstruction'
}
Write-Host "[manual] reconstructed $Output"
Write-Host "[manual] SHA256 $hash"

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

New-Item -ItemType Directory -Force -Path (Join-Path $Root 'internal\installer\payload') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Root 'dist') | Out-Null

Write-Host '[release] Reconstructing the QA-approved final Phase 1 manual'
& (Join-Path $PSScriptRoot 'reconstruct_final_manual.ps1')
$Manual = Join-Path $Root 'docs\TiAlloyStudio-Manual.docx'
$ExpectedManualHash = '0389d2d967023c2a77fafc1e15ec0dfde28e0725b354672388ae2892a78edfda'
$ManualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Manual).Hash.ToLowerInvariant()
if ($ManualHash -ne $ExpectedManualHash) { throw "Final manual hash gate failed: $ManualHash" }

& (Join-Path $PSScriptRoot 'fetch_offline_engines.ps1')

Write-Host '[release] Verifying the real offline engine payload before compilation'
$Bundle = Join-Path $Root 'internal\installer\payload\engine-bundle.zip'
if (-not (Test-Path $Bundle)) { throw 'engine-bundle.zip was not generated' }
if ((Get-Item $Bundle).Length -lt 50MB) { throw 'engine-bundle.zip is suspiciously small; refusing to build an offline installer' }
$RequiredEntries = @(
  'python-3.11.9-amd64.exe',
  'atomsk/atomsk.exe',
  'requirements-offline.txt',
  'wheelhouse/ase-3.29.0-py3-none-any.whl',
  'wheelhouse/spglib-2.7.0-cp311-cp311-win_amd64.whl',
  'wheelhouse/pymatgen_core-2026.7.31-cp311-cp311-win_amd64.whl',
  'wheelhouse/atomman-1.4.11-cp311-cp311-win_amd64.whl'
)
Add-Type -AssemblyName System.IO.Compression.FileSystem
$Zip = [System.IO.Compression.ZipFile]::OpenRead($Bundle)
try {
  $Names = @($Zip.Entries | ForEach-Object { $_.FullName.Replace('\\','/') })
  foreach ($Entry in $RequiredEntries) {
    if ($Names -notcontains $Entry) { throw "Offline bundle missing required entry: $Entry" }
  }
} finally { $Zip.Dispose() }

Write-Host '[release] Staging the final manual into installer payload'
if (-not (Test-Path $Manual)) { throw 'Final Word manual is missing' }
Copy-Item -LiteralPath $Manual -Destination (Join-Path $Root 'internal\installer\payload\TiAlloyStudio-Manual.docx') -Force
$PayloadManualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $Root 'internal\installer\payload\TiAlloyStudio-Manual.docx')).Hash.ToLowerInvariant()
if ($PayloadManualHash -ne $ExpectedManualHash) { throw "Installer payload manual hash mismatch: $PayloadManualHash" }

Write-Host '[release] Running core Go tests before Windows binary generation'
go test ./internal/model ./internal/app ./internal/engines ./internal/httpapi ./internal/studio ./internal/webapp
if ($LASTEXITCODE -ne 0) { throw 'Core Go tests failed' }

Write-Host '[release] Building Windows x64 application into installer payload'
$env:GOOS='windows'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go build -trimpath -ldflags '-s -w -H windowsgui' -o internal\installer\payload\TiAlloyStudio.exe .\cmd\studio
if ($LASTEXITCODE -ne 0) { throw 'Windows application build failed' }

Write-Host '[release] Running full Go test suite and vet with real payload present'
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }

Write-Host '[release] Building Windows x64 offline installer'
go build -trimpath -ldflags '-s -w -H windowsgui' -o dist\TiAlloyStudio-Setup-x64-Offline.exe .\cmd\installer
if ($LASTEXITCODE -ne 0) { throw 'Windows installer build failed' }

Write-Host '[release] Computing SHA256'
Get-FileHash -Algorithm SHA256 $Manual
Get-FileHash -Algorithm SHA256 internal\installer\payload\engine-bundle.zip
Get-FileHash -Algorithm SHA256 internal\installer\payload\TiAlloyStudio.exe
Get-FileHash -Algorithm SHA256 dist\TiAlloyStudio-Setup-x64-Offline.exe

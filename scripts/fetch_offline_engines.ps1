$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Root = Split-Path -Parent $PSScriptRoot
$Out = Join-Path $Root 'build\offline-engine-bundle'
$Wheelhouse = Join-Path $Out 'wheelhouse'
$Spec = Join-Path $Root 'offline-engine-spec\requirements-offline.in'
$Lock = Join-Path $Out 'requirements-offline.txt'

Remove-Item $Out -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $Wheelhouse | Out-Null

Write-Host '[1/7] Downloading official CPython 3.11.9 Windows x64 installer'
Invoke-WebRequest -UseBasicParsing `
  -Uri 'https://www.python.org/ftp/python/3.11.9/python-3.11.9-amd64.exe' `
  -OutFile (Join-Path $Out 'python-3.11.9-amd64.exe')

Write-Host '[2/7] Downloading official Atomsk 0.13.1 Windows archive'
Invoke-WebRequest -UseBasicParsing `
  -Uri 'https://atomsk.univ-lille.fr/code/atomsk_b0.13.1_Windows.zip' `
  -OutFile (Join-Path $Out 'atomsk_b0.13.1_Windows.zip')

Write-Host '[3/7] Resolving a fully pinned Python 3.11 science environment'
$BuildPython = (Get-Command python -ErrorAction Stop).Source
$BuildPyVersion = & $BuildPython -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')"
if ($BuildPyVersion -ne '3.11') { throw "Release builder requires Python 3.11, found $BuildPyVersion at $BuildPython" }
$Stage = Join-Path $env:TEMP ('TiAlloyStudio-lock-' + [guid]::NewGuid())
& $BuildPython -m venv $Stage
$Py = Join-Path $Stage 'Scripts\python.exe'
& $Py -m pip install --disable-pip-version-check --upgrade pip
& $Py -m pip install --disable-pip-version-check -r $Spec
& $Py -m pip freeze --all | Where-Object { $_ -notmatch '^(pip|setuptools)==' } | Set-Content -LiteralPath $Lock -Encoding ASCII

Write-Host '[4/7] Downloading complete Windows wheelhouse'
& $Py -m pip download --disable-pip-version-check --only-binary=:all: --dest $Wheelhouse -r $Lock

Write-Host '[5/7] Verifying top-level official wheels'
$Expected = @{
  'ase-3.29.0-py3-none-any.whl' = '7b9dd103f007810339c24acfee2f6b677c0c48443b21d3c98e52959246cf4ebf'
  'spglib-2.7.0-cp311-cp311-win_amd64.whl' = '468879702577124dcde0607a75396576e256f1cfa2d8fe48da4a928fbb27abc6'
  'pymatgen_core-2026.7.31-cp311-cp311-win_amd64.whl' = 'ec9a62b8c73c70f36856797184d5b3bd33cd5fd39fc0dc9c79cf469de172531e'
  'atomman-1.4.11-cp311-cp311-win_amd64.whl' = 'c6a31689f42b243d56daf926d4ccbec6e90b29ff7c762d9dfef4b01c51fb64f4'
}
foreach ($Name in $Expected.Keys) {
  $File = Join-Path $Wheelhouse $Name
  if (-not (Test-Path $File)) { throw "Required wheel missing: $Name" }
  $Hash = (Get-FileHash -Algorithm SHA256 $File).Hash.ToLowerInvariant()
  if ($Hash -ne $Expected[$Name]) { throw "SHA256 mismatch for $Name: $Hash" }
}

Write-Host '[6/7] Proving the wheelhouse installs with NO network access'
$Offline = Join-Path $env:TEMP ('TiAlloyStudio-offline-' + [guid]::NewGuid())
& $BuildPython -m venv $Offline
$OfflinePy = Join-Path $Offline 'Scripts\python.exe'
& $OfflinePy -m pip install --disable-pip-version-check --no-index --find-links $Wheelhouse -r $Lock
& $OfflinePy -c "import ase,spglib,atomman;from pymatgen.io.vasp import Poscar;print('offline-python-stack-PASS')"
if ($LASTEXITCODE -ne 0) { throw 'Offline science stack verification failed' }

Write-Host '[7/7] Recording hashes and creating engine-bundle.zip'
Get-ChildItem $Out -File -Recurse | Sort-Object FullName | ForEach-Object {
  $rel = $_.FullName.Substring($Out.Length + 1)
  $sha = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLowerInvariant()
  "$sha  $rel"
} | Set-Content -LiteralPath (Join-Path $Out 'SHA256SUMS.txt') -Encoding ASCII

$Bundle = Join-Path $Root 'internal\installer\payload\engine-bundle.zip'
Remove-Item $Bundle -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $Out '*') -DestinationPath $Bundle -CompressionLevel Optimal

Remove-Item $Stage -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $Offline -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "Offline engine bundle ready: $Bundle"

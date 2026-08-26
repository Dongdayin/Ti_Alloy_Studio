$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Root = Split-Path -Parent $PSScriptRoot
$Out = Join-Path $Root 'build\offline-engine-bundle'
$Work = Join-Path $Root 'build\offline-engine-work'
$Wheelhouse = Join-Path $Work 'wheelhouse'
$Runtime = Join-Path $Work 'python-runtime'
$Spec = Join-Path $Root 'offline-engine-spec\requirements-offline.in'
$Lock = Join-Path $Out 'requirements-offline.txt'

Remove-Item $Out -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $Work -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $Wheelhouse | Out-Null
New-Item -ItemType Directory -Force -Path $Out | Out-Null

Write-Host '[1/9] Preparing the official CPython 3.11.9 embeddable x64 runtime'
$PythonEmbed = Join-Path $Work 'python-3.11.9-embed-amd64.zip'
Invoke-WebRequest -UseBasicParsing `
  -Uri 'https://www.python.org/ftp/python/3.11.9/python-3.11.9-embed-amd64.zip' `
  -OutFile $PythonEmbed
New-Item -ItemType Directory -Force -Path $Runtime | Out-Null
Expand-Archive -LiteralPath $PythonEmbed -DestinationPath $Runtime -Force
@(
  'python311.zip'
  '.'
  'Lib\site-packages'
  'import site'
) | Set-Content -LiteralPath (Join-Path $Runtime 'python311._pth') -Encoding ASCII

Write-Host '[2/9] Downloading and staging official Atomsk 0.13.1 Windows binary'
$AtomskArchive = Join-Path $env:TEMP ('TiAlloyStudio-atomsk-' + [guid]::NewGuid() + '.zip')
$AtomskUnpack = Join-Path $env:TEMP ('TiAlloyStudio-atomsk-unpack-' + [guid]::NewGuid())
$AtomskInstall = Join-Path $env:TEMP ('TiAlloyStudio-atomsk-install-' + [guid]::NewGuid())
$AtomskSmokeDir = Join-Path $env:TEMP ('TiAlloyStudio-atomsk-smoke-' + [guid]::NewGuid())
try {
  Invoke-WebRequest -UseBasicParsing `
    -Uri 'https://atomsk.univ-lille.fr/code/atomsk_b0.13.1_Windows.zip' `
    -OutFile $AtomskArchive
  New-Item -ItemType Directory -Force -Path $AtomskUnpack | Out-Null
  Expand-Archive -LiteralPath $AtomskArchive -DestinationPath $AtomskUnpack -Force

  # Some Atomsk distributions may expose the static executable directly. The
  # official beta-0.13.1 Windows package normally contains a setup program, so
  # release CI installs that setup into an isolated temporary directory and
  # extracts the resulting static atomsk.exe for redistribution in our private
  # engine payload. End-user installation never invokes Atomsk's setup program.
  $AtomskExe = Get-ChildItem $AtomskUnpack -Recurse -File -Filter 'atomsk.exe' | Select-Object -First 1
  if (-not $AtomskExe) {
    $SetupCandidates = @(Get-ChildItem $AtomskUnpack -Recurse -File -Filter '*.exe' | Where-Object { $_.Name -notmatch '^unins\d*\.exe$' })
    if ($SetupCandidates.Count -eq 0) {
      throw 'Official Atomsk Windows archive contains neither atomsk.exe nor an executable setup candidate'
    }
    $Setup = $SetupCandidates | Where-Object { $_.Name -match '(?i)(setup|install)' } | Select-Object -First 1
    if (-not $Setup) { $Setup = $SetupCandidates | Select-Object -First 1 }
    Write-Host ("Atomsk setup candidate: {0}" -f $Setup.FullName)
    New-Item -ItemType Directory -Force -Path $AtomskInstall | Out-Null
    $SetupArgs = @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/SP-',("/DIR=$AtomskInstall"))
    $SetupProc = Start-Process -FilePath $Setup.FullName -ArgumentList $SetupArgs -Wait -PassThru
    if ($SetupProc.ExitCode -ne 0) {
      throw "Atomsk setup failed in release staging with exit code $($SetupProc.ExitCode)"
    }
    $AtomskExe = Get-ChildItem $AtomskInstall -Recurse -File -Filter 'atomsk.exe' | Select-Object -First 1
    if (-not $AtomskExe) {
      $InstalledFiles = @(Get-ChildItem $AtomskInstall -Recurse -File | Select-Object -ExpandProperty FullName)
      throw ("Atomsk setup completed but atomsk.exe was not found under staging directory. Files: " + ($InstalledFiles -join '; '))
    }
  }

  $AtomskDir = Join-Path $Out 'atomsk'
  New-Item -ItemType Directory -Force -Path $AtomskDir | Out-Null
  $AtomskBundledExe = Join-Path $AtomskDir 'atomsk.exe'
  Copy-Item -LiteralPath $AtomskExe.FullName -Destination $AtomskBundledExe -Force
  if ((Get-Item $AtomskBundledExe).Length -lt 100KB) {
    throw 'Staged atomsk.exe is suspiciously small'
  }

  New-Item -ItemType Directory -Force -Path $AtomskSmokeDir | Out-Null
  $AtomskSmoke = Join-Path $AtomskSmokeDir 'al-fcc.xsf'
  $AtomskProc = Start-Process -FilePath $AtomskBundledExe -ArgumentList @('--create','fcc','4.02','Al',$AtomskSmoke) -WorkingDirectory $AtomskSmokeDir -Wait -PassThru
  if ($AtomskProc.ExitCode -ne 0) {
    throw "Staged Atomsk scientific smoke failed with exit code $($AtomskProc.ExitCode)"
  }
  if (-not (Test-Path $AtomskSmoke) -or (Get-Item $AtomskSmoke).Length -lt 50) {
    throw 'Staged Atomsk did not generate the expected FCC Al smoke-test structure'
  }
  $AtomskHash = (Get-FileHash -Algorithm SHA256 $AtomskBundledExe).Hash.ToLowerInvariant()
  Write-Host "atomsk-staged-PASS sha256=$AtomskHash"
}
finally {
  Remove-Item $AtomskArchive -Force -ErrorAction SilentlyContinue
  Remove-Item $AtomskUnpack -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item $AtomskInstall -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item $AtomskSmokeDir -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host '[3/9] Resolving a fully pinned Python 3.11 science environment'
$BuildPython = (Get-Command python -ErrorAction Stop).Source
$BuildPyVersion = & $BuildPython -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')"
if ($BuildPyVersion -ne '3.11') { throw "Release builder requires Python 3.11, found $BuildPyVersion at $BuildPython" }
$Stage = Join-Path $env:TEMP ('TiAlloyStudio-lock-' + [guid]::NewGuid())
& $BuildPython -m venv $Stage
$Py = Join-Path $Stage 'Scripts\python.exe'
& $Py -m pip install --disable-pip-version-check --upgrade pip
if ($LASTEXITCODE -ne 0) { throw 'pip upgrade failed in release-builder environment' }
& $Py -m pip install --disable-pip-version-check -r $Spec
if ($LASTEXITCODE -ne 0) { throw 'Pinned science environment resolution failed' }
& $Py -m pip freeze --all | Where-Object { $_ -notmatch '^(pip|setuptools)==' } | Set-Content -LiteralPath $Lock -Encoding ASCII

Write-Host '[4/9] Building complete Windows wheelhouse (build sdists such as bibtexparser when necessary)'
& $Py -m pip wheel --disable-pip-version-check --wheel-dir $Wheelhouse -r $Lock
if ($LASTEXITCODE -ne 0) { throw 'Wheelhouse build failed' }

Write-Host '[5/9] Verifying top-level official wheels'
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
  if ($Hash -ne $Expected[$Name]) { throw "SHA256 mismatch for ${Name}: $Hash" }
}

Write-Host '[6/9] Proving the wheelhouse installs with NO network access'
$Offline = Join-Path $env:TEMP ('TiAlloyStudio-offline-' + [guid]::NewGuid())
& $BuildPython -m venv $Offline
$OfflinePy = Join-Path $Offline 'Scripts\python.exe'
& $OfflinePy -m pip install --disable-pip-version-check --no-index --find-links $Wheelhouse -r $Lock
if ($LASTEXITCODE -ne 0) { throw 'Offline wheelhouse installation failed' }
& $OfflinePy -c "import ase,spglib,atomman;from pymatgen.io.vasp import Poscar;print('offline-python-stack-PASS')"
if ($LASTEXITCODE -ne 0) { throw 'Offline science stack verification failed' }

Write-Host '[7/9] Preinstalling the complete science stack into the app-local runtime'
$RuntimeSite = Join-Path $Runtime 'Lib\site-packages'
New-Item -ItemType Directory -Force -Path $RuntimeSite | Out-Null
& $Py -m pip install --disable-pip-version-check --no-index --find-links $Wheelhouse --target $RuntimeSite -r $Lock
if ($LASTEXITCODE -ne 0) { throw 'App-local Python science stack staging failed' }
$RuntimePython = Join-Path $Runtime 'python.exe'
& $RuntimePython -c "import sys,ase,spglib,atomman;from pymatgen.io.vasp import Poscar;print('app-local-python-stack-PASS',sys.executable)"
if ($LASTEXITCODE -ne 0) { throw 'App-local Python runtime validation failed' }
$RuntimeBundle = Join-Path $Out 'python-runtime.zip'
Compress-Archive -Path (Join-Path $Runtime '*') -DestinationPath $RuntimeBundle -CompressionLevel Optimal

Write-Host '[8/9] Recording hashes for every offline payload file'
Get-ChildItem $Out -File -Recurse | Sort-Object FullName | ForEach-Object {
  $rel = $_.FullName.Substring($Out.Length + 1)
  $sha = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLowerInvariant()
  "$sha  $rel"
} | Set-Content -LiteralPath (Join-Path $Out 'SHA256SUMS.txt') -Encoding ASCII

Write-Host '[9/9] Creating engine-bundle.zip'
$Bundle = Join-Path $Root 'internal\installer\payload\engine-bundle.zip'
Remove-Item $Bundle -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $Out '*') -DestinationPath $Bundle -CompressionLevel Optimal

Remove-Item $Stage -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $Offline -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $Work -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "Offline engine bundle ready: $Bundle"

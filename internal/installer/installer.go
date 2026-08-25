package installer

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed payload/*
var payloadFS embed.FS

func PayloadFiles() (map[string][]byte, error) {
	sub, err := fs.Sub(payloadFS, "payload")
	if err != nil {
		return nil, err
	}
	names := []string{"TiAlloyStudio.exe", "TiAlloyStudio-Manual.docx", "README.txt", "THIRD_PARTY_NOTICES.txt", "engine-bundle.zip"}
	out := map[string][]byte{}
	for _, name := range names {
		b, err := fs.ReadFile(sub, name)
		if err != nil {
			return nil, err
		}
		out[name] = b
	}
	return out, nil
}

func InstallPayloadTo(dir string, payload map[string][]byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for name, data := range payload {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func DefaultInstallDir(local string) string {
	if local == "" {
		return "TiAlloyStudio"
	}
	return filepath.Join(local, "Programs", "TiAlloyStudio")
}

type smokeEnvelope struct {
	Result struct {
		Status string `json:"status"`
	} `json:"result"`
}

func SmokePass(data []byte) bool {
	var v smokeEnvelope
	return json.Unmarshal(data, &v) == nil && v.Result.Status == "PASS"
}

func VerifyOfflineEngineBundle(data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid offline engine bundle: %w", err)
	}
	found := map[string]int64{}
	for _, f := range zr.File {
		found[strings.ReplaceAll(f.Name, `\`, `/`)] = int64(f.UncompressedSize64)
	}
	required := []string{
		"python-3.11.9-amd64.exe",
		"atomsk/atomsk.exe",
		"requirements-offline.txt",
		"wheelhouse/ase-3.29.0-py3-none-any.whl",
		"wheelhouse/spglib-2.7.0-cp311-cp311-win_amd64.whl",
		"wheelhouse/pymatgen_core-2026.7.31-cp311-cp311-win_amd64.whl",
		"wheelhouse/atomman-1.4.11-cp311-cp311-win_amd64.whl",
	}
	for _, n := range required {
		if found[n] < 1 {
			return fmt.Errorf("offline engine bundle missing required artifact %s", n)
		}
	}
	return nil
}

func OfflineEngineInstallScript(root, bundle string) string {
	return OfflineEngineInstallScriptWithProgress(root, bundle, "")
}

func OfflineEngineInstallScriptWithProgress(root, bundle, progressPath string) string {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	progress := ""
	if strings.TrimSpace(progressPath) != "" {
		progress = `$progress=` + q(progressPath) + `;function Report([int]$pct,[string]$msg){Set-Content -LiteralPath $progress -Value ($pct.ToString()+'|'+$msg) -Encoding UTF8};`
	} else {
		progress = `function Report([int]$pct,[string]$msg){};`
	}
	return `$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue';` + progress +
		`$root=` + q(root) + `;$bundle=` + q(bundle) + `;New-Item -ItemType Directory -Force -Path $root|Out-Null;` +
		`$pySetup=Join-Path $bundle 'python-3.11.9-amd64.exe';$req=Join-Path $bundle 'requirements-offline.txt';$wh=Join-Path $bundle 'wheelhouse';$atomskSource=Join-Path $bundle 'atomsk\atomsk.exe';foreach($p in @($pySetup,$req,$wh,$atomskSource)){if(-not(Test-Path -LiteralPath $p)){throw ('Offline asset missing: '+$p)}};` +
		`Report 32 'Installing private Python runtime';$py=Join-Path $root 'python';$args=@('/quiet','InstallAllUsers=0',('TargetDir='+$py),'Include_pip=1','Include_launcher=0','InstallLauncherAllUsers=0','AssociateFiles=0','Shortcuts=0','PrependPath=0','Include_doc=0','Include_test=0','Include_tcltk=0');$p=Start-Process -FilePath $pySetup -ArgumentList $args -Wait -PassThru;if($p.ExitCode -ne 0){throw ('Private Python failed: '+$p.ExitCode)};` +
		`Report 48 'Installing bundled scientific Python packages';$python=Join-Path $py 'python.exe';& $python -m pip install --disable-pip-version-check --no-index --find-links $wh -r $req;if($LASTEXITCODE -ne 0){throw 'Offline wheels failed'};` +
		`Report 70 'Validating ASE, spglib, pymatgen and AtomMan';& $python -c "import ase,spglib,atomman;from pymatgen.io.vasp import Poscar;print('science-ok')";if($LASTEXITCODE -ne 0){throw 'Python validation failed'};` +
		`Report 80 'Installing bundled Atomsk';$ad=Join-Path $root 'atomsk';New-Item -ItemType Directory -Force -Path $ad|Out-Null;$atomskDest=Join-Path $ad 'atomsk.exe';Copy-Item -LiteralPath $atomskSource -Destination $atomskDest -Force;if(-not(Test-Path -LiteralPath $atomskDest)){throw 'Private Atomsk copy failed'}`
}

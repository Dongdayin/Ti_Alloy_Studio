package installer

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
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
	found := map[string]*zip.File{}
	for _, f := range zr.File {
		found[strings.ReplaceAll(f.Name, `\`, `/`)] = f
	}
	required := []string{"python-runtime.zip", "atomsk/atomsk.exe"}
	for _, n := range required {
		if found[n] == nil || found[n].UncompressedSize64 < 1 {
			return fmt.Errorf("offline engine bundle missing required artifact %s", n)
		}
	}
	runtimeData, err := readZipFile(found["python-runtime.zip"])
	if err != nil {
		return fmt.Errorf("read private Python runtime: %w", err)
	}
	runtimeZip, err := zip.NewReader(bytes.NewReader(runtimeData), int64(len(runtimeData)))
	if err != nil {
		return fmt.Errorf("invalid private Python runtime: %w", err)
	}
	runtimeFiles := map[string]uint64{}
	for _, f := range runtimeZip.File {
		runtimeFiles[strings.ReplaceAll(f.Name, `\`, `/`)] = f.UncompressedSize64
	}
	for _, n := range []string{
		"python.exe",
		"python311.dll",
		"python311._pth",
		"Lib/site-packages/ase/__init__.py",
		"Lib/site-packages/ase/io/__init__.py",
		"Lib/site-packages/ase/dft/dos.py",
	} {
		if runtimeFiles[n] < 1 {
			return fmt.Errorf("private Python runtime missing required artifact %s", n)
		}
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func safeZipDestination(root, name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, `\`, `/`)))
	if cleanName == "." || filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	root = filepath.Clean(root)
	dst := filepath.Join(root, cleanName)
	rel, err := filepath.Rel(root, dst)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive path escapes destination %q", name)
	}
	return dst, nil
}

func extractZip(data []byte, root string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		dst, err := safeZipDestination(root, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(dst, 0755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			r.Close()
			return err
		}
		_, copyErr := io.Copy(out, r)
		closeErr := out.Close()
		r.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// InstallOfflineEngines deploys the bundled application-local runtime without
// invoking a system Python installer or pip on the end user's computer.
func InstallOfflineEngines(root string, bundle []byte) error {
	if err := VerifyOfflineEngineBundle(bundle); err != nil {
		return err
	}
	zr, _ := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[strings.ReplaceAll(f.Name, `\`, `/`)] = f
	}
	runtimeData, err := readZipFile(files["python-runtime.zip"])
	if err != nil {
		return fmt.Errorf("read private Python runtime: %w", err)
	}
	if err = extractZip(runtimeData, filepath.Join(root, "python")); err != nil {
		return fmt.Errorf("extract private Python runtime: %w", err)
	}
	atomskData, err := readZipFile(files["atomsk/atomsk.exe"])
	if err != nil {
		return fmt.Errorf("read private Atomsk: %w", err)
	}
	atomskDir := filepath.Join(root, "atomsk")
	if err = os.MkdirAll(atomskDir, 0755); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(atomskDir, "atomsk.exe"), atomskData, 0644); err != nil {
		return fmt.Errorf("write private Atomsk: %w", err)
	}
	return nil
}

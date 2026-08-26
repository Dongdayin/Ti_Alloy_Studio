package installer

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func appLocalEngineBundle(t *testing.T) []byte {
	t.Helper()
	runtime := zipBytes(t, map[string][]byte{
		"python.exe":                        []byte("private-python"),
		"python311.dll":                     []byte("python-dll"),
		"python311._pth":                    []byte("Lib/site-packages\nimport site\n"),
		"Lib/site-packages/ase/__init__.py": []byte("ase"),
	})
	return zipBytes(t, map[string][]byte{
		"python-runtime.zip": runtime,
		"atomsk/atomsk.exe":  []byte("atomsk"),
	})
}

func TestVerifyOfflineEngineBundleAcceptsAppLocalRuntime(t *testing.T) {
	if err := VerifyOfflineEngineBundle(appLocalEngineBundle(t)); err != nil {
		t.Fatalf("app-local engine bundle rejected: %v", err)
	}
}

func TestInstallOfflineEnginesWritesPrivateRuntimeAndAtomsk(t *testing.T) {
	root := t.TempDir()
	if err := InstallOfflineEngines(root, appLocalEngineBundle(t)); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		filepath.Join(root, "python", "python.exe"):                                 "private-python",
		filepath.Join(root, "python", "Lib", "site-packages", "ase", "__init__.py"): "ase",
		filepath.Join(root, "atomsk", "atomsk.exe"):                                 "atomsk",
	}
	for path, want := range checks {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read installed file %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("installed file %s = %q, want %q", path, data, want)
		}
	}
}

func TestInstallOfflineEnginesRejectsPathTraversal(t *testing.T) {
	runtime := zipBytes(t, map[string][]byte{
		"python.exe":                        []byte("private-python"),
		"python311.dll":                     []byte("python-dll"),
		"python311._pth":                    []byte("import site\n"),
		"Lib/site-packages/ase/__init__.py": []byte("ase"),
		"../escaped.txt":                    []byte("must-not-escape"),
	})
	bundle := zipBytes(t, map[string][]byte{
		"python-runtime.zip": runtime,
		"atomsk/atomsk.exe":  []byte("atomsk"),
	})
	parent := t.TempDir()
	root := filepath.Join(parent, "engines")
	if err := InstallOfflineEngines(root, bundle); err == nil {
		t.Fatal("path traversal bundle was accepted")
	}
	if _, err := os.Stat(filepath.Join(parent, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("path traversal wrote outside engine root: %v", err)
	}
}

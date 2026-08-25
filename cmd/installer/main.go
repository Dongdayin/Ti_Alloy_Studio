package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	inst "tialloystudio/internal/installer"
)

const product = "Ti Alloy Studio"
const uninstallKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\TiAlloyStudio`

func psq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func ps(script string) error {
	return exec.Command("powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script).Run()
}

func choose(def string) (string, bool, error) {
	parent := filepath.Dir(def)
	script := `Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.FolderBrowserDialog;$d.Description='Choose parent folder for Ti Alloy Studio';$d.SelectedPath=` + psq(parent) + `;if($d.ShowDialog() -eq 'OK'){[Console]::Write($d.SelectedPath)}`
	b, err := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script).Output()
	if err != nil {
		return "", false, err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false, nil
	}
	if strings.EqualFold(filepath.Base(s), "TiAlloyStudio") {
		return s, true, nil
	}
	return filepath.Join(s, "TiAlloyStudio"), true, nil
}

func notify(quiet bool, s string) {
	if quiet {
		fmt.Fprintln(os.Stderr, s)
		return
	}
	_ = ps(`Add-Type -AssemblyName PresentationFramework;[System.Windows.MessageBox]::Show(` + psq(s) + `,` + psq(product) + `)|Out-Null`)
}

func copySelf(dir string) error {
	p, err := os.Executable()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "Uninstall.exe"), b, 0644)
}

func shortcuts(dir string) error {
	exe := filepath.Join(dir, "TiAlloyStudio.exe")
	un := filepath.Join(dir, "Uninstall.exe")
	script := `$w=New-Object -ComObject WScript.Shell;` +
		`$d=$w.SpecialFolders.Item('Desktop');$s=$w.CreateShortcut((Join-Path $d 'Ti Alloy Studio.lnk'));$s.TargetPath=` + psq(exe) + `;$s.WorkingDirectory=` + psq(dir) + `;$s.Save();` +
		`$p=[Environment]::GetFolderPath('Programs');$s=$w.CreateShortcut((Join-Path $p 'Ti Alloy Studio.lnk'));$s.TargetPath=` + psq(exe) + `;$s.WorkingDirectory=` + psq(dir) + `;$s.Save();` +
		`$u=$w.CreateShortcut((Join-Path $p 'Uninstall Ti Alloy Studio.lnk'));$u.TargetPath=` + psq(un) + `;$u.Arguments='--uninstall';$u.WorkingDirectory=` + psq(dir) + `;$u.Save()`
	return ps(script)
}

func removeShortcuts() {
	_ = ps(`$w=New-Object -ComObject WScript.Shell;$d=$w.SpecialFolders.Item('Desktop');Remove-Item (Join-Path $d 'Ti Alloy Studio.lnk') -Force -ErrorAction SilentlyContinue;$p=[Environment]::GetFolderPath('Programs');Remove-Item (Join-Path $p 'Ti Alloy Studio.lnk') -Force -ErrorAction SilentlyContinue;Remove-Item (Join-Path $p 'Uninstall Ti Alloy Studio.lnk') -Force -ErrorAction SilentlyContinue`)
}

func registerUninstall(dir string) error {
	un := `"` + filepath.Join(dir, "Uninstall.exe") + `" --uninstall`
	values := [][2]string{
		{"DisplayName", product},
		{"DisplayVersion", "0.1.4-phase1"},
		{"Publisher", "Ti Alloy Studio"},
		{"InstallLocation", dir},
		{"UninstallString", un},
		{"QuietUninstallString", un + " --quiet"},
		{"NoModify", "1"},
		{"NoRepair", "1"},
	}
	for _, v := range values {
		typ := "REG_SZ"
		if v[0] == "NoModify" || v[0] == "NoRepair" {
			typ = "REG_DWORD"
		}
		if out, err := exec.Command("reg.exe", "ADD", uninstallKey, "/v", v[0], "/t", typ, "/d", v[1], "/f").CombinedOutput(); err != nil {
			return fmt.Errorf("register uninstall %s: %v: %s", v[0], err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func unregisterUninstall() { _, _ = exec.Command("reg.exe", "DELETE", uninstallKey, "/f").CombinedOutput() }

func installEngines(dir string) error {
	zipPath := filepath.Join(dir, "engine-bundle.zip")
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return err
	}
	if err = inst.VerifyOfflineEngineBundle(data); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "tialloy-bundle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err = ps(`Expand-Archive -LiteralPath ` + psq(zipPath) + ` -DestinationPath ` + psq(tmp) + ` -Force`); err != nil {
		return err
	}
	eng := filepath.Join(dir, "engines")
	_ = os.RemoveAll(eng)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", inst.OfflineEngineInstallScript(eng, tmp))
	out, err := cmd.CombinedOutput()
	_ = os.WriteFile(filepath.Join(dir, "engine-install.log"), out, 0644)
	if err != nil {
		return fmt.Errorf("offline scientific engine installation failed: %w", err)
	}
	return nil
}

func smoke(dir string) error {
	p := filepath.Join(dir, "smoke.json")
	if err := exec.Command(filepath.Join(dir, "TiAlloyStudio.exe"), "--smoke-test-file", p).Run(); err != nil {
		return fmt.Errorf("native scientific smoke executable failed: %w", err)
	}
	b, err := os.ReadFile(p)
	if err != nil || !inst.SmokePass(b) {
		return fmt.Errorf("native scientific smoke failed")
	}
	e := filepath.Join(dir, "engine-smoke.json")
	if err = exec.Command(filepath.Join(dir, "TiAlloyStudio.exe"), "--engine-smoke-file", e).Run(); err != nil {
		return fmt.Errorf("mature-engine smoke executable failed: %w", err)
	}
	var v struct {
		Status string `json:"status"`
	}
	b, err = os.ReadFile(e)
	if err != nil {
		return err
	}
	if json.Unmarshal(b, &v) != nil || v.Status != "PASS" {
		return fmt.Errorf("mature-engine cross-check failed")
	}
	return nil
}

func diagnosticPath() string {
	root := strings.TrimSpace(os.Getenv("RUNNER_TEMP"))
	if root == "" {
		root = os.TempDir()
	}
	return filepath.Join(root, "TiAlloyStudio-install-error.log")
}

func preserveFailureDiagnostic(dir string, installErr error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Ti Alloy Studio installation failure diagnostic\r\ntime_utc=%s\r\ninstall_dir=%s\r\nerror=%v\r\n", time.Now().UTC().Format(time.RFC3339), dir, installErr)
	for _, name := range []string{"engine-install.log", "smoke.json", "engine-smoke.json"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(&b, "\r\n===== %s =====\r\n<not available: %v>\r\n", name, err)
			continue
		}
		fmt.Fprintf(&b, "\r\n===== %s =====\r\n%s\r\n", name, string(data))
	}
	_ = os.WriteFile(diagnosticPath(), []byte(b.String()), 0644)
}

func uninstall(quiet bool) int {
	exe, err := os.Executable()
	if err != nil {
		notify(quiet, "Cannot resolve uninstall path: "+err.Error())
		return 1
	}
	dir := filepath.Dir(exe)
	if !quiet {
		notify(false, "Ti Alloy Studio will be removed.")
	}
	removeShortcuts()
	unregisterUninstall()
	tmp := filepath.Join(os.TempDir(), "TiAlloyStudio-remove.cmd")
	script := fmt.Sprintf("@echo off\r\ntimeout /t 2 /nobreak >nul\r\nrmdir /s /q \"%s\"\r\ndel \"%%~f0\"\r\n", dir)
	if err = os.WriteFile(tmp, []byte(script), 0644); err != nil {
		notify(quiet, "Cannot create uninstall cleanup: "+err.Error())
		return 1
	}
	if err = exec.Command("cmd.exe", "/C", "start", "", "/min", tmp).Start(); err != nil {
		notify(quiet, "Cannot start uninstall cleanup: "+err.Error())
		return 1
	}
	return 0
}

func run() int {
	un := flag.Bool("uninstall", false, "uninstall")
	quiet := flag.Bool("quiet", false, "suppress GUI messages for automated verification")
	noLaunch := flag.Bool("no-launch", false, "do not launch application after installation")
	idir := flag.String("install-dir", "", "install directory")
	flag.Parse()

	if runtime.GOOS != "windows" {
		return 2
	}
	if *un {
		return uninstall(*quiet)
	}

	dir := strings.TrimSpace(*idir)
	if dir == "" {
		if *quiet {
			notify(true, "--quiet installation requires --install-dir")
			return 2
		}
		var ok bool
		var err error
		dir, ok, err = choose(inst.DefaultInstallDir(os.Getenv("LOCALAPPDATA")))
		if err != nil {
			notify(false, "Install directory selection failed: "+err.Error())
			return 1
		}
		if !ok {
			return 3
		}
	}

	_ = os.Remove(diagnosticPath())
	payload, err := inst.PayloadFiles()
	if err != nil {
		notify(*quiet, "Installer payload error: "+err.Error())
		return 1
	}
	if err = inst.InstallPayloadTo(dir, payload); err == nil {
		err = copySelf(dir)
	}
	if err == nil {
		err = installEngines(dir)
	}
	if err == nil {
		err = smoke(dir)
	}
	if err == nil {
		err = shortcuts(dir)
	}
	if err == nil {
		err = registerUninstall(dir)
	}
	if err != nil {
		preserveFailureDiagnostic(dir, err)
		removeShortcuts()
		unregisterUninstall()
		_ = os.RemoveAll(dir)
		notify(*quiet, "Installation failed and was rolled back: "+err.Error())
		return 1
	}

	notify(*quiet, "Installation completed offline. TiModelCore and all bundled scientific engines passed validation.")
	if !*noLaunch {
		if err = exec.Command("cmd.exe", "/C", "start", "", filepath.Join(dir, "TiAlloyStudio.exe")).Start(); err != nil {
			notify(*quiet, "Installed successfully, but launch failed: "+err.Error())
			return 1
		}
	}
	return 0
}

func main() { os.Exit(run()) }

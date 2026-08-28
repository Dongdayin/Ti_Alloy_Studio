package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copySelf(dir string) error {
	p, err := os.Executable()
	if err != nil {
		return err
	}
	return copyFile(p, filepath.Join(dir, "Uninstall.exe"))
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
		{"DisplayVersion", "0.2.0-phase1-r12"},
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

func unregisterUninstall() {
	_, _ = exec.Command("reg.exe", "DELETE", uninstallKey, "/f").CombinedOutput()
}

func installEngines(dir string) error {
	progressSet(18, "Verifying bundled offline scientific engines")
	zipPath := filepath.Join(dir, "engine-bundle.zip")
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return err
	}
	if err = inst.VerifyOfflineEngineBundle(data); err != nil {
		return err
	}
	eng := filepath.Join(dir, "engines")
	_ = os.RemoveAll(eng)
	progressSet(32, "Extracting application-local scientific runtime")
	if err = inst.InstallOfflineEngines(eng, data); err != nil {
		_ = os.WriteFile(filepath.Join(dir, "engine-install.log"), []byte(err.Error()+"\r\n"), 0644)
		return fmt.Errorf("offline scientific engine deployment failed: %w", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "engine-install.log"), []byte("application-local Python runtime and Atomsk extracted successfully\r\n"), 0644)
	progressSet(84, "Bundled scientific engines installed")
	return nil
}

func smoke(dir string) error {
	progressSet(87, "Running TiModelCore scientific smoke test")
	p := filepath.Join(dir, "smoke.json")
	if err := exec.Command(filepath.Join(dir, "TiAlloyStudio.exe"), "--smoke-test-file", p).Run(); err != nil {
		return fmt.Errorf("native scientific smoke executable failed: %w", err)
	}
	b, err := os.ReadFile(p)
	if err != nil || !inst.SmokePass(b) {
		return fmt.Errorf("native scientific smoke failed")
	}
	progressSet(92, "Cross-checking bundled mature scientific engines")
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

func cleanupHelperPath(temp string, stamp int64) string {
	return filepath.Join(temp, fmt.Sprintf("TiAlloyStudio-uninstall-helper-%d.exe", stamp))
}

func cleanupHelperArgs(dir string, parentPID int, quiet bool) []string {
	args := []string{"--cleanup-install-dir", dir, "--parent-pid", strconv.Itoa(parentPID)}
	if quiet {
		args = append(args, "--quiet")
	}
	return args
}

func isCleanupMode(target string) bool { return strings.TrimSpace(target) != "" }

func removeInstallTree(dir string) error {
	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		cmd := exec.CommandContext(ctx, "cmd.exe", "/D", "/Q", "/C", "rmdir", "/S", "/Q", dir)
		err := cmd.Run()
		cancel()
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			return nil
		}
		if ctx.Err() == context.DeadlineExceeded {
			lastErr = fmt.Errorf("remove attempt %d timed out", attempt)
		} else if err != nil {
			lastErr = fmt.Errorf("remove attempt %d failed: %w", attempt, err)
		} else {
			lastErr = fmt.Errorf("remove attempt %d returned but directory still exists", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

func scheduleHelperSelfDelete() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	quoted := `"` + strings.ReplaceAll(exe, `"`, `""`) + `"`
	script := "ping -n 2 127.0.0.1 >nul & del /f /q " + quoted + " >nul 2>nul"
	cmd := exec.Command("cmd.exe", "/D", "/Q", "/C", script)
	cmd.Dir = os.TempDir()
	_ = cmd.Start()
}

func cleanupInstalledTree(dir string, parentPID int, quiet bool) int {
	if strings.TrimSpace(dir) == "" {
		return 2
	}
	// Stage 1 exits immediately after starting this helper. A short grace period
	// releases the running Uninstall.exe before deletion starts. parentPID is
	// retained in the command line for diagnostics and provenance.
	if parentPID > 0 {
		time.Sleep(500 * time.Millisecond)
	}
	if err := removeInstallTree(dir); err != nil {
		notify(quiet, "Uninstall cleanup failed: "+err.Error())
		return 1
	}
	scheduleHelperSelfDelete()
	return 0
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

	// Stage 1 never deletes the directory that contains the running executable.
	// Instead it copies this executable to %TEMP%, launches it in cleanup mode,
	// and returns. Stage 2 therefore runs outside the installation tree and can
	// remove the private Python runtime, Atomsk, application, manual and itself
	// without a parent/child deletion cycle.
	helper := cleanupHelperPath(os.TempDir(), time.Now().UnixNano())
	if err = copyFile(exe, helper); err != nil {
		notify(quiet, "Cannot prepare uninstall helper: "+err.Error())
		return 1
	}
	cmd := exec.Command(helper, cleanupHelperArgs(dir, os.Getpid(), true)...)
	cmd.Dir = os.TempDir()
	if err = cmd.Start(); err != nil {
		_ = os.Remove(helper)
		notify(quiet, "Cannot start uninstall helper: "+err.Error())
		return 1
	}
	return 0
}

func run() int {
	un := flag.Bool("uninstall", false, "uninstall")
	quiet := flag.Bool("quiet", false, "suppress GUI messages for automated verification")
	noLaunch := flag.Bool("no-launch", false, "do not launch application after installation")
	idir := flag.String("install-dir", "", "install directory")
	cleanupDir := flag.String("cleanup-install-dir", "", "internal: stage-2 uninstall target")
	parentPID := flag.Int("parent-pid", 0, "internal: stage-1 uninstaller process id")
	flag.Parse()

	if runtime.GOOS != "windows" {
		return 2
	}
	if isCleanupMode(*cleanupDir) {
		return cleanupInstalledTree(*cleanupDir, *parentPID, *quiet)
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

	progress := startInstallProgress(*quiet)
	activeInstallProgress = progress
	progressSet(4, "Reading embedded offline payload")
	_ = os.Remove(diagnosticPath())
	payload, err := inst.PayloadFiles()
	if err != nil {
		progress.close(false)
		activeInstallProgress = nil
		notify(*quiet, "Installer payload error: "+err.Error())
		return 1
	}
	progressSet(9, "Writing Ti Alloy Studio application files")
	if err = inst.InstallPayloadTo(dir, payload); err == nil {
		progressSet(14, "Preparing uninstaller")
		err = copySelf(dir)
	}
	if err == nil {
		err = installEngines(dir)
	}
	if err == nil {
		err = smoke(dir)
	}
	if err == nil {
		progressSet(96, "Creating desktop and Start Menu shortcuts")
		err = shortcuts(dir)
	}
	if err == nil {
		progressSet(98, "Registering Windows uninstall information")
		err = registerUninstall(dir)
	}
	if err != nil {
		preserveFailureDiagnostic(dir, err)
		removeShortcuts()
		unregisterUninstall()
		_ = os.RemoveAll(dir)
		progress.close(false)
		activeInstallProgress = nil
		notify(*quiet, "Installation failed and was rolled back: "+err.Error())
		return 1
	}

	progress.close(true)
	activeInstallProgress = nil
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

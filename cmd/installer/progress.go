package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type installProgress struct {
	quiet bool
	path  string
	cmd   *exec.Cmd
	mu    sync.Mutex
}

func progressPowerShell(path string) string {
	q := psq(path)
	return `Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing;` +
		`$path=` + q + `;` +
		`$f=New-Object System.Windows.Forms.Form;$f.Text='Ti Alloy Studio - Installing';$f.StartPosition='CenterScreen';$f.ClientSize=New-Object System.Drawing.Size(520,150);$f.FormBorderStyle='FixedDialog';$f.MaximizeBox=$false;$f.MinimizeBox=$true;` +
		`$title=New-Object System.Windows.Forms.Label;$title.Location=New-Object System.Drawing.Point(20,18);$title.Size=New-Object System.Drawing.Size(480,24);$title.Font=New-Object System.Drawing.Font('Segoe UI',11,[System.Drawing.FontStyle]::Bold);$title.Text='Installing Ti Alloy Studio';$f.Controls.Add($title);` +
		`$status=New-Object System.Windows.Forms.Label;$status.Location=New-Object System.Drawing.Point(20,51);$status.Size=New-Object System.Drawing.Size(400,22);$status.Text='Preparing installation...';$f.Controls.Add($status);` +
		`$pct=New-Object System.Windows.Forms.Label;$pct.Location=New-Object System.Drawing.Point(430,51);$pct.Size=New-Object System.Drawing.Size(70,22);$pct.TextAlign='MiddleRight';$pct.Text='0%';$f.Controls.Add($pct);` +
		`$bar=New-Object System.Windows.Forms.ProgressBar;$bar.Location=New-Object System.Drawing.Point(20,82);$bar.Size=New-Object System.Drawing.Size(480,24);$bar.Minimum=0;$bar.Maximum=100;$bar.Style='Continuous';$f.Controls.Add($bar);` +
		`$hint=New-Object System.Windows.Forms.Label;$hint.Location=New-Object System.Drawing.Point(20,115);$hint.Size=New-Object System.Drawing.Size(480,20);$hint.ForeColor=[System.Drawing.Color]::DimGray;$hint.Text='Bundled scientific engines are installed locally; no network access is required.';$f.Controls.Add($hint);` +
		`$timer=New-Object System.Windows.Forms.Timer;$timer.Interval=150;$timer.Add_Tick({try{if(Test-Path -LiteralPath $path){$line=(Get-Content -LiteralPath $path -Raw -ErrorAction Stop).Trim();$parts=$line.Split('|',3);if($parts.Count -ge 2){$n=0;if([int]::TryParse($parts[0],[ref]$n)){$n=[Math]::Max(0,[Math]::Min(100,$n));$bar.Value=$n;$pct.Text=($n.ToString()+'%')};$status.Text=$parts[1]}}}catch{}});` +
		`$timer.Start();[void]$f.ShowDialog();$timer.Stop();`
}

func startInstallProgress(quiet bool) *installProgress {
	p := &installProgress{quiet: quiet}
	if quiet {
		return p
	}
	f, err := os.CreateTemp("", "TiAlloyStudio-install-progress-*.txt")
	if err != nil {
		return p
	}
	p.path = f.Name()
	_ = f.Close()
	p.set(1, "Preparing installation")
	p.cmd = exec.Command("powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", progressPowerShell(p.path))
	_ = p.cmd.Start()
	return p
}

func (p *installProgress) set(percent int, message string) {
	if p == nil || p.quiet || p.path == "" {
		return
	}
	if percent < 0 { percent = 0 }
	if percent > 100 { percent = 100 }
	message = strings.ReplaceAll(strings.ReplaceAll(message, "|", "/"), "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = os.WriteFile(p.path, []byte(fmt.Sprintf("%d|%s", percent, message)), 0644)
}

func (p *installProgress) close(success bool) {
	if p == nil || p.quiet {
		return
	}
	if success {
		p.set(100, "Installation completed successfully")
		time.Sleep(650 * time.Millisecond)
	} else {
		p.set(100, "Installation failed - rolling back")
		time.Sleep(350 * time.Millisecond)
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
	if p.path != "" {
		_ = os.Remove(p.path)
	}
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tialloystudio/internal/app"
	"tialloystudio/internal/studio"
	"tialloystudio/internal/webapp"
)

func edgeExecutable() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	candidates := []string{}
	for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles") } {
		if strings.TrimSpace(root) != "" {
			candidates = append(candidates, filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"))
		}
	}
	if p, err := exec.LookPath("msedge.exe"); err == nil {
		candidates = append([]string{p}, candidates...)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// openBrowser launches a dedicated Edge app profile when possible and returns
// a cleanup function that terminates only that Ti Alloy Studio browser session.
func openBrowser(url string) func() {
	if runtime.GOOS == "windows" {
		if edge := edgeExecutable(); edge != "" {
			profile, err := os.MkdirTemp("", "TiAlloyStudio-Edge-")
			if err == nil {
				cmd := exec.Command(edge, "--app="+url, "--user-data-dir="+profile, "--no-first-run", "--no-default-browser-check")
				if err = cmd.Start(); err == nil {
					var once sync.Once
					return func() {
						once.Do(func() {
							// Edge can hand work to child processes; select only processes whose
							// command line contains this unique user-data-dir.
							script := `$p=` + psQuote(profile) + `; Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and $_.CommandLine.Contains($p) } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`
							_ = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Run()
							if cmd.Process != nil {
								_ = cmd.Process.Kill()
							}
							_ = os.RemoveAll(profile)
						})
					}
				}
				_ = os.RemoveAll(profile)
			}
		}
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		_ = cmd.Start()
		return func() {}
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
	return func() {}
}

func runSmoke(path string) int {
	result, err := studio.ScientificSmoke()
	payload := map[string]any{"result": result}
	if err != nil { payload["error"] = err.Error() }
	data, _ := json.MarshalIndent(payload, "", "  ")
	if path != "" {
		if e := os.WriteFile(path, data, 0644); e != nil { fmt.Fprintln(os.Stderr, e); return 2 }
	} else { fmt.Println(string(data)) }
	if err != nil || result.Status != "PASS" { return 1 }
	return 0
}

func manualFile(w http.ResponseWriter) {
	exe, err := os.Executable()
	if err != nil { http.Error(w, "manual unavailable", 404); return }
	p := filepath.Join(filepath.Dir(exe), "TiAlloyStudio-Manual.docx")
	data, err := os.ReadFile(p)
	if err != nil { http.Error(w, "manual unavailable", 404); return }
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", `attachment; filename="TiAlloyStudio-Manual.docx"`)
	_, _ = w.Write(data)
}

func runEngineSmoke(path string) int {
	st := app.NewState()
	r, err := st.Build(app.BuildRequest{Module: "crystal", Phase: "alpha", NX: 2, NY: 2, NZ: 2})
	payload := map[string]any{"status": "PASS", "engines": r.Engines}
	if err != nil { payload["status"] = "FAIL"; payload["error"] = err.Error() }
	if len(r.Engines) < 2 { payload["status"] = "FAIL"; payload["error"] = "managed engine reports missing" }
	for _, er := range r.Engines {
		if er.Status != "PASS" { payload["status"] = "FAIL"; payload["error"] = fmt.Sprintf("%s: %s", er.Name, er.Message); break }
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	if path != "" {
		if e := os.WriteFile(path, data, 0644); e != nil { fmt.Fprintln(os.Stderr, e); return 2 }
	} else { fmt.Println(string(data)) }
	if payload["status"] != "PASS" { return 1 }
	return 0
}

func main() {
	smoke := flag.Bool("smoke-test", false, "run scientific smoke test and exit")
	smokeFile := flag.String("smoke-test-file", "", "write native smoke-test JSON to file")
	engineSmokeFile := flag.String("engine-smoke-file", "", "write managed-engine cross-check JSON to file")
	noBrowser := flag.Bool("no-browser", false, "start server without launching browser")
	flag.Parse()
	if *smoke || *smokeFile != "" { os.Exit(runSmoke(*smokeFile)) }
	if *engineSmokeFile != "" { os.Exit(runEngineSmoke(*engineSmokeFile)) }

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { log.Fatal(err) }
	state := app.NewState()
	base := webapp.New(state)
	var lastBeat atomic.Int64
	var seen atomic.Bool
	started := time.Now()
	var srv *http.Server
	browserClose := func() {}
	var browserOnce sync.Once
	closeBrowser := func() { browserOnce.Do(browserClose) }

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/heartbeat":
			if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
			seen.Store(true)
			lastBeat.Store(time.Now().UnixNano())
			w.WriteHeader(http.StatusNoContent)
			return
		case "/api/exit":
			if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusNoContent)
			if f, ok := w.(http.Flusher); ok { f.Flush() }
			go func() {
				time.Sleep(350 * time.Millisecond)
				closeBrowser()
				_ = srv.Close()
			}()
			return
		case "/manual":
			manualFile(w)
			return
		default:
			base.ServeHTTP(w, r)
		}
	})

	srv = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for range t.C {
			if seen.Load() {
				if time.Since(time.Unix(0, lastBeat.Load())) > 35*time.Second {
					closeBrowser(); _ = srv.Close(); return
				}
			} else if time.Since(started) > 2*time.Minute {
				closeBrowser(); _ = srv.Close(); return
			}
		}
	}()

	url := "http://" + ln.Addr().String() + "/"
	fmt.Println("Ti Alloy Studio:", url)
	if !*noBrowser {
		browserClose = openBrowser(url)
		// Rebind the once-protected closure after the session exists.
		browserOnce = sync.Once{}
		closeBrowser = func() { browserOnce.Do(browserClose) }
	}
	defer closeBrowser()
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed { log.Fatal(err) }
	_ = strconv.IntSize // keep architecture-specific build diagnostics explicit to the compiler
}

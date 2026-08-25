package engines

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

type EnvironmentTool struct {
	Name    string `json:"name"`
	Scope   string `json:"scope"`
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

type EnvironmentReport struct {
	HostOS         string            `json:"host_os"`
	HostArch       string            `json:"host_arch"`
	WSLAvailable   bool              `json:"wsl_available"`
	WSLDistros     []string          `json:"wsl_distros,omitempty"`
	SelectedDistro string            `json:"selected_distro,omitempty"`
	Tools          []EnvironmentTool `json:"tools"`
}

func parseToolPathLines(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, "\x00", ""))
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return out
}

func buildWSLProbeScript(toolNames []string) string {
	var script strings.Builder
	for _, name := range toolNames {
		q := quoteBash(name)
		// command -v is authoritative when the tool is on PATH. If it is not,
		// search a bounded set of common user/software locations without changing PATH.
		fmt.Fprintf(&script,
			"p=$(command -v %s 2>/dev/null || true); "+
				"if [ -z \"$p\" ]; then p=$(find \"$HOME\" /usr/local /opt -maxdepth 5 -type f -name %s -perm -u+x -print -quit 2>/dev/null || true); fi; "+
				"printf '%%s|%%s\\n' %s \"$p\"; ",
			q, q, q)
	}
	return script.String()
}

func chooseWSLDistro(distros []string, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if len(distros) == 0 {
			return ""
		}
		return distros[0]
	}
	for _, d := range distros {
		if d == requested {
			return d
		}
	}
	return ""
}

func toolStatus(name, scope, path, version, message string) EnvironmentTool {
	status := "UNAVAILABLE"
	if strings.TrimSpace(path) != "" {
		status = "AVAILABLE"
	}
	return EnvironmentTool{Name: name, Scope: scope, Status: status, Path: path, Version: version, Message: message}
}

func safeVersion(exe string, args ...string) string {
	if strings.TrimSpace(exe) == "" {
		return ""
	}
	b, err := exec.Command(exe, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(b))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if len(line) > 160 {
		line = line[:160]
	}
	return line
}

func DetectEnvironment(requestedDistro string) EnvironmentReport {
	report := EnvironmentReport{HostOS: runtime.GOOS, HostArch: runtime.GOARCH}

	managedPython := filepathJoin(engineDir(), "python", "python.exe")
	managedAtomsk := filepathJoin(engineDir(), "atomsk", "atomsk.exe")
	pythonPath := ""
	atomskPath := ""
	if _, err := os.Stat(managedPython); err == nil {
		pythonPath = managedPython
	}
	if _, err := os.Stat(managedAtomsk); err == nil {
		atomskPath = managedAtomsk
	}
	report.Tools = append(report.Tools,
		toolStatus("Bundled Python", "managed", pythonPath, safeVersion(pythonPath, "--version"), "Ti Alloy Studio private offline runtime"),
		toolStatus("Bundled Atomsk", "managed", atomskPath, safeVersion(atomskPath, "--version"), "Ti Alloy Studio private offline Atomsk"),
	)

	if lmp, err := exec.LookPath("lmp.exe"); err == nil {
		report.Tools = append(report.Tools, toolStatus("LAMMPS", "Windows PATH", lmp, safeVersion(lmp, "-h"), "Detected only; Ti Alloy Studio does not select a potential automatically"))
	} else if lmp, err := exec.LookPath("lmp"); err == nil {
		report.Tools = append(report.Tools, toolStatus("LAMMPS", "host PATH", lmp, safeVersion(lmp, "-h"), "Detected only; Ti Alloy Studio does not select a potential automatically"))
	} else {
		report.Tools = append(report.Tools, toolStatus("LAMMPS", "host PATH", "", "", "Optional external tool not found on PATH"))
	}

	wslExe, err := exec.LookPath("wsl.exe")
	if err != nil {
		report.Tools = append(report.Tools, toolStatus("WSL", "Windows", "", "", "wsl.exe not found; ATAT/mcsqs integration is unavailable until WSL is configured"))
		return report
	}
	report.WSLAvailable = true
	report.Tools = append(report.Tools, toolStatus("WSL", "Windows", wslExe, "", "Read-only environment detection"))

	listOut, listErr := exec.Command(wslExe, "-l", "-q").CombinedOutput()
	if listErr != nil {
		report.Tools = append(report.Tools, EnvironmentTool{Name: "WSL distributions", Scope: "Windows", Status: "UNAVAILABLE", Message: fmt.Sprintf("wsl.exe -l -q failed: %v", listErr)})
		return report
	}
	report.WSLDistros = ParseWSLDistros(string(listOut))
	report.SelectedDistro = chooseWSLDistro(report.WSLDistros, requestedDistro)
	if strings.TrimSpace(requestedDistro) != "" && report.SelectedDistro == "" {
		report.Tools = append(report.Tools, EnvironmentTool{Name: "Requested WSL distro", Scope: "WSL", Status: "UNAVAILABLE", Message: fmt.Sprintf("Requested distro %q is not installed", requestedDistro)})
		return report
	}
	if report.SelectedDistro == "" {
		report.Tools = append(report.Tools, EnvironmentTool{Name: "WSL distribution", Scope: "WSL", Status: "UNAVAILABLE", Message: "No WSL distribution was found"})
		return report
	}

	toolNames := []string{"python3", "atomsk", "mcsqs", "corrdump", "lmp", "lmp_serial", "lmp_mpi", "gpumd", "nep", "vasp_std"}
	script := buildWSLProbeScript(toolNames)
	args := append(wslPrefix(report.SelectedDistro), "--", "bash", "-lc", script)
	pathsOut, pathsErr := exec.Command(wslExe, args...).CombinedOutput()
	paths := map[string]string{}
	if pathsErr == nil {
		paths = parseToolPathLines(string(pathsOut))
	}
	for _, name := range toolNames {
		message := "Optional external tool not found in selected WSL distribution (PATH and bounded common-location search checked)"
		if name == "mcsqs" || name == "corrdump" {
			message = "ATAT tool; required for the ATAT SQS backend"
		}
		if name == "python3" {
			message = "External WSL Python; not required by the bundled offline core"
		}
		report.Tools = append(report.Tools, toolStatus(name, "WSL:"+report.SelectedDistro, paths[name], "", message))
	}
	sort.SliceStable(report.Tools, func(i, j int) bool {
		if report.Tools[i].Scope == report.Tools[j].Scope {
			return report.Tools[i].Name < report.Tools[j].Name
		}
		return report.Tools[i].Scope < report.Tools[j].Scope
	})
	return report
}

// Kept as a tiny wrapper to centralize path construction for environment tests/builds.
func filepathJoin(parts ...string) string {
	return strings.Join(parts, string(os.PathSeparator))
}

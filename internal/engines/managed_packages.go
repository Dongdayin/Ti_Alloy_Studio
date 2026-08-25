package engines

import (
	"os/exec"
	"strings"
)

func parseManagedPackageLines(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) != 3 {
			continue
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[2])
	}
	return out
}

func managedPackageTools(python string) []EnvironmentTool {
	if strings.TrimSpace(python) == "" {
		return nil
	}
	code := `import importlib.metadata as m
items=[('ASE','ase'),('spglib','spglib'),('pymatgen','pymatgen-core'),('AtomMan','atomman'),('NumPy','numpy'),('SciPy','scipy')]
for display,dist in items:
    try: print(f'{display}|{dist}|{m.version(dist)}')
    except m.PackageNotFoundError: print(f'{display}|{dist}|')`
	b, err := exec.Command(python, "-c", code).CombinedOutput()
	versions := map[string]string{}
	if err == nil {
		versions = parseManagedPackageLines(string(b))
	}
	out := make([]EnvironmentTool, 0, 6)
	for _, name := range []string{"ASE", "spglib", "pymatgen", "AtomMan", "NumPy", "SciPy"} {
		v := strings.TrimSpace(versions[name])
		status := "UNAVAILABLE"
		message := "Bundled Python package metadata not available"
		if v != "" {
			status = "AVAILABLE"
			message = "Ti Alloy Studio private offline Python science package"
		}
		out = append(out, EnvironmentTool{Name: "Bundled " + name, Scope: "managed", Status: status, Path: python, Version: v, Message: message})
	}
	return out
}

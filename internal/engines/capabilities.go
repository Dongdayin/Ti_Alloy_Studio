package engines

import (
	"os"
	"runtime"
)

type Capability struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message,omitempty"`
}

type CapabilityReport struct {
	HostOS       string       `json:"host_os"`
	HostArch     string       `json:"host_arch"`
	Capabilities []Capability `json:"capabilities"`
}

func managedCapability(id, name, relativePath, message string) Capability {
	p := filepathJoin(append([]string{engineDir()}, splitPath(relativePath)...)...)
	status := "NOT_INSTALLED"
	if _, err := os.Stat(p); err == nil {
		status = "AVAILABLE"
	}
	return Capability{ID: id, Name: name, Category: "built_in", Status: status, Required: true, Path: p, Message: message}
}

func splitPath(p string) []string {
	if p == "python/python.exe" {
		return []string{"python", "python.exe"}
	}
	return []string{"atomsk", "atomsk.exe"}
}

// DetectCapabilities reports the self-contained modeling features. It never
// probes WSL, PATH, or locally installed production solvers.
func DetectCapabilities() CapabilityReport {
	capabilities := []Capability{
		{ID: "native_modeling", Name: "Crystal and alloy modeling", Category: "built_in", Status: "AVAILABLE", Required: true, Message: "Native offline structure generator"},
		{ID: "native_sqs", Name: "Native SQS modeling", Category: "built_in", Status: "AVAILABLE", Required: true, Message: "Deterministic correlation-based structure modeling"},
		{ID: "revision_projects", Name: "Revision projects", Category: "built_in", Status: "AVAILABLE", Required: true, Message: "Edit, branch, save, reopen and re-export immutable model revisions"},
		managedCapability("bundled_python", "Bundled scientific Python", "python/python.exe", "Private offline ASE/spglib/pymatgen/AtomMan validation runtime"),
		managedCapability("bundled_atomsk", "Bundled Atomsk", "atomsk/atomsk.exe", "Private offline independent structure engine"),
	}
	for _, item := range []struct{ id, name string }{
		{"poscar", "VASP POSCAR"}, {"xyz", "XYZ"}, {"extxyz", "Extended XYZ"},
		{"lammps_data", "LAMMPS data"}, {"cif", "CIF"},
	} {
		capabilities = append(capabilities, Capability{ID: item.id, Name: item.name, Category: "export_format", Status: "SUPPORTED", Required: true})
	}
	for _, item := range []struct{ id, name string }{
		{"atat", "ATAT connector"}, {"lammps_runner", "LAMMPS runner"},
		{"gpumd", "GPUMD runner"}, {"vasp", "VASP runner"},
	} {
		capabilities = append(capabilities, Capability{ID: item.id, Name: item.name, Category: "external_connector", Status: "NOT_CONFIGURED", Required: false, Message: "Optional legacy connector; not required for modeling or export"})
	}
	return CapabilityReport{HostOS: runtime.GOOS, HostArch: runtime.GOARCH, Capabilities: capabilities}
}

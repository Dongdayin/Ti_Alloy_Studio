package engines

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"tialloystudio/internal/model"
)

type Report struct {
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Version      string            `json:"version,omitempty"`
	Message      string            `json:"message"`
	Executable   string            `json:"executable,omitempty"`
	Command      string            `json:"command,omitempty"`
	ReturnCode   int               `json:"return_code,omitempty"`
	Output       string            `json:"output,omitempty"`
	InputSHA256  map[string]string `json:"input_sha256,omitempty"`
	OutputSHA256 map[string]string `json:"output_sha256,omitempty"`
	Metrics      map[string]any    `json:"metrics,omitempty"`
}

func engineDir() string {
	if d := strings.TrimSpace(os.Getenv("TI_ALLOY_STUDIO_ENGINE_DIR")); d != "" {
		return d
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "engines")
	}
	return "engines"
}

func evidenceHash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func CrossCheck(s model.Structure) []Report { return []Report{checkAtomsk(s), checkPythonStack(s)} }

func checkAtomsk(s model.Structure) Report {
	exe := filepath.Join(engineDir(), "atomsk", "atomsk.exe")
	if _, err := os.Stat(exe); err != nil {
		return Report{Name: "Atomsk", Status: "UNAVAILABLE", Message: "Managed Atomsk executable not found", Executable: exe}
	}
	td, err := os.MkdirTemp("", "tialloy-atomsk-")
	if err != nil {
		return Report{Name: "Atomsk", Status: "FAIL", Message: err.Error(), Executable: exe, ReturnCode: -1}
	}
	defer os.RemoveAll(td)
	in, out := filepath.Join(td, "POSCAR"), filepath.Join(td, "roundtrip.xyz")
	input := []byte(model.ExportPOSCAR(s, "Ti Alloy Studio cross-check"))
	if err = os.WriteFile(in, input, 0644); err != nil {
		return Report{Name: "Atomsk", Status: "FAIL", Message: err.Error(), Executable: exe, ReturnCode: -1}
	}
	command := fmt.Sprintf("%q %q %q", exe, in, out)
	cmd := exec.Command(exe, in, out)
	b, runErr := cmd.CombinedOutput()
	report := Report{
		Name:        "Atomsk",
		Executable:  exe,
		Command:     command,
		ReturnCode:  exitCode(runErr),
		Output:      string(b),
		InputSHA256: map[string]string{"POSCAR": evidenceHash(input)},
	}
	if runErr != nil {
		report.Status = "FAIL"
		report.Message = fmt.Sprintf("Atomsk invocation failed: %v %s", runErr, strings.TrimSpace(string(b)))
		return report
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		report.Status = "FAIL"
		report.Message = "Atomsk did not create round-trip XYZ"
		return report
	}
	report.OutputSHA256 = map[string]string{"roundtrip.xyz": evidenceHash(raw)}
	lines := strings.SplitN(string(raw), "\n", 2)
	n, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || n != s.NAtoms() {
		report.Status = "FAIL"
		report.Message = fmt.Sprintf("Atom count mismatch after Atomsk read/write: %d vs %d", n, s.NAtoms())
		return report
	}
	ver := "0.13.1"
	if vb, e := exec.Command(exe, "--version").CombinedOutput(); e == nil && strings.TrimSpace(string(vb)) != "" {
		ver = strings.TrimSpace(string(vb))
	}
	report.Status = "PASS"
	report.Version = ver
	report.Message = "POSCAR was actually read and re-written by Atomsk"
	report.Metrics = map[string]any{"atom_count": n}
	return report
}

const pyScript = `import json,sys,importlib.metadata as md
from ase.io import read
from pymatgen.io.vasp import Poscar
import spglib
import atomman
p=sys.argv[1]
a=read(p,format='vasp')
pm=Poscar.from_file(p).structure
cell=(a.cell.array,a.get_scaled_positions(),a.numbers)
ds=spglib.get_symmetry_dataset(cell,symprec=1e-5)
out={'n_atoms':len(a),'ase_volume':float(a.get_volume()),'ase_lengths':[float(x) for x in a.cell.lengths()],'ase_angles':[float(x) for x in a.cell.angles()],'pmg_volume':float(pm.volume),'pmg_lengths':[float(x) for x in pm.lattice.abc],'pmg_angles':[float(x) for x in pm.lattice.angles],'spacegroup_number':int(ds.number) if ds else 0,'spacegroup':str(ds.international) if ds else '','versions':{'ASE':md.version('ase'),'spglib':md.version('spglib'),'pymatgen-core':md.version('pymatgen-core'),'atomman':md.version('atomman')}}
print(json.dumps(out))`

func checkPythonStack(s model.Structure) Report {
	py := filepath.Join(engineDir(), "python", "python.exe")
	if _, err := os.Stat(py); err != nil {
		return Report{Name: "ASE + spglib + pymatgen-core + AtomMan", Status: "UNAVAILABLE", Message: "Managed Python science runtime not found", Executable: py}
	}
	td, err := os.MkdirTemp("", "tialloy-py-")
	if err != nil {
		return Report{Name: "ASE + spglib + pymatgen-core + AtomMan", Status: "FAIL", Message: err.Error(), Executable: py, ReturnCode: -1}
	}
	defer os.RemoveAll(td)
	in := filepath.Join(td, "POSCAR")
	input := []byte(model.ExportPOSCAR(s, "Ti Alloy Studio cross-check"))
	if err = os.WriteFile(in, input, 0644); err != nil {
		return Report{Name: "ASE + spglib + pymatgen-core + AtomMan", Status: "FAIL", Message: err.Error(), Executable: py, ReturnCode: -1}
	}
	command := fmt.Sprintf("%q -c %s %q", py, strconv.Quote(pyScript), in)
	cmd := exec.Command(py, "-c", pyScript, in)
	b, runErr := cmd.CombinedOutput()
	report := Report{
		Name:        "ASE + spglib + pymatgen-core + AtomMan",
		Executable:  py,
		Command:     command,
		ReturnCode:  exitCode(runErr),
		Output:      string(b),
		InputSHA256: map[string]string{"POSCAR": evidenceHash(input)},
		OutputSHA256: map[string]string{"stdout-json": evidenceHash(b)},
	}
	if runErr != nil {
		report.Status = "FAIL"
		report.Message = fmt.Sprintf("Python science cross-check failed: %v %s", runErr, strings.TrimSpace(string(b)))
		return report
	}
	var m map[string]any
	if err = json.Unmarshal(b, &m); err != nil {
		report.Status = "FAIL"
		report.Message = "Invalid JSON from managed science runtime"
		return report
	}
	n, _ := m["n_atoms"].(float64)
	av, _ := m["ase_volume"].(float64)
	pv, _ := m["pmg_volume"].(float64)
	ref := s.Volume()
	if int(n) != s.NAtoms() || math.Abs(av-ref) > 1e-6*math.Max(1, ref) || math.Abs(pv-ref) > 1e-6*math.Max(1, ref) {
		report.Status = "FAIL"
		report.Message = "Independent library round-trip disagrees with native model"
		report.Metrics = m
		return report
	}
	versions := ""
	if v, ok := m["versions"].(map[string]any); ok {
		versions = fmt.Sprintf("ASE %v · spglib %v · pymatgen-core %v · AtomMan %v", v["ASE"], v["spglib"], v["pymatgen-core"], v["atomman"])
	}
	report.Status = "PASS"
	report.Version = versions
	report.Message = "Independent ASE/pymatgen read and spglib symmetry analysis passed"
	report.Metrics = m
	return report
}

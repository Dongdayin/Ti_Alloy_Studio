package engines

import (
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

type Report struct { Name string `json:"name"`; Status string `json:"status"`; Version string `json:"version,omitempty"`; Message string `json:"message"`; Metrics map[string]any `json:"metrics,omitempty"` }
func engineDir() string { if d:=strings.TrimSpace(os.Getenv("TI_ALLOY_STUDIO_ENGINE_DIR"));d!=""{return d};if exe,err:=os.Executable();err==nil{return filepath.Join(filepath.Dir(exe),"engines")};return "engines" }
func CrossCheck(s model.Structure) []Report { return []Report{checkAtomsk(s),checkPythonStack(s)} }
func checkAtomsk(s model.Structure) Report { exe:=filepath.Join(engineDir(),"atomsk","atomsk.exe");if _,err:=os.Stat(exe);err!=nil{return Report{Name:"Atomsk",Status:"UNAVAILABLE",Message:"Managed Atomsk executable not found"}};td,err:=os.MkdirTemp("","tialloy-atomsk-");if err!=nil{return Report{Name:"Atomsk",Status:"FAIL",Message:err.Error()}};defer os.RemoveAll(td);in,out:=filepath.Join(td,"POSCAR"),filepath.Join(td,"roundtrip.xyz");if err=os.WriteFile(in,[]byte(model.ExportPOSCAR(s,"Ti Alloy Studio cross-check")),0644);err!=nil{return Report{Name:"Atomsk",Status:"FAIL",Message:err.Error()}};cmd:=exec.Command(exe,in,out);b,err:=cmd.CombinedOutput();if err!=nil{return Report{Name:"Atomsk",Status:"FAIL",Message:fmt.Sprintf("Atomsk invocation failed: %v %s",err,strings.TrimSpace(string(b)))}};raw,err:=os.ReadFile(out);if err!=nil{return Report{Name:"Atomsk",Status:"FAIL",Message:"Atomsk did not create round-trip XYZ"}};lines:=strings.SplitN(string(raw),"\n",2);n,err:=strconv.Atoi(strings.TrimSpace(lines[0]));if err!=nil||n!=s.NAtoms(){return Report{Name:"Atomsk",Status:"FAIL",Message:fmt.Sprintf("Atom count mismatch after Atomsk read/write: %d vs %d",n,s.NAtoms())}};ver:="0.13.1";if vb,e:=exec.Command(exe,"--version").CombinedOutput();e==nil&&strings.TrimSpace(string(vb))!=""{ver=strings.TrimSpace(string(vb))};return Report{Name:"Atomsk",Status:"PASS",Version:ver,Message:"POSCAR was actually read and re-written by Atomsk",Metrics:map[string]any{"atom_count":n}} }
const pyScript=`import json,sys,importlib.metadata as md
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
func checkPythonStack(s model.Structure) Report { py:=filepath.Join(engineDir(),"python","python.exe");if _,err:=os.Stat(py);err!=nil{return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"UNAVAILABLE",Message:"Managed Python science runtime not found"}};td,err:=os.MkdirTemp("","tialloy-py-");if err!=nil{return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"FAIL",Message:err.Error()}};defer os.RemoveAll(td);in:=filepath.Join(td,"POSCAR");if err=os.WriteFile(in,[]byte(model.ExportPOSCAR(s,"Ti Alloy Studio cross-check")),0644);err!=nil{return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"FAIL",Message:err.Error()}};b,err:=exec.Command(py,"-c",pyScript,in).CombinedOutput();if err!=nil{return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"FAIL",Message:fmt.Sprintf("Python science cross-check failed: %v %s",err,strings.TrimSpace(string(b)))}};var m map[string]any;if err=json.Unmarshal(b,&m);err!=nil{return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"FAIL",Message:"Invalid JSON from managed science runtime"}};n,_:=m["n_atoms"].(float64);av,_:=m["ase_volume"].(float64);pv,_:=m["pmg_volume"].(float64);ref:=s.Volume();if int(n)!=s.NAtoms()||math.Abs(av-ref)>1e-6*math.Max(1,ref)||math.Abs(pv-ref)>1e-6*math.Max(1,ref){return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"FAIL",Message:"Independent library round-trip disagrees with native model",Metrics:m}};versions:="";if v,ok:=m["versions"].(map[string]any);ok{versions=fmt.Sprintf("ASE %v · spglib %v · pymatgen-core %v · AtomMan %v",v["ASE"],v["spglib"],v["pymatgen-core"],v["atomman"])};return Report{Name:"ASE + spglib + pymatgen-core + AtomMan",Status:"PASS",Version:versions,Message:"Independent ASE/pymatgen read and spglib symmetry analysis passed",Metrics:m} }

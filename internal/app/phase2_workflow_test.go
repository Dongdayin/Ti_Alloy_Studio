package app

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func requireGeometryOnlyResponse(t *testing.T, res BuildResponse) {
	t.Helper()
	if res.Structure.Meta["scientific_state"] != "not_relaxed" {
		t.Fatalf("%s scientific_state = %v, want not_relaxed", res.Module, res.Structure.Meta["scientific_state"])
	}
	if res.Structure.Meta["calculation_state"] != "not_calculated" {
		t.Fatalf("%s calculation_state = %v, want not_calculated", res.Module, res.Structure.Meta["calculation_state"])
	}
	for _, bag := range []map[string]any{res.Analysis, res.Series} {
		for key := range bag {
			lower := strings.ToLower(key)
			for _, forbidden := range []string{"energy", "force", "stress", "gamma_value", "stable_fault"} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("%s exposes calculated quantity %q", res.Module, key)
				}
			}
		}
	}
}

func TestPhase2BuildUserCreatesModelingOperationsFromTitaniumAlloyBase(t *testing.T) {
	st := NewState()
	req := BuildRequest{
		Module:        "dislocation",
		AlloyMode:     "random",
		Phase:         "alpha",
		NX:            5,
		NY:            5,
		NZ:            4,
		CompositionWt: map[string]float64{"Al": 6, "V": 4},
		Seed:          91,
		SlipSystem:    "alpha_basal_a",
		LineDirection: "screw",
	}
	res, err := st.BuildUser(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Module != "dislocation" {
		t.Fatalf("module = %q, want dislocation", res.Module)
	}
	if res.Structure.SpeciesCounts()["Al"] == 0 || res.Structure.SpeciesCounts()["V"] == 0 {
		t.Fatalf("operation did not start from requested alloy composition: %v", res.Structure.SpeciesCounts())
	}
	if res.Analysis["burgers_vector"] == nil || res.Analysis["slip_plane_normal"] == nil || res.Analysis["line_direction"] == nil {
		t.Fatalf("dislocation geometry diagnostics missing: %v", res.Analysis)
	}
	requireGeometryOnlyResponse(t, res)

	for _, req := range []BuildRequest{
		{Module: "grain_boundary", Phase: "beta", NX: 4, NY: 4, NZ: 4, GBAngleDeg: 10, GBAxis: "[001]"},
		{Module: "twin", Phase: "alpha", NX: 4, NY: 4, NZ: 5, TwinSystem: "alpha_10-12"},
		{Module: "local_chemistry", AlloyMode: "crystal", Phase: "alpha", NX: 4, NY: 4, NZ: 4, ClusterSpec: "Al:6:center", Seed: 44},
		{Module: "crack", Phase: "beta", NX: 5, NY: 5, NZ: 4, CrackSpec: "plane=(010),front=[001],length=6"},
		{Module: "nanoindentation", Phase: "alpha", NX: 5, NY: 5, NZ: 4, IndenterSpec: "sphere,radius=8,depth=1.5"},
		{Module: "polycrystal", Phase: "beta", NX: 5, NY: 5, NZ: 5, GrainCount: 4, Seed: 19},
		{Module: "neb", Phase: "alpha", NX: 4, NY: 4, NZ: 4, SeriesCount: 3},
		{Module: "training_set", Phase: "alpha", NX: 3, NY: 3, NZ: 3, SeriesCount: 4},
	} {
		res, err := st.BuildUser(req)
		if err != nil {
			t.Fatalf("%s build failed: %v", req.Module, err)
		}
		if res.Module != req.Module {
			t.Fatalf("module = %q, want %q", res.Module, req.Module)
		}
		if res.Structure.NAtoms() == 0 {
			t.Fatalf("%s produced empty structure", req.Module)
		}
		requireGeometryOnlyResponse(t, res)
	}
}

func TestPhase2RequestControlsDislocationVectorsCrackPlaneAndTrainingExtXYZ(t *testing.T) {
	st := NewState()
	disl, err := st.BuildUser(BuildRequest{
		Module:                 "dislocation",
		Phase:                  "beta",
		NX:                     5,
		NY:                     5,
		NZ:                     5,
		SlipSystem:             "beta_110_111",
		BurgersVector:          "0 0 2.5",
		LineDirection:          "0 1 0",
		DislocationCharacter:   "mixed",
		DislocationArrangement: "quadrupole",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := disl.Analysis["dislocation_core_count"]; got != 4 {
		t.Fatalf("analysis core count = %v, want 4", got)
	}
	helpers, ok := disl.Analysis["viewer_helpers"].(map[string]any)
	if !ok || helpers["burgers_vector"] == nil || helpers["line_direction"] == nil || helpers["slip_plane_normal"] == nil {
		t.Fatalf("3D viewer helper vectors missing: %v", disl.Analysis["viewer_helpers"])
	}
	requireGeometryOnlyResponse(t, disl)

	crack, err := st.BuildUser(BuildRequest{
		Module:    "crack",
		Phase:     "alpha",
		NX:        5,
		NY:        5,
		NZ:        4,
		CrackSpec: "plane=(10-10),front=[11-20],length=7,opening=1.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if crack.Analysis["crack_plane"] != "(10-10)" || crack.Analysis["crack_front"] != "[11-20]" {
		t.Fatalf("crack plane/front did not come from crack_spec: %v", crack.Analysis)
	}
	requireGeometryOnlyResponse(t, crack)

	if _, err = st.BuildUser(BuildRequest{Module: "training_set", Phase: "alpha", NX: 3, NY: 3, NZ: 3, SeriesCount: 2, DatasetKind: "nep"}); err != nil {
		t.Fatal(err)
	}
	name, mime, data, err := st.ExportBatch("extxyz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, ".zip") || mime != "application/zip" {
		t.Fatalf("training-set export identity = %q %q", name, mime)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	extXYZCount := 0
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".extxyz") {
			continue
		}
		extXYZCount++
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("calculation_state=not_calculated")) {
			t.Fatalf("%s missing not_calculated metadata:\n%s", f.Name, body)
		}
		for _, forbidden := range []string{"energy", "forces", "stress"} {
			if bytes.Contains(bytes.ToLower(body), []byte(forbidden)) {
				t.Fatalf("%s contains calculated field %q:\n%s", f.Name, forbidden, body)
			}
		}
	}
	if extXYZCount != 5 {
		t.Fatalf("extXYZ file count = %d, want 5", extXYZCount)
	}
}

func TestPhase2RequestControlsGrainBoundaryOrientation(t *testing.T) {
	st := NewState()
	gb, err := st.BuildUser(BuildRequest{
		Module:            "grain_boundary",
		Phase:             "beta",
		NX:                5,
		NY:                5,
		NZ:                5,
		GBType:            "general",
		GBAxis:            "[010]",
		GBNormal:          "[010]",
		GBAngleDeg:        20,
		SurfacePreset:     "vacuum",
		Grain1Orientation: "1 0 0; 0 1 0; 0 0 1",
		Grain2Orientation: "0 -1 0; 1 0 0; 0 0 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gb.Analysis["gb_axis"] != "[010]" || gb.Analysis["gb_normal"] != "[010]" {
		t.Fatalf("GB axis/normal diagnostics missing: %v", gb.Analysis)
	}
	if got, _ := gb.Analysis["in_plane_periodic_matching_mismatch_percent"].(float64); got <= 0 {
		t.Fatalf("GB mismatch diagnostic = %.6g, want positive for rotated grains", got)
	}
	if got, _ := gb.Analysis["interface_count"].(int); got != 1 {
		t.Fatalf("interface_count = %v, want 1 for vacuum bicrystal", gb.Analysis["interface_count"])
	}
	requireGeometryOnlyResponse(t, gb)
}

func TestPhase2SeriesExportArchiveContainsManifestAndNoCalculatedFields(t *testing.T) {
	st := NewState()
	if _, err := st.BuildUser(BuildRequest{
		Module:      "stacking_fault",
		Phase:       "alpha",
		NX:          4,
		NY:          4,
		NZ:          5,
		GSFEPreset:  "alpha_basal_a",
		SeriesCount: 4,
	}); err != nil {
		t.Fatal(err)
	}
	name, mime, data, err := st.ExportBatch("poscar")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, ".zip") || mime != "application/zip" {
		t.Fatalf("archive identity = %q %q, want zip/application-zip", name, mime)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest string
	poscarCount := 0
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/POSCAR") {
			poscarCount++
		}
		if f.Name == "manifest.csv" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			manifest = string(b)
		}
	}
	if poscarCount != 5 {
		t.Fatalf("POSCAR count = %d, want 5", poscarCount)
	}
	for _, needle := range []string{"index,kind,lambda,shift_x_angstrom,shift_y_angstrom,shift_z_angstrom,atoms,pbc,path", "stacking_fault"} {
		if !strings.Contains(manifest, needle) {
			t.Fatalf("manifest missing %q:\n%s", needle, manifest)
		}
	}
	for _, forbidden := range []string{"energy", "force", "stress", "gamma_value", "stable_fault"} {
		if strings.Contains(strings.ToLower(manifest), forbidden) {
			t.Fatalf("manifest contains calculated field %q:\n%s", forbidden, manifest)
		}
	}
}

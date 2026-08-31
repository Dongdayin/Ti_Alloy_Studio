package webapp

import (
	"strings"
	"testing"
)

func TestWorkbenchExposesPhase2ModelingPagesWithoutCalculationProjectLanguage(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`data-module="dislocation"`,
		`data-module="grain_boundary"`,
		`data-module="stacking_fault"`,
		`data-module="twin"`,
		`data-module="local_chemistry"`,
		`data-module="crack"`,
		`data-module="nanoindentation"`,
		`data-module="polycrystal"`,
		`data-module="neb"`,
		`data-module="training_set"`,
		"位错",
		"晶界",
		"孪晶",
		"层错 / γ-surface 构型",
		"局域化学",
		"力学构型",
		"构型集导出",
	} {
		if !strings.Contains(page, needle) {
			t.Fatalf("phase 2 modeling UI missing %q", needle)
		}
	}
	for _, forbidden := range []string{"EOS 计算", "计算总能", "计算能量", "γ value", "stable fault"} {
		if strings.Contains(strings.ToLower(page), strings.ToLower(forbidden)) {
			t.Fatalf("primary UI still exposes calculation-project language %q", forbidden)
		}
	}
}

func TestWorkbenchSendsPhase2ControlsAndSupportsSeriesPackageExport(t *testing.T) {
	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"phase2ModuleKind",
		"slip_system",
		"burgers_vector",
		"line_direction",
		"gb_axis",
		"gb_angle_deg",
		"twin_system",
		"cluster_spec",
		"precipitate_spec",
		"crack_spec",
		"indenter_spec",
		"series_count",
		"/api/export-batch?format=poscar",
		"构型系列包已保存",
	} {
		if !strings.Contains(appJS, needle) {
			t.Fatalf("phase 2 UI behavior missing %q", needle)
		}
	}
	if strings.Contains(appJS, "Calculated total energies") || strings.Contains(appJS, "Apply energies") {
		t.Fatal("front end still exposes calculation-energy workflow")
	}
}

func TestWorkbenchExposesPhase2PrecisionControlsAndViewerHelpers(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`id="burgersVector"`,
		`id="lineDirection"`,
		`id="grain1Orientation"`,
		`id="grain2Orientation"`,
		`id="crackPlane"`,
		`id="crackFront"`,
		`id="trainingExportFormat"`,
		`id="validationMode"`,
		`id="calculationPackageTarget"`,
		`id="calculationWorkflowPreset"`,
		`id="vaspKpoints"`,
		`id="vaspEncut"`,
		`id="lammpsPairStyle"`,
		`id="lammpsPairCoeff"`,
		`id="gpumdEnsemble"`,
		`id="gpumdRunSteps"`,
		`id="calculationPackageBtn"`,
	} {
		if !strings.Contains(page, needle) {
			t.Fatalf("precision Phase 2 UI control missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"drawDirectionHelpers",
		"viewer_helpers",
		"burgers_vector: $('burgersVector')",
		"line_direction: $('lineDirection')",
		"grain_1_orientation",
		"grain_2_orientation",
		"trainingExportFormat",
		"validation_mode",
		"/api/calculation-package/save",
		"workflow_preset",
		"vasp_kpoints",
		"vasp_encut_ev",
		"lammps_pair_style",
		"gpumd_ensemble",
	} {
		if !strings.Contains(appJS, needle) {
			t.Fatalf("precision Phase 2 behavior missing %q", needle)
		}
	}
}

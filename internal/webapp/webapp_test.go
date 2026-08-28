package webapp

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"tialloystudio/internal/app"
)

func servedAsset(t *testing.T, path string) string {
	t.Helper()
	h := New(app.NewState())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	b, _ := io.ReadAll(w.Result().Body)
	return string(b)
}

func TestWorkbenchServesInteractiveUI(t *testing.T) {
	s := servedAsset(t, "/")
	for _, needle := range []string{"Ti Alloy Studio", "Crystal / Alloy", "SQS", "Defects / Surface", "α/β Interface", "EOS", "GSFE", "structureCanvas", "analysisCanvas", "validationPanel"} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %q", needle)
		}
	}
}

func TestWorkbenchUsesRepeatableBlobDownloadsAndInteractiveValues(t *testing.T) {
	s := servedAsset(t, "/app.js")
	if strings.Contains(s, "location.href=`/api/export") {
		t.Fatal("export still navigates the application away")
	}
	for _, needle := range []string{"URL.createObjectURL", "chartTooltip", "atomTooltip"} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %q", needle)
		}
	}
}

func TestWorkbenchResponsiveRevisionWorkflowAndBundledCapabilities(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{`class="mobileTabs"`, `data-mobile-panel="model"`, `data-mobile-panel="structure"`, `data-mobile-panel="validation"`, `data-mobile-panel="export"`, `id="activeRevisionLabel"`, "Offline modeling package", "Troubleshooting details"} {
		if !strings.Contains(page, needle) {
			t.Errorf("page missing %q", needle)
		}
	}
	css := servedAsset(t, "/style.css")
	if strings.Contains(css, "min-width:1024px") || strings.Contains(css, "min-width:940px") {
		t.Fatal("fixed body minimum width still blocks narrow-window controls")
	}
	compactCSS := strings.Join(strings.Fields(css), "")
	for _, needle := range []string{"@media(max-width:1099px)", "mobileTabs", "data-mobile-panel"} {
		if !strings.Contains(compactCSS, needle) {
			t.Errorf("responsive CSS missing %q", needle)
		}
	}
	project := servedAsset(t, "/project.js")
	for _, needle := range []string{"revisionHistory", "/api/project/select", "/api/project/edit", "/api/project/derive", ".tias-project"} {
		if !strings.Contains(project, needle) {
			t.Errorf("revision workflow missing %q", needle)
		}
	}
	appJS := servedAsset(t, "/app.js")
	if !strings.Contains(appJS, "/api/capabilities") || !strings.Contains(appJS, "revision_id") {
		t.Fatal("app does not use bundled capability catalog and revision-aware export")
	}
	if strings.Contains(appJS, "refreshEnvironment();") {
		t.Fatal("startup still automatically probes external WSL/solver environment")
	}
}

func TestInspectorPanelsDoNotShrinkOrCreateNestedScrollAreas(t *testing.T) {
	css := strings.Join(strings.Fields(servedAsset(t, "/style.css")), "")
	for _, needle := range []string{
		`.inspector>.panel{flex:00auto`,
		`.checks,.enginePanel{display:flex;flex-direction:column;gap:6px;overflow:visible`,
	} {
		if !strings.Contains(css, needle) {
			t.Errorf("inspector overflow protection missing %q", needle)
		}
	}
}

func TestWorkbenchKeepsTechnicalDiagnosticsOutOfThePrimaryInterface(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{"Offline modeling package", "Troubleshooting details"} {
		if !strings.Contains(page, needle) {
			t.Errorf("simplified interface missing %q", needle)
		}
	}
	if strings.Contains(page, `id="versionBadge"`) {
		t.Error("technical version badge is still visible in the top bar")
	}
	if !strings.Contains(page, `id="diagnosticVersion"`) {
		t.Error("version information is not available inside troubleshooting details")
	}

	project := servedAsset(t, "/project.js")
	for _, obsolete := range []string{"UUID: ${manifest.project_uuid", "record.id.slice(0,8)", "record.scientific_state", "parent ${esc(record.parent_id", "${esc(record.created_at"} {
		if strings.Contains(project, obsolete) {
			t.Errorf("project history still exposes technical detail %q", obsolete)
		}
	}
	for _, needle := range []string{"Version ${", "Current", "Continue editing", "More actions"} {
		if !strings.Contains(project, needle) {
			t.Errorf("simplified revision history missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, obsolete := range []string{"item.message, item.path", "r.message)}${r.version"} {
		if strings.Contains(appJS, obsolete) {
			t.Errorf("main diagnostics still expose implementation detail %q", obsolete)
		}
	}
}

func TestPhaseSelectionRefreshesPhaseSpecificControls(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{`id="phaseHint"`, `id="latticeSummary"`, `data-phase-field="alpha"`, `data-phase-field="beta"`} {
		if !strings.Contains(page, needle) {
			t.Errorf("phase-aware form markup missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{"function updatePhaseControls", "$('phase').addEventListener('change', updatePhaseControls)", "syncPhaseOptions('surfacePreset'", "syncPhaseOptions('gsfePreset'"} {
		if !strings.Contains(appJS, needle) {
			t.Errorf("phase-aware form behavior missing %q", needle)
		}
	}

	css := strings.Join(strings.Fields(servedAsset(t, "/style.css")), "")
	if !strings.Contains(css, "[hidden]{display:none!important}") {
		t.Error("hidden phase-specific controls must stay hidden even when label display rules apply")
	}
}

func TestInterfaceRecipeUsesAlphaBetaSpecificControls(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`id="phaseControl"`,
		`id="interfaceOrientationPreset"`,
		`id="interfaceTopology"`,
		`id="interfaceMatchLimit"`,
		`id="interfaceVacuumLabel"`,
		`α (0001) ∥ β (110)`,
		`α [11-20] ∥ β [1-11]`,
	} {
		if !strings.Contains(page, needle) {
			t.Errorf("interface controls missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"function setSinglePhaseControlsVisible",
		"active !== 'interface'",
		"interfaceTopology",
		"interfaceVacuumLabel",
	} {
		if !strings.Contains(appJS, needle) {
			t.Errorf("interface behavior missing %q", needle)
		}
	}
}

func TestStructureViewerHasRenderModesAndAtomColorControls(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`id="renderMode"`,
		`value="element"`,
		`value="phase"`,
		`value="depth"`,
		`id="colorTi"`,
		`id="colorAl"`,
		`id="colorV"`,
		`id="colorAlpha"`,
		`id="colorBeta"`,
	} {
		if !strings.Contains(page, needle) {
			t.Errorf("3D viewer control missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"function currentAtomColor",
		"function bindColorControl",
		"$('renderMode').onchange",
		"bindColorControl('colorTi', 'Ti')",
		"bindColorControl('colorAlpha', 'alpha')",
	} {
		if !strings.Contains(appJS, needle) {
			t.Errorf("3D viewer behavior missing %q", needle)
		}
	}
}

func TestResultPanelsExposeReadableHighlightsInsteadOfRawMetricPlots(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`id="compositionHeadline"`,
		`id="analysisHeadline"`,
		`id="energyBox"`,
	} {
		if !strings.Contains(page, needle) {
			t.Errorf("result highlight markup missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"function analysisHeadlineText",
		"function setAnalysisChartVisible",
		"function updateEnergyBox",
		"excludedAnalysisMetrics",
	} {
		if !strings.Contains(appJS, needle) {
			t.Errorf("readable result behavior missing %q", needle)
		}
	}

	css := strings.Join(strings.Fields(servedAsset(t, "/style.css")), "")
	for _, needle := range []string{".chartHeadline{", "font-size:13px", ".analysisMuted"} {
		if !strings.Contains(css, needle) {
			t.Errorf("readability CSS missing %q", needle)
		}
	}
}

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
	for _, needle := range []string{"Ti Alloy Studio", "钛合金专属建模", "基础模型", "缺陷", "表面", "α/β 界面", "structureCanvas", "analysisCanvas", "validationPanel"} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %q", needle)
		}
	}
	if !strings.Contains(s, `rel="icon" href="data:,"`) {
		t.Fatal("page should suppress browser favicon 404 noise")
	}
	for _, obsolete := range []string{`data-module="eos"`, `data-module="gsfe"`, "EOS batch ZIP", "GSFE batch ZIP", "Calculated total energies"} {
		if strings.Contains(s, obsolete) {
			t.Fatalf("standalone project-calculation UI is still exposed: %q", obsolete)
		}
	}
}

func TestWorkbenchUsesSaveAsExportsAndInteractiveValues(t *testing.T) {
	s := servedAsset(t, "/app.js")
	if strings.Contains(s, "location.href=`/api/export") {
		t.Fatal("export still navigates the application away")
	}
	for _, needle := range []string{"/api/export/save", "showExportResult", "openExportFolder", "chartTooltip", "atomTooltip"} {
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
	for _, obsolete := range []string{"Version ${", "Current", "Continue editing", "More actions", "versions"} {
		if strings.Contains(project, obsolete) {
			t.Errorf("project history still uses old technical wording %q", obsolete)
		}
	}
	for _, needle := range []string{"结构 ${", "当前", "修改此结构", "操作"} {
		if !strings.Contains(project, needle) {
			t.Errorf("modeling history wording missing %q", needle)
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
	for _, needle := range []string{"function updatePhaseControls", "$('phase').addEventListener('change', updatePhaseControls)", "syncPhaseOptions('surfacePreset'"} {
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
	if strings.Contains(page, "This recipe always builds") || strings.Contains(page, "No single-phase selector") {
		t.Error("interface panel still contains unnecessary explanatory wording")
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
		`id="renderQuality"`,
		`id="exportPngBtn"`,
		`value="element"`,
		`value="phase"`,
		`value="depth"`,
		`value="quality"`,
		`value="publication"`,
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
		"function drawQualityAtom",
		"function exportStructurePNG",
		"function bindColorControl",
		"$('renderMode').onchange",
		"$('renderQuality').onchange",
		"bindColorControl('colorTi', 'Ti')",
		"bindColorControl('colorAlpha', 'alpha')",
	} {
		if !strings.Contains(appJS, needle) {
			t.Errorf("3D viewer behavior missing %q", needle)
		}
	}
}

func TestStructureViewerOffersBundledTachyonStyleRendering(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`value="tachyon"`,
		"Tachyon 风格",
		"内置光线追踪风格",
	} {
		if !strings.Contains(page, needle) {
			t.Errorf("Tachyon-style render control missing %q", needle)
		}
	}
	if strings.Contains(page, "tachyon.exe") {
		t.Fatal("Tachyon renderer must not require an external executable in the primary UI")
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"function drawTachyonBackdrop",
		"function drawTachyonAtom",
		"quality === 'tachyon'",
		"TiAlloyStudio-tachyon-style.png",
	} {
		if !strings.Contains(appJS, needle) {
			t.Errorf("Tachyon-style render behavior missing %q", needle)
		}
	}
	if strings.Contains(appJS, "tachyon.exe") || strings.Contains(appJS, "Start-Process") {
		t.Fatal("browser renderer must remain bundled and must not invoke an external Tachyon process")
	}
}

func TestResultPanelsExposeReadableHighlightsInsteadOfRawMetricPlots(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`id="compositionHeadline"`,
		`id="analysisHeadline"`,
	} {
		if !strings.Contains(page, needle) {
			t.Errorf("result highlight markup missing %q", needle)
		}
	}

	appJS := servedAsset(t, "/app.js")
	for _, needle := range []string{
		"function analysisHeadlineText",
		"function setAnalysisChartVisible",
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

func TestTitaniumAlloyWorkflowPutsCompositionBeforeOperations(t *testing.T) {
	page := servedAsset(t, "/")
	for _, needle := range []string{
		`id="titaniumAlloyControls"`,
		`id="baseCrystalControls"`,
		`id="operationHint"`,
		`id="alloyType"`,
		`value="crystal"`,
		`value="random"`,
		`value="sqs"`,
		"Ti 是基体元素",
		"不填写/不选择操作就跳过",
	} {
		if !strings.Contains(page, needle) {
			t.Errorf("workflow markup missing %q", needle)
		}
	}
	for _, obsolete := range []string{"Crystal / Alloy", "Pure phase crystal"} {
		if strings.Contains(page, obsolete) {
			t.Errorf("generic/non-titanium workflow wording remains %q", obsolete)
		}
	}
}

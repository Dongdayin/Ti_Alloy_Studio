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
	for _, needle := range []string{`class="mobileTabs"`, `data-mobile-panel="model"`, `data-mobile-panel="structure"`, `data-mobile-panel="validation"`, `data-mobile-panel="export"`, `id="activeRevisionLabel"`, "Bundled modeling capabilities", "Optional connectors"} {
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

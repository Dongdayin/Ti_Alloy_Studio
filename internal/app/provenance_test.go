package app

import (
	"strings"
	"testing"

	"tialloystudio/internal/model"
)

func TestProjectManifestRecordsBuildLineageAndDefersArtifactHashes(t *testing.T) {
	st := NewState()
	_, err := st.BuildTracked(BuildRequest{Module: "random", Phase: "alpha", NX: 2, NY: 2, NZ: 2, CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 11})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.BuildTracked(BuildRequest{Module: "random", Phase: "alpha", NX: 2, NY: 2, NZ: 2, CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 22})
	if err != nil {
		t.Fatal(err)
	}
	m := st.ProjectManifest("phase1-provenance-test")
	if m.SchemaVersion != 2 || m.ProjectID == "" || m.Name != "phase1-provenance-test" {
		t.Fatalf("invalid manifest identity: %+v", m)
	}
	if len(m.History) != 2 {
		t.Fatalf("history = %d, want 2", len(m.History))
	}
	first, second := m.History[0], m.History[1]
	if first.ParentID != "" || second.ParentID != first.ID {
		t.Fatalf("invalid lineage: first=%q second-parent=%q first-id=%q", first.ParentID, second.ParentID, first.ID)
	}
	if second.Request.Seed != 22 || second.Module != "random" {
		t.Fatalf("latest request not preserved: %+v", second.Request)
	}
	if len(second.StructureSHA256) != 64 {
		t.Fatalf("structure sha256 = %q", second.StructureSHA256)
	}
	if len(second.ExportSHA256) != 0 {
		t.Fatalf("build precomputed export hashes: %+v", second.ExportSHA256)
	}
	if len(second.Validation.Checks) == 0 {
		t.Fatal("validation report was not recorded")
	}

	_, _, content, err := st.ExportRevision(second.ID, "poscar")
	if err != nil {
		t.Fatal(err)
	}
	exported := st.ProjectManifest("").History[1].ExportSHA256["poscar"]
	if len(exported) != 64 || strings.Trim(exported, "0123456789abcdef") != "" || exported != sha256Text(content) {
		t.Fatalf("lazy poscar hash was not recorded as lowercase SHA-256: %q", exported)
	}
}

func TestLegacyProjectImportRebuildsOnceAndPreservesLineage(t *testing.T) {
	original := NewState()
	_, err := original.BuildTracked(BuildRequest{Module: "gsfe", Phase: "alpha", NX: 2, NY: 2, NZ: 4, GSFEPreset: "basal_a", GSFESteps: 8})
	if err != nil {
		t.Fatal(err)
	}
	m := original.ProjectManifest("restore-test")
	m.SchemaVersion = 1
	m.ActiveRevisionID = ""
	for i := range m.History {
		m.History[i].Structure = model.Structure{}
		m.History[i].ScientificState = ""
	}

	restored := NewState()
	res, err := restored.ImportProject(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.Module != "gsfe" {
		t.Fatalf("restored module = %q", res.Module)
	}
	m2 := restored.ProjectManifest("")
	if m2.ProjectID != m.ProjectID || m2.Name != "restore-test" {
		t.Fatalf("project identity not restored: %+v", m2)
	}
	if m2.SchemaVersion != 2 || len(m2.History) != len(m.History) {
		t.Fatalf("restored schema/history = %d/%d, want 2/%d", m2.SchemaVersion, len(m2.History), len(m.History))
	}
	last := m2.History[len(m2.History)-1]
	if last.ID != m.History[len(m.History)-1].ID || last.StructureSHA256 != m.History[len(m.History)-1].StructureSHA256 {
		t.Fatalf("legacy revision identity/hash changed: %+v", last)
	}
	if m2.ActiveRevisionID != last.ID || structureSHA256(res.Structure) != last.StructureSHA256 {
		t.Fatal("legacy import did not activate the rebuilt final revision")
	}
}

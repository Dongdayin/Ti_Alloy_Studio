package app

import (
	"testing"
)

func TestTrackedRevisionStoresExactImmutableSnapshotAndCanBeSelected(t *testing.T) {
	st := NewState()
	first, err := st.BuildTracked(BuildRequest{
		Module: "random", Phase: "alpha", NX: 2, NY: 2, NZ: 2,
		CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 11,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := st.ProjectManifest("")
	if len(manifest.History) != 1 {
		t.Fatalf("history = %d, want 1", len(manifest.History))
	}
	revision := manifest.History[0]
	if revision.ScientificState != "not_relaxed" {
		t.Fatalf("scientific state = %q, want not_relaxed", revision.ScientificState)
	}
	if structureSHA256(revision.Structure) != structureSHA256(first.Structure) {
		t.Fatal("tracked revision did not store the exact generated structure")
	}
	if manifest.ActiveRevisionID != revision.ID {
		t.Fatalf("active revision = %q, want %q", manifest.ActiveRevisionID, revision.ID)
	}

	// Mutating a value returned by ProjectManifest must not alias project state.
	revision.Structure.Species[0] = "X"
	revision.Request.CompositionWt["Al"] = 99
	again := st.ProjectManifest("").History[0]
	if again.Structure.Species[0] == "X" || again.Request.CompositionWt["Al"] == 99 {
		t.Fatal("returned revision aliases immutable project state")
	}

	_, err = st.BuildTracked(BuildRequest{
		Module: "random", Phase: "alpha", NX: 2, NY: 2, NZ: 2,
		CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 22,
	})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := st.SelectRevision(revision.ID)
	if err != nil {
		t.Fatal(err)
	}
	if structureSHA256(selected.Structure) != structureSHA256(first.Structure) {
		t.Fatal("selecting a revision did not restore its exact structure snapshot")
	}
	afterSelect := st.ProjectManifest("")
	if len(afterSelect.History) != 2 {
		t.Fatalf("selection mutated history length to %d", len(afterSelect.History))
	}
	if afterSelect.ActiveRevisionID != revision.ID {
		t.Fatalf("active revision after selection = %q", afterSelect.ActiveRevisionID)
	}
	if _, err = st.BuildTracked(BuildRequest{Module: "crystal", Phase: "beta", NX: 1, NY: 1, NZ: 1}); err != nil {
		t.Fatal(err)
	}
	branched := st.ProjectManifest("")
	if got := branched.History[len(branched.History)-1].ParentID; got != revision.ID {
		t.Fatalf("build after selection parent = %q, want selected revision %q", got, revision.ID)
	}
}

func TestBuildChildUsesExplicitParentAndFailureDoesNotMutateProject(t *testing.T) {
	st := NewState()
	_, err := st.BuildTracked(BuildRequest{Module: "crystal", Phase: "alpha", NX: 2, NY: 2, NZ: 2})
	if err != nil {
		t.Fatal(err)
	}
	firstID := st.ProjectManifest("").ActiveRevisionID
	_, err = st.BuildTracked(BuildRequest{Module: "crystal", Phase: "beta", NX: 2, NY: 2, NZ: 2})
	if err != nil {
		t.Fatal(err)
	}

	child, err := st.BuildChild(firstID, BuildRequest{Module: "random", Phase: "alpha", NX: 2, NY: 2, NZ: 2, Seed: 9})
	if err != nil {
		t.Fatal(err)
	}
	m := st.ProjectManifest("")
	last := m.History[len(m.History)-1]
	if last.ParentID != firstID {
		t.Fatalf("child parent = %q, want explicit parent %q", last.ParentID, firstID)
	}
	if structureSHA256(last.Structure) != structureSHA256(child.Structure) {
		t.Fatal("child snapshot differs from successful result")
	}

	before := st.ProjectManifest("")
	beforeCurrent := structureSHA256(child.Structure)
	if _, err = st.BuildChild(firstID, BuildRequest{Module: "does-not-exist"}); err == nil {
		t.Fatal("invalid child build unexpectedly succeeded")
	}
	after := st.ProjectManifest("")
	if len(after.History) != len(before.History) || after.ActiveRevisionID != before.ActiveRevisionID {
		t.Fatal("failed child build changed revision history or active revision")
	}
	st.mu.RLock()
	afterCurrent := structureSHA256(st.Current.Structure)
	st.mu.RUnlock()
	if afterCurrent != beforeCurrent {
		t.Fatal("failed child build changed the active structure")
	}
}

func TestDeriveRevisionAppliesDefectToSelectedSnapshot(t *testing.T) {
	st := NewState()
	parent, err := st.BuildTracked(BuildRequest{
		Module: "random", Phase: "alpha", NX: 2, NY: 2, NZ: 2,
		CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 17,
	})
	if err != nil {
		t.Fatal(err)
	}
	parentID := st.ProjectManifest("").ActiveRevisionID
	parentHash := structureSHA256(parent.Structure)

	derived, err := st.DeriveRevision(parentID, DeriveRequest{Operation: "substitution", SiteID: 0, NewSpecies: "Nb"})
	if err != nil {
		t.Fatal(err)
	}
	if derived.Structure.NAtoms() != parent.Structure.NAtoms() || derived.Structure.Species[0] != "Nb" {
		t.Fatalf("unexpected derived substitution: atoms=%d first=%q", derived.Structure.NAtoms(), derived.Structure.Species[0])
	}
	m := st.ProjectManifest("")
	last := m.History[len(m.History)-1]
	if last.ParentID != parentID || last.Module != "substitution" {
		t.Fatalf("derived lineage/module = parent %q module %q", last.ParentID, last.Module)
	}
	if last.Request.Seed != 17 {
		t.Fatalf("derived recipe lost parent seed: %d", last.Request.Seed)
	}
	if structureSHA256(m.History[0].Structure) != parentHash {
		t.Fatal("derivation mutated the parent snapshot")
	}

	before := st.ProjectManifest("")
	if _, err = st.DeriveRevision(parentID, DeriveRequest{Operation: "vacancy", SiteID: parent.Structure.NAtoms()}); err == nil {
		t.Fatal("out-of-range derivation unexpectedly succeeded")
	}
	after := st.ProjectManifest("")
	if len(after.History) != len(before.History) || after.ActiveRevisionID != before.ActiveRevisionID {
		t.Fatal("failed derivation changed project state")
	}
}

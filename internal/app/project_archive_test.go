package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestProjectArchiveV2RoundTripRestoresExactRevisionsAndActiveSelection(t *testing.T) {
	source := NewState()
	first, err := source.BuildTracked(BuildRequest{Module: "crystal", Phase: "alpha", NX: 2, NY: 2, NZ: 2})
	if err != nil {
		t.Fatal(err)
	}
	firstID := source.ProjectManifest("").ActiveRevisionID
	if _, err = source.BuildTracked(BuildRequest{Module: "random", Phase: "beta", NX: 2, NY: 2, NZ: 2, Seed: 31}); err != nil {
		t.Fatal(err)
	}
	if _, err = source.SelectRevision(firstID); err != nil {
		t.Fatal(err)
	}

	data, err := source.ExportProjectArchive("portable project")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["manifest.json"] || !names["revisions/"+firstID+"/record.json"] || !names["revisions/"+firstID+"/structure.json"] {
		t.Fatalf("archive members missing: %v", names)
	}

	restored := NewState()
	active, err := restored.ImportProjectArchive(data)
	if err != nil {
		t.Fatal(err)
	}
	m := restored.ProjectManifest("")
	if m.SchemaVersion != 2 || m.Name != "portable project" || len(m.History) != 2 || m.ActiveRevisionID != firstID {
		t.Fatalf("unexpected restored manifest: %+v", m)
	}
	if structureSHA256(active.Structure) != structureSHA256(first.Structure) {
		t.Fatal("active revision structure changed across archive round trip")
	}
}

func TestProjectArchiveRejectsTamperedStructureWithoutChangingCurrentProject(t *testing.T) {
	source := NewState()
	if _, err := source.BuildTracked(BuildRequest{Module: "crystal", Phase: "alpha"}); err != nil {
		t.Fatal(err)
	}
	data, err := source.ExportProjectArchive("source")
	if err != nil {
		t.Fatal(err)
	}
	tampered := rewriteZipMember(t, data, "/structure.json", func(b []byte) []byte {
		return bytes.Replace(b, []byte(`"Ti"`), []byte(`"Al"`), 1)
	})

	destination := NewState()
	if _, err = destination.BuildTracked(BuildRequest{Module: "crystal", Phase: "beta"}); err != nil {
		t.Fatal(err)
	}
	before := destination.ProjectManifest("destination")
	if _, err = destination.ImportProjectArchive(tampered); err == nil || !strings.Contains(strings.ToLower(err.Error()), "sha-256") {
		t.Fatalf("tampered archive error = %v, want SHA-256 failure", err)
	}
	after := destination.ProjectManifest("")
	if after.ProjectID != before.ProjectID || after.ActiveRevisionID != before.ActiveRevisionID || len(after.History) != len(before.History) {
		t.Fatal("failed archive import changed the destination project")
	}
}

func TestProjectArchiveRejectsNonCanonicalRevisionPaths(t *testing.T) {
	st := NewState()
	if _, err := st.BuildTracked(BuildRequest{Module: "crystal", Phase: "alpha"}); err != nil {
		t.Fatal(err)
	}
	data, err := st.ExportProjectArchive("paths")
	if err != nil {
		t.Fatal(err)
	}
	members := unzipMembers(t, data)
	var manifest archiveManifest
	if err = json.Unmarshal(members["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	rev := &manifest.Revisions[0]
	members["record.json"] = members[rev.RecordPath]
	members["structure.json"] = members[rev.StructurePath]
	delete(members, rev.RecordPath)
	delete(members, rev.StructurePath)
	rev.RecordPath = "record.json"
	rev.StructurePath = "structure.json"
	members["manifest.json"], _ = json.Marshal(manifest)
	noncanonical := zipMembers(t, members)
	if _, err = NewState().ImportProjectArchive(noncanonical); err == nil || !strings.Contains(strings.ToLower(err.Error()), "path") {
		t.Fatalf("noncanonical revision paths error = %v", err)
	}
}

func unzipMembers(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	members := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		members[f.Name], err = io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return members
}

func zipMembers(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for name, data := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func rewriteZipMember(t *testing.T, data []byte, suffix string, transform func([]byte) []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	changed := false
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(f.Name, suffix) {
			b = transform(b)
			changed = true
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("no member matched suffix %q", suffix)
	}
	return out.Bytes()
}

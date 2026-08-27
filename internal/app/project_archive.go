package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

const (
	projectArchiveSchema = 2
	maxArchiveMembers    = 4096
	maxArchiveMemberSize = 64 << 20
	maxArchiveTotalSize  = 256 << 20
)

type archiveRevision struct {
	ID              string `json:"id"`
	ParentID        string `json:"parent_id,omitempty"`
	RecordPath      string `json:"record_path"`
	StructurePath   string `json:"structure_path"`
	RecordSHA256    string `json:"record_sha256"`
	StructureSHA256 string `json:"structure_sha256"`
}

type archiveManifest struct {
	SchemaVersion    int               `json:"schema_version"`
	ProjectID        string            `json:"project_uuid"`
	Name             string            `json:"name"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
	ActiveRevisionID string            `json:"active_revision_id"`
	Revisions        []archiveRevision `json:"revisions"`
}

func recordWithoutStructure(record BuildRecord) ([]byte, error) {
	b, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	delete(fields, "structure")
	return json.Marshal(fields)
}

func deterministicZipFile(zw *zip.Writer, name string, data []byte) error {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ExportProjectArchive serializes every immutable revision into a portable,
// self-verifying .tias-project ZIP.
func (s *State) ExportProjectArchive(name string) ([]byte, error) {
	m := s.ProjectManifest(name)
	if len(m.History) == 0 {
		return nil, errors.New("project has no revisions to export")
	}
	am := archiveManifest{
		SchemaVersion: projectArchiveSchema, ProjectID: m.ProjectID, Name: m.Name,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, ActiveRevisionID: m.ActiveRevisionID,
		Revisions: make([]archiveRevision, 0, len(m.History)),
	}
	members := map[string][]byte{}
	for _, record := range m.History {
		base := "revisions/" + record.ID
		recordPath := base + "/record.json"
		structurePath := base + "/structure.json"
		recordBytes, err := recordWithoutStructure(record)
		if err != nil {
			return nil, fmt.Errorf("serialize revision %q record: %w", record.ID, err)
		}
		structureBytes, err := json.Marshal(record.Structure)
		if err != nil {
			return nil, fmt.Errorf("serialize revision %q structure: %w", record.ID, err)
		}
		members[recordPath] = recordBytes
		members[structurePath] = structureBytes
		am.Revisions = append(am.Revisions, archiveRevision{
			ID: record.ID, ParentID: record.ParentID,
			RecordPath: recordPath, StructurePath: structurePath,
			RecordSHA256: sha256Bytes(recordBytes), StructureSHA256: sha256Bytes(structureBytes),
		})
	}
	manifestBytes, err := json.Marshal(am)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err = deterministicZipFile(zw, "manifest.json", manifestBytes); err != nil {
		return nil, err
	}
	for _, rev := range am.Revisions {
		if err = deterministicZipFile(zw, rev.RecordPath, members[rev.RecordPath]); err != nil {
			return nil, err
		}
		if err = deterministicZipFile(zw, rev.StructurePath, members[rev.StructurePath]); err != nil {
			return nil, err
		}
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func safeArchiveName(name string) bool {
	return name != "" && !strings.Contains(name, "\\") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && !strings.HasPrefix(name, "../")
}

func readProjectArchive(data []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open project archive: %w", err)
	}
	if len(zr.File) > maxArchiveMembers {
		return nil, fmt.Errorf("project archive has too many members: %d", len(zr.File))
	}
	members := make(map[string][]byte, len(zr.File))
	total := uint64(0)
	for _, f := range zr.File {
		if !safeArchiveName(f.Name) {
			return nil, fmt.Errorf("unsafe project archive member %q", f.Name)
		}
		if _, exists := members[f.Name]; exists {
			return nil, fmt.Errorf("duplicate project archive member %q", f.Name)
		}
		if f.UncompressedSize64 > maxArchiveMemberSize || total+f.UncompressedSize64 > maxArchiveTotalSize {
			return nil, fmt.Errorf("project archive member %q exceeds size limit", f.Name)
		}
		total += f.UncompressedSize64
		r, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open project archive member %q: %w", f.Name, err)
		}
		b, readErr := io.ReadAll(io.LimitReader(r, maxArchiveMemberSize+1))
		closeErr := r.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read project archive member %q: %w", f.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close project archive member %q: %w", f.Name, closeErr)
		}
		if len(b) > maxArchiveMemberSize {
			return nil, fmt.Errorf("project archive member %q exceeds size limit", f.Name)
		}
		members[f.Name] = b
	}
	return members, nil
}

// ImportProjectArchive validates the complete archive before replacing any
// current state, so corrupt or hostile input cannot partially open a project.
func (s *State) ImportProjectArchive(data []byte) (BuildResponse, error) {
	members, err := readProjectArchive(data)
	if err != nil {
		return BuildResponse{}, err
	}
	manifestBytes, ok := members["manifest.json"]
	if !ok {
		return BuildResponse{}, errors.New("project archive is missing manifest.json")
	}
	var am archiveManifest
	dec := json.NewDecoder(bytes.NewReader(manifestBytes))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&am); err != nil {
		return BuildResponse{}, fmt.Errorf("decode project archive manifest: %w", err)
	}
	if am.SchemaVersion != projectArchiveSchema {
		return BuildResponse{}, fmt.Errorf("unsupported project archive schema version %d", am.SchemaVersion)
	}
	if strings.TrimSpace(am.ProjectID) == "" || len(am.Revisions) == 0 {
		return BuildResponse{}, errors.New("project archive requires a project UUID and at least one revision")
	}
	seen := map[string]bool{}
	history := make([]BuildRecord, 0, len(am.Revisions))
	for _, entry := range am.Revisions {
		if strings.TrimSpace(entry.ID) == "" || seen[entry.ID] {
			return BuildResponse{}, fmt.Errorf("duplicate or empty revision id %q", entry.ID)
		}
		if entry.ParentID != "" && !seen[entry.ParentID] {
			return BuildResponse{}, fmt.Errorf("revision %q has unknown or forward parent %q", entry.ID, entry.ParentID)
		}
		expectedBase := "revisions/" + entry.ID
		if entry.RecordPath != expectedBase+"/record.json" || entry.StructurePath != expectedBase+"/structure.json" {
			return BuildResponse{}, fmt.Errorf("revision %q uses noncanonical archive paths", entry.ID)
		}
		recordBytes, recordOK := members[entry.RecordPath]
		structureBytes, structureOK := members[entry.StructurePath]
		if !recordOK || !structureOK {
			return BuildResponse{}, fmt.Errorf("revision %q is missing record or structure member", entry.ID)
		}
		if sha256Bytes(recordBytes) != entry.RecordSHA256 {
			return BuildResponse{}, fmt.Errorf("revision %q record SHA-256 mismatch", entry.ID)
		}
		if sha256Bytes(structureBytes) != entry.StructureSHA256 {
			return BuildResponse{}, fmt.Errorf("revision %q structure SHA-256 mismatch", entry.ID)
		}
		var record BuildRecord
		if err = json.Unmarshal(recordBytes, &record); err != nil {
			return BuildResponse{}, fmt.Errorf("decode revision %q record: %w", entry.ID, err)
		}
		if err = json.Unmarshal(structureBytes, &record.Structure); err != nil {
			return BuildResponse{}, fmt.Errorf("decode revision %q structure: %w", entry.ID, err)
		}
		if record.ID != entry.ID || record.ParentID != entry.ParentID {
			return BuildResponse{}, fmt.Errorf("revision %q metadata does not match manifest", entry.ID)
		}
		if structureSHA256(record.Structure) != record.StructureSHA256 {
			return BuildResponse{}, fmt.Errorf("revision %q recorded structure SHA-256 mismatch", entry.ID)
		}
		seen[entry.ID] = true
		history = append(history, record)
	}
	if !seen[am.ActiveRevisionID] {
		return BuildResponse{}, fmt.Errorf("active revision %q is not present", am.ActiveRevisionID)
	}
	name := strings.TrimSpace(am.Name)
	if name == "" {
		name = "Imported Project"
	}
	m := ProjectManifest{
		SchemaVersion: projectArchiveSchema, ProjectID: am.ProjectID, Name: name,
		CreatedAt: am.CreatedAt, UpdatedAt: am.UpdatedAt,
		ActiveRevisionID: am.ActiveRevisionID, History: history,
	}
	var active BuildRecord
	for _, record := range history {
		if record.ID == am.ActiveRevisionID {
			active = record
			break
		}
	}
	out := responseFromRecord(active)
	projectRegistry.Store(s, &projectState{manifest: cloneManifest(m)})
	s.mu.Lock()
	s.Current = cloneBuildResponse(out)
	s.CurrentRequest = active.Request
	s.mu.Unlock()
	return cloneBuildResponse(out), nil
}

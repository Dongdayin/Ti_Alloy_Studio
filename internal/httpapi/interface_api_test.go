package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tialloystudio/internal/app"
)

func TestInterfaceAPIDefaultsToPeriodicBicrystal(t *testing.T) {
	h := NewHandler(app.NewState())
	body := []byte(`{"module":"interface","nx":2,"ny":2,"nz":2,"a_alpha":2.951,"c_alpha":4.684,"a_beta":3.306,"interface_max_repeat":4,"interface_distance":2.5}`)
	r := httptest.NewRequest(http.MethodPost, "/api/build", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out app.BuildResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Structure.PBC != [3]bool{true, true, true} {
		t.Fatalf("PBC=%v, want fully periodic", out.Structure.PBC)
	}
	if got, _ := out.Analysis["interface_topology"].(string); got != "periodic_bicrystal" {
		t.Fatalf("topology=%q", got)
	}
	if got, ok := out.Analysis["interface_count"].(float64); !ok || int(got) != 2 {
		t.Fatalf("interface_count=%v", out.Analysis["interface_count"])
	}
}

func TestInterfaceAPIAllowsExplicitSingleInterfaceSlab(t *testing.T) {
	h := NewHandler(app.NewState())
	body := []byte(`{"module":"interface","nx":2,"ny":2,"nz":2,"a_alpha":2.951,"c_alpha":4.684,"a_beta":3.306,"interface_max_repeat":4,"interface_distance":2.5,"vacuum":12,"surface_preset":"interface_single_slab"}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/build", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out app.BuildResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Structure.PBC != [3]bool{true, true, false} {
		t.Fatalf("PBC=%v, want slab topology", out.Structure.PBC)
	}
	if got, _ := out.Analysis["interface_topology"].(string); got != "single_interface_slab" {
		t.Fatalf("topology=%q", got)
	}
}

package model

import "testing"

func TestCrystalTemplateRegistryMarksPhaseOneSupportExplicitly(t *testing.T) {
	templates := CrystalTemplates()
	byID := map[string]CrystalTemplate{}
	for _, x := range templates {
		byID[x.ID] = x
	}
	for _, id := range []string{"alpha_ti_hcp", "beta_ti_bcc"} {
		x, ok := byID[id]
		if !ok || !x.Implemented {
			t.Fatalf("%s must be an implemented Phase 1 template: %+v", id, x)
		}
	}
	for _, id := range []string{"ti3al_d019", "tial_l10"} {
		x, ok := byID[id]
		if !ok || x.Implemented {
			t.Fatalf("%s must exist as an explicit unimplemented placeholder: %+v", id, x)
		}
	}
}

package model

type CrystalTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Material    string `json:"material"`
	Phase       string `json:"phase"`
	Prototype   string `json:"prototype"`
	SpaceGroup  string `json:"space_group"`
	Implemented bool   `json:"implemented"`
	Note        string `json:"note"`
}

func CrystalTemplates() []CrystalTemplate {
	return []CrystalTemplate{
		{
			ID:          "alpha_ti_hcp",
			Name:        "α-Ti",
			Material:    "Ti",
			Phase:       "alpha",
			Prototype:   "HCP",
			SpaceGroup:  "P63/mmc (No. 194)",
			Implemented: true,
			Note:        "User-overridable a and c lattice parameters; generated with a two-atom conventional HCP basis.",
		},
		{
			ID:          "beta_ti_bcc",
			Name:        "β-Ti",
			Material:    "Ti",
			Phase:       "beta",
			Prototype:   "BCC",
			SpaceGroup:  "Im-3m (No. 229)",
			Implemented: true,
			Note:        "User-overridable cubic lattice parameter. β-Ti stability depends on temperature and alloy chemistry; lattice input must match the intended state.",
		},
		{
			ID:          "ti3al_d019",
			Name:        "Ti3Al (α2)",
			Material:    "Ti3Al",
			Phase:       "alpha2",
			Prototype:   "D019",
			SpaceGroup:  "P63/mmc (No. 194)",
			Implemented: false,
			Note:        "Phase 1 data-model placeholder only. Automatic ordered-basis generation is intentionally not exposed until validated in a later phase.",
		},
		{
			ID:          "tial_l10",
			Name:        "γ-TiAl",
			Material:    "TiAl",
			Phase:       "gamma",
			Prototype:   "L10",
			SpaceGroup:  "P4/mmm (No. 123)",
			Implemented: false,
			Note:        "Phase 1 data-model placeholder only. Automatic ordered-basis generation is intentionally not exposed until validated in a later phase.",
		},
	}
}

package model

import "errors"

func CreateVacancy(s Structure, site int) (Structure, error) {
	if site < 0 || site >= s.NAtoms() {
		return Structure{}, errors.New("site id out of range")
	}
	before := s.SpeciesCounts()
	out := s
	out.Positions = append([]Vec3{}, s.Positions[:site]...)
	out.Positions = append(out.Positions, s.Positions[site+1:]...)
	out.Species = append([]string{}, s.Species[:site]...)
	out.Species = append(out.Species, s.Species[site+1:]...)
	out.Meta = cloneMeta(s.Meta)
	out.Meta["defect_type"] = "vacancy"
	out.Meta["defect_site_id"] = site
	out.Meta["removed_species"] = s.Species[site]
	out.Meta["composition_before_counts"] = before
	out.Meta["composition_after_counts"] = out.SpeciesCounts()
	out.Meta["defect_periodic_image_distance_angstrom"] = ShortestPeriodicTranslation(s)
	if len(s.SiteLabels) == len(s.Species) {
		out.SiteLabels = append([]string{}, s.SiteLabels[:site]...)
		out.SiteLabels = append(out.SiteLabels, s.SiteLabels[site+1:]...)
	}
	return out, nil
}

func CreateSubstitution(s Structure, site int, newSpecies string) (Structure, error) {
	if site < 0 || site >= s.NAtoms() {
		return Structure{}, errors.New("site id out of range")
	}
	if _, ok := AtomicWeights[newSpecies]; !ok {
		return Structure{}, errors.New("unknown element")
	}
	before := s.SpeciesCounts()
	out := s
	out.Species = append([]string(nil), s.Species...)
	old := out.Species[site]
	out.Species[site] = newSpecies
	out.Meta = cloneMeta(s.Meta)
	out.Meta["defect_type"] = "substitution"
	out.Meta["defect_site_id"] = site
	out.Meta["original_species"] = old
	out.Meta["new_species"] = newSpecies
	out.Meta["composition_before_counts"] = before
	out.Meta["composition_after_counts"] = out.SpeciesCounts()
	out.Meta["defect_periodic_image_distance_angstrom"] = ShortestPeriodicTranslation(s)
	return out, nil
}

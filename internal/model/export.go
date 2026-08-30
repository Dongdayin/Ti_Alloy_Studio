package model

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func elementOrder(s Structure) []string {
	m := s.SpeciesCounts()
	out := []string{}
	if _, ok := m["Ti"]; ok {
		out = append(out, "Ti")
		delete(m, "Ti")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return append(out, keys...)
}
func ExportPOSCAR(s Structure, comment string) string {
	if comment == "" {
		comment = "Ti Alloy Studio"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n1.0\n", comment)
	for _, v := range s.Cell {
		fmt.Fprintf(&b, "  %.16f  %.16f  %.16f\n", v[0], v[1], v[2])
	}
	els := elementOrder(s)
	for _, e := range els {
		fmt.Fprintf(&b, "  %s", e)
	}
	b.WriteString("\n")
	counts := s.SpeciesCounts()
	for _, e := range els {
		fmt.Fprintf(&b, "  %d", counts[e])
	}
	b.WriteString("\nDirect\n")
	frac := s.Fractional(true)
	for _, e := range els {
		for i, sp := range s.Species {
			if sp == e {
				f := frac[i]
				fmt.Fprintf(&b, "  %.16f  %.16f  %.16f\n", f[0], f[1], f[2])
			}
		}
	}
	return b.String()
}
func ExportXYZ(s Structure) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d\nTi Alloy Studio; PBC=%t,%t,%t\n", s.NAtoms(), s.PBC[0], s.PBC[1], s.PBC[2])
	for i, p := range s.Positions {
		fmt.Fprintf(&b, "%-3s %.12f %.12f %.12f\n", s.Species[i], p[0], p[1], p[2])
	}
	return b.String()
}
func restrictedCell(cell Mat3) Mat3 {
	a, b, c := cell[0], cell[1], cell[2]
	lx := Norm(a)
	xh := Unit(a)
	xy := Dot(b, xh)
	bp := VSub(b, VScale(xh, xy))
	ly := Norm(bp)
	yh := Unit(bp)
	xz := Dot(c, xh)
	yz := Dot(c, yh)
	cp := VSub(VSub(c, VScale(xh, xz)), VScale(yh, yz))
	lz := Norm(cp)
	return Mat3{{lx, 0, 0}, {xy, ly, 0}, {xz, yz, lz}}
}
func ExportLAMMPS(s Structure) string {
	rc := restrictedCell(s.Cell)
	frac := s.Fractional(true)
	els := elementOrder(s)
	types := map[string]int{}
	for i, e := range els {
		types[e] = i + 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Ti Alloy Studio LAMMPS data (atom_style atomic)\n\n%d atoms\n%d atom types\n\n", s.NAtoms(), len(els))
	fmt.Fprintf(&b, "0.0 %.16f xlo xhi\n0.0 %.16f ylo yhi\n0.0 %.16f zlo zhi\n%.16f %.16f %.16f xy xz yz\n\nMasses\n\n", rc[0][0], rc[1][1], rc[2][2], rc[1][0], rc[2][0], rc[2][1])
	for _, e := range els {
		fmt.Fprintf(&b, "%d %.10f # %s\n", types[e], AtomicWeights[e], e)
	}
	b.WriteString("\nAtoms # atomic\n\n")
	for i, f := range frac {
		p := FracToCart(f, rc)
		fmt.Fprintf(&b, "%d %d %.16f %.16f %.16f\n", i+1, types[s.Species[i]], p[0], p[1], p[2])
	}
	return b.String()
}
func cellParams(cell Mat3) (float64, float64, float64, float64, float64, float64) {
	a, b, c := Norm(cell[0]), Norm(cell[1]), Norm(cell[2])
	angle := func(x, y Vec3) float64 { return math.Acos(Clamp(Dot(x, y)/(Norm(x)*Norm(y)), -1, 1)) * 180 / math.Pi }
	return a, b, c, angle(cell[1], cell[2]), angle(cell[0], cell[2]), angle(cell[0], cell[1])
}
func ExportCIF(s Structure) string {
	a, b, c, al, be, ga := cellParams(s.Cell)
	var out strings.Builder
	fmt.Fprintf(&out, "data_ti_alloy_studio\n_symmetry_space_group_name_H-M 'P 1'\n_symmetry_Int_Tables_number 1\n_cell_length_a %.12f\n_cell_length_b %.12f\n_cell_length_c %.12f\n_cell_angle_alpha %.12f\n_cell_angle_beta %.12f\n_cell_angle_gamma %.12f\nloop_\n_atom_site_label\n_atom_site_type_symbol\n_atom_site_fract_x\n_atom_site_fract_y\n_atom_site_fract_z\n", a, b, c, al, be, ga)
	frac := s.Fractional(true)
	n := map[string]int{}
	for i, e := range s.Species {
		n[e]++
		f := frac[i]
		fmt.Fprintf(&out, "%s%d %s %.12f %.12f %.12f\n", e, n[e], e, f[0], f[1], f[2])
	}
	return out.String()
}

func ExportExtXYZ(s Structure) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d\n", s.NAtoms())
	fmt.Fprintf(&b, "Lattice=\"")
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i != 0 || j != 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%.16g", s.Cell[i][j])
		}
	}
	fmt.Fprintf(&b, "\" Properties=species:S:1:pos:R:3")
	for _, key := range []string{"scientific_state", "calculation_state", "model_kind", "dataset_kind", "dataset_name"} {
		if value, ok := s.Meta[key]; ok {
			text := strings.ReplaceAll(fmt.Sprint(value), `"`, `'`)
			if strings.ContainsAny(text, " \t\r\n") {
				fmt.Fprintf(&b, " %s=\"%s\"", key, text)
			} else {
				fmt.Fprintf(&b, " %s=%s", key, text)
			}
		}
	}
	fmt.Fprintf(&b, " pbc=\"")
	for i, p := range s.PBC {
		if i > 0 {
			b.WriteByte(' ')
		}
		if p {
			b.WriteByte('T')
		} else {
			b.WriteByte('F')
		}
	}
	b.WriteString("\"\n")
	for i, p := range s.Positions {
		fmt.Fprintf(&b, "%s %.16g %.16g %.16g\n", s.Species[i], p[0], p[1], p[2])
	}
	return b.String()
}

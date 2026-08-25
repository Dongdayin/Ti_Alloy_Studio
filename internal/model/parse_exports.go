package model

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func parseFloat3(fields []string) (Vec3, error) {
	if len(fields) < 3 {
		return Vec3{}, errors.New("expected three floating-point values")
	}
	var v Vec3
	for i := 0; i < 3; i++ {
		x, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return Vec3{}, err
		}
		v[i] = x
	}
	return v, nil
}

func nonEmptyLines(text string) []string {
	out := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ParsePOSCAR parses the VASP 5-style Direct-coordinate POSCAR emitted by
// ExportPOSCAR. It deliberately rejects unsupported variants rather than
// guessing their meaning.
func ParsePOSCAR(text string) (Structure, error) {
	lines := nonEmptyLines(text)
	if len(lines) < 8 {
		return Structure{}, errors.New("POSCAR is incomplete")
	}
	scale, err := strconv.ParseFloat(strings.Fields(lines[1])[0], 64)
	if err != nil || scale <= 0 {
		return Structure{}, errors.New("POSCAR scale must be a positive number")
	}
	var cell Mat3
	for i := 0; i < 3; i++ {
		v, err := parseFloat3(strings.Fields(lines[2+i]))
		if err != nil {
			return Structure{}, fmt.Errorf("POSCAR lattice vector %d: %w", i+1, err)
		}
		cell[i] = VScale(v, scale)
	}
	elements := strings.Fields(lines[5])
	countFields := strings.Fields(lines[6])
	if len(elements) == 0 || len(elements) != len(countFields) {
		return Structure{}, errors.New("POSCAR element/count lines are inconsistent")
	}
	counts := make([]int, len(elements))
	total := 0
	for i, f := range countFields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return Structure{}, errors.New("POSCAR atom count is invalid")
		}
		counts[i] = n
		total += n
	}
	modeIndex := 7
	if strings.HasPrefix(strings.ToLower(lines[modeIndex]), "s") {
		modeIndex++
	}
	if modeIndex >= len(lines) || !strings.HasPrefix(strings.ToLower(lines[modeIndex]), "d") {
		return Structure{}, errors.New("only Direct-coordinate POSCAR emitted by Ti Alloy Studio is supported")
	}
	if len(lines) < modeIndex+1+total {
		return Structure{}, errors.New("POSCAR has fewer coordinate lines than declared atoms")
	}
	out := Structure{Cell: cell, PBC: [3]bool{true, true, true}, Meta: map[string]any{"source": "Ti Alloy Studio POSCAR round-trip parser"}}
	line := modeIndex + 1
	for ei, e := range elements {
		if _, ok := AtomicWeights[e]; !ok {
			return Structure{}, fmt.Errorf("unknown POSCAR element %q", e)
		}
		for j := 0; j < counts[ei]; j++ {
			f, err := parseFloat3(strings.Fields(lines[line]))
			if err != nil {
				return Structure{}, fmt.Errorf("POSCAR coordinate %d: %w", line-modeIndex, err)
			}
			out.Species = append(out.Species, e)
			out.Positions = append(out.Positions, FracToCart(f, cell))
			line++
		}
	}
	return out, nil
}

func quotedHeaderValue(header, key string) (string, bool) {
	needle := key + "=\""
	start := strings.Index(header, needle)
	if start < 0 {
		return "", false
	}
	start += len(needle)
	end := strings.Index(header[start:], "\"")
	if end < 0 {
		return "", false
	}
	return header[start : start+end], true
}

// ParseExtXYZ parses the exact extxyz schema emitted by ExportExtXYZ:
// species:S:1 + pos:R:3 with Lattice and pbc header attributes.
func ParseExtXYZ(text string) (Structure, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return Structure{}, errors.New("extxyz is incomplete")
	}
	n, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || n < 0 {
		return Structure{}, errors.New("invalid extxyz atom count")
	}
	latticeText, ok := quotedHeaderValue(lines[1], "Lattice")
	if !ok {
		return Structure{}, errors.New("extxyz Lattice attribute is missing")
	}
	lf := strings.Fields(latticeText)
	if len(lf) != 9 {
		return Structure{}, errors.New("extxyz Lattice must contain nine values")
	}
	var cell Mat3
	for i := 0; i < 3; i++ {
		v, err := parseFloat3(lf[i*3 : i*3+3])
		if err != nil {
			return Structure{}, fmt.Errorf("extxyz lattice: %w", err)
		}
		cell[i] = v
	}
	pbcText, ok := quotedHeaderValue(lines[1], "pbc")
	if !ok {
		return Structure{}, errors.New("extxyz pbc attribute is missing")
	}
	pf := strings.Fields(pbcText)
	if len(pf) != 3 {
		return Structure{}, errors.New("extxyz pbc must contain three flags")
	}
	var pbc [3]bool
	for i, f := range pf {
		switch strings.ToUpper(f) {
		case "T", "TRUE", "1":
			pbc[i] = true
		case "F", "FALSE", "0":
			pbc[i] = false
		default:
			return Structure{}, fmt.Errorf("invalid extxyz pbc flag %q", f)
		}
	}
	out := Structure{Cell: cell, PBC: pbc, Meta: map[string]any{"source": "Ti Alloy Studio extxyz round-trip parser"}}
	for i := 0; i < n; i++ {
		lineIndex := i + 2
		if lineIndex >= len(lines) {
			return Structure{}, errors.New("extxyz has fewer atom lines than declared")
		}
		f := strings.Fields(lines[lineIndex])
		if len(f) < 4 {
			return Structure{}, fmt.Errorf("extxyz atom line %d is incomplete", i+1)
		}
		if _, ok := AtomicWeights[f[0]]; !ok {
			return Structure{}, fmt.Errorf("unknown extxyz element %q", f[0])
		}
		p, err := parseFloat3(f[1:4])
		if err != nil {
			return Structure{}, fmt.Errorf("extxyz atom line %d: %w", i+1, err)
		}
		out.Species = append(out.Species, f[0])
		out.Positions = append(out.Positions, p)
	}
	return out, nil
}

// ParseLAMMPSAtomicData parses the restricted-triclinic atom_style atomic data
// emitted by ExportLAMMPS. Boundary conditions and force-field choices belong
// in a LAMMPS input script and therefore are not inferred from the data file.
func ParseLAMMPSAtomicData(text string) (Structure, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	section := "header"
	typeToElement := map[int]string{}
	atomLines := []string{}
	lx, ly, lz := 0.0, 0.0, 0.0
	xy, xz, yz := 0.0, 0.0, 0.0
	declaredAtoms := -1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "Masses" {
			section = "masses"
			continue
		}
		if strings.HasPrefix(line, "Atoms") {
			section = "atoms"
			continue
		}
		fields := strings.Fields(line)
		switch section {
		case "header":
			if len(fields) >= 2 && fields[1] == "atoms" {
				declaredAtoms, _ = strconv.Atoi(fields[0])
				continue
			}
			if len(fields) >= 4 && fields[len(fields)-2] == "xlo" && fields[len(fields)-1] == "xhi" {
				hi, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return Structure{}, err
				}
				lx = hi
				continue
			}
			if len(fields) >= 4 && fields[len(fields)-2] == "ylo" && fields[len(fields)-1] == "yhi" {
				hi, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return Structure{}, err
				}
				ly = hi
				continue
			}
			if len(fields) >= 4 && fields[len(fields)-2] == "zlo" && fields[len(fields)-1] == "zhi" {
				hi, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return Structure{}, err
				}
				lz = hi
				continue
			}
			if len(fields) >= 6 && fields[3] == "xy" && fields[4] == "xz" && fields[5] == "yz" {
				var err error
				xy, err = strconv.ParseFloat(fields[0], 64)
				if err != nil {
					return Structure{}, err
				}
				xz, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return Structure{}, err
				}
				yz, err = strconv.ParseFloat(fields[2], 64)
				if err != nil {
					return Structure{}, err
				}
			}
		case "masses":
			parts := strings.SplitN(line, "#", 2)
			base := strings.Fields(parts[0])
			if len(base) < 2 || len(parts) != 2 {
				continue
			}
			typeID, err := strconv.Atoi(base[0])
			if err != nil {
				return Structure{}, err
			}
			element := strings.TrimSpace(parts[1])
			if _, ok := AtomicWeights[element]; !ok {
				return Structure{}, fmt.Errorf("unknown LAMMPS element comment %q", element)
			}
			typeToElement[typeID] = element
		case "atoms":
			atomLines = append(atomLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return Structure{}, err
	}
	if lx <= 0 || ly <= 0 || lz <= 0 {
		return Structure{}, errors.New("LAMMPS box dimensions are missing or invalid")
	}
	if declaredAtoms < 0 || len(atomLines) != declaredAtoms {
		return Structure{}, fmt.Errorf("LAMMPS atom count mismatch: header=%d parsed=%d", declaredAtoms, len(atomLines))
	}
	cell := Mat3{{lx, 0, 0}, {xy, ly, 0}, {xz, yz, lz}}
	out := Structure{Cell: cell, PBC: [3]bool{true, true, true}, Meta: map[string]any{"source": "Ti Alloy Studio LAMMPS-data round-trip parser", "boundary_note": "LAMMPS boundary conditions are defined by the input script, not the data file"}}
	for _, line := range atomLines {
		f := strings.Fields(line)
		if len(f) < 5 {
			return Structure{}, errors.New("LAMMPS atomic line is incomplete")
		}
		typeID, err := strconv.Atoi(f[1])
		if err != nil {
			return Structure{}, err
		}
		element, ok := typeToElement[typeID]
		if !ok {
			return Structure{}, fmt.Errorf("LAMMPS atom references unknown type %d", typeID)
		}
		p, err := parseFloat3(f[2:5])
		if err != nil {
			return Structure{}, err
		}
		out.Species = append(out.Species, element)
		out.Positions = append(out.Positions, p)
	}
	return out, nil
}

package model

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

type SlipSystem struct {
	Phase           string `json:"phase"`
	Preset          string `json:"preset"`
	Plane           string `json:"plane"`
	Direction       string `json:"direction"`
	BurgersVector   Vec3   `json:"burgers_vector"`
	LineDirection   Vec3   `json:"line_direction"`
	SlipPlaneNormal Vec3   `json:"slip_plane_normal"`
}

type DislocationOptions struct {
	SlipSystem    string
	BurgersVector Vec3
	LineDirection Vec3
	Character     string
	Arrangement   string
	CoreRadius    float64
}

type DislocationModel struct {
	Structure             Structure  `json:"structure"`
	SlipSystem            SlipSystem `json:"slip_system"`
	Character             string     `json:"character"`
	Arrangement           string     `json:"arrangement"`
	PeriodicImageDistance float64    `json:"periodic_image_distance_angstrom"`
}

type GrainBoundaryOptions struct {
	Type               string
	Axis               string
	AngleDeg           float64
	Normal             string
	Periodic           bool
	OverlapCutoff      float64
	TranslationVariant int
	Grain1Orientation  string
	Grain2Orientation  string
}

type GrainBoundaryModel struct {
	Structure                 Structure `json:"structure"`
	Type                      string    `json:"type"`
	Grain1Orientation         Mat3      `json:"grain_1_orientation"`
	Grain2Orientation         Mat3      `json:"grain_2_orientation"`
	MisorientationAngleDeg    float64   `json:"misorientation_angle_deg"`
	GBPlaneNormal             Vec3      `json:"gb_plane_normal"`
	InPlaneMismatchPercent    float64   `json:"in_plane_mismatch_percent"`
	RemovedOverlapAtomCount   int       `json:"removed_overlap_atom_count"`
	InterfaceCount            int       `json:"interface_count"`
	TranslationCandidateIndex int       `json:"translation_candidate_index"`
}

type FaultSeriesOptions struct {
	Preset     string
	Steps      int
	Cut        float64
	NormalAxis int
}

type FaultSeries = GSFESeries

type TwinOptions struct {
	TwinSystem    string
	ShearFraction float64
}

type TwinModel struct {
	Structure     Structure `json:"structure"`
	TwinSystem    string    `json:"twin_system"`
	ShearFraction float64   `json:"shear_fraction"`
}

type LocalChemistryOptions struct {
	Kind          string
	TargetElement string
	ClusterSize   int
	Seed          int64
	Region        string
}

type LocalChemistryModel struct {
	Structure     Structure          `json:"structure"`
	Kind          string             `json:"kind"`
	TargetElement string             `json:"target_element"`
	ClusterSize   int                `json:"cluster_size"`
	Seed          int64              `json:"seed"`
	RegionInside  map[string]int     `json:"region_inside"`
	RegionOutside map[string]int     `json:"region_outside"`
	PairCounts    map[string]int     `json:"nearest_neighbor_pair_counts"`
	WarrenCowley  map[string]float64 `json:"warren_cowley"`
}

type CrackOptions struct {
	Plane   string
	Front   string
	Length  float64
	Opening float64
	Vacuum  float64
}

type CrackModel struct {
	Structure        Structure `json:"structure"`
	Plane            string    `json:"plane"`
	Front            string    `json:"front"`
	RemovedAtomCount int       `json:"removed_atom_count"`
}

type IndenterOptions struct {
	Radius float64
	Depth  float64
}

type NanoindentationModel struct {
	Structure      Structure `json:"structure"`
	IndenterRadius float64   `json:"indenter_radius_angstrom"`
	Depth          float64   `json:"depth_angstrom"`
	IndenterCenter Vec3      `json:"indenter_center"`
}

type PolycrystalOptions struct {
	GrainCount int
	Seed       int64
}

type PolycrystalModel struct {
	Structure       Structure      `json:"structure"`
	GrainAtomCounts map[string]int `json:"grain_atom_counts"`
	Orientations    []Mat3         `json:"orientations"`
}

type NEBOptions struct {
	MovingSite int
	FinalShift Vec3
	Images     int
}

type NEBPoint struct {
	Index     int       `json:"index"`
	Lambda    float64   `json:"lambda"`
	Structure Structure `json:"structure"`
}

type NEBSeries struct {
	Reference Structure  `json:"reference"`
	Points    []NEBPoint `json:"points"`
}

type DatasetOptions struct {
	Kind string
	Name string
}

type TrainingCandidateOptions struct {
	Count              int
	Seed               int64
	MaxRattleAngstrom  float64
	MaxStrainMagnitude float64
}

type TrainingSet struct {
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Structures []Structure `json:"structures"`
}

func markGeometryOnly(s *Structure, kind string) {
	if s.Meta == nil {
		s.Meta = map[string]any{}
	} else {
		s.Meta = cloneMeta(s.Meta)
	}
	s.Meta["model_kind"] = kind
	s.Meta["scientific_state"] = "not_relaxed"
	s.Meta["calculation_state"] = "not_calculated"
	s.Meta["geometry_only"] = true
}

func copyStructure(s Structure) Structure {
	out := s
	out.Positions = append([]Vec3(nil), s.Positions...)
	out.Species = append([]string(nil), s.Species...)
	if len(s.SiteLabels) > 0 {
		out.SiteLabels = append([]string(nil), s.SiteLabels...)
	}
	out.Meta = cloneMeta(s.Meta)
	return out
}

func bounds(s Structure) (min, max Vec3) {
	min = Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}
	max = Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range s.Positions {
		for axis := 0; axis < 3; axis++ {
			if p[axis] < min[axis] {
				min[axis] = p[axis]
			}
			if p[axis] > max[axis] {
				max[axis] = p[axis]
			}
		}
	}
	return min, max
}

func centerOf(s Structure) Vec3 {
	min, max := bounds(s)
	return VScale(VAdd(min, max), 0.5)
}

func vectorIsSet(v Vec3) bool {
	return Norm(v) > 1e-12
}

func scanVectorNumbers(spec string) []float64 {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	repl := strings.NewReplacer("[", " ", "]", " ", "(", " ", ")", " ", "{", " ", "}", " ", "<", " ", ">", " ", ",", " ", ";", " ", "|", " ", "\t", " ")
	cleaned := strings.TrimSpace(repl.Replace(spec))
	fields := strings.Fields(cleaned)
	if len(fields) >= 3 {
		nums := make([]float64, 0, len(fields))
		for _, f := range fields {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return nil
			}
			nums = append(nums, v)
		}
		return nums
	}
	if cleaned == "" {
		return nil
	}
	nums := []float64{}
	sign := 1.0
	for _, r := range cleaned {
		switch {
		case r == '-':
			sign = -1
		case r == '+':
			sign = 1
		case r >= '0' && r <= '9':
			nums = append(nums, sign*float64(r-'0'))
			sign = 1
		case r == ' ':
			continue
		default:
			return nil
		}
	}
	return nums
}

func VectorFromSpec(spec string) (Vec3, bool) {
	nums := scanVectorNumbers(spec)
	if len(nums) < 3 {
		return Vec3{}, false
	}
	return Vec3{nums[0], nums[1], nums[2]}, true
}

func OrientationMatrixFromSpec(spec string) (Mat3, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Mat3{}, false
	}
	rows := strings.FieldsFunc(spec, func(r rune) bool { return r == ';' || r == '|' || r == '\n' || r == '\r' })
	if len(rows) == 3 {
		var m Mat3
		for i, row := range rows {
			v, ok := VectorFromSpec(row)
			if !ok || !vectorIsSet(v) {
				return Mat3{}, false
			}
			m[i] = Unit(v)
		}
		return m, true
	}
	nums := scanVectorNumbers(spec)
	if len(nums) != 9 {
		return Mat3{}, false
	}
	m := Mat3{{nums[0], nums[1], nums[2]}, {nums[3], nums[4], nums[5]}, {nums[6], nums[7], nums[8]}}
	for i := range m {
		if !vectorIsSet(m[i]) {
			return Mat3{}, false
		}
		m[i] = Unit(m[i])
	}
	return m, true
}

func rotationAroundAxis(axis Vec3, angleRad float64) Mat3 {
	u := Unit(axis)
	if !vectorIsSet(u) {
		u = Vec3{0, 0, 1}
	}
	x, y, z := u[0], u[1], u[2]
	c, s := math.Cos(angleRad), math.Sin(angleRad)
	t := 1 - c
	return Mat3{
		{t*x*x + c, t*x*y - s*z, t*x*z + s*y},
		{t*x*y + s*z, t*y*y + c, t*y*z - s*x},
		{t*x*z - s*y, t*y*z + s*x, t*z*z + c},
	}
}

func applyMatrix(m Mat3, v Vec3) Vec3 {
	return Vec3{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func matrixMisorientationDeg(a, b Mat3) float64 {
	r := MatMul(b, Transpose(a))
	trace := r[0][0] + r[1][1] + r[2][2]
	return math.Acos(Clamp((trace-1)/2, -1, 1)) * 180 / math.Pi
}

func dominantAxis(v Vec3) int {
	axis := 0
	best := math.Abs(v[0])
	for i := 1; i < 3; i++ {
		if a := math.Abs(v[i]); a > best {
			best = a
			axis = i
		}
	}
	return axis
}

func slipSystem(phase, preset string, cell Mat3) (SlipSystem, error) {
	phase = strings.ToLower(strings.TrimSpace(phase))
	preset = strings.ToLower(strings.TrimSpace(preset))
	if preset == "" {
		if phase == "beta" {
			preset = "beta_110_111"
		} else {
			preset = "alpha_basal_a"
		}
	}
	a0 := Norm(cell[0])
	switch preset {
	case "alpha_basal_a", "basal_a":
		return SlipSystem{Phase: "alpha", Preset: "alpha_basal_a", Plane: "{0001}", Direction: "<11-20>", BurgersVector: VScale(Unit(cell[0]), a0), LineDirection: Unit(cell[0]), SlipPlaneNormal: Unit(Cross(cell[0], cell[1]))}, nil
	case "alpha_prismatic_a", "prismatic_a":
		return SlipSystem{Phase: "alpha", Preset: "alpha_prismatic_a", Plane: "{10-10}", Direction: "<11-20>", BurgersVector: VScale(Unit(cell[0]), a0), LineDirection: Unit(cell[2]), SlipPlaneNormal: Unit(Cross(cell[0], cell[2]))}, nil
	case "alpha_pyramidal_ca", "pyramidal_ca":
		b := VAdd(cell[0], cell[2])
		return SlipSystem{Phase: "alpha", Preset: "alpha_pyramidal_ca", Plane: "{10-11}", Direction: "<c+a>", BurgersVector: b, LineDirection: Unit(cell[1]), SlipPlaneNormal: Unit(Cross(b, cell[1]))}, nil
	case "beta_110_111", "110_111":
		return SlipSystem{Phase: "beta", Preset: "beta_110_111", Plane: "{110}", Direction: "<111>", BurgersVector: VScale(Unit(Vec3{1, 1, 1}), a0*math.Sqrt(3)/2), LineDirection: Unit(Vec3{1, -1, 1}), SlipPlaneNormal: Unit(Vec3{1, -1, 0})}, nil
	case "beta_112_111", "112_111":
		return SlipSystem{Phase: "beta", Preset: "beta_112_111", Plane: "{112}", Direction: "<111>", BurgersVector: VScale(Unit(Vec3{1, 1, 1}), a0*math.Sqrt(3)/2), LineDirection: Unit(Vec3{1, -1, 0}), SlipPlaneNormal: Unit(Vec3{1, 1, -2})}, nil
	default:
		return SlipSystem{}, fmt.Errorf("unsupported slip system %q", preset)
	}
}

func BuildDislocation(host Structure, phase string, opts DislocationOptions) (DislocationModel, error) {
	if host.NAtoms() == 0 {
		return DislocationModel{}, errors.New("empty host structure")
	}
	sys, err := slipSystem(phase, opts.SlipSystem, host.Cell)
	if err != nil {
		return DislocationModel{}, err
	}
	if vectorIsSet(opts.BurgersVector) {
		sys.BurgersVector = opts.BurgersVector
	}
	if vectorIsSet(opts.LineDirection) {
		sys.LineDirection = Unit(opts.LineDirection)
	}
	out := copyStructure(host)
	character := strings.ToLower(strings.TrimSpace(opts.Character))
	if character == "" {
		character = "screw"
	}
	arrangement := strings.ToLower(strings.TrimSpace(opts.Arrangement))
	if arrangement == "" {
		arrangement = "single"
	}
	coreRadius := opts.CoreRadius
	if coreRadius <= 0 {
		coreRadius = math.Max(1.5, 0.75*Norm(sys.BurgersVector))
	}
	c := centerOf(host)
	centers := []Vec3{c}
	signs := []float64{1}
	switch arrangement {
	case "dipole":
		offset := 0.18 * ShortestPeriodicTranslation(host)
		centers = []Vec3{VAdd(c, Vec3{-offset, 0, 0}), VAdd(c, Vec3{offset, 0, 0})}
		signs = []float64{1, -1}
	case "quadrupole":
		offset := 0.18 * ShortestPeriodicTranslation(host)
		centers = []Vec3{
			VAdd(c, Vec3{-offset, -offset, 0}),
			VAdd(c, Vec3{offset, -offset, 0}),
			VAdd(c, Vec3{-offset, offset, 0}),
			VAdd(c, Vec3{offset, offset, 0}),
		}
		signs = []float64{1, -1, -1, 1}
	}
	out.SiteLabels = make([]string, out.NAtoms())
	for i, p := range out.Positions {
		disp := Vec3{}
		inCore := false
		for j, core := range centers {
			dx := p[0] - core[0]
			dy := p[1] - core[1]
			r2 := dx*dx + dy*dy
			theta := math.Atan2(dy, dx)
			scale := signs[j] * theta / (2 * math.Pi)
			part := VScale(sys.BurgersVector, 0.18*scale)
			if character == "screw" {
				part = VScale(sys.LineDirection, 0.18*Norm(sys.BurgersVector)*scale)
			} else if character == "mixed" {
				part = VScale(VAdd(Unit(sys.BurgersVector), sys.LineDirection), 0.09*Norm(sys.BurgersVector)*scale)
			}
			disp = VAdd(disp, part)
			if r2 <= coreRadius*coreRadius {
				inCore = true
			}
		}
		out.Positions[i] = VAdd(p, disp)
		if inCore {
			out.SiteLabels[i] = "dislocation_core"
		} else {
			out.SiteLabels[i] = "matrix"
		}
	}
	markGeometryOnly(&out, "dislocation")
	out.Meta["operation"] = "dislocation"
	out.Meta["dislocation_slip_system"] = sys.Preset
	out.Meta["dislocation_plane"] = sys.Plane
	out.Meta["dislocation_direction"] = sys.Direction
	out.Meta["dislocation_character"] = character
	out.Meta["dislocation_arrangement"] = arrangement
	out.Meta["burgers_vector"] = sys.BurgersVector
	out.Meta["line_direction"] = sys.LineDirection
	out.Meta["slip_plane_normal"] = sys.SlipPlaneNormal
	out.Meta["burgers_dot_plane_normal"] = Dot(sys.BurgersVector, sys.SlipPlaneNormal)
	out.Meta["dislocation_core_count"] = len(centers)
	out.Meta["dislocation_core_centers"] = centers
	out.Meta["core_region_unrelaxed"] = true
	out.Meta["defect_periodic_image_distance_angstrom"] = ShortestPeriodicTranslation(host)
	return DislocationModel{Structure: out, SlipSystem: sys, Character: character, Arrangement: arrangement, PeriodicImageDistance: ShortestPeriodicTranslation(host)}, nil
}

func zRotation(angleRad float64) Mat3 {
	c, s := math.Cos(angleRad), math.Sin(angleRad)
	return Mat3{{c, -s, 0}, {s, c, 0}, {0, 0, 1}}
}

func rotateAround(p, c Vec3, angleRad float64) Vec3 {
	d := VSub(p, c)
	cs, sn := math.Cos(angleRad), math.Sin(angleRad)
	return VAdd(c, Vec3{cs*d[0] - sn*d[1], sn*d[0] + cs*d[1], d[2]})
}

func BuildGrainBoundary(host Structure, opts GrainBoundaryOptions) (GrainBoundaryModel, error) {
	if host.NAtoms() < 2 {
		return GrainBoundaryModel{}, errors.New("grain boundary requires at least two atoms")
	}
	gbType := strings.ToLower(strings.TrimSpace(opts.Type))
	if gbType == "" {
		gbType = "tilt"
	}
	angle := opts.AngleDeg
	if angle == 0 {
		angle = 10
	}
	axis := Vec3{0, 0, 1}
	if v, ok := VectorFromSpec(opts.Axis); ok && vectorIsSet(v) {
		axis = Unit(v)
	}
	normal := Unit(host.Cell[0])
	if v, ok := VectorFromSpec(opts.Normal); ok && vectorIsSet(v) {
		normal = Unit(v)
	}
	orient1 := rotationAroundAxis(axis, -angle*math.Pi/360)
	orient2 := rotationAroundAxis(axis, angle*math.Pi/360)
	if m, ok := OrientationMatrixFromSpec(opts.Grain1Orientation); ok {
		orient1 = m
	}
	if m, ok := OrientationMatrixFromSpec(opts.Grain2Orientation); ok {
		orient2 = m
	}
	c := centerOf(host)
	out := copyStructure(host)
	out.SiteLabels = make([]string, 0, out.NAtoms())
	positions := make([]Vec3, 0, out.NAtoms())
	species := make([]string, 0, out.NAtoms())
	labels := make([]string, 0, out.NAtoms())
	removed := 0
	cutoff := opts.OverlapCutoff
	if cutoff <= 0 {
		cutoff = 1.2
	}
	for i, p := range host.Positions {
		label := "grain_1"
		orient := orient1
		if Dot(VSub(p, c), normal) >= 0 {
			label = "grain_2"
			orient = orient2
		}
		q := VAdd(c, applyMatrix(orient, VSub(p, c)))
		if label == "grain_2" && shouldRemoveOverlap(q, positions, labels, cutoff) {
			removed++
			continue
		}
		positions = append(positions, q)
		species = append(species, host.Species[i])
		labels = append(labels, label)
	}
	out.Positions = positions
	out.Species = species
	out.SiteLabels = labels
	out.PBC = [3]bool{true, true, true}
	interfaceCount := 1
	if opts.Periodic {
		interfaceCount = 2
	} else {
		out.PBC[dominantAxis(normal)] = false
	}
	misorientation := matrixMisorientationDeg(orient1, orient2)
	mismatch := math.Abs(misorientation) / 180 * 100
	markGeometryOnly(&out, "grain_boundary")
	out.Meta["operation"] = "grain_boundary"
	out.Meta["grain_boundary_type"] = gbType
	out.Meta["grain_boundary_axis"] = opts.Axis
	out.Meta["grain_boundary_normal"] = opts.Normal
	out.Meta["misorientation_angle_deg"] = misorientation
	out.Meta["gb_plane_normal"] = normal
	out.Meta["grain_1_orientation"] = orient1
	out.Meta["grain_2_orientation"] = orient2
	out.Meta["in_plane_periodic_matching_mismatch_percent"] = mismatch
	out.Meta["removed_overlap_atom_count"] = removed
	out.Meta["interface_count"] = interfaceCount
	out.Meta["rigid_translation_candidate_index"] = opts.TranslationVariant
	return GrainBoundaryModel{
		Structure:                 out,
		Type:                      gbType,
		Grain1Orientation:         orient1,
		Grain2Orientation:         orient2,
		MisorientationAngleDeg:    misorientation,
		GBPlaneNormal:             normal,
		InPlaneMismatchPercent:    mismatch,
		RemovedOverlapAtomCount:   removed,
		InterfaceCount:            interfaceCount,
		TranslationCandidateIndex: opts.TranslationVariant,
	}, nil
}

func shouldRemoveOverlap(p Vec3, positions []Vec3, labels []string, cutoff float64) bool {
	cut2 := cutoff * cutoff
	for i, q := range positions {
		if labels[i] == "grain_1" && Dot(VSub(p, q), VSub(p, q)) < cut2 {
			return true
		}
	}
	return false
}

func GenerateFaultSeries(host Structure, opts FaultSeriesOptions) (FaultSeries, error) {
	if host.NAtoms() == 0 {
		return FaultSeries{}, errors.New("empty host structure")
	}
	steps := opts.Steps
	if steps < 1 {
		steps = 10
	}
	cut := opts.Cut
	if cut <= 0 || cut >= 1 {
		cut = 0.5
	}
	normalAxis := opts.NormalAxis
	if normalAxis < 0 || normalAxis > 2 {
		normalAxis = 2
	}
	preset := strings.ToLower(strings.TrimSpace(opts.Preset))
	if preset == "" {
		preset = "alpha_basal_a"
	}
	plane, dir := "{0001}", "<11-20>"
	path := VScale(host.Cell[0], 1/math.Max(1, math.Round(float64(steps))))
	if strings.Contains(preset, "prismatic") {
		plane = "{10-10}"
	}
	if strings.Contains(preset, "beta") || strings.Contains(preset, "110") {
		plane, dir = "{110}", "<111>"
	}
	series := GenerateGSFE(host, path, normalAxis, steps, cut, plane, dir, preset)
	for i := range series.Points {
		markGeometryOnly(&series.Points[i].Structure, "stacking_fault_geometry")
		series.Points[i].Structure.Meta["operation"] = "stacking_fault"
		series.Points[i].Structure.Meta["lambda"] = series.Points[i].Lambda
		series.Points[i].Structure.Meta["displacement_vector_angstrom"] = series.Points[i].Shift
	}
	series.Reference = copyStructure(host)
	markGeometryOnly(&series.Reference, "stacking_fault_reference")
	return series, nil
}

func BuildTwin(host Structure, opts TwinOptions) (TwinModel, error) {
	if host.NAtoms() == 0 {
		return TwinModel{}, errors.New("empty host structure")
	}
	system := strings.TrimSpace(opts.TwinSystem)
	if system == "" {
		system = "alpha_10-12"
	}
	shear := opts.ShearFraction
	if shear == 0 {
		shear = 0.16
	}
	out := copyStructure(host)
	frac := host.Fractional(true)
	out.SiteLabels = make([]string, len(out.Species))
	shearVector := VScale(host.Cell[0], shear)
	for i, f := range frac {
		if f[2] >= 0.5 {
			out.Positions[i] = VAdd(out.Positions[i], shearVector)
			out.SiteLabels[i] = "twin"
		} else {
			out.SiteLabels[i] = "parent"
		}
	}
	markGeometryOnly(&out, "twin")
	out.Meta["operation"] = "twin"
	out.Meta["twin_system"] = system
	out.Meta["twin_shear_fraction"] = shear
	out.Meta["mirror_or_shear_geometry"] = "sheared_upper_half"
	return TwinModel{Structure: out, TwinSystem: system, ShearFraction: shear}, nil
}

func ApplyLocalChemistry(host Structure, opts LocalChemistryOptions) (LocalChemistryModel, error) {
	if host.NAtoms() == 0 {
		return LocalChemistryModel{}, errors.New("empty host structure")
	}
	target := strings.TrimSpace(opts.TargetElement)
	if target == "" {
		target = "Al"
	}
	if _, ok := AtomicWeights[target]; !ok {
		return LocalChemistryModel{}, fmt.Errorf("unknown element %q", target)
	}
	size := opts.ClusterSize
	if size < 1 {
		size = int(math.Max(1, math.Round(0.05*float64(host.NAtoms()))))
	}
	if size > host.NAtoms() {
		size = host.NAtoms()
	}
	seed := opts.Seed
	if seed == 0 {
		seed = 20260829
	}
	kind := strings.ToLower(strings.TrimSpace(opts.Kind))
	if kind == "" {
		kind = "solute_cluster"
	}
	out := copyStructure(host)
	out.Species = append([]string(nil), host.Species...)
	out.SiteLabels = make([]string, host.NAtoms())
	center := centerOf(host)
	order := make([]int, host.NAtoms())
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		di := Norm(VSub(host.Positions[order[i]], center))
		dj := Norm(VSub(host.Positions[order[j]], center))
		if math.Abs(di-dj) < 1e-12 {
			return order[i] < order[j]
		}
		return di < dj
	})
	if kind == "segregation" || strings.Contains(kind, "surface") {
		sort.SliceStable(order, func(i, j int) bool {
			return host.Positions[order[i]][2] > host.Positions[order[j]][2]
		})
	} else if seed != 0 && strings.Contains(kind, "sro") {
		r := rand.New(rand.NewSource(seed))
		r.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	selected := map[int]bool{}
	for i := 0; i < size; i++ {
		selected[order[i]] = true
		out.Species[order[i]] = target
	}
	for i := range out.SiteLabels {
		if selected[i] {
			out.SiteLabels[i] = "local_chemistry_region"
		} else {
			out.SiteLabels[i] = "matrix"
		}
	}
	inside, outside := map[string]int{}, map[string]int{}
	for i, e := range out.Species {
		if selected[i] {
			inside[e]++
		} else {
			outside[e]++
		}
	}
	pairs := nearestPairCounts(out, 1.25)
	wc := map[string]float64{}
	totalPairs := 0
	for _, n := range pairs {
		totalPairs += n
	}
	if totalPairs > 0 {
		targetPairs := pairs[target+"-Ti"] + pairs["Ti-"+target]
		wc[target+"-Ti"] = 1 - float64(targetPairs)/float64(totalPairs)
	} else {
		wc[target+"-Ti"] = 0
	}
	markGeometryOnly(&out, "local_chemistry")
	out.Meta["operation"] = "local_chemistry"
	out.Meta["local_chemistry_kind"] = kind
	out.Meta["target_element"] = target
	out.Meta["cluster_size"] = size
	out.Meta["random_seed"] = seed
	out.Meta["region"] = opts.Region
	out.Meta["region_inside_counts"] = inside
	out.Meta["region_outside_counts"] = outside
	out.Meta["nearest_neighbor_pair_counts"] = pairs
	out.Meta["warren_cowley_diagnostic"] = wc
	return LocalChemistryModel{Structure: out, Kind: kind, TargetElement: target, ClusterSize: size, Seed: seed, RegionInside: inside, RegionOutside: outside, PairCounts: pairs, WarrenCowley: wc}, nil
}

func nearestPairCounts(s Structure, cutoffFactor float64) map[string]int {
	pairs := map[string]int{}
	if s.NAtoms() < 2 {
		return pairs
	}
	ref := s.MinimumDistance()
	if !finite(ref) || ref <= 0 {
		return pairs
	}
	cut := ref * cutoffFactor
	for i := 0; i < s.NAtoms()-1; i++ {
		for j := i + 1; j < s.NAtoms(); j++ {
			if Norm(VSub(s.Positions[i], s.Positions[j])) <= cut {
				a, b := s.Species[i], s.Species[j]
				if a > b {
					a, b = b, a
				}
				pairs[a+"-"+b]++
			}
		}
	}
	return pairs
}

func BuildCrack(host Structure, opts CrackOptions) (CrackModel, error) {
	if host.NAtoms() == 0 {
		return CrackModel{}, errors.New("empty host structure")
	}
	min, max := bounds(host)
	c := centerOf(host)
	length := opts.Length
	if length <= 0 {
		length = 0.35 * (max[0] - min[0])
	}
	opening := opts.Opening
	if opening <= 0 {
		opening = math.Max(0.8, 0.25*host.MinimumDistance())
	}
	positions := []Vec3{}
	species := []string{}
	labels := []string{}
	removed := 0
	for i, p := range host.Positions {
		inNotch := math.Abs(p[1]-c[1]) <= opening && p[0] <= c[0] && p[0] >= c[0]-length
		if inNotch {
			removed++
			continue
		}
		label := "bulk"
		if math.Abs(p[1]-c[1]) <= 2.2*opening && p[0] <= c[0]+host.MinimumDistance() && p[0] >= c[0]-length-host.MinimumDistance() {
			label = "crack_surface"
		}
		positions = append(positions, p)
		species = append(species, host.Species[i])
		labels = append(labels, label)
	}
	out := copyStructure(host)
	out.Positions = positions
	out.Species = species
	out.SiteLabels = labels
	if opts.Vacuum > 0 {
		out.Cell[1] = VAdd(out.Cell[1], Vec3{0, opts.Vacuum, 0})
		out.PBC[1] = false
	}
	markGeometryOnly(&out, "crack")
	out.Meta["operation"] = "crack"
	out.Meta["crack_plane"] = defaultString(opts.Plane, "(010)")
	out.Meta["crack_front"] = defaultString(opts.Front, "[001]")
	out.Meta["crack_length_angstrom"] = length
	out.Meta["crack_opening_angstrom"] = opening
	out.Meta["removed_atom_count"] = removed
	out.Meta["free_surface_or_vacuum"] = opts.Vacuum > 0
	return CrackModel{Structure: out, Plane: defaultString(opts.Plane, "(010)"), Front: defaultString(opts.Front, "[001]"), RemovedAtomCount: removed}, nil
}

func BuildNanoindentation(host Structure, opts IndenterOptions) (NanoindentationModel, error) {
	if host.NAtoms() == 0 {
		return NanoindentationModel{}, errors.New("empty host structure")
	}
	min, max := bounds(host)
	radius := opts.Radius
	if radius <= 0 {
		radius = 0.25 * math.Min(max[0]-min[0], max[1]-min[1])
	}
	depth := opts.Depth
	if depth <= 0 {
		depth = 0.5 * host.MinimumDistance()
	}
	center := Vec3{(min[0] + max[0]) / 2, (min[1] + max[1]) / 2, max[2] + radius - depth}
	out := copyStructure(host)
	out.SiteLabels = make([]string, out.NAtoms())
	for i, p := range out.Positions {
		rxy := math.Hypot(p[0]-center[0], p[1]-center[1])
		if rxy <= radius && p[2] > max[2]-2*depth-host.MinimumDistance() {
			out.SiteLabels[i] = "near_indenter"
		} else {
			out.SiteLabels[i] = "substrate"
		}
	}
	out.PBC[2] = false
	markGeometryOnly(&out, "nanoindentation")
	out.Meta["operation"] = "nanoindentation"
	out.Meta["indenter_shape"] = "sphere_reference"
	out.Meta["indenter_radius_angstrom"] = radius
	out.Meta["indentation_depth_angstrom"] = depth
	out.Meta["indenter_center"] = center
	out.Meta["contact_solution_absent"] = true
	return NanoindentationModel{Structure: out, IndenterRadius: radius, Depth: depth, IndenterCenter: center}, nil
}

func BuildPolycrystal(host Structure, opts PolycrystalOptions) (PolycrystalModel, error) {
	if host.NAtoms() == 0 {
		return PolycrystalModel{}, errors.New("empty host structure")
	}
	grains := opts.GrainCount
	if grains < 1 {
		grains = 4
	}
	if grains > host.NAtoms() {
		grains = host.NAtoms()
	}
	seed := opts.Seed
	if seed == 0 {
		seed = 20260829
	}
	r := rand.New(rand.NewSource(seed))
	centers := make([]Vec3, grains)
	for i := range centers {
		centers[i] = Vec3{r.Float64(), r.Float64(), r.Float64()}
	}
	out := copyStructure(host)
	frac := host.Fractional(true)
	out.SiteLabels = make([]string, out.NAtoms())
	counts := map[string]int{}
	for i, f := range frac {
		best := 0
		bestD := math.Inf(1)
		for g, c := range centers {
			d := periodicFracDistance(f, c)
			if d < bestD {
				bestD = d
				best = g
			}
		}
		label := fmt.Sprintf("grain_%d", best)
		out.SiteLabels[i] = label
		counts[label]++
	}
	orientations := make([]Mat3, grains)
	for i := range orientations {
		orientations[i] = zRotation((float64(i) / float64(grains)) * math.Pi / 3)
	}
	markGeometryOnly(&out, "polycrystal")
	out.Meta["operation"] = "polycrystal"
	out.Meta["grain_count"] = grains
	out.Meta["grain_atom_counts"] = counts
	out.Meta["voronoi_seed"] = seed
	out.Meta["boundary_labels"] = "grain_i site labels"
	return PolycrystalModel{Structure: out, GrainAtomCounts: counts, Orientations: orientations}, nil
}

func periodicFracDistance(a, b Vec3) float64 {
	d2 := 0.0
	for i := 0; i < 3; i++ {
		x := a[i] - b[i]
		x -= math.Round(x)
		d2 += x * x
	}
	return math.Sqrt(d2)
}

func GenerateNEBSeries(host Structure, opts NEBOptions) (NEBSeries, error) {
	if host.NAtoms() == 0 {
		return NEBSeries{}, errors.New("empty host structure")
	}
	site := opts.MovingSite
	if site < 0 || site >= host.NAtoms() {
		site = 0
	}
	images := opts.Images
	if images < 0 {
		images = 0
	}
	shift := opts.FinalShift
	if Norm(shift) == 0 {
		shift = VScale(host.Cell[0], 1/math.Max(1, math.Round(math.Cbrt(float64(host.NAtoms())))))
	}
	out := NEBSeries{Reference: copyStructure(host)}
	markGeometryOnly(&out.Reference, "neb_reference")
	total := images + 2
	for i := 0; i < total; i++ {
		lam := float64(i) / float64(total-1)
		s := copyStructure(host)
		s.Positions[site] = VAdd(s.Positions[site], VScale(shift, lam))
		markGeometryOnly(&s, "neb_image")
		s.Meta["operation"] = "neb_initial_final_or_interpolated"
		s.Meta["neb_lambda"] = lam
		s.Meta["moving_site"] = site
		s.Meta["final_shift_angstrom"] = shift
		out.Points = append(out.Points, NEBPoint{Index: i, Lambda: lam, Structure: s})
	}
	return out, nil
}

func GenerateTrainingCandidates(host Structure, opts TrainingCandidateOptions) []Structure {
	count := opts.Count
	if count < 1 {
		count = 1
	}
	baseFrac := host.Fractional(true)
	out := make([]Structure, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, generateTrainingCandidate(host, baseFrac, opts, i))
	}
	return out
}

func GenerateTrainingCandidate(host Structure, opts TrainingCandidateOptions, index int) Structure {
	if index < 0 {
		index = 0
	}
	return generateTrainingCandidate(host, host.Fractional(true), opts, index)
}

func generateTrainingCandidate(host Structure, baseFrac []Vec3, opts TrainingCandidateOptions, index int) Structure {
	rattle := opts.MaxRattleAngstrom
	if rattle <= 0 {
		rattle = 0.025
	}
	strain := opts.MaxStrainMagnitude
	if strain <= 0 {
		strain = 0.015
	}
	s := copyStructure(host)
	variant := "base"
	if index > 0 {
		amp := float64((index%7)-3) / 3 * strain
		shear := float64(((index*2)%5)-2) / 2 * strain * 0.4
		cell := s.Cell
		cell[0] = VScale(host.Cell[0], 1+amp)
		cell[1] = VAdd(VScale(host.Cell[1], 1-0.5*amp), VScale(host.Cell[0], shear))
		cell[2] = VScale(host.Cell[2], 1+0.25*amp)
		s.Cell = cell
		rng := rand.New(rand.NewSource(opts.Seed + int64(index)*7919))
		for site, f := range baseFrac {
			p := FracToCart(f, cell)
			disp := Vec3{
				(rng.Float64()*2 - 1) * rattle,
				(rng.Float64()*2 - 1) * rattle,
				(rng.Float64()*2 - 1) * rattle,
			}
			s.Positions[site] = VAdd(p, disp)
		}
		variant = "strained_rattled"
	}
	markGeometryOnly(&s, "training_configuration")
	s.Meta["operation"] = "training_set_candidate"
	s.Meta["training_candidate_index"] = index
	s.Meta["training_variant"] = variant
	s.Meta["random_seed"] = opts.Seed
	s.Meta["max_rattle_angstrom"] = rattle
	s.Meta["max_strain_magnitude"] = strain
	return s
}

func BuildTrainingSet(structures []Structure, opts DatasetOptions) TrainingSet {
	kind := strings.ToLower(strings.TrimSpace(opts.Kind))
	if kind == "" {
		kind = "nep"
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = "TiAlloyStudio-training-set"
	}
	out := TrainingSet{Kind: kind, Name: name}
	for i, s := range structures {
		cp := copyStructure(s)
		markGeometryOnly(&cp, "training_configuration")
		cp.Meta["dataset_kind"] = kind
		cp.Meta["dataset_name"] = name
		cp.Meta["dataset_index"] = i
		out.Structures = append(out.Structures, cp)
	}
	return out
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

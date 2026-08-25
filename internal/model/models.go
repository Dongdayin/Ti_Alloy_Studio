package model

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
)

type EOSPoint struct {
	Index       int       `json:"index"`
	LinearScale float64   `json:"linear_scale"`
	VolumeRatio float64   `json:"volume_ratio"`
	Structure   Structure `json:"structure"`
}
type EOSSeries struct {
	Reference Structure  `json:"reference"`
	Points    []EOSPoint `json:"points"`
}

func GenerateEOS(s Structure, ratios []float64) EOSSeries {
	out := EOSSeries{Reference: s}
	frac := s.Fractional(true)
	for i, r := range ratios {
		scale := math.Cbrt(r)
		cell := s.Cell
		for k := 0; k < 3; k++ {
			cell[k] = VScale(cell[k], scale)
		}
		pos := make([]Vec3, len(frac))
		for j, f := range frac {
			pos[j] = FracToCart(f, cell)
		}
		p := s
		p.Cell = cell
		p.Positions = pos
		p.Meta = cloneMeta(s.Meta)
		p.Meta["model_kind"] = "eos_point"
		p.Meta["volume_ratio"] = r
		out.Points = append(out.Points, EOSPoint{i, scale, r, p})
	}
	return out
}

type SurfaceModel struct {
	Structure Structure `json:"structure"`
	Plane     string    `json:"plane"`
	Normal    Vec3      `json:"normal"`
	Area      float64   `json:"area_angstrom2"`
	Thickness float64   `json:"thickness_angstrom"`
	Vacuum    float64   `json:"vacuum_angstrom"`
}

func rewrapCell(s Structure, cell Mat3) Structure {
	frac := make([]Vec3, len(s.Positions))
	for i, p := range s.Positions {
		f := CartToFrac(p, cell)
		for a := 0; a < 3; a++ {
			if s.PBC[a] {
				f[a] = Wrap01(f[a])
			}
		}
		frac[i] = f
	}
	out := s
	out.Cell = cell
	out.Positions = make([]Vec3, len(frac))
	for i, f := range frac {
		out.Positions[i] = FracToCart(f, cell)
	}
	return out
}
func makeSurface(base Structure, plane string, rxy [2]int, nz int, vac float64) SurfaceModel {
	slab := base.Repeat(rxy[0], rxy[1], nz)
	cross := Cross(slab.Cell[0], slab.Cell[1])
	area := Norm(cross)
	normal := Unit(cross)
	h := Dot(slab.Cell[2], normal)
	if h < 0 {
		normal = VScale(normal, -1)
		h = -h
	}
	cell := slab.Cell
	cell[2] = VAdd(cell[2], VScale(normal, vac))
	pos := make([]Vec3, len(slab.Positions))
	for i, p := range slab.Positions {
		pos[i] = VAdd(p, VScale(normal, vac/2))
	}
	out := slab
	out.Cell = cell
	out.Positions = pos
	out.PBC = [3]bool{true, true, false}
	out.Meta = cloneMeta(slab.Meta)
	out.Meta["model_kind"] = "surface_slab"
	out.Meta["surface_plane"] = plane
	out.Meta["vacuum_angstrom"] = vac
	return SurfaceModel{out, plane, normal, area, h, vac}
}
func AlphaSurface(preset string, a, c float64, rxy [2]int, nz int, vac float64) SurfaceModel {
	p := BuildAlphaTi(a, c)
	if preset == "prismatic_10-10" {
		cell := Mat3{p.Cell[1], p.Cell[2], p.Cell[0]}
		p = rewrapCell(p, cell)
		return makeSurface(p, "(10-10)", rxy, nz, vac)
	}
	return makeSurface(p, "(0001)", rxy, nz, vac)
}
func BetaSurface100(a float64, rxy [2]int, nz int, vac float64) SurfaceModel {
	cell := Mat3{{0, a, 0}, {0, 0, a}, {a, 0, 0}}
	frac := []Vec3{{0, 0, 0}, {.5, .5, .5}}
	pos := []Vec3{FracToCart(frac[0], cell), FracToCart(frac[1], cell)}
	s := Structure{Cell: cell, Positions: pos, Species: []string{"Ti", "Ti"}, PBC: [3]bool{true, true, true}, Meta: map[string]any{"phase": "beta"}}
	return makeSurface(s, "(100)", rxy, nz, vac)
}

type GSFEPoint struct {
	Index     int       `json:"index"`
	Lambda    float64   `json:"lambda"`
	Shift     Vec3      `json:"shift"`
	Structure Structure `json:"structure"`
}
type GSFESeries struct {
	Reference   Structure   `json:"reference"`
	Points      []GSFEPoint `json:"points"`
	NormalAxis  int         `json:"normal_axis"`
	PlaneNormal Vec3        `json:"plane_normal"`
	Path        Vec3        `json:"path"`
	Area        float64     `json:"area_angstrom2"`
	FaultCount  int         `json:"fault_count"`
	Cut         float64     `json:"cut_fraction"`
	Plane       string      `json:"plane"`
	Direction   string      `json:"direction"`
	Preset      string      `json:"preset"`
}

func GenerateGSFE(ref Structure, path Vec3, normalAxis, nSteps int, cut float64, plane, dir, preset string) GSFESeries {
	axes := []int{}
	for a := 0; a < 3; a++ {
		if a != normalAxis {
			axes = append(axes, a)
		}
	}
	cross := Cross(ref.Cell[axes[0]], ref.Cell[axes[1]])
	area := Norm(cross)
	normal := Unit(cross)
	faults := 1
	if ref.PBC[normalAxis] {
		faults = 2
	}
	rf := ref.Fractional(true)
	upper := make([]bool, len(rf))
	for i, f := range rf {
		upper[i] = f[normalAxis] >= cut
	}
	series := GSFESeries{Reference: ref, NormalAxis: normalAxis, PlaneNormal: normal, Path: path, Area: area, FaultCount: faults, Cut: cut, Plane: plane, Direction: dir, Preset: preset}
	for i := 0; i <= nSteps; i++ {
		lam := float64(i) / float64(nSteps)
		shift := VScale(path, lam)
		p := ref
		p.Positions = append([]Vec3(nil), ref.Positions...)
		for j := range p.Positions {
			if upper[j] {
				p.Positions[j] = VAdd(p.Positions[j], shift)
			}
			f := CartToFrac(p.Positions[j], ref.Cell)
			for a := 0; a < 3; a++ {
				if ref.PBC[a] {
					f[a] = Wrap01(f[a])
				}
			}
			p.Positions[j] = FracToCart(f, ref.Cell)
		}
		p.Meta = cloneMeta(ref.Meta)
		p.Meta["model_kind"] = "gsfe_point"
		p.Meta["lambda"] = lam
		series.Points = append(series.Points, GSFEPoint{i, lam, shift, p})
	}
	return series
}
func AlphaGSFE(preset string, a, c float64, rep [3]int, steps int, cut float64) GSFESeries {
	p := BuildAlphaTi(a, c)
	plane, dir := "{0001}", "<11-20>"
	if preset == "prismatic_a" {
		p = rewrapCell(p, Mat3{p.Cell[1], p.Cell[2], p.Cell[0]})
		plane = "{10-10}"
	}
	ref := p.Repeat(rep[0], rep[1], rep[2])
	path := VScale(ref.Cell[0], 1/float64(rep[0]))
	return GenerateGSFE(ref, path, 2, steps, cut, plane, dir, preset)
}
func BetaGSFE(a float64, rep [3]int, steps int, cut float64) GSFESeries {
	p := BuildBetaTiPrimitive(a)
	tr := [3][3]int{{0, 1, 0}, {-3, -1, 0}, {1, 1, 2}}
	ori, err := IntegerSupercell(p, tr)
	if err != nil {
		return GSFESeries{}
	}
	ref := ori.Repeat(rep[0], rep[1], rep[2])
	path := VScale(ref.Cell[0], 1/float64(rep[0]))
	return GenerateGSFE(ref, path, 2, steps, cut, "{110}", "<111>", "110_111")
}

type BurgersOR struct {
	AlphaU, AlphaV, AlphaN Vec3
	BetaU, BetaV, BetaN    Vec3
	Rotation               Mat3
	NormalErrorDeg         float64 `json:"normal_error_deg"`
	DirectionErrorDeg      float64 `json:"direction_error_deg"`
}
type InterfaceCandidate struct {
	ARX, ARY, BRX, BRY           int `json:"-"`
	AlphaRepeatX                 int `json:"alpha_repeat_x"`
	AlphaRepeatY                 int `json:"alpha_repeat_y"`
	BetaRepeatX                  int `json:"beta_repeat_x"`
	BetaRepeatY                  int `json:"beta_repeat_y"`
	AlphaX, AlphaY, BetaX, BetaY float64
	CommonX                      float64 `json:"common_x_angstrom"`
	CommonY                      float64 `json:"common_y_angstrom"`
	MismatchXPercent             float64 `json:"mismatch_x_percent"`
	MismatchYPercent             float64 `json:"mismatch_y_percent"`
	AlphaStrainXPercent          float64 `json:"alpha_strain_x_percent"`
	AlphaStrainYPercent          float64 `json:"alpha_strain_y_percent"`
	BetaStrainXPercent           float64 `json:"beta_strain_x_percent"`
	BetaStrainYPercent           float64 `json:"beta_strain_y_percent"`
	MaxImposedStrainPercent      float64 `json:"max_imposed_strain_percent"`
	Score                        float64 `json:"score"`
}

func angleDeg(a, b Vec3) float64 {
	return math.Acos(Clamp(Dot(Unit(a), Unit(b)), -1, 1)) * 180 / math.Pi
}
func frameRotation(a1, a2, a3, b1, b2, b3 Vec3) Mat3 {
	A := Mat3{{a1[0], a2[0], a3[0]}, {a1[1], a2[1], a3[1]}, {a1[2], a2[2], a3[2]}}
	B := Mat3{{b1[0], b2[0], b3[0]}, {b1[1], b2[1], b3[1]}, {b1[2], b2[2], b3[2]}}
	return MatMul(A, Transpose(B))
}
func matVec(m Mat3, v Vec3) Vec3 {
	return Vec3{m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2], m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2], m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2]}
}
func BurgersGeometry(aa, ca, ab float64) BurgersOR {
	au := Vec3{aa, 0, 0}
	av := Vec3{0, math.Sqrt(3) * aa, 0}
	an := Vec3{0, 0, 1}
	bu := VScale(Vec3{1, -1, 1}, .5*ab)
	bv := VScale(Vec3{1, -1, -2}, ab)
	bn := Unit(Vec3{1, 1, 0})
	ae1 := Unit(au)
	ae3 := Unit(an)
	ae2 := Unit(Cross(ae3, ae1))
	be1 := Unit(bu)
	be3 := bn
	be2 := Unit(Cross(be3, be1))
	if Dot(bv, be2) < 0 {
		bv = VScale(bv, -1)
	}
	r := frameRotation(ae1, ae2, ae3, be1, be2, be3)
	return BurgersOR{au, av, an, bu, bv, bn, r, angleDeg(an, matVec(r, bn)), angleDeg(au, matVec(r, bu))}
}

type interfaceCandidateMaxHeap []InterfaceCandidate
func (h interfaceCandidateMaxHeap) Len() int { return len(h) }
func (h interfaceCandidateMaxHeap) Less(i, j int) bool { return h[i].Score > h[j].Score }
func (h interfaceCandidateMaxHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *interfaceCandidateMaxHeap) Push(x any) { *h = append(*h, x.(InterfaceCandidate)) }
func (h *interfaceCandidateMaxHeap) Pop() any { old:=*h; n:=len(old); x:=old[n-1]; *h=old[:n-1]; return x }

func SearchBurgersMatches(g BurgersOR, maxRep, limit int) []InterfaceCandidate {
	if maxRep < 1 || limit < 1 { return nil }
	ax, ay, bx, by := Norm(g.AlphaU), Norm(g.AlphaV), Norm(g.BetaU), Norm(g.BetaV)
	best := &interfaceCandidateMaxHeap{}
	heap.Init(best)
	for arx := 1; arx <= maxRep; arx++ {
		for brx := 1; brx <= maxRep; brx++ {
			lax, lbx := float64(arx)*ax, float64(brx)*bx
			mx := 100 * (lbx - lax) / lax
			commonX := 2 * lax * lbx / (lax + lbx)
			asx := 100 * (commonX/lax - 1)
			bsx := 100 * (commonX/lbx - 1)
			for ary := 1; ary <= maxRep; ary++ {
				for bry := 1; bry <= maxRep; bry++ {
					lay, lby := float64(ary)*ay, float64(bry)*by
					my := 100 * (lby - lay) / lay
					commonY := 2 * lay * lby / (lay + lby)
					asy := 100 * (commonY/lay - 1)
					bsy := 100 * (commonY/lby - 1)
					maxstrain := math.Max(math.Max(math.Abs(asx), math.Abs(asy)), math.Max(math.Abs(bsx), math.Abs(bsy)))
					score := maxstrain + 1e-6*float64(arx+brx+ary+bry)
					c := InterfaceCandidate{ARX: arx, ARY: ary, BRX: brx, BRY: bry, AlphaRepeatX: arx, AlphaRepeatY: ary, BetaRepeatX: brx, BetaRepeatY: bry, AlphaX: lax, AlphaY: lay, BetaX: lbx, BetaY: lby, CommonX: commonX, CommonY: commonY, MismatchXPercent: mx, MismatchYPercent: my, AlphaStrainXPercent: asx, AlphaStrainYPercent: asy, BetaStrainXPercent: bsx, BetaStrainYPercent: bsy, MaxImposedStrainPercent: maxstrain, Score: score}
					if best.Len() < limit { heap.Push(best,c) } else if c.Score < (*best)[0].Score { heap.Pop(best); heap.Push(best,c) }
				}
			}
		}
	}
	out := append([]InterfaceCandidate(nil), (*best)...)
	sort.Slice(out, func(i,j int) bool { return out[i].Score < out[j].Score })
	return out
}

func IntegerSupercell(s Structure, t [3][3]int) (Structure, error) {
	det := t[0][0]*(t[1][1]*t[2][2]-t[1][2]*t[2][1]) - t[0][1]*(t[1][0]*t[2][2]-t[1][2]*t[2][0]) + t[0][2]*(t[1][0]*t[2][1]-t[1][1]*t[2][0])
	if det == 0 { return Structure{}, fmt.Errorf("singular transform") }
	var nc Mat3
	for i:=0;i<3;i++ { for j:=0;j<3;j++ { for k:=0;k<3;k++ { nc[i][j]+=float64(t[i][k])*s.Cell[k][j] } } }
	inv,_:=Inverse(nc); old:=s.Fractional(true)
	minv,maxv:=[3]int{0,0,0},[3]int{0,0,0}; first:=true
	for bits:=0;bits<8;bits++ { var v [3]int; for j:=0;j<3;j++ { if bits&(1<<j)!=0 { for k:=0;k<3;k++ { v[k]+=t[j][k] } } }; if first {minv=v;maxv=v;first=false} else {for k:=0;k<3;k++ {if v[k]<minv[k]{minv[k]=v[k]};if v[k]>maxv[k]{maxv[k]=v[k]}}}}
	for k:=0;k<3;k++ {minv[k]--;maxv[k]++}
	type rec struct{f Vec3;e string}; sel:=map[string]rec{}
	for tx:=minv[0];tx<=maxv[0];tx++ {for ty:=minv[1];ty<=maxv[1];ty++ {for tz:=minv[2];tz<=maxv[2];tz++ {sh:=Vec3{float64(tx),float64(ty),float64(tz)};for i,f:=range old {cart:=FracToCart(VAdd(f,sh),s.Cell);fn:=Vec3{cart[0]*inv[0][0]+cart[1]*inv[1][0]+cart[2]*inv[2][0],cart[0]*inv[0][1]+cart[1]*inv[1][1]+cart[2]*inv[2][1],cart[0]*inv[0][2]+cart[1]*inv[1][2]+cart[2]*inv[2][2]};ok:=true;for k:=0;k<3;k++ {if fn[k]<-1e-9||fn[k]>=1-1e-9{ok=false}};if ok{key:=fmt.Sprintf("%.9f,%.9f,%.9f,%s",fn[0],fn[1],fn[2],s.Species[i]);sel[key]=rec{fn,s.Species[i]}}}}}}
	expected:=s.NAtoms()*int(math.Abs(float64(det)));if len(sel)!=expected{return Structure{},fmt.Errorf("supercell atoms %d expected %d",len(sel),expected)}
	keys:=make([]string,0,len(sel));for k:=range sel{keys=append(keys,k)};sort.Strings(keys);out:=Structure{Cell:nc,PBC:s.PBC,Meta:cloneMeta(s.Meta)};for _,k:=range keys{r:=sel[k];out.Positions=append(out.Positions,FracToCart(r.f,nc));out.Species=append(out.Species,r.e)};return out,nil
}

type BurgersInterface struct { Structure Structure `json:"structure"`; Candidate InterfaceCandidate `json:"candidate"`; AlphaAtoms int `json:"alpha_atoms"`; BetaAtoms int `json:"beta_atoms"`; InterfaceDistance float64 `json:"interface_distance"`; Vacuum float64 `json:"vacuum"` }
func BuildBurgersInterface(g BurgersOR, c InterfaceCandidate, aa, ca, ab float64, az, bz int, dist, vac float64) BurgersInterface {
	alpha,_:=IntegerSupercell(BuildAlphaTi(aa,ca),[3][3]int{{1,0,0},{1,2,0},{0,0,1}});alpha=alpha.Repeat(c.ARX,c.ARY,az)
	raw,_:=IntegerSupercell(BuildBetaTiPrimitive(ab),[3][3]int{{0,1,0},{-3,-1,0},{1,1,2}});var bc Mat3;for i:=0;i<3;i++{bc[i]=matVec(g.Rotation,raw.Cell[i])};bp:=make([]Vec3,len(raw.Positions));for i,p:=range raw.Positions{bp[i]=matVec(g.Rotation,p)};beta:=Structure{Cell:bc,Positions:bp,Species:raw.Species,PBC:raw.PBC,Meta:map[string]any{"phase":"beta"}}.Repeat(c.BRX,c.BRY,bz)
	af,bf:=alpha.Fractional(true),beta.Fractional(true);xdir:=Unit(alpha.Cell[0]);ydir:=Unit(alpha.Cell[1]);common0:=VScale(xdir,c.CommonX);common1:=VScale(ydir,c.CommonY);alpha.Cell[0],alpha.Cell[1]=common0,common1;beta.Cell[0],beta.Cell[1]=common0,common1;for i,f:=range af{alpha.Positions[i]=FracToCart(f,alpha.Cell)};for i,f:=range bf{beta.Positions[i]=FracToCart(f,beta.Cell)}
	minA,maxA:=math.Inf(1),math.Inf(-1);for _,p:=range alpha.Positions{minA=math.Min(minA,p[2])};for i:=range alpha.Positions{alpha.Positions[i][2]+=vac/2-minA;maxA=math.Max(maxA,alpha.Positions[i][2])};minB,maxB:=math.Inf(1),math.Inf(-1);for _,p:=range beta.Positions{minB=math.Min(minB,p[2])};for i:=range beta.Positions{beta.Positions[i][2]+=maxA+dist-minB;maxB=math.Max(maxB,beta.Positions[i][2])}
	cell:=Mat3{alpha.Cell[0],alpha.Cell[1],Vec3{0,0,maxB+vac/2}};pos:=append(append([]Vec3{},alpha.Positions...),beta.Positions...);sp:=append(append([]string{},alpha.Species...),beta.Species...);labels:=make([]string,0,len(sp));for range alpha.Species{labels=append(labels,"alpha")};for range beta.Species{labels=append(labels,"beta")};s:=Structure{Cell:cell,Positions:pos,Species:sp,SiteLabels:labels,PBC:[3]bool{true,true,false},Meta:map[string]any{"model_kind":"alpha_beta_interface","orientation_relation":"Burgers","interface_distance_angstrom":dist,"vacuum_angstrom":vac,"mismatch_x_percent":c.MismatchXPercent,"mismatch_y_percent":c.MismatchYPercent,"max_imposed_strain_percent":c.MaxImposedStrainPercent,"strain_strategy":"balanced_harmonic_mean"}};return BurgersInterface{s,c,alpha.NAtoms(),beta.NAtoms(),dist,vac}
}

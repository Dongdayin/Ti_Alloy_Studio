package model

import "math"

type Vec3 [3]float64
type Mat3 [3]Vec3

func VAdd(a, b Vec3) Vec3           { return Vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func VSub(a, b Vec3) Vec3           { return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func VScale(a Vec3, s float64) Vec3 { return Vec3{a[0] * s, a[1] * s, a[2] * s} }
func Dot(a, b Vec3) float64         { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func Cross(a, b Vec3) Vec3 {
	return Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func Norm(a Vec3) float64 { return math.Sqrt(Dot(a, a)) }
func Unit(a Vec3) Vec3 {
	n := Norm(a)
	if n == 0 {
		return Vec3{}
	}
	return VScale(a, 1/n)
}
func Wrap01(x float64) float64 {
	x = math.Mod(x, 1)
	if x < 0 {
		x += 1
	}
	if math.Abs(x-1) < 1e-12 || math.Abs(x) < 1e-12 {
		return 0
	}
	return x
}

func Determinant(m Mat3) float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) + m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}
func Inverse(m Mat3) (Mat3, bool) {
	d := Determinant(m)
	if math.Abs(d) < 1e-14 {
		return Mat3{}, false
	}
	inv := Mat3{
		{(m[1][1]*m[2][2] - m[1][2]*m[2][1]) / d, (m[0][2]*m[2][1] - m[0][1]*m[2][2]) / d, (m[0][1]*m[1][2] - m[0][2]*m[1][1]) / d},
		{(m[1][2]*m[2][0] - m[1][0]*m[2][2]) / d, (m[0][0]*m[2][2] - m[0][2]*m[2][0]) / d, (m[0][2]*m[1][0] - m[0][0]*m[1][2]) / d},
		{(m[1][0]*m[2][1] - m[1][1]*m[2][0]) / d, (m[0][1]*m[2][0] - m[0][0]*m[2][1]) / d, (m[0][0]*m[1][1] - m[0][1]*m[1][0]) / d},
	}
	return inv, true
}

// row vector multiplied by matrix whose row vectors are cell basis.
func FracToCart(f Vec3, cell Mat3) Vec3 {
	return Vec3{f[0]*cell[0][0] + f[1]*cell[1][0] + f[2]*cell[2][0], f[0]*cell[0][1] + f[1]*cell[1][1] + f[2]*cell[2][1], f[0]*cell[0][2] + f[1]*cell[1][2] + f[2]*cell[2][2]}
}
func CartToFrac(r Vec3, cell Mat3) Vec3 {
	inv, ok := Inverse(cell)
	if !ok {
		return Vec3{math.NaN(), math.NaN(), math.NaN()}
	}
	return Vec3{r[0]*inv[0][0] + r[1]*inv[1][0] + r[2]*inv[2][0], r[0]*inv[0][1] + r[1]*inv[1][1] + r[2]*inv[2][1], r[0]*inv[0][2] + r[1]*inv[1][2] + r[2]*inv[2][2]}
}
func MatMul(a, b Mat3) Mat3 {
	var c Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				c[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return c
}
func Transpose(a Mat3) Mat3 {
	return Mat3{{a[0][0], a[1][0], a[2][0]}, {a[0][1], a[1][1], a[2][1]}, {a[0][2], a[1][2], a[2][2]}}
}
func VecMat(v Vec3, m Mat3) Vec3 {
	return Vec3{v[0]*m[0][0] + v[1]*m[1][0] + v[2]*m[2][0], v[0]*m[0][1] + v[1]*m[1][1] + v[2]*m[2][1], v[0]*m[0][2] + v[1]*m[1][2] + v[2]*m[2][2]}
}
func Clamp(x, a, b float64) float64 {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}

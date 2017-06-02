package gonvim

// RGBA is
type RGBA struct {
	R float64
	G float64
	B float64
	A float64
}

func (rgba *RGBA) copy() *RGBA {
	return &RGBA{
		R: rgba.R,
		G: rgba.G,
		B: rgba.B,
		A: rgba.A,
	}
}

func (rgba *RGBA) equals(other *RGBA) bool {
	return rgba.R == other.R && rgba.G == other.G && rgba.B == other.B && rgba.A == other.A
}

func calcColor(c int) *RGBA {
	b := float64(c&255) / 255
	g := float64((c>>8)&255) / 255
	r := float64((c>>16)&255) / 255
	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: 1,
	}
}

func newRGBA(r int, g int, b int, a float64) *RGBA {
	return &RGBA{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
		A: a,
	}
}

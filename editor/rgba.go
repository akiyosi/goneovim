package editor

import (
	"fmt"
	"math"

	"github.com/therecipe/qt/gui"
)

// RGBA is
type RGBA struct {
	R int
	G int
	B int
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

func (rgba *RGBA) String() string {
	return fmt.Sprintf("rgba(%d, %d, %d, %f)", rgba.R, rgba.G, rgba.B, rgba.A)
}

func (rgba *RGBA) StringTransparent() string {
	transparent := editor.config.Editor.Transparent
	return fmt.Sprintf("rgba(%d, %d, %d, %f)", rgba.R, rgba.G, rgba.B, transparent)
}

func transparent() float64 {
	t := editor.config.Editor.Transparent
	return t * t
}

// Hex is
func (rgba *RGBA) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(rgba.R), uint8(rgba.G), uint8(rgba.B))
}

// input color *RGBA, aplpha (0.0...-1.0..) int
func (rgba *RGBA) brend(color *RGBA, alpha float64) *RGBA {
	return &RGBA{
		R: int((float64(rgba.R) * float64(1-alpha)) + (float64(color.R) * float64(alpha))),
		G: int((float64(rgba.R) * float64(1-alpha)) + (float64(color.G) * float64(alpha))),
		B: int((float64(rgba.R) * float64(1-alpha)) + (float64(color.B) * float64(alpha))),
		A: 1,
	}
}

// QColor is
func (rgba *RGBA) QColor() *gui.QColor {
	return gui.NewQColor3(rgba.R, rgba.G, rgba.B, int(rgba.A*255))
}

func calcColor(c int) *RGBA {
	b := c & 255
	g := (c >> 8) & 255
	r := (c >> 16) & 255
	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: 1,
	}
}

func complementaryColor(rgba *RGBA) *RGBA {
	if rgba == nil {
		return &RGBA{255, 255, 255, 1}
	}
	r := float64(rgba.R)
	g := float64(rgba.G)
	b := float64(rgba.B)
	max := math.Max(math.Max(r, g), b)
	min := math.Min(math.Min(r, g), b)
	c := max + min

	return &RGBA{
		R: int(c - r),
		G: int(c - g),
		B: int(c - b),
		A: 1,
	}
}

func invertColor(rgba *RGBA) *RGBA {
	if rgba == nil {
		return &RGBA{0, 0, 0, 1}
	}
	var r, g, b int

	r = 255 - rgba.R
	g = 255 - rgba.G
	b = 255 - rgba.B

	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: rgba.A,
	}
}

func warpColor(rgba *RGBA, v int) *RGBA {
	if rgba == nil {
		return &RGBA{0, 0, 0, 1}
	}
	var r, g, b int

	if rgba.R > 128 {
		r = rgba.R + v
	} else {
		r = rgba.R - (2 * v)
	}

	if rgba.G > 128 {
		g = rgba.G + v
	} else {
		g = rgba.G - (2 * v)
	}

	if rgba.B > 128 {
		b = rgba.B + v
	} else {
		b = rgba.B - (2 * v)
	}

	if r <= 0 {
		r = 0
	}
	if g <= 0 {
		g = 0
	}
	if b <= 0 {
		b = 0
	}

	if r >= 255 {
		r = 255
	}
	if g >= 255 {
		g = 255
	}
	if b >= 255 {
		b = 255
	}

	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: rgba.A,
	}
}

func shiftColor(rgba *RGBA, v int) *RGBA {
	if rgba == nil {
		return &RGBA{0, 0, 0, 1}
	}
	var r, g, b int
	r = rgba.R - v
	g = rgba.G - v
	b = rgba.B - v

	if r <= 0 {
		r = 0
	}
	if g <= 0 {
		g = 0
	}
	if b <= 0 {
		b = 0
	}

	if r >= 255 {
		r = 255
	}
	if g >= 255 {
		g = 255
	}
	if b >= 255 {
		b = 255
	}

	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: rgba.A,
	}
}

func newRGBA(r int, g int, b int, a float64) *RGBA {
	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: a, // Not use this value
	}
}

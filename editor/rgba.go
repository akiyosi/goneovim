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

type HSV struct {
	H, S, V float64
}

type XYZ struct {
	X, Y, Z float64
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
	if rgba == nil {
		return false
	}
	if other == nil {
		return false
	}
	return rgba.R == other.R && rgba.G == other.G && rgba.B == other.B
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
		R: int((float64(rgba.R) * (1.0 - alpha)) + (float64(color.R) * alpha)),
		G: int((float64(rgba.G) * (1.0 - alpha)) + (float64(color.G) * alpha)),
		B: int((float64(rgba.B) * (1.0 - alpha)) + (float64(color.B) * alpha)),
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
		r = rgba.R - v
	}

	if rgba.G > 128 {
		g = rgba.G + v
	} else {
		g = rgba.G - v
	}

	if rgba.B > 128 {
		b = rgba.B + v
	} else {
		b = rgba.B - v
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

func newRGBA(r int, g int, b int, a float64) *RGBA {
	return &RGBA{
		R: r,
		G: g,
		B: b,
		A: a, // Not use this value
	}
}

func (rgba *RGBA) XYZ() *XYZ {
	r := float64(rgba.R) / 255.0
	g := float64(rgba.G) / 255.0
	b := float64(rgba.B) / 255.0

	return &XYZ{
		X: 2.76883*r + 1.75171*g + 1.13014*b,
		Y: 1.00000*r + 4.59061*g + 0.06007*b,
		Z: 0.00000*r + 0.05651*g + 5.59417*b,
	}
}

func (rgba *RGBA) diff(t *RGBA) float64 {
	dx := rgba.XYZ().X - t.XYZ().X
	dy := rgba.XYZ().Y - t.XYZ().Y
	dz := rgba.XYZ().Z - t.XYZ().Z

	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (rgba *RGBA) HSV() *HSV {
	r := float64(rgba.R) / 255.0
	g := float64(rgba.G) / 255.0
	b := float64(rgba.B) / 255.0

	min := math.Min(r, math.Min(g, b))
	max := math.Max(r, math.Max(g, b))
	del := max - min

	v := max

	var h, s float64

	if del == 0 {
		h = 0
		s = 0
	} else {
		s = del / max

		delR := (((max - r) / 6) + (del / 2)) / del
		delG := (((max - g) / 6) + (del / 2)) / del
		delB := (((max - b) / 6) + (del / 2)) / del

		if r == max {
			h = delB - delG
		} else if g == max {
			h = (1 / 3) + delR - delB
		} else if b == max {
			h = (2 / 3) + delG - delR
		}

		if h < 0 {
			h += 1
		}
		if h > 1 {
			h -= 1
		}
	}

	return &HSV{h, s, v}
}

func (hsv *HSV) Colorfulness() *HSV {
	return &HSV{
		H: hsv.H,
		S: math.Sqrt(hsv.S),
		V: math.Sqrt(hsv.V),
	}
}

func (hsv *HSV) RGB() *RGBA {
	var r, g, b float64
	if hsv.S == 0 {
		r = hsv.V * 255
		g = hsv.V * 255
		b = hsv.V * 255
	} else {
		h := hsv.H * 6
		if h == 6 {
			h = 0
		}
		i := math.Floor(h)
		v1 := hsv.V * (1 - hsv.S)
		v2 := hsv.V * (1 - hsv.S*(h-i))
		v3 := hsv.V * (1 - hsv.S*(1-(h-i)))

		if i == 0 {
			r = hsv.V
			g = v3
			b = v1
		} else if i == 1 {
			r = v2
			g = hsv.V
			b = v1
		} else if i == 2 {
			r = v1
			g = hsv.V
			b = v3
		} else if i == 3 {
			r = v1
			g = v2
			b = hsv.V
		} else if i == 4 {
			r = v3
			g = v1
			b = hsv.V
		} else {
			r = hsv.V
			g = v1
			b = v2
		}

		r = r * 255
		g = g * 255
		b = b * 255
	}

	return &RGBA{
		R: int(r),
		G: int(g),
		B: int(b),
		A: 255,
	}
}

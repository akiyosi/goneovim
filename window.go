package gonvim

import (
	"math"
	"strings"

	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"
)

// Window is
type Window struct {
	AreaHandler
	win        nvim.Window
	width      int
	height     int
	pos        [2]int
	tab        nvim.Tabpage
	statusline string
}

// Draw the window
func (w *Window) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	font := editor.font
	row := int(math.Ceil(dp.ClipY/float64(font.lineHeight))) + w.pos[0]
	col := int(math.Ceil(dp.ClipX/float64(font.width))) + w.pos[1]
	rows := int(math.Ceil(dp.ClipHeight / float64(font.lineHeight)))
	cols := int(math.Ceil(dp.ClipWidth / float64(font.width)))

	p := ui.NewPath(ui.Winding)
	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
	p.End()

	bg := editor.Background

	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    bg.R,
		G:    bg.G,
		B:    bg.B,
		A:    1,
	})
	p.Free()

	for y := row; y < row+rows; y++ {
		if y >= editor.rows {
			continue
		}
		w.fillHightlight(dp, y, col, cols)
		w.drawText(dp, y, col, cols)
	}

	if col-w.pos[1]+cols > w.width {
		w.drawBorder(dp)
	}
}

func (w *Window) drawBorder(dp *ui.AreaDrawParams) {
	drawRect(dp, w.width*editor.font.width, 0, editor.font.width, w.height*editor.font.lineHeight, &editor.Background)

	color := newRGBA(0, 0, 0, 1)

	p := ui.NewPath(ui.Winding)
	p.AddRectangle(
		(float64(w.width))*float64(editor.font.width),
		float64(0),
		float64(editor.font.width)*1,
		float64(w.height*editor.font.lineHeight),
	)
	p.End()
	stops := []ui.GradientStop{}
	n := 100
	for i := 0; i < n; i++ {
		s := ui.GradientStop{
			Pos: float64(i) / float64(n),
			R:   float64(i+10) / 255 / 2,
			G:   float64(i+10) / 255 / 2,
			B:   float64(i+10) / 255 / 2,
			A:   1 - (float64(i) / float64(n)),
		}
		stops = append(stops, s)
	}
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.LinearGradient,
		R:    color.R,
		G:    color.G,
		B:    color.B,
		A:    1,

		X0:    (float64(w.width) + 1.5) * float64(editor.font.width),
		Y0:    0,
		X1:    (float64(w.width) - 0.5) * float64(editor.font.width),
		Y1:    0,
		Stops: stops,
	})
	p.Free()

	p = ui.NewPath(ui.Winding)
	p.AddRectangle((float64(w.width)+1)*float64(editor.font.width),
		float64(0),
		0.5,
		float64(w.height*editor.font.lineHeight),
	)
	// p.AddRectangle(
	// 	0,
	// 	float64(w.height*editor.font.lineHeight)-1,
	// 	float64((w.width+1)*editor.font.width),
	// 	1,
	// )
	p.End()
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    color.R,
		G:    color.G,
		B:    color.B,
		A:    color.A,
	})
	p.Free()
}

func (w *Window) drawText(dp *ui.AreaDrawParams, y int, col int, cols int) {
	screen := editor.screen
	if y >= len(screen.content) {
		return
	}
	line := screen.content[y]
	text := ""
	var specialChars []int
	start := -1
	end := col
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char == nil {
			text += " "
			continue
		}
		if char.char == " " {
			text += " "
			continue
		}
		if char.char == "" {
			text += " "
			continue
		}
		if !isNormalWidth(char.char) {
			text += " "
			specialChars = append(specialChars, x)
			continue
		}
		text += char.char
		if start == -1 {
			start = x
		}
		end = x
	}
	if start == -1 {
		return
	}
	text = strings.TrimSpace(text)
	textLayout := ui.NewTextLayout(text, editor.font.font, -1)
	shift := (editor.font.lineHeight - editor.font.height) / 2

	for x := start; x <= end; x++ {
		char := line[x]
		if char == nil || char.char == " " {
			continue
		}
		fg := editor.Foreground
		if char.highlight.foreground != nil {
			fg = *(char.highlight.foreground)
		}
		textLayout.SetColor(x-start, x-start+1, fg.R, fg.G, fg.B, fg.A)
	}
	dp.Context.Text(float64((start-w.pos[1])*editor.font.width), float64((y-w.pos[0])*editor.font.lineHeight+shift), textLayout)
	textLayout.Free()

	for _, x := range specialChars {
		char := line[x]
		if char == nil || char.char == " " {
			continue
		}
		fg := editor.Foreground
		if char.highlight.foreground != nil {
			fg = *(char.highlight.foreground)
		}
		textLayout := ui.NewTextLayout(char.char, editor.font.font, -1)
		textLayout.SetColor(0, 1, fg.R, fg.G, fg.B, fg.A)
		dp.Context.Text(float64((x-w.pos[1])*editor.font.width), float64((y-w.pos[0])*editor.font.lineHeight+shift), textLayout)
		textLayout.Free()
	}
}

func (w *Window) fillHightlight(dp *ui.AreaDrawParams, y int, col int, cols int) {
	screen := editor.screen
	if y >= len(screen.content) {
		return
	}
	line := screen.content[y]
	start := -1
	end := -1
	var lastBg *RGBA
	var bg *RGBA
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char != nil {
			bg = char.highlight.background
		} else {
			bg = nil
		}
		if bg != nil {
			if lastBg == nil {
				start = x
				end = x
				lastBg = bg
			} else {
				if lastBg.equals(bg) {
					end = x
				} else {
					// last bg is different; draw the previous and start a new one
					p := ui.NewPath(ui.Winding)
					p.AddRectangle(
						float64((start-w.pos[1])*editor.font.width),
						float64((y-w.pos[0])*editor.font.lineHeight),
						float64((end-start+1)*editor.font.width),
						float64(editor.font.lineHeight),
					)
					p.End()
					dp.Context.Fill(p, &ui.Brush{
						Type: ui.Solid,
						R:    lastBg.R,
						G:    lastBg.G,
						B:    lastBg.B,
						A:    lastBg.A,
					})
					p.Free()

					// start a new one
					start = x
					end = x
					lastBg = bg
				}
			}
		} else {
			if lastBg != nil {
				p := ui.NewPath(ui.Winding)
				p.AddRectangle(
					float64((start-w.pos[1])*editor.font.width),
					float64((y-w.pos[0])*editor.font.lineHeight),
					float64((end-start+1)*editor.font.width),
					float64(editor.font.lineHeight),
				)
				p.End()
				dp.Context.Fill(p, &ui.Brush{
					Type: ui.Solid,
					R:    lastBg.R,
					G:    lastBg.G,
					B:    lastBg.B,
					A:    lastBg.A,
				})
				p.Free()

				// start a new one
				start = x
				end = x
				lastBg = nil
			}
		}
	}
	if lastBg != nil {
		p := ui.NewPath(ui.Winding)
		p.AddRectangle(
			float64((start-w.pos[1])*editor.font.width),
			float64((y-w.pos[0])*editor.font.lineHeight),
			float64((end-start+1)*editor.font.width),
			float64(editor.font.lineHeight),
		)
		p.End()

		dp.Context.Fill(p, &ui.Brush{
			Type: ui.Solid,
			R:    lastBg.R,
			G:    lastBg.G,
			B:    lastBg.B,
			A:    lastBg.A,
		})
		p.Free()
	}
}

func (w *Window) queueRedraw(x, y, width, heigt int) {
	ui.QueueMain(func() {
		w.area.QueueRedraw(
			float64(x*editor.font.width),
			float64(y*editor.font.lineHeight),
			float64(width*editor.font.width),
			float64(heigt*editor.font.lineHeight),
		)
	})
}

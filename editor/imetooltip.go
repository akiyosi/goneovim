package editor

import (
	"fmt"
	"runtime"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
)

// IMETooltip is the tooltip for Input Method Editor
type IMETooltip struct {
	Tooltip

	cursorPos       int
	selectionLength int
	cursorVisualPos int
	isShown         bool
}

// func (i *IMETooltip) setQpainterFont(p *gui.QPainter) {
// 	if i.font == nil {
// 		return
// 	}
// 	if i.font.qfont == nil {
// 		return
// 	}
//
// 	p.SetFont(i.getFont())
// }

func (i *IMETooltip) getFont() *Font {
	if i.s.ws.palette != nil && i.s.ws.palette.widget.IsVisible() {
		return editor.font
	} else {
		return i.font
	}
}

func (i *IMETooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(i)

	i.drawContent(p, i.getFont)

	p.DestroyQPainter()
}

func (i *IMETooltip) pos() (int, int, int, int) {
	var x, y, candX, candY int
	ws := i.s.ws
	s := i.s
	if s.lenWindows() == 0 {
		return 0, 0, 0, 0
	}

	win, ok := s.getWindow(s.ws.cursor.gridid)
	if !ok {
		return 0, 0, 0, 0
	}
	font := win.getFont()

	if ws.palette != nil && ws.palette.widget.IsVisible() {
		x = ws.palette.cursorX + ws.palette.patternPadding
		y = ws.palette.patternPadding + ws.palette.padding - (font.lineHeight-ws.cursor.height)/2
		candX = x + ws.palette.widget.Pos().X()
		candY = y + ws.palette.widget.Pos().Y()
	} else {
		i.setFont(font)
		row := s.cursor[0]
		col := s.cursor[1]
		if i.s.ws.isTerminalMode {
			if row < len(win.content) {
				if col < len(win.content[row]) {
					if win.content[row][col].normalWidth {
						col += 1
					} else {
						col += 2
					}
				}
			}
		}
		x = int(float64(col) * font.cellwidth)
		y = row * font.lineHeight

		posx, posy := win.position()
		candX = int(float64(col+posx) * font.cellwidth)
		tablineMarginTop := 0
		if ws.tabline != nil {
			tablineMarginTop = ws.tabline.marginTop
		}
		tablineHeight := 0
		if ws.tabline != nil {
			tablineHeight = ws.tabline.height
		}
		tablineMarginBottom := 0
		if ws.tabline != nil {
			tablineMarginBottom = ws.tabline.marginBottom
		}
		candY = (row+posy)*font.lineHeight + tablineMarginTop + tablineHeight + tablineMarginBottom
	}

	candX = candX + i.cursorVisualPos
	editor.putLog(
		fmt.Sprintf(
			"IME preeditstr:: cursor pos in preeditstr: %d",
			int(float64(i.cursorVisualPos)/font.cellwidth),
		),
	)

	return x, y, candX, candY
}

func (i *IMETooltip) move(x int, y int) {
	padding := 0
	if i.s.ws.palette != nil && i.s.ws.palette.widget.IsVisible() {
		padding = i.s.ws.palette.padding
	}
	i.Move(core.NewQPoint2(x+padding, y))
}

func (i *IMETooltip) hide() {
	i.isShown = false
	i.Hide()
}

func (i *IMETooltip) show() {
	if !(i.s.ws.palette != nil && i.s.ws.palette.widget.IsVisible()) {
		win, ok := i.s.getWindow(i.s.ws.cursor.gridid)
		if ok {
			i.SetParent(win)
		}
		i.setFont(win.getFont())
	} else {
		i.SetParent(i.s.ws.palette.widget)
		i.setFont(i.s.font)
	}

	i.Show()
	i.Raise()
	i.isShown = true
}

func (i *IMETooltip) updateVirtualCursorPos() {
	start := i.s.tooltip.cursorPos

	var x float64
	var k int
	for _, chunk := range i.text {
		for _, _ = range chunk.str {
			if k == start {
				i.cursorVisualPos = int(x)
				return
			}
			x += chunk.width
			k++
		}
	}

	i.cursorVisualPos = int(x)
}

func (i *IMETooltip) parsePreeditString(preeditStr string) {

	i.clearText()

	length := i.s.tooltip.selectionLength
	start := i.s.tooltip.cursorPos
	if runtime.GOOS == "darwin" {
		start = i.s.tooltip.cursorPos - length
	}

	g := &Highlight{}
	h := &Highlight{}

	if i.s.ws.foreground == nil || i.s.ws.background == nil {
		return
	}
	g.foreground = i.s.ws.foreground
	g.background = i.s.ws.background
	if i.s.ws.screenbg == "light" {
		h.foreground = warpColor(i.s.ws.background, -30)
		h.background = warpColor(i.s.ws.foreground, -30)
	} else {
		h.foreground = warpColor(i.s.ws.foreground, 30)
		h.background = warpColor(i.s.ws.background, 30)
	}
	h.underline = true

	if preeditStr != "" {
		r := []rune(preeditStr)

		if length > 0 {
			if start > 0 {
				i.updateText(g, string(r[:start]), i.s.ws.font.letterSpace, i.getFont().qfont)
				if start+length < len(r) {
					i.updateText(h, string(r[start:start+length]), i.s.ws.font.letterSpace, i.getFont().qfont)
					i.updateText(g, string(r[start+length:]), i.s.ws.font.letterSpace, i.getFont().qfont)
				} else {
					i.updateText(h, string(r[start:]), i.s.ws.font.letterSpace, i.getFont().qfont)
				}
			} else if start == 0 && length < len(r) {
				i.updateText(h, string(r[0:length]), i.s.ws.font.letterSpace, i.getFont().qfont)
				i.updateText(g, string(r[length:]), i.s.ws.font.letterSpace, i.getFont().qfont)
			} else {
				i.updateText(g, preeditStr, i.s.ws.font.letterSpace, i.getFont().qfont)
			}
		} else {
			i.updateText(g, preeditStr, i.s.ws.font.letterSpace, i.getFont().qfont)
		}
	}
}

package gonvim

import (
	"fmt"
	"math"
	"runtime/debug"
	"strings"

	"github.com/dzhou121/ui"
)

// AreaHandler is
type AreaHandler struct {
	area         *ui.Area
	dp           *ui.AreaDrawParams
	highlight    Highlight
	cursor       []int
	lastCursor   []int
	content      [][]*Char
	scrollRegion []int
	width        int
	height       int
	span         *ui.Area
}

func initArea() *AreaHandler {
	ah := &AreaHandler{
		cursor:       []int{0, 0},
		lastCursor:   []int{0, 0},
		scrollRegion: []int{0, 0, 0, 0},
	}
	area := ui.NewArea(ah)
	ah.area = area
	return ah
}

func (ah *AreaHandler) cursorGoto(args []interface{}) {
	pos, _ := args[0].([]interface{})
	ah.cursor[0] = reflectToInt(pos[0])
	ah.cursor[1] = reflectToInt(pos[1])
}

func (ah *AreaHandler) put(args []interface{}) {
	numChars := 0
	x := ah.cursor[1]
	y := ah.cursor[0]
	row := ah.cursor[0]
	col := ah.cursor[1]
	if row >= editor.rows {
		return
	}
	line := ah.content[row]
	for _, arg := range args {
		chars := arg.([]interface{})
		for _, c := range chars {
			char := line[col]
			if char == nil {
				char = &Char{}
				line[col] = char
			}
			char.char = c.(string)
			char.highlight = ah.highlight
			col++
			numChars++
		}
	}
	ah.cursor[1] = col
	// we redraw one character more than the chars put for double width characters
	areaQueueRedraw(x, y, numChars+1, 1)
}

func (ah *AreaHandler) resize(args []interface{}) {
	ah.cursor[0] = 0
	ah.cursor[1] = 0
	ah.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		ah.content[i] = make([]*Char, editor.cols)
	}
	ui.QueueMain(func() {
		editor.area.QueueRedrawAll()
	})
}

func (ah *AreaHandler) clear(args []interface{}) {
	ah.cursor[0] = 0
	ah.cursor[1] = 0
	ah.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		ah.content[i] = make([]*Char, editor.cols)
	}
	ui.QueueMain(func() {
		editor.area.QueueRedrawAll()
	})
}

func (ah *AreaHandler) eolClear(args []interface{}) {
	row := ah.cursor[0]
	col := ah.cursor[1]
	line := ah.content[row]
	numChars := 0
	for x := col; x < len(line); x++ {
		line[x] = nil
		numChars++
	}
	areaQueueRedraw(col, row, numChars, 1)
}

func (ah *AreaHandler) highlightSet(args []interface{}) {
	for _, arg := range args {
		hl := arg.([]interface{})[0].(map[string]interface{})
		_, ok := hl["reverse"]
		if ok {
			highlight := Highlight{}
			highlight.foreground = ah.highlight.background
			highlight.background = ah.highlight.foreground
			ah.highlight = highlight
			continue
		}

		highlight := Highlight{}
		fg, ok := hl["foreground"]
		if ok {
			rgba := calcColor(reflectToInt(fg))
			highlight.foreground = &rgba
		} else {
			highlight.foreground = &editor.Foreground
		}

		bg, ok := hl["background"]
		if ok {
			rgba := calcColor(reflectToInt(bg))
			highlight.background = &rgba
		} else {
			highlight.background = &editor.Background
		}
		ah.highlight = highlight
	}
}

func (ah *AreaHandler) setScrollRegion(args []interface{}) {
	arg := args[0].([]interface{})
	top := reflectToInt(arg[0])
	bot := reflectToInt(arg[1])
	left := reflectToInt(arg[2])
	right := reflectToInt(arg[3])
	ah.scrollRegion = []int{int(top), int(bot), int(left), int(right)}
}

func (ah *AreaHandler) modeChange(args []interface{}) {
	arg := args[0].([]interface{})
	editor.mode = arg[0].(string)
}

func (ah *AreaHandler) scroll(args []interface{}) {
	count := int(args[0].([]interface{})[0].(int64))
	top := ah.scrollRegion[0]
	bot := ah.scrollRegion[1]
	left := ah.scrollRegion[2]
	right := ah.scrollRegion[3]

	//areaScrollRect(left, top, (right - left + 1), (bot - top + 1), 0, -count)
	areaQueueRedraw(left, top, (right - left + 1), (bot - top + 1))

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			for col := left; col <= right; col++ {
				ah.content[row][col] = ah.content[row+count][col]
			}
		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				ah.content[row][col] = nil
			}
		}
		areaQueueRedraw(left, (bot - count + 1), (right - left), count)
		if top > 0 {
			areaQueueRedraw(left, (top - count), (right - left), count)
		}
	} else {
		for row := bot; row >= top-count; row-- {
			for col := left; col <= right; col++ {
				ah.content[row][col] = ah.content[row+count][col]
			}
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				ah.content[row][col] = nil
			}
		}
		areaQueueRedraw(left, top, (right - left), -count)
		if bot < editor.rows-1 {
			areaQueueRedraw(left, bot+1, (right - left), -count)
		}
	}
}

// Draw is
func (ah *AreaHandler) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	if ah.area == nil {
		return
	}
	if editor == nil {
		return
	}
	if !editor.nvimAttached {
		return
	}

	font := editor.font
	row := int(math.Ceil(dp.ClipY / float64(font.lineHeight)))
	col := int(math.Ceil(dp.ClipX / float64(font.width)))
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
		ah.fillHightlight(dp, y, col, cols)
		ah.drawText(dp, y, col, cols)
	}
}

func (ah *AreaHandler) drawText(dp *ui.AreaDrawParams, y int, col int, cols int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r, debug.Stack())
		}
	}()
	if y >= len(ah.content) {
		return
	}
	line := ah.content[y]
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
	dp.Context.Text(float64(start*editor.font.width), float64(y*editor.font.lineHeight+shift), textLayout)
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
		dp.Context.Text(float64(x*editor.font.width), float64(y*editor.font.lineHeight+shift), textLayout)
		textLayout.Free()
	}
}

func (ah *AreaHandler) fillHightlight(dp *ui.AreaDrawParams, y int, col int, cols int) {
	if y >= len(ah.content) {
		return
	}
	line := ah.content[y]
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
						float64(start*editor.font.width),
						float64(y*editor.font.lineHeight),
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
					float64(start*editor.font.width),
					float64(y*editor.font.lineHeight),
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
			float64(start*editor.font.width),
			float64(y*editor.font.lineHeight),
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

// MouseEvent is
func (ah *AreaHandler) MouseEvent(a *ui.Area, me *ui.AreaMouseEvent) {
}

// MouseCrossed is
func (ah *AreaHandler) MouseCrossed(a *ui.Area, left bool) {
}

// DragBroken is
func (ah *AreaHandler) DragBroken(a *ui.Area) {
}

// KeyEvent is
func (ah *AreaHandler) KeyEvent(a *ui.Area, key *ui.AreaKeyEvent) (handled bool) {
	if key.Up {
		return false
	}
	if key.Key == 0 && key.ExtKey == 0 {
		return false
	}
	namedKey := ""
	mod := ""

	switch key.Modifiers {
	case ui.Ctrl:
		mod = "C-"
	case ui.Alt:
		mod = "A-"
	case ui.Super:
		mod = "M-"
	}

	switch key.ExtKey {
	case ui.Escape:
		namedKey = "Esc"
	case ui.Insert:
		namedKey = "Insert"
	case ui.Delete:
		namedKey = "Del"
	case ui.Left:
		namedKey = "Left"
	case ui.Right:
		namedKey = "Right"
	case ui.Down:
		namedKey = "Down"
	case ui.Up:
		namedKey = "Up"
	}

	char := ""
	char = string(key.Key)
	if char == "\n" || char == "\r" {
		namedKey = "Enter"
	} else if char == "\t" {
		namedKey = "Tab"
	} else if key.Key == 127 {
		namedKey = "BS"
	} else if char == "<" {
		namedKey = "LT"
	}

	input := ""
	if namedKey != "" {
		input = fmt.Sprintf("<%s>", namedKey)
	} else if mod != "" {
		input = fmt.Sprintf("<%s%s>", mod, char)
	} else {
		input = char
	}
	editor.nvim.Input(input)
	return true
}

func areaQueueRedraw(x, y, width, heigt int) {
	ui.QueueMain(func() {
		editor.area.QueueRedraw(
			float64(x*editor.font.width),
			float64(y*editor.font.lineHeight),
			float64(width*editor.font.width),
			float64(heigt*editor.font.lineHeight),
		)
	})
}

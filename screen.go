package gonvim

import (
	"math"
	"strings"

	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"
)

// Window is
type Window struct {
	win    nvim.Window
	width  int
	height int
	pos    [2]int
	tab    nvim.Tabpage
}

// Screen is the main editor area
type Screen struct {
	AreaHandler
	box             *ui.Box
	wins            map[nvim.Window]*Window
	cursor          [2]int
	lastCursor      [2]int
	content         [][]*Char
	scrollRegion    []int
	curtab          nvim.Tabpage
	highlight       Highlight
	curWins         map[nvim.Window]*Window
	queueRedrawArea [4]int
}

func initScreen(width, height int) *Screen {
	box := ui.NewHorizontalBox()
	screen := &Screen{
		box:          box,
		wins:         map[nvim.Window]*Window{},
		cursor:       [2]int{0, 0},
		lastCursor:   [2]int{0, 0},
		scrollRegion: []int{0, 0, 0, 0},
	}
	area := ui.NewArea(screen)
	screen.area = area
	screen.setSize(width, height)
	box.Append(area, false)
	box.SetSize(width, height)
	return screen
}

func (s *Screen) getWindows() map[nvim.Window]*Window {
	wins := map[nvim.Window]*Window{}
	neovim := editor.nvim
	curtab, _ := neovim.CurrentTabpage()
	s.curtab = curtab
	nwins, _ := neovim.Windows()
	b := neovim.NewBatch()
	for _, nwin := range nwins {
		win := &Window{
			win: nwin,
		}
		b.WindowWidth(nwin, &win.width)
		b.WindowHeight(nwin, &win.height)
		b.WindowPosition(nwin, &win.pos)
		b.WindowTabpage(nwin, &win.tab)
		wins[nwin] = win
	}
	err := b.Execute()
	if err != nil {
		return nil
	}
	return wins
}

// Draw the screen
func (s *Screen) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
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
		fillHightlight(dp, y, col, cols, [2]int{0, 0})
		drawText(dp, y, col, cols, [2]int{0, 0})
	}

	s.drawBorder(dp, row, col, rows, cols)
}

func (s *Screen) drawBorder(dp *ui.AreaDrawParams, row, col, rows, cols int) {
	for _, win := range s.curWins {
		if win.pos[0]+win.height < row && (win.pos[1]+win.width+1) < col {
			continue
		}
		if win.pos[0] > (row+rows) && (win.pos[1]+win.width) > (col+cols) {
			continue
		}

		win.drawBorder(dp)
	}
}

func (s *Screen) redrawWindows() {
	wins := map[nvim.Window]*Window{}
	neovim := editor.nvim
	curtab, _ := neovim.CurrentTabpage()
	s.curtab = curtab
	nwins, _ := neovim.TabpageWindows(curtab)
	b := neovim.NewBatch()
	for _, nwin := range nwins {
		win := &Window{
			win: nwin,
		}
		b.WindowWidth(nwin, &win.width)
		b.WindowHeight(nwin, &win.height)
		b.WindowPosition(nwin, &win.pos)
		b.WindowTabpage(nwin, &win.tab)
		wins[nwin] = win
	}
	err := b.Execute()
	if err != nil {
		return
	}
	s.curWins = wins
}

func (s *Screen) resize(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		s.content[i] = make([]*Char, editor.cols)
	}
	s.queueRedrawAll()
}

func (s *Screen) clear(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		s.content[i] = make([]*Char, editor.cols)
	}
	s.queueRedrawAll()
}

func (s *Screen) eolClear(args []interface{}) {
	row := s.cursor[0]
	col := s.cursor[1]
	line := s.content[row]
	numChars := 0
	for x := col; x < len(line); x++ {
		line[x] = nil
		numChars++
	}
	s.queueRedraw(col, row, numChars+1, 1)
}

func (s *Screen) cursorGoto(args []interface{}) {
	pos, _ := args[0].([]interface{})
	s.cursor[0] = reflectToInt(pos[0])
	s.cursor[1] = reflectToInt(pos[1])
}

func (s *Screen) put(args []interface{}) {
	numChars := 0
	x := s.cursor[1]
	y := s.cursor[0]
	row := s.cursor[0]
	col := s.cursor[1]
	if row >= editor.rows {
		return
	}
	line := s.content[row]
	for _, arg := range args {
		chars := arg.([]interface{})
		for _, c := range chars {
			char := line[col]
			if char == nil {
				char = &Char{}
				line[col] = char
			}
			char.char = c.(string)
			char.highlight = s.highlight
			col++
			numChars++
		}
	}
	s.cursor[1] = col
	// we redraw one character more than the chars put for double width characters
	s.queueRedraw(x, y, numChars+1, 1)
}

func (s *Screen) highlightSet(args []interface{}) {
	for _, arg := range args {
		hl := arg.([]interface{})[0].(map[string]interface{})
		_, ok := hl["reverse"]
		if ok {
			highlight := Highlight{}
			highlight.foreground = s.highlight.background
			highlight.background = s.highlight.foreground
			s.highlight = highlight
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
		s.highlight = highlight
	}
}

func (s *Screen) setScrollRegion(args []interface{}) {
	arg := args[0].([]interface{})
	top := reflectToInt(arg[0])
	bot := reflectToInt(arg[1])
	left := reflectToInt(arg[2])
	right := reflectToInt(arg[3])
	s.scrollRegion = []int{int(top), int(bot), int(left), int(right)}
}

func (s *Screen) scroll(args []interface{}) {
	count := int(args[0].([]interface{})[0].(int64))
	top := s.scrollRegion[0]
	bot := s.scrollRegion[1]
	left := s.scrollRegion[2]
	right := s.scrollRegion[3]

	//areaScrollRect(left, top, (right - left + 1), (bot - top + 1), 0, -count)
	s.queueRedraw(left, top, (right - left + 1), (bot - top + 1))

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			for col := left; col <= right; col++ {
				s.content[row][col] = s.content[row+count][col]
			}
		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				s.content[row][col] = nil
			}
		}
		s.queueRedraw(left, (bot - count + 1), (right - left), count)
		if top > 0 {
			s.queueRedraw(left, (top - count), (right - left), count)
		}
	} else {
		for row := bot; row >= top-count; row-- {
			for col := left; col <= right; col++ {
				s.content[row][col] = s.content[row+count][col]
			}
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				s.content[row][col] = nil
			}
		}
		s.queueRedraw(left, top, (right - left), -count)
		if bot < editor.rows-1 {
			s.queueRedraw(left, bot+1, (right - left), -count)
		}
	}
}

func (s *Screen) redraw() {
	x := s.queueRedrawArea[0]
	y := s.queueRedrawArea[1]
	width := s.queueRedrawArea[2] - x
	height := s.queueRedrawArea[3] - y
	ui.QueueMain(func() {
		s.area.QueueRedraw(
			float64(x*editor.font.width),
			float64(y*editor.font.lineHeight),
			float64(width*editor.font.width),
			float64(height*editor.font.lineHeight),
		)
	})
	s.queueRedrawArea[0] = 0
	s.queueRedrawArea[1] = 0
	s.queueRedrawArea[2] = 0
	s.queueRedrawArea[3] = 0
}

func (s *Screen) queueRedrawAll() {
	s.queueRedrawArea = [4]int{0, 0, editor.cols, editor.rows}
}

func (s *Screen) queueRedraw(x, y, width, height int) {
	if x < s.queueRedrawArea[0] {
		s.queueRedrawArea[0] = x
	}
	if y < s.queueRedrawArea[1] {
		s.queueRedrawArea[1] = y
	}
	if (x + width) > s.queueRedrawArea[2] {
		s.queueRedrawArea[2] = x + width
	}
	if (y + height) > s.queueRedrawArea[3] {
		s.queueRedrawArea[3] = y + height
	}
}

func (s *Screen) posWin(x, y int) *Window {
	for _, win := range s.curWins {
		if win.pos[0] <= y && win.pos[1] <= x && (win.pos[0]+win.height+1) >= y && (win.pos[1]+win.width >= x) {
			return win
		}
	}
	return nil
}

func (s *Screen) cursorWin() *Window {
	return s.posWin(s.cursor[1], s.cursor[0])
}

func fillHightlight(dp *ui.AreaDrawParams, y int, col int, cols int, pos [2]int) {
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
						float64((start-pos[1])*editor.font.width),
						float64((y-pos[0])*editor.font.lineHeight),
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
					float64((start-pos[1])*editor.font.width),
					float64((y-pos[0])*editor.font.lineHeight),
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
			float64((start-pos[1])*editor.font.width),
			float64((y-pos[0])*editor.font.lineHeight),
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

func drawText2(dp *ui.AreaDrawParams, y int, col int, cols int, pos [2]int) {
	screen := editor.screen
	if y >= len(screen.content) {
		return
	}
	line := screen.content[y]
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char == nil {
			continue
		}
		// if char.highlight.background != nil {
		// 	drawRect2(dp, x, y, 1, 1, char.highlight.background)
		// }
		if char.char == " " || char.char == "" {
			continue
		}
		fg := editor.Foreground
		if char.highlight.foreground != nil {
			fg = *(char.highlight.foreground)
		}
		textLayout := ui.NewTextLayout(char.char, editor.font.font, -1)
		textLayout.SetColor(0, 1, fg.R, fg.G, fg.B, fg.A)
		dp.Context.Text(float64(x*editor.font.width), float64(y*editor.font.lineHeight+editor.font.shift), textLayout)
		textLayout.Free()
	}
}

func drawRect2(dp *ui.AreaDrawParams, col, row, cols, rows int, color *RGBA) {
	p := ui.NewPath(ui.Winding)
	p.AddRectangle(
		float64(col*editor.font.width),
		float64(row*editor.font.lineHeight),
		float64(cols*editor.font.width),
		float64(rows*editor.font.lineHeight),
	)
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

func drawText(dp *ui.AreaDrawParams, y int, col int, cols int, pos [2]int) {
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
	shift := editor.font.shift

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
	dp.Context.Text(float64((start-pos[1])*editor.font.width), float64((y-pos[0])*editor.font.lineHeight+shift), textLayout)
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
		dp.Context.Text(float64((x-pos[1])*editor.font.width), float64((y-pos[0])*editor.font.lineHeight+shift), textLayout)
		textLayout.Free()
	}
}

func (w *Window) drawBorder(dp *ui.AreaDrawParams) {
	drawRect(dp, (w.pos[1]+w.width)*editor.font.width, w.pos[0]*editor.font.lineHeight, editor.font.width, w.height*editor.font.lineHeight, &editor.Background)

	color := newRGBA(0, 0, 0, 1)

	p := ui.NewPath(ui.Winding)
	p.AddRectangle(
		(float64(w.width+w.pos[1]))*float64(editor.font.width),
		float64(w.pos[0]*editor.font.lineHeight),
		float64(editor.font.width),
		float64(w.height*editor.font.lineHeight),
	)
	p.End()
	stops := []ui.GradientStop{}
	n := 10
	for i := 0; i <= n; i++ {
		s := ui.GradientStop{
			Pos: float64(i) / float64(n),
			R:   float64(10) / 255,
			G:   float64(10) / 255,
			B:   float64(10) / 255,
			A:   (1 - (float64(i) / float64(n))) / 2,
		}
		stops = append(stops, s)
	}
	dp.Context.Fill(p, &ui.Brush{
		Type:  ui.LinearGradient,
		X0:    (float64(w.width+w.pos[1]) + 1) * float64(editor.font.width),
		Y0:    0,
		X1:    (float64(w.width + w.pos[1])) * float64(editor.font.width),
		Y1:    0,
		Stops: stops,
	})
	p.Free()

	p = ui.NewPath(ui.Winding)
	p.AddRectangle((float64(w.width+w.pos[1])+1)*float64(editor.font.width)-1,
		float64(w.pos[0]*editor.font.lineHeight),
		1,
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

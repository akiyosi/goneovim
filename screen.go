package gonvim

import (
	"fmt"

	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"
)

// Screen is the main editor area
type Screen struct {
	AreaHandler
	box          *ui.Box
	wins         map[nvim.Window]*Window
	cursor       [2]int
	lastCursor   [2]int
	content      [][]*Char
	scrollRegion []int
	curtab       nvim.Tabpage
	highlight    Highlight
	curWins      []*Window
	cmdheight    int
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
		// b.WindowOption(nwin, "statusline", &win.statusline)
		wins[nwin] = win
	}
	err := b.Execute()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return wins
}

// Draw the screen
func (s *Screen) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
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
}

func (s *Screen) redrawWindows() {
	wins := s.getWindows()
	if len(wins) == 0 {
		return
	}
	for _, w := range wins {
		win, ok := s.wins[w.win]
		if !ok {
			fmt.Println("new win")
			area := ui.NewArea(w)
			w.area = area
			ui.QueueMain(func() {
				s.box.Append(area, false)
			})
			win = w
			s.wins[w.win] = w
		} else {
			win.width = w.width
			win.height = w.height
			win.tab = w.tab
			win.pos = w.pos
		}
	}
	s.curWins = []*Window{}
	s.cmdheight = editor.rows
	for _, win := range s.wins {
		_, ok := wins[win.win]
		if !ok {
			fmt.Println("rem win")
			delete(s.wins, win.win)
			area := win.area
			ui.QueueMain(func() {
				s.box.DeleteChild(area)
				area.Destroy()
			})
			continue
		}
		if win.tab == s.curtab {
			cmdheight := editor.rows - (win.height + win.pos[0])
			if cmdheight < s.cmdheight {
				s.cmdheight = cmdheight
			}
			s.curWins = append(s.curWins, win)
			win.setSize((win.width+1)*editor.font.width, (win.height+1)*editor.font.lineHeight)
			win.setPosition(win.pos[1]*editor.font.width, win.pos[0]*editor.font.lineHeight)
			win.show()
		} else {
			win.hide()
		}
	}
	fmt.Println("cmdheight is", s.cmdheight)
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
func (s *Screen) queueRedrawAll() {
	for _, win := range s.curWins {
		area := win.area
		ui.QueueMain(func() {
			area.QueueRedrawAll()
		})
	}
}

func (s *Screen) queueRedraw(x, y, width, height int) {
	win := s.posWin(x, y)
	if win != nil && width <= win.width && height <= win.height {
		win.queueRedraw(x-win.pos[1], y-win.pos[0], width, height)
	} else {
		s.queueRedrawAll()
	}
}

func (s *Screen) posWin(x, y int) *Window {
	for _, win := range s.curWins {
		if win.pos[0] <= y && win.pos[1] <= x && (win.pos[0]+win.height) >= y && (win.pos[1]+win.width >= x) {
			return win
		}
	}
	return nil
}

func (s *Screen) cursorWin() *Window {
	return s.posWin(s.cursor[1], s.cursor[0])
}

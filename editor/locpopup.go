package editor

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/akiyosi/goneovim/util"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Locpopup is the location popup
type Locpopup struct {
	ws     *Workspace
	mutex  sync.Mutex
	widget *widgets.QWidget
	//typeLabel    *widgets.QLabel
	typeLabel    *svg.QSvgWidget
	typeText     string
	contentLabel *widgets.QLabel
	contentText  string
	shown        bool
	updates      chan []interface{}
}

func initLocpopup() *Locpopup {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(8, 8, 8, 8)
	layout := widgets.NewQHBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(4)
	widget.SetLayout(layout)
	typeLabel := svg.NewQSvgWidget(nil)
	typeLabel.SetFixedSize2(editor.iconSize-1, editor.iconSize-1)
	// typeLabel.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0); }")

	contentLabel := widgets.NewQLabel(nil, 0)
	contentLabel.SetContentsMargins(0, 0, 0, 0)
	// contentLabel.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0); }")

	loc := &Locpopup{
		widget:       widget,
		typeLabel:    typeLabel,
		contentLabel: contentLabel,
		updates:      make(chan []interface{}, 1000),
	}

	layout.AddWidget(loc.typeLabel, 0, 0)
	layout.AddWidget(loc.contentLabel, 0, 0)
	loc.widget.SetGraphicsEffect(util.DropShadow(0, 6, 30, 80))

	return loc
}

func (l *Locpopup) setColor() {
	fg := editor.colors.widgetFg.String()
	bg := editor.colors.widgetBg
	// transparent := editor.config.Editor.Transparent / 2.0
	transparent := transparent()
	l.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d, %f);  color: %s; }", bg.R, bg.G, bg.B, transparent, fg))
}

func (l *Locpopup) subscribe() {
	if !l.ws.drawLint {
		return
	}
	l.ws.signal.ConnectLocpopupSignal(func() {
		l.updateLocpopup()
	})
	l.ws.nvim.RegisterHandler("LocPopup", func(args ...interface{}) {
		l.handle(args)
	})
	l.ws.nvim.Subscribe("LocPopup")
}

func (l *Locpopup) updateLocpopup() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if !l.shown {
		l.widget.Hide()
		return
	}
	l.contentLabel.SetText(l.contentText)
	if l.typeText == "E" {
		//l.typeLabel.SetText("Error")
		//l.typeLabel.SetStyleSheet("background-color: rgba(204, 62, 68, 1);")
		svgContent := editor.getSvg("linterr", newRGBA(204, 62, 68, 1))
		l.typeLabel.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	} else if l.typeText == "W" {
		//l.typeLabel.SetText("Warning")
		//l.typeLabel.SetStyleSheet("background-color: rgba(203, 203, 65, 1);")
		svgContent := editor.getSvg("lintwrn", newRGBA(253, 190, 65, 1))
		l.typeLabel.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
	l.widget.Hide()
	l.widget.Show()
	l.widget.Raise()
}

func (l *Locpopup) handle(args []interface{}) {
	if len(args) < 1 {
		return
	}
	event, ok := args[0].(string)
	if !ok {
		return
	}
	switch event {
	case "update":
		l.update(args[1:])
	}
}

func (l *Locpopup) updatePos() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if !l.shown {
		return
	}

	col := l.ws.screen.cursor[1]
	row := l.ws.screen.cursor[0]
	x, y, _, _ := l.ws.getPointInWidget(col, row, l.ws.cursor.gridid)

	if row < 3 {
		y += l.widget.Height()
	} else {
		y -= l.widget.Height()
	}

	l.widget.Move2(x, y)
}

func (l *Locpopup) update(args []interface{}) {
	l.mutex.Lock()
	shown := false
	defer func() {
		if !shown {
			l.shown = false
			l.ws.signal.LocpopupSignal()
		}
		l.mutex.Unlock()
	}()

	doneChannel := make(chan nvim.Buffer, 5)
	var buf, buftmp nvim.Buffer
	go func() {
		buftmp, _ = l.ws.nvim.CurrentBuffer()
		doneChannel <- buftmp
	}()
	select {
	case buf = <-doneChannel:
	case <-time.After(20 * time.Millisecond):
		return
	}

	buftype := new(string)
	err := l.ws.nvim.BufferOption(buf, "buftype", buftype)
	if err != nil {
		return
	}
	if *buftype == "terminal" {
		return
	}

	mode := new(string)
	err = l.ws.nvim.Call("mode", mode, "")
	if err != nil {
		return
	}
	if *mode != "n" {
		return
	}

	curWin, err := l.ws.nvim.CurrentWindow()
	if err != nil {
		return
	}
	pos, err := l.ws.nvim.WindowCursor(curWin)
	if err != nil {
		return
	}
	result := new([]map[string]interface{})
	err = l.ws.nvim.Call("getloclist", result, "winnr(\"$\")")
	if err != nil {
		return
	}

	errors := 0
	warnings := 0
	locs := []map[string]interface{}{}
	for _, loc := range *result {
		lnumInterface := loc["lnum"]
		if lnumInterface == nil {
			continue
		}
		lnum := util.ReflectToInt(lnumInterface)
		if lnum == pos[0] {
			locs = append(locs, loc)
		}
		locType := loc["type"].(string)
		switch locType {
		case "E":
			errors++
		case "W":
			warnings++
		}
	}
	l.ws.statusline.lint.redraw(errors, warnings)
	if len(locs) == 0 {
		return
	}
	if len(locs) > 1 {
		sort.Sort(ByCol(locs))
	}
	var loc map[string]interface{}
	for _, loc = range locs {
		if pos[1] >= util.ReflectToInt(loc["col"])-1 {
			break
		}
	}

	locType := loc["type"].(string)
	text := loc["text"].(string)
	shown = true
	if locType != l.typeText || text != l.contentText || shown != l.shown {
		l.typeText = locType
		l.contentText = text
		l.shown = shown
		l.ws.signal.LocpopupSignal()
	}
}

// ByCol sorts locations by column
type ByCol []map[string]interface{}

// Len of locations
func (s ByCol) Len() int {
	return len(s)
}

// Swap locations
func (s ByCol) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less than
func (s ByCol) Less(i, j int) bool {
	return util.ReflectToInt(s[i]["col"]) > util.ReflectToInt(s[j]["col"])
}

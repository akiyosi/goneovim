package gonvim

import (
	"sort"
	"sync"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

// Locpopup is the location popup
type Locpopup struct {
	mutex        sync.Mutex
	widget       *widgets.QWidget
	typeLabel    *widgets.QLabel
	typeText     string
	contentLabel *widgets.QLabel
	contentText  string
	lastType     string
	lastText     string
	lastShown    bool
}

func initLocpopup() *Locpopup {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(4, 4, 4, 4)
	layout := widgets.NewQHBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(4)
	widget.SetLayout(layout)
	widget.SetStyleSheet("color: rgba(14, 17, 18, 1); background-color: rgba(212, 215, 214, 1);")

	typeLabel := widgets.NewQLabel(nil, 0)
	typeLabel.SetContentsMargins(4, 1, 4, 1)

	contentLabel := widgets.NewQLabel(nil, 0)
	contentLabel.SetContentsMargins(0, 0, 0, 0)

	layout.AddWidget(typeLabel, 0, 0)
	layout.AddWidget(contentLabel, 0, 0)
	widget.Hide()

	loc := &Locpopup{
		widget:       widget,
		typeLabel:    typeLabel,
		contentLabel: contentLabel,
	}
	widget.ConnectCustomEvent(func(event *core.QEvent) {
		switch event.Type() {
		case core.QEvent__Show:
			widget.Show()
		case core.QEvent__Hide:
			widget.Hide()
		}
	})
	contentLabel.ConnectCustomEvent(func(event *core.QEvent) {
		switch event.Type() {
		case core.QEvent__UpdateRequest:
			contentLabel.SetText(loc.contentText)
		}
	})
	typeLabel.ConnectCustomEvent(func(event *core.QEvent) {
		switch event.Type() {
		case core.QEvent__UpdateRequest:
			if loc.typeText == "Error" {
				typeLabel.SetText("Error")
				typeLabel.SetStyleSheet("background-color: rgba(204, 62, 68, 1); color: rgba(212, 215, 214, 1);")
			} else if loc.typeText == "Warning" {
				typeLabel.SetText("Warning")
				typeLabel.SetStyleSheet("background-color: rgba(203, 203, 65, 1); color: rgba(212, 215, 214, 1);")
			}
		}
	})
	return loc
}

func (l *Locpopup) subscribe() {
	editor.nvim.Subscribe("LocPopup")
	editor.nvim.RegisterHandler("LocPopup", func(args ...interface{}) {
		go l.handle(args...)
	})
	editor.nvim.Command(`autocmd CursorMoved,CursorHold,InsertEnter,InsertLeave,BufEnter,BufLeave * call rpcnotify(0, "LocPopup", "update")`)
}

func (l *Locpopup) handle(args ...interface{}) {
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

func (l *Locpopup) update(args []interface{}) {
	l.mutex.Lock()
	shown := false
	defer func() {
		if shown {
			l.lastShown = true
		} else {
			if l.lastShown {
				l.lastShown = false
				l.lastText = ""
				l.widget.Hide()
			}
		}
		l.mutex.Unlock()
	}()
	buf, err := editor.nvim.CurrentBuffer()
	if err != nil {
		return
	}
	buftype := new(string)
	err = editor.nvim.BufferOption(buf, "buftype", buftype)
	if err != nil {
		return
	}
	if *buftype == "terminal" {
		return
	}

	mode := new(string)
	err = editor.nvim.Call("mode", mode, "")
	if err != nil {
		return
	}
	if *mode != "n" {
		return
	}

	curWin, err := editor.nvim.CurrentWindow()
	if err != nil {
		return
	}
	pos, err := editor.nvim.WindowCursor(curWin)
	if err != nil {
		return
	}
	result := new([]map[string]interface{})
	err = editor.nvim.Call("getloclist", result, "winnr(\"$\")")
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
		lnum := reflectToInt(lnumInterface)
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
	editor.statusline.lint.redraw(errors, warnings)
	if len(locs) == 0 {
		return
	}
	if len(locs) > 1 {
		sort.Sort(ByCol(locs))
	}
	var loc map[string]interface{}
	for _, loc = range locs {
		if pos[1] >= reflectToInt(loc["col"])-1 {
			break
		}
	}

	locType := loc["type"].(string)
	text := loc["text"].(string)
	if locType != l.lastType || text != l.lastText {
		if locType != l.lastType {
			switch locType {
			case "E":
				l.typeText = "Error"
			case "W":
				l.typeText = "Warning"
			}
			l.typeLabel.CustomEvent(core.NewQEvent(core.QEvent__UpdateRequest))
		}
		if text != l.lastText {
			l.contentText = text
			l.contentLabel.CustomEvent(core.NewQEvent(core.QEvent__UpdateRequest))
		}
		l.lastText = text
		l.lastType = locType
		l.widget.CustomEvent(core.NewQEvent(core.QEvent__Hide))
		l.widget.CustomEvent(core.NewQEvent(core.QEvent__Show))
	}
	shown = true
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
	return reflectToInt(s[i]["col"]) > reflectToInt(s[j]["col"])
}

package gonvim

import (
	"fmt"
	"sync"

	"github.com/dzhou121/gonvim-fuzzy/rplugin/go/fzf"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Palette is the popup for fuzzy finder, cmdline etc
type Palette struct {
	widget           *widgets.QWidget
	patternText      string
	resultItems      []*PaletteResultItem
	resultWidget     *widgets.QWidget
	resultMainWidget *widgets.QWidget
	itemHeight       int
	refreshMutex     sync.Mutex
	mutex            sync.Mutex
	width            int
	cursor           *widgets.QWidget
	cursorX          int
	resultType       string
	itemTypes        []string
	max              int
	showTotal        int
	pattern          *widgets.QLabel
	patternPadding   int
	patternWidget    *widgets.QWidget
	scrollBar        *widgets.QWidget
	scrollBarPos     int
	scrollCol        *widgets.QWidget
}

// PaletteResultItem is the result item
type PaletteResultItem struct {
	hidden     bool
	icon       *svg.QSvgWidget
	iconType   string
	iconHidden bool
	base       *widgets.QLabel
	baseText   string
	widget     *widgets.QWidget
	selected   bool
}

func initPalette() *Palette {
	width := 600
	mainLayout := widgets.NewQVBoxLayout()
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)
	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(1, 1, 1, 1)
	widget.SetFixedWidth(width)
	widget.SetObjectName("palette")
	widget.SetStyleSheet("QWidget#palette {border: 1px solid #000;} .QWidget {background-color: rgba(21, 25, 27, 1); } * { color: rgba(205, 211, 222, 1); }")
	shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	shadow.SetBlurRadius(20)
	shadow.SetColor(gui.NewQColor3(0, 0, 0, 255))
	shadow.SetOffset3(0, 2)
	widget.SetGraphicsEffect(shadow)

	resultMainLayout := widgets.NewQHBoxLayout()
	resultMainLayout.SetContentsMargins(0, 0, 0, 0)
	resultMainLayout.SetSpacing(0)

	padding := 8
	resultLayout := widgets.NewQVBoxLayout()
	resultLayout.SetContentsMargins(0, 0, 0, 0)
	resultLayout.SetSpacing(0)
	resultWidget := widgets.NewQWidget(nil, 0)
	resultWidget.SetLayout(resultLayout)
	resultWidget.SetContentsMargins(0, 0, 0, 0)

	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)
	scrollBar.SetStyleSheet("background-color: #3c3c3c;")

	resultMainWidget := widgets.NewQWidget(nil, 0)
	resultMainWidget.SetContentsMargins(0, 0, 0, 0)
	resultMainLayout.AddWidget(resultWidget, 0, 0)
	resultMainLayout.AddWidget(scrollCol, 0, 0)
	resultMainWidget.SetLayout(resultMainLayout)

	pattern := widgets.NewQLabel(nil, 0)
	pattern.SetContentsMargins(padding, padding, padding, padding)
	pattern.SetStyleSheet("background-color: #3c3c3c;")
	patternLayout := widgets.NewQVBoxLayout()
	patternLayout.AddWidget(pattern, 0, 0)
	patternLayout.SetContentsMargins(0, 0, 0, 0)
	patternLayout.SetSpacing(0)
	patternWidget := widgets.NewQWidget(nil, 0)
	patternWidget.SetLayout(patternLayout)
	patternWidget.SetContentsMargins(padding, padding, padding, padding)

	cursor := widgets.NewQWidget(nil, 0)
	cursor.SetParent(pattern)
	cursor.SetFixedSize2(1, pattern.SizeHint().Height()-padding*2)
	cursor.Move2(padding, padding)
	cursor.SetStyleSheet("background-color: rgba(205, 211, 222, 1);")

	mainLayout.AddWidget(patternWidget, 0, 0)
	mainLayout.AddWidget(resultMainWidget, 0, 0)

	resultItems := []*PaletteResultItem{}
	max := 30
	for i := 0; i < max; i++ {
		itemWidget := widgets.NewQWidget(nil, 0)
		itemWidget.SetContentsMargins(0, 0, 0, 0)
		itemLayout := newVFlowLayout(padding, padding*2, 0, 0)
		itemWidget.SetLayout(itemLayout)
		resultLayout.AddWidget(itemWidget, 0, 0)
		icon := svg.NewQSvgWidget(nil)
		icon.SetFixedWidth(14)
		icon.SetFixedHeight(14)
		icon.SetContentsMargins(0, 0, 0, 0)
		base := widgets.NewQLabel(nil, 0)
		base.SetText("base")
		base.SetContentsMargins(0, padding, 0, padding)
		base.SetStyleSheet("background-color: none; white-space: pre-wrap;")
		itemLayout.AddWidget(icon)
		itemLayout.AddWidget(base)
		resultItem := &PaletteResultItem{
			widget: itemWidget,
			icon:   icon,
			base:   base,
		}
		resultItems = append(resultItems, resultItem)
	}
	palette := &Palette{
		width:            width,
		widget:           widget,
		resultItems:      resultItems,
		resultWidget:     resultWidget,
		resultMainWidget: resultMainWidget,
		max:              max,
		pattern:          pattern,
		patternPadding:   padding,
		patternWidget:    patternWidget,
		scrollCol:        scrollCol,
		scrollBar:        scrollBar,
		cursor:           cursor,
	}
	return palette
}

func (p *Palette) resize() {
	x := (editor.screen.width - p.width) / 2
	p.widget.Move2(x, 0)
	itemHeight := p.resultItems[0].widget.SizeHint().Height()
	p.itemHeight = itemHeight
	p.showTotal = int(float64(editor.screen.height)/float64(itemHeight)*0.5) - 1
	fzf.UpdateMax(editor.nvim, p.showTotal)

	for i := p.showTotal; i < len(p.resultItems); i++ {
		p.resultItems[i].hide()
	}
}

func (p *Palette) showResult() {
	p.resultMainWidget.Show()
}

func (p *Palette) hideResult() {
	p.resultMainWidget.Hide()
}

func (p *Palette) refresh() {
	p.refreshMutex.Lock()
	p.resultWidget.Hide()
	p.resultWidget.Show()
	p.hide()
	p.show()
	p.hide()
	p.show()
	p.refreshMutex.Unlock()
}

func (p *Palette) show() {
	p.widget.Show()
}

func (p *Palette) hide() {
	p.widget.Hide()
}

func (p *Palette) setPattern(text string) {
	p.patternText = text
	p.pattern.SetText(text)
}

func (p *Palette) cursorMove(x int) {
	p.cursorX = int(editor.font.defaultFontMetrics.Width(string(p.patternText[:x])))
	p.cursor.Move2(p.cursorX+p.patternPadding, p.patternPadding)
}

func (p *Palette) showSelected(selected int) {
	if p.resultType == "file_line" {
		n := 0
		for i := 0; i <= selected; i++ {
			for n++; n < len(p.itemTypes) && p.itemTypes[n] == "file"; n++ {
			}
		}
		selected = n
	}
	for i, resultItem := range p.resultItems {
		resultItem.setSelected(selected == i)
	}
}

func (f *PaletteResultItem) update() {
	if f.selected {
		f.widget.SetStyleSheet(fmt.Sprintf(".QWidget {background-color: %s;}", editor.selectedBg))
	} else {
		f.widget.SetStyleSheet("")
	}
}

func (f *PaletteResultItem) setSelected(selected bool) {
	if f.selected == selected {
		return
	}
	f.selected = selected
	f.update()
}

func (f *PaletteResultItem) show() {
	if f.hidden {
		f.hidden = false
		f.widget.Show()
	}
}

func (f *PaletteResultItem) hide() {
	if !f.hidden {
		f.hidden = true
		f.widget.Hide()
	}
}

func (f *PaletteResultItem) setItem(text string, itemType string, match []int) {
	iconType := ""
	path := false
	if itemType == "dir" {
		iconType = "folder"
		path = true
	} else if itemType == "file" {
		iconType = getFileType(text)
		path = true
	} else if itemType == "file_line" {
		iconType = "empty"
	}
	if iconType != "" {
		if iconType != f.iconType {
			f.iconType = iconType
			f.updateIcon()
		}
		f.showIcon()
	} else {
		f.hideIcon()
	}

	formattedText := formatText(text, match, path)
	if formattedText != f.baseText {
		f.baseText = formattedText
		f.base.SetText(f.baseText)
	}
}

func (f *PaletteResultItem) updateIcon() {
	svgContent := getSvg(f.iconType, nil)
	f.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (f *PaletteResultItem) showIcon() {
	if f.iconHidden {
		f.iconHidden = false
		f.icon.Show()
	}
}

func (f *PaletteResultItem) hideIcon() {
	if !f.iconHidden {
		f.iconHidden = true
		f.icon.Hide()
	}
}

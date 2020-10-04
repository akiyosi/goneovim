package editor

import (
	"fmt"
	"math"
	"runtime"

	"github.com/akiyosi/goneovim/fuzzy"
	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Palette is the popup for fuzzy finder, cmdline etc
type Palette struct {
	// mu               sync.Mutex
	// procCount        int
	ws               *Workspace
	hidden           bool
	widget           *widgets.QWidget
	padding          int
	patternText      string
	isHTMLText       bool
	resultItems      []*PaletteResultItem
	resultWidget     *widgets.QWidget
	resultMainWidget *widgets.QWidget
	itemHeight       int
	width            int
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
	p          *Palette
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
	padding := 8

	mainLayout := widgets.NewQVBoxLayout()
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)
	mainLayout.SetSizeConstraint(widgets.QLayout__SetMinAndMaxSize)
	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(1, 1, 1, 1)
	// widget.SetFixedWidth(width)
	widget.SetObjectName("palette")

	widget.SetGraphicsEffect(util.DropShadow(0, 15, 130, 120))

	resultMainLayout := widgets.NewQHBoxLayout()
	resultMainLayout.SetContentsMargins(0, 0, 0, 0)
	resultMainLayout.SetSpacing(0)
	resultMainLayout.SetSizeConstraint(widgets.QLayout__SetMinAndMaxSize)

	resultLayout := widgets.NewQVBoxLayout()
	resultLayout.SetContentsMargins(0, 0, 0, 0)
	resultLayout.SetSpacing(0)
	resultLayout.SetSizeConstraint(widgets.QLayout__SetMinAndMaxSize)
	resultWidget := widgets.NewQWidget(nil, 0)
	resultWidget.SetLayout(resultLayout)
	resultWidget.SetStyleSheet("background-color: rgba(0, 0, 0, 0); white-space: pre-wrap;")
	resultWidget.SetContentsMargins(0, 0, 0, 0)

	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)

	resultMainWidget := widgets.NewQWidget(nil, 0)
	resultMainWidget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0); }")
	resultMainWidget.SetContentsMargins(0, 0, 0, 0)
	resultMainLayout.AddWidget(resultWidget, 0, 0)
	resultMainLayout.AddWidget(scrollCol, 0, 0)
	resultMainWidget.SetLayout(resultMainLayout)

	pattern := widgets.NewQLabel(nil, 0)
	pattern.SetContentsMargins(padding, padding, padding, padding)
	pattern.SetFixedWidth(width - padding*2)
	pattern.SetSizePolicy2(widgets.QSizePolicy__Preferred, widgets.QSizePolicy__Maximum)
	patternLayout := widgets.NewQVBoxLayout()
	patternLayout.AddWidget(pattern, 0, 0)
	patternLayout.SetContentsMargins(0, 0, 0, 0)
	patternLayout.SetSpacing(0)
	patternLayout.SetSizeConstraint(widgets.QLayout__SetMinAndMaxSize)
	patternWidget := widgets.NewQWidget(nil, 0)
	patternWidget.SetLayout(patternLayout)
	patternWidget.SetContentsMargins(padding, padding, padding, padding)

	mainLayout.AddWidget(patternWidget, 0, 0)
	mainLayout.AddWidget(resultMainWidget, 0, 0)

	palette := &Palette{
		width:            width,
		widget:           widget,
		padding:          padding,
		resultWidget:     resultWidget,
		resultMainWidget: resultMainWidget,
		pattern:          pattern,
		patternPadding:   padding,
		patternWidget:    patternWidget,
		scrollCol:        scrollCol,
		scrollBar:        scrollBar,
	}

	resultItems := []*PaletteResultItem{}
	max := editor.config.Palette.MaxNumberOfResultItems
	for i := 0; i < max; i++ {
		itemWidget := widgets.NewQWidget(nil, 0)
		itemWidget.SetContentsMargins(0, 0, 0, 0)
		itemLayout := util.NewVFlowLayout(padding, padding*2, 0, 0, 9999)
		itemLayout.SetSizeConstraint(widgets.QLayout__SetMinAndMaxSize)
		itemWidget.SetLayout(itemLayout)
		// itemWidget.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
		resultLayout.AddWidget(itemWidget, 0, 0)
		icon := svg.NewQSvgWidget(nil)
		icon.SetFixedWidth(editor.iconSize - 1)
		icon.SetFixedHeight(editor.iconSize - 1)
		icon.SetContentsMargins(0, 0, 0, 0)
		// icon.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
		base := widgets.NewQLabel(nil, 0)
		base.SetText("base")
		base.SetContentsMargins(0, padding, 0, padding)
		// base.SetStyleSheet("background-color: rgba(0, 0, 0, 0); white-space: pre-wrap;")
		base.SetSizePolicy2(widgets.QSizePolicy__Preferred, widgets.QSizePolicy__Maximum)
		itemLayout.AddWidget(icon)
		itemLayout.AddWidget(base)
		resultItem := &PaletteResultItem{
			p:      palette,
			widget: itemWidget,
			icon:   icon,
			base:   base,
		}
		resultItems = append(resultItems, resultItem)
	}
	palette.max = max
	palette.resultItems = resultItems
	return palette
}

func (p *Palette) setColor() {
	fg := editor.colors.widgetFg.String()
	bg := editor.colors.widgetBg
	inactiveFg := editor.colors.inactiveFg
	transparent := transparent() * transparent()
	if editor.config.Palette.Transparent < 1.0 {
		transparent = editor.config.Palette.Transparent * editor.config.Palette.Transparent
	}
	p.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background-color: rgba(%d, %d, %d, %f); } * { color: %s; } ", bg.R, bg.G, bg.B, transparent, fg))
	p.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, %f);", inactiveFg.R, inactiveFg.G, inactiveFg.B, transparent))
	for _, item := range p.resultItems {
		item.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background-color: rgba(0, 0, 0, 0.0); } * { color: %s; } ", fg))
	}
	if transparent < 1.0 {
		p.patternWidget.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
		p.pattern.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
	} else {
		p.pattern.SetStyleSheet(fmt.Sprintf("background-color: %s;", bg.String()))
	}
}

func (p *Palette) resize() {
	eWidth := editor.window.Width() - 10
	width := int(math.Trunc(float64(eWidth) * 0.7))
	cursorBoundary := p.padding*4 + p.textLength() + p.patternPadding
	if cursorBoundary > width {
		width = cursorBoundary
	}
	if width > eWidth {
		width = eWidth
		p.pattern.SetAlignment(core.Qt__AlignRight | core.Qt__AlignCenter)
	} else if width <= eWidth {
		if p.pattern.Alignment() != core.Qt__AlignLeft {
			p.pattern.SetAlignment(core.Qt__AlignLeft)
		}
	}

	if p.width == width {
		return
	}
	p.width = width
	p.pattern.SetFixedWidth(p.width - p.padding*2)
	p.widget.SetMaximumWidth(p.width)
	p.widget.SetMinimumWidth(p.width)

	x := eWidth - p.width
	if x < 0 {
		x = 0
	}
	p.widget.Move2(x/2, 10)

	p.showTotal = 0
	for i := p.showTotal; i < len(p.resultItems); i++ {
		p.resultItems[i].hide()
	}
}

func (p *Palette) resizeResultItems() {
	if p.showTotal != 0 {
		return
	}
	itemHeight := p.resultItems[0].widget.SizeHint().Height()
	p.itemHeight = itemHeight
	p.showTotal = int(float64(p.ws.height)/float64(itemHeight)*editor.config.Palette.AreaRatio) - 1
	if p.ws.uiAttached {
		fuzzy.UpdateMax(p.ws.nvim, p.showTotal)
	}
}

func (p *Palette) show() {
	if !p.hidden {
		return
	}
	p.resizeResultItems()
	p.hidden = false
	p.widget.Raise()
	p.widget.SetWindowOpacity(1.0)
	p.widget.Show()
}

func (p *Palette) hide() {
	if p.hidden {
		return
	}
	p.hidden = true
	p.widget.Hide()
}

func (p *Palette) setPattern(text string) {
	p.patternText = text
	p.pattern.SetText(text)
}

func (p *Palette) cursorMove(x int) {
	X := p.textLength()
	var stickOutLen int
	boundary := p.pattern.Width() - (p.padding * 2)
	if X >= boundary {
		stickOutLen = X - boundary
	}
	pos := p.cursorPos(x) - stickOutLen
	if pos < 0 {
		pos = 0
	}

	p.cursorX = pos
	p.ws.cursor.x = p.cursorX + p.patternPadding
	p.ws.cursor.y = p.patternPadding + p.ws.cursor.shift
	p.ws.cursor.widget.Move2(p.ws.cursor.x, p.ws.cursor.y+p.ws.cursor.shift+1)
	p.ws.cursor.widget.SetParent(p.pattern)

	p.redrawAllContentInWindows()
}

func (p *Palette) redrawAllContentInWindows() {
	if runtime.GOOS != "windows" {
		return
	}
	if p.ws.markdown.webview.IsVisible() {
		return
	}

	p.hide()
	p.resultWidget.Hide()
	p.resultMainWidget.Hide()
	p.show()
	p.resultWidget.Show()
	p.resultMainWidget.Show()
}

func (p *Palette) updateFont() {
	font := gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false)
	p.widget.SetFont(font)
	p.pattern.SetFont(font)
}

func (p *Palette) textLength() int {
	font := gui.NewQFontMetricsF(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
	l := 0
	if p.isHTMLText {
		t := gui.NewQTextDocument(nil)
		t.SetHtml(p.patternText)
		l = int(
			font.HorizontalAdvance(
				t.ToPlainText(),
				-1,
			),
		)
	} else {
		l = int(
			font.HorizontalAdvance(
				p.patternText,
				-1,
			),
		)
	}

	return l
}

func (p *Palette) cursorPos(x int) int {
	font := gui.NewQFontMetricsF(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
	l := 0
	if p.isHTMLText {
		t := gui.NewQTextDocument(nil)
		t.SetHtml(p.patternText)
		l = int(
			font.HorizontalAdvance(
				t.ToPlainText()[:x],
				-1,
			),
		)
	} else {
		l = int(
			font.HorizontalAdvance(
				p.patternText[:x],
				-1,
			),
		)
	}

	return l
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
	c := editor.colors.selectedBg
	// transparent := editor.config.Editor.Transparent
	transparent := transparent()
	if f.selected {
		f.widget.SetStyleSheet(fmt.Sprintf(".QWidget {background-color: rgba(%d, %d, %d, %f);}", c.R, c.G, c.B, transparent))
	} else {
		f.widget.SetStyleSheet("")
	}
	// f.p.widget.Hide()
	// f.p.widget.Show()

}

func (f *PaletteResultItem) setSelected(selected bool) {
	if f.selected == selected {
		return
	}
	f.selected = selected
	f.update()
}

func (f *PaletteResultItem) show() {
	// if f.hidden {
	f.hidden = false
	f.widget.Show()
	// }
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
	svgContent := editor.getSvg(f.iconType, nil)
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

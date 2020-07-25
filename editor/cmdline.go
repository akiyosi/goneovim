package editor

import (
	"fmt"
	"strings"

	"github.com/akiyosi/goneovim/util"
)

// CmdContent is the content of the cmdline
type CmdContent struct {
	indent  int
	firstc  string
	prompt  string
	content string
}

// Cmdline is the cmdline
type Cmdline struct {
	shown         bool
	ws            *Workspace
	pos           int
	content       *CmdContent
	preContent    *CmdContent
	function      []*CmdContent
	inFunction    bool
	rawItems      []interface{}
	wildmenuShown bool
	top           int
}

func initCmdline() *Cmdline {
	return &Cmdline{
		content:  &CmdContent{},
		rawItems: make([]interface{}, 0),
	}
}

func (c *CmdContent) getText() string {
	indentStr := ""
	for i := 0; i < c.indent; i++ {
		indentStr += " "
	}
	return fmt.Sprintf("%s%s", indentStr, c.content)
}

func (c *Cmdline) getText(ch string) string {
	indentStr := ""
	for i := 0; i < c.content.indent; i++ {
		indentStr += " "
	}
	if c.pos > len(c.content.content) {
		c.pos = len(c.content.content) - 1
	}
	if len(c.content.content) == 0 {
		c.pos = 0
	}
	return fmt.Sprintf("%s%s%s", c.content.firstc, indentStr, c.content.content[:c.pos]+ch+c.content.content[c.pos:])
}

func sanitize(s string) string {
	s = strings.Replace(s, " ", `&nbsp;`, -1)
	s = strings.Replace(s, "\t", `&nbsp;`, -1)
	s = strings.Replace(s, "<", `&lt;`, -1)
	s = strings.Replace(s, ">", `&gt;`, -1)

	return s
}

func (c *Cmdline) show(args []interface{}) {
	palette := c.ws.palette
	arg := args[0].([]interface{})

	content := ""
	contentChunks := arg[0].([]interface{})
	for _, e := range contentChunks {
		a := e.([]interface{})

		if len(a) < 2 {
			// content += a[0].(string)
			content += strings.Replace(a[0].(string), "\t", " ", -1)
		} else {
			color := c.ws.foreground
			_, ok := c.ws.screen.hlAttrDef[util.ReflectToInt(a[0])]
			if ok {
				color = c.ws.screen.hlAttrDef[util.ReflectToInt(a[0])].foreground
			}

			// I don't know how to set sticking out direction of
			// the contents of a qlabel with html text to the left.
			if len(contentChunks) == 1 {
				// content += a[1].(string)
				content += strings.Replace(a[1].(string), "\t", " ", -1)
			} else {
				content += fmt.Sprintf(
					"<font color='%s'>%s</font>",
					color.Hex(),
					sanitize(a[1].(string)),
				)
				palette.isHTMLText = true
			}
		}
	}
	// content := arg[0].([]interface{})[0].([]interface{})[1].(string)

	pos := util.ReflectToInt(arg[1])
	firstc := arg[2].(string)
	prompt := arg[3].(string)
	indent := util.ReflectToInt(arg[4])
	// level := util.ReflectToInt(arg[5])
	// fmt.Println("cmdline show", content, pos, firstc, prompt, indent, level)

	c.pos = pos
	c.content.firstc = firstc
	isResize := c.content.content != content
	c.content.content = content
	c.content.indent = indent
	c.content.prompt = prompt
	text := c.getText("")
	palette.setPattern(text)
	c.cursorMove()
	if isResize {
		palette.resize()
	}
	if !c.wildmenuShown {
		c.showAddition()
		palette.scrollCol.Hide()
	}
	palette.show()
	c.shown = true
}

func (c *Cmdline) showAddition() {
	lines := append(c.getPromptLines(), c.getFunctionLines()...)
	palette := c.ws.palette
	for i, resultItem := range palette.resultItems {
		if i >= len(lines) || i >= palette.showTotal {
			resultItem.hide()
			continue
		}
		resultItem.setItem(lines[i], "", []int{})
		resultItem.setSelected(false)
		resultItem.show()
	}
}

func (c *Cmdline) getPromptLines() []string {
	result := []string{}
	if c.content.prompt == "" {
		return result
	}

	lines := strings.Split(c.content.prompt, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

func (c *Cmdline) getFunctionLines() []string {
	result := []string{}
	if !c.inFunction {
		return result
	}

	for _, content := range c.function {
		result = append(result, content.getText())
	}
	return result
}

func (c *Cmdline) cursorMove() {
	c.ws.palette.cursorMove(c.pos + len(c.content.firstc) + c.content.indent)
}

func (c *Cmdline) hide() {
	palette := c.ws.palette
	palette.hide()
	if c.inFunction {
		c.function = append(c.function, c.content)
	}
	c.preContent = c.content
	c.content = &CmdContent{}
	// Move cursor into window
	// win, ok := c.ws.screen.windows[c.ws.cursor.gridid]
	// if ok && win != nil {
	// 	c.ws.cursor.widget.SetParent(win.widget)
	// 	c.ws.cursor.widget.Hide()
	// 	c.ws.cursor.widget.Show()
	// }
	win, ok := c.ws.screen.getWindow(c.ws.cursor.gridid)
	if ok {
		c.ws.cursor.widget.SetParent(win.widget)
		c.ws.cursor.widget.Hide()
		c.ws.cursor.widget.Show()
	}

	c.shown = false
}

func (c *Cmdline) functionShow() {
	c.inFunction = true
	c.function = []*CmdContent{c.preContent}
}

func (c *Cmdline) functionHide() {
	c.inFunction = false
}

func (c *Cmdline) changePos(args []interface{}) {
	args = args[0].([]interface{})
	pos := util.ReflectToInt(args[0])
	// level := util.ReflectToInt(args[1])
	// fmt.Println("change pos", pos, level)
	c.pos = pos
	c.cursorMove()
}

func (c *Cmdline) putChar(args []interface{}) {
	args = args[0].([]interface{})
	ch := args[0].(string)
	// shift := util.ReflectToInt(args[1])
	// level := util.ReflectToInt(args[2])
	// fmt.Println("putChar", ch, shift, level)
	text := c.getText(ch)
	palette := c.ws.palette
	palette.setPattern(text)
}

func (c *Cmdline) wildmenuShow(args []interface{}) {
	c.wildmenuShown = true
	args = args[0].([]interface{})
	c.rawItems = args[0].([]interface{})
	palette := c.ws.palette
	c.top = 0
	for i := 0; i < palette.showTotal; i++ {
		resultItem := palette.resultItems[i]
		if i >= len(c.rawItems) {
			resultItem.hide()
			continue
		}
		text := c.rawItems[i].(string)
		resultItem.setItem(text, "", []int{})
		resultItem.show()
		resultItem.setSelected(false)
	}

	total := len(c.rawItems)
	if total > palette.showTotal {
		height := int(float64(palette.showTotal) / float64(total) * float64(palette.itemHeight*palette.showTotal))
		if height == 0 {
			height = 1
		}
		palette.scrollBar.SetFixedHeight(height)
		palette.scrollBarPos = 0
		palette.scrollBar.Move2(0, palette.scrollBarPos)
		palette.scrollCol.Show()
	} else {
		palette.scrollCol.Hide()
	}
}

func (c *Cmdline) wildmenuSelect(args []interface{}) {
	selected := util.ReflectToInt(args[0].([]interface{})[0])
	// fmt.Println("selected is", selected)
	showTotal := c.ws.palette.showTotal
	if selected == -1 && c.top > 0 {
		c.wildmenuScroll(-c.top)
	}
	if selected-c.top >= showTotal {
		c.wildmenuScroll(selected - c.top - showTotal + 1)
	}
	if selected >= 0 && selected-c.top < 0 {
		c.wildmenuScroll(-1)
	}
	palette := c.ws.palette
	for i := 0; i < palette.showTotal; i++ {
		item := palette.resultItems[i]
		item.setSelected(selected == i+c.top)
	}
}

func (c *Cmdline) wildmenuScroll(n int) {
	c.top += n
	palette := c.ws.palette
	for i := 0; i < palette.showTotal; i++ {
		resultItem := palette.resultItems[i]
		if i >= len(c.rawItems) {
			resultItem.hide()
			continue
		}
		text := c.rawItems[i+c.top].(string)
		resultItem.setItem(text, "", []int{})
		resultItem.show()
		resultItem.setSelected(false)
	}
	palette.scrollBarPos = int((float64(c.top) / float64(len(c.rawItems))) * float64(palette.itemHeight*palette.showTotal))
	palette.scrollBar.Move2(0, palette.scrollBarPos)
}

func (c *Cmdline) wildmenuHide() {
	c.wildmenuShown = false
}

func (c *Cmdline) cmdWildmenuShow(args []interface{}) {
	c.wildmenuShown = true

	for _, arg := range args {
		c.rawItems = (arg.([]interface{}))[0].([]interface{})

		palette := c.ws.palette
		c.top = 0
		for i := 0; i < palette.showTotal; i++ {
			resultItem := palette.resultItems[i]
			if i >= len(c.rawItems) {
				resultItem.hide()
				continue
			}
			text := (c.rawItems[i].([]interface{}))[0].(string)
			resultItem.setItem(text, "", []int{})
			resultItem.show()
			resultItem.setSelected(false)
		}

		total := len(c.rawItems)
		if total > palette.showTotal {
			height := int(float64(palette.showTotal) / float64(total) * float64(palette.itemHeight*palette.showTotal))
			if height == 0 {
				height = 1
			}
			palette.scrollBar.SetFixedHeight(height)
			palette.scrollBarPos = 0
			palette.scrollBar.Move2(0, palette.scrollBarPos)
			palette.scrollCol.Show()
		} else {
			palette.scrollCol.Hide()
		}
	}
}

func (c *Cmdline) cmdWildmenuSelect(args []interface{}) {
	selected := util.ReflectToInt(args[0].([]interface{})[0])
	// fmt.Println("selected is", selected)
	showTotal := c.ws.palette.showTotal
	if selected == -1 && c.top > 0 {
		c.cmdWildmenuScroll(-c.top)
	}
	if selected-c.top >= showTotal {
		c.cmdWildmenuScroll(selected - c.top - showTotal + 1)
	}
	if selected >= 0 && selected-c.top < 0 {
		c.cmdWildmenuScroll(-1)
	}
	palette := c.ws.palette
	for i := 0; i < palette.showTotal; i++ {
		item := palette.resultItems[i]
		item.setSelected(selected == i+c.top)
	}
}

func (c *Cmdline) cmdWildmenuScroll(n int) {
	c.top += n
	palette := c.ws.palette
	for i := 0; i < palette.showTotal; i++ {
		resultItem := palette.resultItems[i]
		if i >= len(c.rawItems) {
			resultItem.hide()
			continue
		}
		text := (c.rawItems[i+c.top].([]interface{}))[0].(string)
		resultItem.setItem(text, "", []int{})
		resultItem.show()
		resultItem.setSelected(false)
	}
	palette.scrollBarPos = int((float64(c.top) / float64(len(c.rawItems))) * float64(palette.itemHeight*palette.showTotal))
	palette.scrollBar.Move2(0, palette.scrollBarPos)
}

func (c *Cmdline) cmdWildmenuHide() {
	c.wildmenuShown = false
}

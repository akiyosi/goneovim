package editor

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/akiyosi/goneovim/util"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/webchannel"
	"github.com/therecipe/qt/webengine"
	"github.com/therecipe/qt/widgets"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
)

//
const (
	GonvimMarkdownBufName = "__GonvimMarkdownPreview__"
)

// Markdown is the markdown preview window
type Markdown struct {
	webview         *webengine.QWebEngineView
	webpage         *webengine.QWebEnginePage
	ws              *Workspace
	markdownUpdates chan string
	container       *widgets.QPlainTextEdit
	htmlSet         bool
}

func newMarkdown(workspace *Workspace) *Markdown {
	webview := webengine.NewQWebEngineView(nil)
	// Try to fix issue (#91)
	if runtime.GOOS == "windows" {
		webview.SetAttribute(core.Qt__WA_NativeWindow, true)
	}
	m := &Markdown{
		webview:         webview,
		markdownUpdates: make(chan string, 1000),
		ws:              workspace,
	}
	m.ws.signal.ConnectMarkdownSignal(func() {
		if m.webpage == nil {
			m.webpage = webengine.NewQWebEnginePage(nil)
			m.webview.SetPage(m.webpage)
			m.container = widgets.NewQPlainTextEdit(nil)
			channel := webchannel.NewQWebChannel(nil)
			channel.RegisterObject("content", m.container)
			//m.webpage.SetWebChannel2(channel)
			m.webpage.SetWebChannel(channel)
		}
		content := <-m.markdownUpdates
		// Get file base path
		done := make(chan error, 60)
		var basePath string
		var err error
		go func() {
			basePath, err = m.ws.nvim.CommandOutput(`echo expand('%:p:h')`)
			done <- err
		}()
		select {
		case <-done:
		case <-time.After(40 * time.Millisecond):
		}
		// Create bae url
		baseUrl := `file://` + basePath + `/`
		if !m.htmlSet {
			m.htmlSet = true
			m.webpage.SetHtml(m.getHTML(content), core.NewQUrl3(baseUrl, 0))
		} else {
			m.container.SetPlainTextDefault(content)
			m.container.TextChanged()
		}
		m.updatePos()
	})
	m.webview.ConnectEventFilter(func(watched *core.QObject, event *core.QEvent) bool {
		if event.Type() == core.QEvent__KeyPress {
			keyPress := gui.NewQKeyEventFromPointer(event.Pointer())
			editor.keyPress(keyPress)
			return true
		}
		return m.webview.EventFilterDefault(watched, event)
	})
	// m.webpage.InstallEventFilter(m.webview)
	// m.webpage.ConnectEvent(func(event *core.QEvent) bool {
	// 	fmt.Println("webpage event", event)
	// 	if event.Type() == core.QEvent__ChildAdded {
	// 		fmt.Println("webpage has child")
	// 		childEvent := core.NewQChildEventFromPointer(event.Pointer())
	// 		childEvent.Child().InstallEventFilter(m.webview)
	// 	} else if event.Type() == core.QEvent__KeyPress {
	// 		fmt.Println("webpage has keypress")
	// 	}
	// 	return m.webpage.EventDefault(event)
	// })
	m.webview.ConnectEvent(func(event *core.QEvent) bool {
		if event.Type() == core.QEvent__ChildAdded {
			childEvent := core.NewQChildEventFromPointer(event.Pointer())
			childEvent.Child().InstallEventFilter(m.webview)
		}
		return m.webview.EventDefault(event)
	})
	m.webview.ConnectWheelEvent(m.wheelEvent)
	m.hide()
	// m.webview.SetEnabled(false)
	return m
}

func (m *Markdown) wheelEvent(event *gui.QWheelEvent) {
	var horiz int

	switch runtime.GOOS {
	case "darwin":
		pixels := event.PixelDelta()
		if pixels != nil {
			horiz = pixels.Y()
		}
		m.webpage.RunJavaScript(fmt.Sprintf("window.scrollBy(0, %v)", horiz*(-1)))
	default:
		horiz = event.AngleDelta().Y()
		m.webpage.RunJavaScript(fmt.Sprintf("window.scrollBy(0, %v)", horiz))
	}

	event.Accept()
}

func (m *Markdown) updatePos() {
	needHide := true
	m.ws.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.isMsgGrid || win.isFloatWin {
			return true
		}
		if filepath.Base(win.bufName) == GonvimMarkdownBufName {
			font := win.getFont()
			if !m.webview.IsVisible() {
				m.webview.Resize2(
					int(float64(win.cols)*font.truewidth),
					win.rows*font.lineHeight,
				)
				m.webview.SetParent(win.widget)
				m.show()
			} else {
				m.webview.Resize2(
					int(float64(win.cols)*font.truewidth),
					win.rows*m.ws.font.lineHeight,
				)
			}
			needHide = false

			if m.ws.maxLine == 0 {
				lnITF, err := m.ws.nvimEval("line('$')")
				if err != nil {
					m.ws.maxLine = 0
				} else {
					m.ws.maxLine = util.ReflectToInt(lnITF)
				}
			}

			// baseLine := 0
			// lines := m.ws.botLine - m.ws.topLine
			// if m.ws.curLine < m.ws.topLine + int(float64(lines)/3.0) {
			// 	baseLine = (m.ws.topLine + m.ws.topLine)/2
			// } else if m.ws.curLine >= m.ws.topLine + int(float64(lines)/3.0) && m.ws.curLine < m.ws.topLine + int(float64(lines*2)/3.0) {
			// 	baseLine = m.ws.curLine
			// } else {
			// 	baseLine = (m.ws.curLine + m.ws.topLine)/2
			// }
			// position := float64(baseLine)  / float64(m.ws.maxLine)
			// m.webpage.RunJavaScript(fmt.Sprintf("window.scrollTo(0,%f*document.body.scrollHeight)", position))
			return false
		}
		return true
	})

	if needHide {
		m.hide()
	}
}

func (m *Markdown) show() {
	if m.webview.IsVisible() {
		return
	}
	m.webview.Show()
	m.webview.Raise()
}

func (m *Markdown) hide() {
	if !m.webview.IsVisible() {
		return
	}
	m.webview.Hide()
}

func (m *Markdown) scrollTop() {
	m.webpage.RunJavaScript("window.scrollTo(0, 0)")
}

func (m *Markdown) scrollUp() {
	m.webpage.RunJavaScript("window.scrollBy(0, -20)")
}

func (m *Markdown) scrollDown() {
	m.webpage.RunJavaScript("window.scrollBy(0, 20)")
}

func (m *Markdown) scrollBottom() {
	m.webpage.RunJavaScript("window.scrollTo(0,document.body.scrollHeight)")
}

func (m *Markdown) scrollPageUp() {
	m.webpage.RunJavaScript(fmt.Sprintf("window.scrollBy(0, -%d)", m.webview.Height()))
}

func (m *Markdown) scrollPageDown() {
	m.webpage.RunJavaScript(fmt.Sprintf("window.scrollBy(0, %d)", m.webview.Height()))
}

func (m *Markdown) scrollHalfPageUp() {
	m.webpage.RunJavaScript(fmt.Sprintf("window.scrollBy(0, -%d)", m.webview.Height()/2))
}

func (m *Markdown) scrollHalfPageDown() {
	m.webpage.RunJavaScript(fmt.Sprintf("window.scrollBy(0, %d)", m.webview.Height()/2))
}

func (m *Markdown) toggle() {
	// for _, win := range m.ws.screen.windows {
	// 	if win == nil {
	// 		continue
	// 	}
	// 	if win.isMsgGrid || win.isFloatWin {
	// 		continue
	// 	}
	// 	if filepath.Base(win.bufName) == GonvimMarkdownBufName {
	// 		m.htmlSet = false
	// 		m.hide()
	// 		go func() {
	// 			m.ws.nvim.SetCurrentWindow(win.id)
	// 			m.ws.nvim.Command("close")
	// 		}()
	// 		return
	// 	}
	// }
	isShownPreview := false
	m.ws.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.isMsgGrid || win.isFloatWin {
			return true
		}
		if filepath.Base(win.bufName) == GonvimMarkdownBufName {
			// isShownPreview = true
			m.htmlSet = false
			m.hide()
			isShownPreview = win.isShown()

			curWin, ok := m.ws.screen.getWindow(m.ws.cursor.bufferGridid)
			if !ok {
				return true
			}
			done := make(chan bool, 2)
			go func() {
				m.ws.nvim.SetCurrentWindow(win.id)
				m.ws.nvim.Command("close")
				if !isShownPreview {
					m.ws.nvim.SetCurrentWindow(curWin.id)
					m.htmlSet = true
				}
				done <- true
			}()
			select {
			case <-done:
			case <-time.After(40 * time.Millisecond):
			}

			return false
		}
		return true
	})
	if isShownPreview {
		return
	}
	m.ws.nvim.Command(`keepalt vertical botright split ` + GonvimMarkdownBufName)
	m.ws.nvim.Command("setlocal filetype=" + GonvimMarkdownBufName)
	m.ws.nvim.Command("setlocal buftype=nofile")
	m.ws.nvim.Command("setlocal bufhidden=hide")
	m.ws.nvim.Command("setlocal noswapfile")
	m.ws.nvim.Command("setlocal nobuflisted")
	m.ws.nvim.Command("setlocal nomodifiable")
	m.ws.nvim.Command("setlocal nolist")
	m.ws.nvim.Command("setlocal nowrap")
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> j :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_down",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> k :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_up",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> <C-e> :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_down",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> <C-y> :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_up",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> gg :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_top",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> G :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_bottom",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> <C-b> :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_pageup",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> <C-f> :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_pagedown",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> <C-u> :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_halfpageup",
	))
	m.ws.nvim.Command(fmt.Sprintf(
		"nnoremap <silent> <buffer> <C-d> :call rpcnotify(0, 'Gui', '%s')<CR>",
		"gonvim_markdown_scroll_halfpagedown",
	))
	m.ws.nvim.Command("wincmd p")
}

func (m *Markdown) newBuffer() {
	m.htmlSet = false
	m.update()
}

func (m *Markdown) update() {
	buf, err := m.ws.nvim.CurrentBuffer()
	if err != nil {
		return
	}
	lines, err := m.ws.nvim.BufferLines(buf, 0, -1, false)
	if err != nil {
		return
	}
	content := []byte{}
	for _, line := range lines {
		content = append(content, line...)
		content = append(content, '\n')
	}

	markdown := goldmark.New(
		goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithStyle(editor.config.Markdown.CodeHlStyle),
				highlighting.WithFormatOptions(
					html.WithLineNumbers(editor.config.Markdown.CodeWithLineNumbers),
				),
			),
		),
	)
	var buff bytes.Buffer
	if err := markdown.Convert([]byte(content), &buff); err != nil {
		return
	}
	m.markdownUpdates <- fmt.Sprintf(`
			<div id="placeholder" class="markdown-body">
			%s
			</div>`, buff.String())
	m.ws.signal.MarkdownSignal()
}

func (m *Markdown) getHTML(content string) string {
	js := `
  var placeholder = document.getElementById('placeholder');
  var dd = new diffDOM();

  var updateText = function(text) {
    morphdom(placeholder, text);
  }

function getMethods(obj) {
  var result = [];
  for (var id in obj) {
    try {
      if (typeof(obj[id]) == "function") {
       // result.push(id + ": " + obj[id].toString());
       result.push(id);
      }
    } catch (err) {
      result.push(id + ": inaccessible");
    }
  }
  return result;
}

  new QWebChannel(qt.webChannelTransport,
    function(channel) {
      var content = channel.objects.content;
      content.textChanged.connect(function() {
        var frag = document.createElement('div');
        frag.innerHTML = content.plainText;
        dd.apply(placeholder, dd.diff(placeholder, frag.firstElementChild));
        //placeholder.innerHTML = content.plainText;
        //morphdom(placeholder, content.plainText);
        //console.warn(document.body.innerHTML);
      });
    }
  );
`
	style := `
.markdown-body {
  -ms-text-size-adjust: 100%;
  -webkit-text-size-adjust: 100%;
  line-height: 1.5;
  color: #24292e;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol";
  font-size: 16px;
  line-height: 1.5;
  word-wrap: break-word;
}

.markdown-body .pl-c {
  color: #6a737d;
}

.markdown-body .pl-c1,
.markdown-body .pl-s .pl-v {
  color: #005cc5;
}

.markdown-body .pl-e,
.markdown-body .pl-en {
  color: #6f42c1;
}

.markdown-body .pl-smi,
.markdown-body .pl-s .pl-s1 {
  color: #24292e;
}

.markdown-body .pl-ent {
  color: #22863a;
}

.markdown-body .pl-k {
  color: #d73a49;
}

.markdown-body .pl-s,
.markdown-body .pl-pds,
.markdown-body .pl-s .pl-pse .pl-s1,
.markdown-body .pl-sr,
.markdown-body .pl-sr .pl-cce,
.markdown-body .pl-sr .pl-sre,
.markdown-body .pl-sr .pl-sra {
  color: #032f62;
}

.markdown-body .pl-v,
.markdown-body .pl-smw {
  color: #e36209;
}

.markdown-body .pl-bu {
  color: #b31d28;
}

.markdown-body .pl-ii {
  color: #fafbfc;
  background-color: #b31d28;
}

.markdown-body .pl-c2 {
  color: #fafbfc;
  background-color: #d73a49;
}

.markdown-body .pl-c2::before {
  content: "^M";
}

.markdown-body .pl-sr .pl-cce {
  font-weight: bold;
  color: #22863a;
}

.markdown-body .pl-ml {
  color: #735c0f;
}

.markdown-body .pl-mh,
.markdown-body .pl-mh .pl-en,
.markdown-body .pl-ms {
  font-weight: bold;
  color: #005cc5;
}

.markdown-body .pl-mi {
  font-style: italic;
  color: #24292e;
}

.markdown-body .pl-mb {
  font-weight: bold;
  color: #24292e;
}

.markdown-body .pl-md {
  color: #b31d28;
  background-color: #ffeef0;
}

.markdown-body .pl-mi1 {
  color: #22863a;
  background-color: #f0fff4;
}

.markdown-body .pl-mc {
  color: #e36209;
  background-color: #ffebda;
}

.markdown-body .pl-mi2 {
  color: #f6f8fa;
  background-color: #005cc5;
}

.markdown-body .pl-mdr {
  font-weight: bold;
  color: #6f42c1;
}

.markdown-body .pl-ba {
  color: #586069;
}

.markdown-body .pl-sg {
  color: #959da5;
}

.markdown-body .pl-corl {
  text-decoration: underline;
  color: #032f62;
}

.markdown-body .octicon {
  display: inline-block;
  vertical-align: text-top;
  fill: currentColor;
}

.markdown-body a {
  background-color: transparent;
}

.markdown-body a:active,
.markdown-body a:hover {
  outline-width: 0;
}

.markdown-body strong {
  font-weight: inherit;
}

.markdown-body strong {
  font-weight: bolder;
}

.markdown-body h1 {
  font-size: 2em;
  margin: 0.67em 0;
}

.markdown-body img {
  border-style: none;
}

.markdown-body code,
.markdown-body kbd,
.markdown-body pre {
  font-family: monospace, monospace;
  font-size: 1em;
}

.markdown-body hr {
  box-sizing: content-box;
  height: 0;
  overflow: visible;
}

.markdown-body input {
  font: inherit;
  margin: 0;
}

.markdown-body input {
  overflow: visible;
}

.markdown-body [type="checkbox"] {
  box-sizing: border-box;
  padding: 0;
}

.markdown-body * {
  box-sizing: border-box;
}

.markdown-body input {
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
}

.markdown-body a {
  color: #0366d6;
  text-decoration: none;
}

.markdown-body a:hover {
  text-decoration: underline;
}

.markdown-body strong {
  font-weight: 600;
}

.markdown-body hr {
  height: 0;
  margin: 15px 0;
  overflow: hidden;
  background: transparent;
  border: 0;
  border-bottom: 1px solid #dfe2e5;
}

.markdown-body hr::before {
  display: table;
  content: "";
}

.markdown-body hr::after {
  display: table;
  clear: both;
  content: "";
}

.markdown-body table {
  border-spacing: 0;
  border-collapse: collapse;
}

.markdown-body td,
.markdown-body th {
  padding: 0;
}

.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4,
.markdown-body h5,
.markdown-body h6 {
  margin-top: 0;
  margin-bottom: 0;
}

.markdown-body h1 {
  font-size: 32px;
  font-weight: 600;
}

.markdown-body h2 {
  font-size: 24px;
  font-weight: 600;
}

.markdown-body h3 {
  font-size: 20px;
  font-weight: 600;
}

.markdown-body h4 {
  font-size: 16px;
  font-weight: 600;
}

.markdown-body h5 {
  font-size: 14px;
  font-weight: 600;
}

.markdown-body h6 {
  font-size: 12px;
  font-weight: 600;
}

.markdown-body p {
  margin-top: 0;
  margin-bottom: 10px;
}

.markdown-body blockquote {
  margin: 0;
}

.markdown-body ul,
.markdown-body ol {
  padding-left: 0;
  margin-top: 0;
  margin-bottom: 0;
}

.markdown-body ol ol,
.markdown-body ul ol {
  list-style-type: lower-roman;
}

.markdown-body ul ul ol,
.markdown-body ul ol ol,
.markdown-body ol ul ol,
.markdown-body ol ol ol {
  list-style-type: lower-alpha;
}

.markdown-body dd {
  margin-left: 0;
}

.markdown-body code {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 12px;
}

.markdown-body pre {
  margin-top: 0;
  margin-bottom: 0;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 12px;
}

.markdown-body .octicon {
  vertical-align: text-bottom;
}

.markdown-body .pl-0 {
  padding-left: 0 !important;
}

.markdown-body .pl-1 {
  padding-left: 4px !important;
}

.markdown-body .pl-2 {
  padding-left: 8px !important;
}

.markdown-body .pl-3 {
  padding-left: 16px !important;
}

.markdown-body .pl-4 {
  padding-left: 24px !important;
}

.markdown-body .pl-5 {
  padding-left: 32px !important;
}

.markdown-body .pl-6 {
  padding-left: 40px !important;
}

.markdown-body::before {
  display: table;
  content: "";
}

.markdown-body::after {
  display: table;
  clear: both;
  content: "";
}

.markdown-body>*:first-child {
  margin-top: 0 !important;
}

.markdown-body>*:last-child {
  margin-bottom: 0 !important;
}

.markdown-body a:not([href]) {
  color: inherit;
  text-decoration: none;
}

.markdown-body .anchor {
  float: left;
  padding-right: 4px;
  margin-left: -20px;
  line-height: 1;
}

.markdown-body .anchor:focus {
  outline: none;
}

.markdown-body p,
.markdown-body blockquote,
.markdown-body ul,
.markdown-body ol,
.markdown-body dl,
.markdown-body table,
.markdown-body pre {
  margin-top: 0;
  margin-bottom: 16px;
}

.markdown-body hr {
  height: 0.25em;
  padding: 0;
  margin: 24px 0;
  background-color: #e1e4e8;
  border: 0;
}

.markdown-body blockquote {
  padding: 0 1em;
  color: #6a737d;
  border-left: 0.25em solid #dfe2e5;
}

.markdown-body blockquote>:first-child {
  margin-top: 0;
}

.markdown-body blockquote>:last-child {
  margin-bottom: 0;
}

.markdown-body kbd {
  display: inline-block;
  padding: 3px 5px;
  font-size: 11px;
  line-height: 10px;
  color: #444d56;
  vertical-align: middle;
  background-color: #fafbfc;
  border: solid 1px #c6cbd1;
  border-bottom-color: #959da5;
  border-radius: 3px;
  box-shadow: inset 0 -1px 0 #959da5;
}

.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4,
.markdown-body h5,
.markdown-body h6 {
  margin-top: 24px;
  margin-bottom: 16px;
  font-weight: 600;
  line-height: 1.25;
}

.markdown-body h1 .octicon-link,
.markdown-body h2 .octicon-link,
.markdown-body h3 .octicon-link,
.markdown-body h4 .octicon-link,
.markdown-body h5 .octicon-link,
.markdown-body h6 .octicon-link {
  color: #1b1f23;
  vertical-align: middle;
  visibility: hidden;
}

.markdown-body h1:hover .anchor,
.markdown-body h2:hover .anchor,
.markdown-body h3:hover .anchor,
.markdown-body h4:hover .anchor,
.markdown-body h5:hover .anchor,
.markdown-body h6:hover .anchor {
  text-decoration: none;
}

.markdown-body h1:hover .anchor .octicon-link,
.markdown-body h2:hover .anchor .octicon-link,
.markdown-body h3:hover .anchor .octicon-link,
.markdown-body h4:hover .anchor .octicon-link,
.markdown-body h5:hover .anchor .octicon-link,
.markdown-body h6:hover .anchor .octicon-link {
  visibility: visible;
}

.markdown-body h1 {
  padding-bottom: 0.3em;
  font-size: 2em;
  border-bottom: 1px solid #eaecef;
}

.markdown-body h2 {
  padding-bottom: 0.3em;
  font-size: 1.5em;
  border-bottom: 1px solid #eaecef;
}

.markdown-body h3 {
  font-size: 1.25em;
}

.markdown-body h4 {
  font-size: 1em;
}

.markdown-body h5 {
  font-size: 0.875em;
}

.markdown-body h6 {
  font-size: 0.85em;
  color: #6a737d;
}

.markdown-body ul,
.markdown-body ol {
  padding-left: 2em;
}

.markdown-body ul ul,
.markdown-body ul ol,
.markdown-body ol ol,
.markdown-body ol ul {
  margin-top: 0;
  margin-bottom: 0;
}

.markdown-body li {
  word-wrap: break-all;
}

.markdown-body li>p {
  margin-top: 16px;
}

.markdown-body li+li {
  margin-top: 0.25em;
}

.markdown-body dl {
  padding: 0;
}

.markdown-body dl dt {
  padding: 0;
  margin-top: 16px;
  font-size: 1em;
  font-style: italic;
  font-weight: 600;
}

.markdown-body dl dd {
  padding: 0 16px;
  margin-bottom: 16px;
}

.markdown-body table {
  display: block;
  width: 100%;
  overflow: auto;
}

.markdown-body table th {
  font-weight: 600;
}

.markdown-body table th,
.markdown-body table td {
  padding: 6px 13px;
  border: 1px solid #dfe2e5;
}

.markdown-body table tr {
  background-color: #fff;
  border-top: 1px solid #c6cbd1;
}

.markdown-body table tr:nth-child(2n) {
  background-color: #f6f8fa;
}

.markdown-body img {
  max-width: 100%;
  box-sizing: content-box;
  background-color: #fff;
}

.markdown-body img[align=right] {
  padding-left: 20px;
}

.markdown-body img[align=left] {
  padding-right: 20px;
}

.markdown-body code {
  padding: 0.2em 0.4em;
  margin: 0;
  font-size: 85%;
  background-color: rgba(27,31,35,0.05);
  border-radius: 3px;
}

.markdown-body pre {
  word-wrap: normal;
}

.markdown-body pre>code {
  padding: 0;
  margin: 0;
  font-size: 100%;
  word-break: normal;
  white-space: pre;
  background: transparent;
  border: 0;
}

.markdown-body .highlight {
  margin-bottom: 16px;
}

.markdown-body .highlight pre {
  margin-bottom: 0;
  word-break: normal;
}

.markdown-body .highlight pre,
.markdown-body pre {
  padding: 16px;
  overflow: auto;
  font-size: 85%;
  line-height: 1.45;
  background-color: #f6f8fa;
  border-radius: 3px;
}

.markdown-body pre code {
  display: inline;
  max-width: auto;
  padding: 0;
  margin: 0;
  overflow: visible;
  line-height: inherit;
  word-wrap: normal;
  background-color: transparent;
  border: 0;
}

.markdown-body .full-commit .btn-outline:not(:disabled):hover {
  color: #005cc5;
  border-color: #005cc5;
}

.markdown-body kbd {
  display: inline-block;
  padding: 3px 5px;
  font: 11px "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  line-height: 10px;
  color: #444d56;
  vertical-align: middle;
  background-color: #fafbfc;
  border: solid 1px #d1d5da;
  border-bottom-color: #c6cbd1;
  border-radius: 3px;
  box-shadow: inset 0 -1px 0 #c6cbd1;
}

.markdown-body :checked+.radio-label {
  position: relative;
  z-index: 1;
  border-color: #0366d6;
}

.markdown-body .task-list-item {
  list-style-type: none;
}

.markdown-body .task-list-item+.task-list-item {
  margin-top: 3px;
}

.markdown-body .task-list-item input {
  margin: 0 0.2em 0.25em -1.6em;
  vertical-align: middle;
}

.markdown-body hr {
  border-bottom-color: #eee;
}
`
	morphdomjs := `
(function(root, factory) {
    if (typeof exports !== 'undefined') {
        if (typeof module !== 'undefined' && module.exports) {
            exports = module.exports = factory();
        } else {
            exports.diffDOM = factory();
        }
    } else if (typeof define === 'function') {
        // AMD loader
        define(factory);
    } else {
        // window in the browser, or exports on the server
        root.diffDOM = factory();
    }
})(this, function() {
    "use strict";

    var diffcount;

    var Diff = function(options) {
        var diff = this;
        if (options) {
            var keys = Object.keys(options),
                length = keys.length,
                i;
            for (i = 0; i < length; i++) {
                diff[keys[i]] = options[keys[i]];
            }
        }

    };

    Diff.prototype = {
        toString: function() {
            return JSON.stringify(this);
        },
        setValue: function(aKey, aValue) {
            this[aKey] = aValue;
            return this;
        }
    };

    var SubsetMapping = function SubsetMapping(a, b) {
        this.oldValue = a;
        this.newValue = b;
    };

    SubsetMapping.prototype = {
        contains: function contains(subset) {
            if (subset.length < this.length) {
                return subset.newValue >= this.newValue && subset.newValue < this.newValue + this.length;
            }
            return false;
        },
        toString: function toString() {
            return this.length + " element subset, first mapping: old " + this.oldValue + " ? new " + this.newValue;
        }
    };

    var elementDescriptors = function(el) {
        var output = [];
        if (el.nodeName !== '#text' && el.nodeName !== '#comment') {
            output.push(el.nodeName);
            if (el.attributes) {
                if (el.attributes['class']) {
                    output.push(el.nodeName + '.' + el.attributes['class'].replace(/ /g, '.'));
                }
                if (el.attributes.id) {
                    output.push(el.nodeName + '#' + el.attributes.id);
                }
            }

        }
        return output;
    };

    var findUniqueDescriptors = function(li) {
        var uniqueDescriptors = {},
            duplicateDescriptors = {},
            liLength = li.length,
            nodeLength, node, descriptors, descriptor, inUnique, inDupes, i, j;

        for (i = 0; i < liLength; i++) {
            node = li[i];
            nodeLength = node.length;
            descriptors = elementDescriptors(node);
            for (j = 0; j < nodeLength; j++) {
                descriptor = descriptors[j];
                inUnique = descriptor in uniqueDescriptors;
                inDupes = descriptor in duplicateDescriptors;
                if (!inUnique && !inDupes) {
                    uniqueDescriptors[descriptor] = true;
                } else if (inUnique) {
                    delete uniqueDescriptors[descriptor];
                    duplicateDescriptors[descriptor] = true;
                }
            }
        }

        return uniqueDescriptors;
    };

    var uniqueInBoth = function(l1, l2) {
        var l1Unique = findUniqueDescriptors(l1),
            l2Unique = findUniqueDescriptors(l2),
            inBoth = {},
            keys = Object.keys(l1Unique),
            length = keys.length,
            key,
            i;

        for (i = 0; i < length; i++) {
            key = keys[i];
            if (l2Unique[key]) {
                inBoth[key] = true;
            }
        }

        return inBoth;
    };

    var removeDone = function(tree) {
        delete tree.outerDone;
        delete tree.innerDone;
        delete tree.valueDone;
        if (tree.childNodes) {
            return tree.childNodes.every(removeDone);
        } else {
            return true;
        }
    };

    var isEqual = function(e1, e2) {

        var e1Attributes, e2Attributes;

        if (!['nodeName', 'value', 'checked', 'selected', 'data'].every(function(element) {
                if (e1[element] !== e2[element]) {
                    return false;
                }
                return true;
            })) {
            return false;
        }

        if (Boolean(e1.attributes) !== Boolean(e2.attributes)) {
            return false;
        }

        if (Boolean(e1.childNodes) !== Boolean(e2.childNodes)) {
            return false;
        }

        if (e1.attributes) {
            e1Attributes = Object.keys(e1.attributes);
            e2Attributes = Object.keys(e2.attributes);

            if (e1Attributes.length !== e2Attributes.length) {
                return false;
            }
            if (!e1Attributes.every(function(attribute) {
                    if (e1.attributes[attribute] !== e2.attributes[attribute]) {
                        return false;
                    }
                })) {
                return false;
            }
        }

        if (e1.childNodes) {
            if (e1.childNodes.length !== e2.childNodes.length) {
                return false;
            }
            if (!e1.childNodes.every(function(childNode, index) {
                    return isEqual(childNode, e2.childNodes[index]);
                })) {

                return false;
            }

        }

        return true;

    };


    var roughlyEqual = function(e1, e2, uniqueDescriptors, sameSiblings, preventRecursion) {
        var childUniqueDescriptors, nodeList1, nodeList2;

        if (!e1 || !e2) {
            return false;
        }

        if (e1.nodeName !== e2.nodeName) {
            return false;
        }

        if (e1.nodeName === '#text') {
            // Note that we initially don't care what the text content of a node is,
            // the mere fact that it's the same tag and "has text" means it's roughly
            // equal, and then we can find out the true text difference later.
            return preventRecursion ? true : e1.data === e2.data;
        }


        if (e1.nodeName in uniqueDescriptors) {
            return true;
        }

        if (e1.attributes && e2.attributes) {

            if (e1.attributes.id) {
                if (e1.attributes.id !== e2.attributes.id) {
                    return false;
                } else {
                    var idDescriptor = e1.nodeName + '#' + e1.attributes.id;
                    if (idDescriptor in uniqueDescriptors) {
                        return true;
                    }
                }
            }
            if (e1.attributes['class'] && e1.attributes['class'] === e2.attributes['class']) {
                var classDescriptor = e1.nodeName + '.' + e1.attributes['class'].replace(/ /g, '.');
                if (classDescriptor in uniqueDescriptors) {
                    return true;
                }
            }
        }

        if (sameSiblings) {
            return true;
        }

        nodeList1 = e1.childNodes ? e1.childNodes.slice().reverse() : [];
        nodeList2 = e2.childNodes ? e2.childNodes.slice().reverse() : [];

        if (nodeList1.length !== nodeList2.length) {
            return false;
        }

        if (preventRecursion) {
            return nodeList1.every(function(element, index) {
                return element.nodeName === nodeList2[index].nodeName;
            });
        } else {
            // note: we only allow one level of recursion at any depth. If 'preventRecursion'
            // was not set, we must explicitly force it to true for child iterations.
            childUniqueDescriptors = uniqueInBoth(nodeList1, nodeList2);
            return nodeList1.every(function(element, index) {
                return roughlyEqual(element, nodeList2[index], childUniqueDescriptors, true, true);
            });
        }
    };


    var cloneObj = function(obj) {
        //  TODO: Do we really need to clone here? Is it not enough to just return the original object?
        return JSON.parse(JSON.stringify(obj));
    };

    /**
     * based on https://en.wikibooks.org/wiki/Algorithm_implementation/Strings/Longest_common_substring#JavaScript
     */
    var findCommonSubsets = function(c1, c2, marked1, marked2) {
        var lcsSize = 0,
            index = [],
            c1Length = c1.length,
            c2Length = c2.length,
            matches = Array.apply(null, new Array(c1Length + 1)).map(function() {
                return [];
            }), // set up the matching table
            uniqueDescriptors = uniqueInBoth(c1, c2),
            // If all of the elements are the same tag, id and class, then we can
            // consider them roughly the same even if they have a different number of
            // children. This will reduce removing and re-adding similar elements.
            subsetsSame = c1Length === c2Length,
            origin, ret, c1Index, c2Index, c1Element, c2Element;

        if (subsetsSame) {

            c1.some(function(element, i) {
                var c1Desc = elementDescriptors(element),
                    c2Desc = elementDescriptors(c2[i]);
                if (c1Desc.length !== c2Desc.length) {
                    subsetsSame = false;
                    return true;
                }
                c1Desc.some(function(description, i) {
                    if (description !== c2Desc[i]) {
                        subsetsSame = false;
                        return true;
                    }
                });
                if (!subsetsSame) {
                    return true;
                }

            });
        }

        // fill the matches with distance values
        for (c1Index = 0; c1Index < c1Length; c1Index++) {
            c1Element = c1[c1Index];
            for (c2Index = 0; c2Index < c2Length; c2Index++) {
                c2Element = c2[c2Index];
                if (!marked1[c1Index] && !marked2[c2Index] && roughlyEqual(c1Element, c2Element, uniqueDescriptors, subsetsSame)) {
                    matches[c1Index + 1][c2Index + 1] = (matches[c1Index][c2Index] ? matches[c1Index][c2Index] + 1 : 1);
                    if (matches[c1Index + 1][c2Index + 1] >= lcsSize) {
                        lcsSize = matches[c1Index + 1][c2Index + 1];
                        index = [c1Index + 1, c2Index + 1];
                    }
                } else {
                    matches[c1Index + 1][c2Index + 1] = 0;
                }
            }
        }

        if (lcsSize === 0) {
            return false;
        }
        origin = [index[0] - lcsSize, index[1] - lcsSize];
        ret = new SubsetMapping(origin[0], origin[1]);
        ret.length = lcsSize;

        return ret;
    };

    /**
     * This should really be a predefined function in Array...
     */
    var makeArray = function(n, v) {
        return Array.apply(null, new Array(n)).map(function() {
            return v;
        });
    };

    /**
     * Generate arrays that indicate which node belongs to which subset,
     * or whether it's actually an orphan node, existing in only one
     * of the two trees, rather than somewhere in both.
     *
     * So if t1 = <img><canvas><br>, t2 = <canvas><br><img>.
     * The longest subset is "<canvas><br>" (length 2), so it will group 0.
     * The second longest is "<img>" (length 1), so it will be group 1.
     * gaps1 will therefore be [1,0,0] and gaps2 [0,0,1].
     *
     * If an element is not part of any group, it will stay being 'true', which
     * is the initial value. For example:
     * t1 = <img><p></p><br><canvas>, t2 = <b></b><br><canvas><img>
     *
     * The "<p></p>" and "<b></b>" do only show up in one of the two and will
     * therefore be marked by "true". The remaining parts are parts of the
     * groups 0 and 1:
     * gaps1 = [1, true, 0, 0], gaps2 = [true, 0, 0, 1]
     *
     */
    var getGapInformation = function(t1, t2, stable) {

        var gaps1 = t1.childNodes ? makeArray(t1.childNodes.length, true) : [],
            gaps2 = t2.childNodes ? makeArray(t2.childNodes.length, true) : [],
            group = 0,
            length = stable.length,
            i, j, endOld, endNew, subset;

        // give elements from the same subset the same group number
        for (i = 0; i < length; i++) {
            subset = stable[i];
            endOld = subset.oldValue + subset.length;
            endNew = subset.newValue + subset.length;
            for (j = subset.oldValue; j < endOld; j += 1) {
                gaps1[j] = group;
            }
            for (j = subset.newValue; j < endNew; j += 1) {
                gaps2[j] = group;
            }
            group += 1;
        }

        return {
            gaps1: gaps1,
            gaps2: gaps2
        };
    };

    /**
     * Find all matching subsets, based on immediate child differences only.
     */
    var markSubTrees = function(oldTree, newTree) {
        // note: the child lists are views, and so update as we update old/newTree
        var oldChildren = oldTree.childNodes ? oldTree.childNodes : [],
            newChildren = newTree.childNodes ? newTree.childNodes : [],
            marked1 = makeArray(oldChildren.length, false),
            marked2 = makeArray(newChildren.length, false),
            subsets = [],
            subset = true,
            returnIndex = function() {
                return arguments[1];
            },
            markBoth = function(i) {
                marked1[subset.oldValue + i] = true;
                marked2[subset.newValue + i] = true;
            },
            length, subsetArray, i;

        while (subset) {
            subset = findCommonSubsets(oldChildren, newChildren, marked1, marked2);
            if (subset) {
                subsets.push(subset);
                subsetArray = Array.apply(null, new Array(subset.length)).map(returnIndex);
                length = subsetArray.length;
                for (i = 0; i < length; i++) {
                    markBoth(subsetArray[i]);
                }
            }
        }
        return subsets;
    };


    function swap(obj, p1, p2) {
        var tmp = obj[p1];
        obj[p1] = obj[p2];
        obj[p2] = tmp;
    }


    var DiffTracker = function() {
        this.list = [];
    };

    DiffTracker.prototype = {
        list: false,
        add: function(diffs) {
            this.list.push.apply(this.list, diffs);
        },
        forEach: function(fn) {
            var length = this.list.length,
                i;
            for (i = 0; i < length; i++) {
                fn(this.list[i]);
            }
        }
    };

    var diffDOM = function(options) {

        var defaults = {
                debug: false,
                diffcap: 10, // Limit for how many diffs are accepting when debugging. Inactive when debug is false.
                maxDepth: false, // False or a numeral. If set to a numeral, limits the level of depth that the the diff mechanism looks for differences. If false, goes through the entire tree.
                valueDiffing: true, // Whether to take into consideration the values of forms that differ from auto assigned values (when a user fills out a form).
                // syntax: textDiff: function (node, currentValue, expectedValue, newValue)
                textDiff: function() {
                    arguments[0].data = arguments[3];
                    return;
                },
                // empty functions were benchmarked as running faster than both
                // f && f() and if (f) { f(); }
                preVirtualDiffApply: function() {},
                postVirtualDiffApply: function() {},
                preDiffApply: function() {},
                postDiffApply: function() {},
                filterOuterDiff: null,
                compress: false // Whether to work with compressed diffs
            },
            varNames, i, j;

        if (typeof options === "undefined") {
            options = {};
        }

        for (i in defaults) {
            if (typeof options[i] === "undefined") {
                this[i] = defaults[i];
            } else {
                this[i] = options[i];
            }
        }

        var varNames = {
            'addAttribute': 'addAttribute',
            'modifyAttribute': 'modifyAttribute',
            'removeAttribute': 'removeAttribute',
            'modifyTextElement': 'modifyTextElement',
            'relocateGroup': 'relocateGroup',
            'removeElement': 'removeElement',
            'addElement': 'addElement',
            'removeTextElement': 'removeTextElement',
            'addTextElement': 'addTextElement',
            'replaceElement': 'replaceElement',
            'modifyValue': 'modifyValue',
            'modifyChecked': 'modifyChecked',
            'modifySelected': 'modifySelected',
            'modifyComment': 'modifyComment',
            'action': 'action',
            'route': 'route',
            'oldValue': 'oldValue',
            'newValue': 'newValue',
            'element': 'element',
            'group': 'group',
            'from': 'from',
            'to': 'to',
            'name': 'name',
            'value': 'value',
            'data': 'data',
            'attributes': 'attributes',
            'nodeName': 'nodeName',
            'childNodes': 'childNodes',
            'checked': 'checked',
            'selected': 'selected'
        };

        if (this.compress) {
            j = 0;
            this._const = {};
            for (i in varNames) {
                this._const[i] = j;
                j++;
            }
        } else {
            this._const = varNames;
        }
    };

    diffDOM.Diff = Diff;

    diffDOM.prototype = {

        // ===== Create a diff =====

        diff: function(t1Node, t2Node) {

            var t1 = this.nodeToObj(t1Node),
                t2 = this.nodeToObj(t2Node);

            diffcount = 0;

            if (this.debug) {
                this.t1Orig = this.nodeToObj(t1Node);
                this.t2Orig = this.nodeToObj(t2Node);
            }

            this.tracker = new DiffTracker();
            return this.findDiffs(t1, t2);
        },
        findDiffs: function(t1, t2) {
            var diffs;
            do {
                if (this.debug) {
                    diffcount += 1;
                    if (diffcount > this.diffcap) {
                        window.diffError = [this.t1Orig, this.t2Orig];
                        throw new Error("surpassed diffcap:" + JSON.stringify(this.t1Orig) + " -> " + JSON.stringify(this.t2Orig));
                    }
                }
                diffs = this.findNextDiff(t1, t2, []);
                if (diffs.length === 0) {
                    // Last check if the elements really are the same now.
                    // If not, remove all info about being done and start over.
                    // Sometimes a node can be marked as done, but the creation of subsequent diffs means that it has to be changed anyway.
                    if (!isEqual(t1, t2)) {
                        removeDone(t1);
                        diffs = this.findNextDiff(t1, t2, []);
                    }
                }

                if (diffs.length > 0) {
                    this.tracker.add(diffs);
                    this.applyVirtual(t1, diffs);
                }
            } while (diffs.length > 0);
            return this.tracker.list;
        },
        findNextDiff: function(t1, t2, route) {
            var diffs, fdiffs;

            if (this.maxDepth && route.length > this.maxDepth) {
                return [];
            }
            // outer differences?
            if (!t1.outerDone) {
                diffs = this.findOuterDiff(t1, t2, route);
                if (this.filterOuterDiff) {
                    fdiffs = this.filterOuterDiff(t1, t2, diffs);
                    if (fdiffs) diffs = fdiffs;
                }
                if (diffs.length > 0) {
                    t1.outerDone = true;
                    return diffs;
                } else {
                    t1.outerDone = true;
                }
            }
            // inner differences?
            if (!t1.innerDone) {
                diffs = this.findInnerDiff(t1, t2, route);
                if (diffs.length > 0) {
                    return diffs;
                } else {
                    t1.innerDone = true;
                }
            }

            if (this.valueDiffing && !t1.valueDone) {
                // value differences?
                diffs = this.findValueDiff(t1, t2, route);

                if (diffs.length > 0) {
                    t1.valueDone = true;
                    return diffs;
                } else {
                    t1.valueDone = true;
                }
            }

            // no differences
            return [];
        },
        findOuterDiff: function(t1, t2, route) {
            var t = this;
            var diffs = [],
                attr,
                attr1, attr2, attrLength, pos, i;

            if (t1.nodeName !== t2.nodeName) {
                return [new Diff()
                    .setValue(t._const.action, t._const.replaceElement)
                    .setValue(t._const.oldValue, cloneObj(t1))
                    .setValue(t._const.newValue, cloneObj(t2))
                    .setValue(t._const.route, route)
                ];
            }

            if (t1.data !== t2.data) {
                // Comment or text node.
                if (t1.nodeName === '#text') {
                    return [new Diff()
                        .setValue(t._const.action, t._const.modifyTextElement)
                        .setValue(t._const.route, route)
                        .setValue(t._const.oldValue, t1.data)
                        .setValue(t._const.newValue, t2.data)
                    ];
                } else {
                    return [new Diff()
                        .setValue(t._const.action, t._const.modifyComment)
                        .setValue(t._const.route, route)
                        .setValue(t._const.oldValue, t1.data)
                        .setValue(t._const.newValue, t2.data)
                    ];
                }

            }


            attr1 = t1.attributes ? Object.keys(t1.attributes).sort() : [];
            attr2 = t2.attributes ? Object.keys(t2.attributes).sort() : [];

            attrLength = attr1.length;
            for (i = 0; i < attrLength; i++) {
                attr = attr1[i];
                pos = attr2.indexOf(attr);
                if (pos === -1) {
                    diffs.push(new Diff()
                        .setValue(t._const.action, t._const.removeAttribute)
                        .setValue(t._const.route, route)
                        .setValue(t._const.name, attr)
                        .setValue(t._const.value, t1.attributes[attr])
                    );
                } else {
                    attr2.splice(pos, 1);
                    if (t1.attributes[attr] !== t2.attributes[attr]) {
                        diffs.push(new Diff()
                            .setValue(t._const.action, t._const.modifyAttribute)
                            .setValue(t._const.route, route)
                            .setValue(t._const.name, attr)
                            .setValue(t._const.oldValue, t1.attributes[attr])
                            .setValue(t._const.newValue, t2.attributes[attr])
                        );
                    }
                }
            }

            attrLength = attr2.length;
            for (i = 0; i < attrLength; i++) {
                attr = attr2[i];
                diffs.push(new Diff()
                    .setValue(t._const.action, t._const.addAttribute)
                    .setValue(t._const.route, route)
                    .setValue(t._const.name, attr)
                    .setValue(t._const.value, t2.attributes[attr])
                );
            }

            return diffs;
        },
        nodeToObj: function(aNode) {
            var objNode = {},
                dobj = this,
                nodeArray, childNode, length, attribute, i;
            objNode.nodeName = aNode.nodeName;
            if (objNode.nodeName === '#text' || objNode.nodeName === '#comment') {
                objNode.data = aNode.data;
            } else {
                if (aNode.attributes && aNode.attributes.length > 0) {
                    objNode.attributes = {};
                    nodeArray = Array.prototype.slice.call(aNode.attributes);
                    length = nodeArray.length;
                    for (i = 0; i < length; i++) {
                        attribute = nodeArray[i];
                        objNode.attributes[attribute.name] = attribute.value;
                    }
                }
                if (objNode.nodeName === 'TEXTAREA') {
                    objNode.value = aNode.value;
                } else if (aNode.childNodes && aNode.childNodes.length > 0) {
                    objNode.childNodes = [];
                    nodeArray = Array.prototype.slice.call(aNode.childNodes);
                    length = nodeArray.length;
                    for (i = 0; i < length; i++) {
                        childNode = nodeArray[i];
                        objNode.childNodes.push(dobj.nodeToObj(childNode));
                    }
                }
                if (this.valueDiffing) {
                    if (aNode.checked !== undefined && aNode.type &&
                        ['radio','checkbox'].indexOf(aNode.type.toLowerCase()) !== -1
                    ) {
                        objNode.checked = aNode.checked;
                    } else if (aNode.value !== undefined) {
                        objNode.value = aNode.value;
                    }
                    if (aNode.selected !== undefined) {
                        objNode.selected = aNode.selected;
                    }
                }
            }
            return objNode;
        },
        objToNode: function(objNode, insideSvg) {
            var node, dobj = this,
                attribute, attributeArray, childNode, childNodeArray, length, i;
            if (objNode.nodeName === '#text') {
                node = document.createTextNode(objNode.data);

            } else if (objNode.nodeName === '#comment') {
                node = document.createComment(objNode.data);
            } else {
                if (objNode.nodeName === 'svg' || insideSvg) {
                    node = document.createElementNS('http://www.w3.org/2000/svg', objNode.nodeName);
                    insideSvg = true;
                } else {
                    node = document.createElement(objNode.nodeName);
                }
                if (objNode.attributes) {
                    attributeArray = Object.keys(objNode.attributes);
                    length = attributeArray.length;
                    for (i = 0; i < length; i++) {
                        attribute = attributeArray[i];
                        node.setAttribute(attribute, objNode.attributes[attribute]);
                    }
                }
                if (objNode.childNodes) {
                    childNodeArray = objNode.childNodes;
                    length = childNodeArray.length;
                    for (i = 0; i < length; i++) {
                        childNode = childNodeArray[i];
                        node.appendChild(dobj.objToNode(childNode, insideSvg));
                    }
                }
                if (this.valueDiffing) {
                    if (objNode.value) {
                        node.value = objNode.value;
                    }
                    if (objNode.checked) {
                        node.checked = objNode.checked;
                    }
                    if (objNode.selected) {
                        node.selected = objNode.selected;
                    }
                }
            }
            return node;
        },
        findInnerDiff: function(t1, t2, route) {
            var t = this;
            var subtrees = (t1.childNodes && t2.childNodes) ? markSubTrees(t1, t2) : [],
                t1ChildNodes = t1.childNodes ? t1.childNodes : [],
                t2ChildNodes = t2.childNodes ? t2.childNodes : [],
                childNodesLengthDifference, diffs = [],
                index = 0,
                last, e1, e2, i;

            if (subtrees.length > 0) {
                /* One or more groups have been identified among the childnodes of t1
                 * and t2.
                 */
                diffs = this.attemptGroupRelocation(t1, t2, subtrees, route);
                if (diffs.length > 0) {
                    return diffs;
                }
            }

            /* 0 or 1 groups of similar child nodes have been found
             * for t1 and t2. 1 If there is 1, it could be a sign that the
             * contents are the same. When the number of groups is below 2,
             * t1 and t2 are made to have the same length and each of the
             * pairs of child nodes are diffed.
             */


            last = Math.max(t1ChildNodes.length, t2ChildNodes.length);
            if (t1ChildNodes.length !== t2ChildNodes.length) {
                childNodesLengthDifference = true;
            }

            for (i = 0; i < last; i += 1) {
                e1 = t1ChildNodes[i];
                e2 = t2ChildNodes[i];

                if (childNodesLengthDifference) {
                    /* t1 and t2 have different amounts of childNodes. Add
                     * and remove as necessary to obtain the same length */
                    if (e1 && !e2) {
                        if (e1.nodeName === '#text') {
                            diffs.push(new Diff()
                                .setValue(t._const.action, t._const.removeTextElement)
                                .setValue(t._const.route, route.concat(index))
                                .setValue(t._const.value, e1.data)
                            );
                            index -= 1;
                        } else {
                            diffs.push(new Diff()
                                .setValue(t._const.action, t._const.removeElement)
                                .setValue(t._const.route, route.concat(index))
                                .setValue(t._const.element, cloneObj(e1))
                            );
                            index -= 1;
                        }

                    } else if (e2 && !e1) {
                        if (e2.nodeName === '#text') {
                            diffs.push(new Diff()
                                .setValue(t._const.action, t._const.addTextElement)
                                .setValue(t._const.route, route.concat(index))
                                .setValue(t._const.value, e2.data)
                            );
                        } else {
                            diffs.push(new Diff()
                                .setValue(t._const.action, t._const.addElement)
                                .setValue(t._const.route, route.concat(index))
                                .setValue(t._const.element, cloneObj(e2))
                            );
                        }
                    }
                }
                /* We are now guaranteed that childNodes e1 and e2 exist,
                 * and that they can be diffed.
                 */
                /* Diffs in child nodes should not affect the parent node,
                 * so we let these diffs be submitted together with other
                 * diffs.
                 */

                if (e1 && e2) {
                    diffs = diffs.concat(this.findNextDiff(e1, e2, route.concat(index)));
                }

                index += 1;

            }
            t1.innerDone = true;
            return diffs;

        },

        attemptGroupRelocation: function(t1, t2, subtrees, route) {
            /* Either t1.childNodes and t2.childNodes have the same length, or
             * there are at least two groups of similar elements can be found.
             * attempts are made at equalizing t1 with t2. First all initial
             * elements with no group affiliation (gaps=true) are removed (if
             * only in t1) or added (if only in t2). Then the creation of a group
             * relocation diff is attempted.
             */
            var t = this;
            var gapInformation = getGapInformation(t1, t2, subtrees),
                gaps1 = gapInformation.gaps1,
                gaps2 = gapInformation.gaps2,
                shortest = Math.min(gaps1.length, gaps2.length),
                destinationDifferent, toGroup,
                group, node, similarNode, testI, diffs = [],
                index1, index2, j;


            for (index2 = 0, index1 = 0; index2 < shortest; index1 += 1, index2 += 1) {
                if (gaps1[index2] === true) {
                    node = t1.childNodes[index1];
                    if (node.nodeName === '#text') {
                        if (t2.childNodes[index2].nodeName === '#text' && node.data !== t2.childNodes[index2].data) {
                            testI = index1;
                            while (t1.childNodes.length > testI + 1 && t1.childNodes[testI + 1].nodeName === '#text') {
                                testI += 1;
                                if (t2.childNodes[index2].data === t1.childNodes[testI].data) {
                                    similarNode = true;
                                    break;
                                }
                            }
                            if (!similarNode) {
                                diffs.push(new Diff()
                                    .setValue(t._const.action, t._const.modifyTextElement)
                                    .setValue(t._const.route, route.concat(index2))
                                    .setValue(t._const.oldValue, node.data)
                                    .setValue(t._const.newValue, t2.childNodes[index2].data)
                                );
                                return diffs;
                            }
                        }
                        diffs.push(new Diff()
                            .setValue(t._const.action, t._const.removeTextElement)
                            .setValue(t._const.route, route.concat(index2))
                            .setValue(t._const.value, node.data)
                        );
                        gaps1.splice(index2, 1);
                        shortest = Math.min(gaps1.length, gaps2.length);
                        index2 -= 1;
                    } else {
                        diffs.push(new Diff()
                            .setValue(t._const.action, t._const.removeElement)
                            .setValue(t._const.route, route.concat(index2))
                            .setValue(t._const.element, cloneObj(node))
                        );
                        gaps1.splice(index2, 1);
                        shortest = Math.min(gaps1.length, gaps2.length);
                        index2 -= 1;
                    }

                } else if (gaps2[index2] === true) {
                    node = t2.childNodes[index2];
                    if (node.nodeName === '#text') {
                        diffs.push(new Diff()
                            .setValue(t._const.action, t._const.addTextElement)
                            .setValue(t._const.route, route.concat(index2))
                            .setValue(t._const.value, node.data)
                        );
                        gaps1.splice(index2, 0, true);
                        shortest = Math.min(gaps1.length, gaps2.length);
                        index1 -= 1;
                    } else {
                        diffs.push(new Diff()
                            .setValue(t._const.action, t._const.addElement)
                            .setValue(t._const.route, route.concat(index2))
                            .setValue(t._const.element, cloneObj(node))
                        );
                        gaps1.splice(index2, 0, true);
                        shortest = Math.min(gaps1.length, gaps2.length);
                        index1 -= 1;
                    }

                } else if (gaps1[index2] !== gaps2[index2]) {
                    if (diffs.length > 0) {
                        return diffs;
                    }
                    // group relocation
                    group = subtrees[gaps1[index2]];
                    toGroup = Math.min(group.newValue, (t1.childNodes.length - group.length));
                    if (toGroup !== group.oldValue) {
                        // Check whether destination nodes are different than originating ones.
                        destinationDifferent = false;
                        for (j = 0; j < group.length; j += 1) {
                            if (!roughlyEqual(t1.childNodes[toGroup + j], t1.childNodes[group.oldValue + j], [], false, true)) {
                                destinationDifferent = true;
                            }
                        }
                        if (destinationDifferent) {
                            return [new Diff()
                                .setValue(t._const.action, t._const.relocateGroup)
                                .setValue('groupLength', group.length)
                                .setValue(t._const.from, group.oldValue)
                                .setValue(t._const.to, toGroup)
                                .setValue(t._const.route, route)
                            ];
                        }
                    }
                }
            }
            return diffs;
        },

        findValueDiff: function(t1, t2, route) {
            // Differences of value. Only useful if the value/selection/checked value
            // differs from what is represented in the DOM. For example in the case
            // of filled out forms, etc.
            var diffs = [];
            var t = this;

            if (t1.selected !== t2.selected) {
                diffs.push(new Diff()
                    .setValue(t._const.action, t._const.modifySelected)
                    .setValue(t._const.oldValue, t1.selected)
                    .setValue(t._const.newValue, t2.selected)
                    .setValue(t._const.route, route)
                );
            }

            if ((t1.value || t2.value) && t1.value !== t2.value && t1.nodeName !== 'OPTION') {
                diffs.push(new Diff()
                    .setValue(t._const.action, t._const.modifyValue)
                    .setValue(t._const.oldValue, t1.value)
                    .setValue(t._const.newValue, t2.value)
                    .setValue(t._const.route, route)
                );
            }
            if (t1.checked !== t2.checked) {
                diffs.push(new Diff()
                    .setValue(t._const.action, t._const.modifyChecked)
                    .setValue(t._const.oldValue, t1.checked)
                    .setValue(t._const.newValue, t2.checked)
                    .setValue(t._const.route, route)
                );
            }

            return diffs;
        },

        // ===== Apply a virtual diff =====

        applyVirtual: function(tree, diffs) {
            var dobj = this,
                length = diffs.length,
                diff, i;
            if (length === 0) {
                return true;
            }
            for (i = 0; i < length; i++) {
                diff = diffs[i];
                dobj.applyVirtualDiff(tree, diff);
            }
            return true;
        },
        getFromVirtualRoute: function(tree, route) {
            var node = tree,
                parentNode, nodeIndex;

            route = route.slice();
            while (route.length > 0) {
                if (!node.childNodes) {
                    return false;
                }
                nodeIndex = route.splice(0, 1)[0];
                parentNode = node;
                node = node.childNodes[nodeIndex];
            }
            return {
                node: node,
                parentNode: parentNode,
                nodeIndex: nodeIndex
            };
        },
        applyVirtualDiff: function(tree, diff) {
            var routeInfo = this.getFromVirtualRoute(tree, diff[this._const.route]),
                node = routeInfo.node,
                parentNode = routeInfo.parentNode,
                nodeIndex = routeInfo.nodeIndex,
                newNode, movedNode, nodeArray, route, length, c, i;

            var t = this;
            // pre-diff hook
            var info = {
                diff: diff,
                node: node
            };

            if (this.preVirtualDiffApply(info)) {
                return true;
            }

            switch (diff[this._const.action]) {
                case this._const.addAttribute:
                    if (!node.attributes) {
                        node.attributes = {};
                    }

                    node.attributes[diff[this._const.name]] = diff[this._const.value];

                    if (diff[this._const.name] === 'checked') {
                        node.checked = true;
                    } else if (diff[this._const.name] === 'selected') {
                        node.selected = true;
                    } else if (node.nodeName === 'INPUT' && diff[this._const.name] === 'value') {
                        node.value = diff[this._const.value];
                    }

                    break;
                case this._const.modifyAttribute:
                    node.attributes[diff[this._const.name]] = diff[this._const.newValue];
                    if (node.nodeName === 'INPUT' && diff[this._const.name] === 'value') {
                        node.value = diff[this._const.value];
                    }
                    break;
                case this._const.removeAttribute:

                    delete node.attributes[diff[this._const.name]];

                    if (Object.keys(node.attributes).length === 0) {
                        delete node.attributes;
                    }

                    if (diff[this._const.name] === 'checked') {
                        node.checked = false;
                    } else if (diff[this._const.name] === 'selected') {
                        delete node.selected;
                    } else if (node.nodeName === 'INPUT' && diff[this._const.name] === 'value') {
                        delete node.value;
                    }

                    break;
                case this._const.modifyTextElement:
                    node.data = diff[this._const.newValue];
                    break;
                case this._const.modifyValue:
                    node.value = diff[this._const.newValue];
                    break;
                case this._const.modifyComment:
                    node.data = diff[this._const.newValue];
                    break;
                case this._const.modifyChecked:
                    node.checked = diff[this._const.newValue];
                    break;
                case this._const.modifySelected:
                    node.selected = diff[this._const.newValue];
                    break;
                case this._const.replaceElement:
                    newNode = cloneObj(diff[this._const.newValue]);
                    newNode.outerDone = true;
                    newNode.innerDone = true;
                    newNode.valueDone = true;
                    parentNode.childNodes[nodeIndex] = newNode;
                    break;
                case this._const.relocateGroup:
                    nodeArray = node.childNodes.splice(diff[this._const.from], diff.groupLength).reverse();
                    length = nodeArray.length;
                    for (i = 0; i < length; i++) {
                        movedNode = nodeArray[i];
                        node.childNodes.splice(diff[t._const.to], 0, movedNode);
                    }
                    break;
                case this._const.removeElement:
                    parentNode.childNodes.splice(nodeIndex, 1);
                    break;
                case this._const.addElement:
                    route = diff[this._const.route].slice();
                    c = route.splice(route.length - 1, 1)[0];
                    node = this.getFromVirtualRoute(tree, route).node;
                    newNode = cloneObj(diff[this._const.element]);
                    newNode.outerDone = true;
                    newNode.innerDone = true;
                    newNode.valueDone = true;

                    if (!node.childNodes) {
                        node.childNodes = [];
                    }

                    if (c >= node.childNodes.length) {
                        node.childNodes.push(newNode);
                    } else {
                        node.childNodes.splice(c, 0, newNode);
                    }
                    break;
                case this._const.removeTextElement:
                    parentNode.childNodes.splice(nodeIndex, 1);
                    if (parentNode.nodeName === 'TEXTAREA') {
                        delete parentNode.value;
                    }
                    break;
                case this._const.addTextElement:
                    route = diff[this._const.route].slice();
                    c = route.splice(route.length - 1, 1)[0];
                    newNode = {};
                    newNode.nodeName = '#text';
                    newNode.data = diff[this._const.value];
                    node = this.getFromVirtualRoute(tree, route).node;
                    if (!node.childNodes) {
                        node.childNodes = [];
                    }

                    if (c >= node.childNodes.length) {
                        node.childNodes.push(newNode);
                    } else {
                        node.childNodes.splice(c, 0, newNode);
                    }
                    if (node.nodeName === 'TEXTAREA') {
                        node.value = diff[this._const.newValue];
                    }
                    break;
                default:
                    console.log('unknown action');
            }

            // capture newNode for the callback
            info.newNode = newNode;
            this.postVirtualDiffApply(info);

            return;
        },




        // ===== Apply a diff =====

        apply: function(tree, diffs) {
            var dobj = this,
                length = diffs.length,
                diff, i;

            if (length === 0) {
                return true;
            }
            for (i = 0; i < length; i++) {
                diff = diffs[i];
                if (!dobj.applyDiff(tree, diff)) {
                    return false;
                }
            }
            return true;
        },
        getFromRoute: function(tree, route) {
            route = route.slice();
            var c, node = tree;
            while (route.length > 0) {
                if (!node.childNodes) {
                    return false;
                }
                c = route.splice(0, 1)[0];
                node = node.childNodes[c];
            }
            return node;
        },
        applyDiff: function(tree, diff) {
            var node = this.getFromRoute(tree, diff[this._const.route]),
                newNode, reference, route, nodeArray, length, childNode, index, c;

            var t = this;
            // pre-diff hook
            var info = {
                diff: diff,
                node: node
            };

            if (this.preDiffApply(info)) {
                return true;
            }

            switch (diff[this._const.action]) {
                case this._const.addAttribute:
                    if (!node || !node.setAttribute) {
                        return false;
                    }
                    node.setAttribute(diff[this._const.name], diff[this._const.value]);
                    break;
                case this._const.modifyAttribute:
                    if (!node || !node.setAttribute) {
                        return false;
                    }
                    node.setAttribute(diff[this._const.name], diff[this._const.newValue]);
                    break;
                case this._const.removeAttribute:
                    if (!node || !node.removeAttribute) {
                        return false;
                    }
                    node.removeAttribute(diff[this._const.name]);
                    break;
                case this._const.modifyTextElement:
                    if (!node || node.nodeType !== 3) {
                        return false;
                    }
                    this.textDiff(node, node.data, diff[this._const.oldValue], diff[this._const.newValue]);
                    break;
                case this._const.modifyValue:
                    if (!node || typeof node.value === 'undefined') {
                        return false;
                    }
                    node.value = diff[this._const.newValue];
                    break;
                case this._const.modifyComment:
                    if (!node || typeof node.data === 'undefined') {
                        return false;
                    }
                    this.textDiff(node, node.data, diff[this._const.oldValue], diff[this._const.newValue]);
                    break;
                case this._const.modifyChecked:
                    if (!node || typeof node.checked === 'undefined') {
                        return false;
                    }
                    node.checked = diff[this._const.newValue];
                    break;
                case this._const.modifySelected:
                    if (!node || typeof node.selected === 'undefined') {
                        return false;
                    }
                    node.selected = diff[this._const.newValue];
                    break;
                case this._const.replaceElement:
                    node.parentNode.replaceChild(this.objToNode(diff[this._const.newValue], node.namespaceURI === 'http://www.w3.org/2000/svg'), node);
                    break;
                case this._const.relocateGroup:
                    nodeArray = Array.apply(null, new Array(diff.groupLength)).map(function() {
                        return node.removeChild(node.childNodes[diff[t._const.from]]);
                    });
                    length = nodeArray.length;
                    for (index = 0; index < length; index++) {
                        childNode = nodeArray[index];
                        if (index === 0) {
                            reference = node.childNodes[diff[t._const.to]];
                        }
                        node.insertBefore(childNode, reference);
                    }
                    break;
                case this._const.removeElement:
                    this.scrollToMiddle(node);
                    node.parentNode.removeChild(node);
                    break;
                case this._const.addElement:
                    route = diff[this._const.route].slice();
                    c = route.splice(route.length - 1, 1)[0];
                    newNode = this.objToNode(diff[this._const.element], node.namespaceURI === 'http://www.w3.org/2000/svg');
                    node = this.getFromRoute(tree, route);
                    node.insertBefore(newNode, node.childNodes[c]);
                    break;
                case this._const.removeTextElement:
                    if (!node || node.nodeType !== 3) {
                        return false;
                    }
                    node.parentNode.removeChild(node);
                    break;
                case this._const.addTextElement:
                    route = diff[this._const.route].slice();
                    c = route.splice(route.length - 1, 1)[0];
                    newNode = document.createTextNode(diff[this._const.value]);
                    node = this.getFromRoute(tree, route);
                    if (!node || !node.childNodes) {
                        return false;
                    }
                    node.insertBefore(newNode, node.childNodes[c]);
                    break;
                default:
                    console.log('unknown action');
            }

            // if a new node was created, we might be interested in it
            // post diff hook
            info.newNode = newNode;
            this.postDiffApply(info);

            // console.warn(diff[this._const.action]);
            if (diff[this._const.action] != this._const.removeElement) {
                if (newNode && newNode.scrollIntoView) {
                    this.scrollToMiddle(newNode);
                } else if (node && node.scrollIntoView) {
                    this.scrollToMiddle(node);
                } else if (node.parentNode && node.parentNode.scrollIntoView) {
                    this.scrollToMiddle(node.parentNode);
                }
            }

            return true;
        },

        scrollToMiddle: function(element) {
            var elementRect = element.getBoundingClientRect();
            var absoluteElementTop = elementRect.top + window.pageYOffset;
            var middle = absoluteElementTop - (window.innerHeight / 2);
            window.scrollTo(0, middle);
        },

        // ===== Undo a diff =====

        undo: function(tree, diffs) {
            var dobj = this, diff, length = diffs.length, i;
            diffs = diffs.slice();
            if (!length) {
                diffs = [diffs];
            }
            diffs.reverse();
            for (i = 0; i < length; i++) {
                diff = diffs[i];
                dobj.undoDiff(tree, diff);
            }
        },
        undoDiff: function(tree, diff) {

            switch (diff[this._const.action]) {
                case this._const.addAttribute:
                    diff[this._const.action] = this._const.removeAttribute;
                    this.applyDiff(tree, diff);
                    break;
                case this._const.modifyAttribute:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.removeAttribute:
                    diff[this._const.action] = this._const.addAttribute;
                    this.applyDiff(tree, diff);
                    break;
                case this._const.modifyTextElement:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.modifyValue:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.modifyComment:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.modifyChecked:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.modifySelected:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.replaceElement:
                    swap(diff, this._const.oldValue, this._const.newValue);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.relocateGroup:
                    swap(diff, this._const.from, this._const.to);
                    this.applyDiff(tree, diff);
                    break;
                case this._const.removeElement:
                    diff[this._const.action] = this._const.addElement;
                    this.applyDiff(tree, diff);
                    break;
                case this._const.addElement:
                    diff[this._const.action] = this._const.removeElement;
                    this.applyDiff(tree, diff);
                    break;
                case this._const.removeTextElement:
                    diff[this._const.action] = this._const.addTextElement;
                    this.applyDiff(tree, diff);
                    break;
                case this._const.addTextElement:
                    diff[this._const.action] = this._const.removeTextElement;
                    this.applyDiff(tree, diff);
                    break;
                default:
                    console.log('unknown action');
            }

        }
    };

    return diffDOM;
});
`
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
  <head>
<script type="text/javascript" src="qrc:///qtwebchannel/qwebchannel.js"></script>
<script>%s</script>
    <style>
     %s
    </style>
  </head>
  <body>
  %s
  <script>
  %s
  </script>
  </body>
</html>
`, morphdomjs, style, content, js)

	return html
}

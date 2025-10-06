// editor/message2.go
// ext_messages -> MiniTooltip(トースト) / Split(下部スプリット)
// - nvim typed API 未対応箇所は Nvim.Call でコアAPI直接呼び出し
// - Splitの1行目に「実行日時 + コマンド」を表示（空行は挟まない）
// - ヘッダは "GoneoMsgHeaderTime"(Commentリンク) / "GoneoMsgHeaderCmd"(Titleリンク)
// - ハイライトは nvim_buf_add_highlight（RPC）で適用
// - redraw中のLua実行を排除

package editor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/neovim/go-client/nvim"
)

// ─────────────────────────────────────────────────────────────
// Model

type MsgEvent string

const (
	MsgShow        MsgEvent = "msg_show"
	MsgClear       MsgEvent = "msg_clear"
	MsgHistoryShow MsgEvent = "msg_history_show"
	MsgShowmode    MsgEvent = "msg_showmode"
	MsgShowcmd     MsgEvent = "msg_showcmd"
	MsgRuler       MsgEvent = "msg_ruler"
)

type MsgChunk struct {
	AttrID int
	Text   string
	HLID   int
}

type UIMessage struct {
	Event       MsgEvent
	Kind        string
	Content     [][]MsgChunk
	ReplaceLast bool
	History     bool
	Append      bool
	MsgID       string
	Level       string
}

// ─────────────────────────────────────────────────────────────
// Views

type ViewName string

const (
	ViewMiniTooltip ViewName = "mini_tooltip"
	ViewSplit       ViewName = "split"
)

type View interface {
	Name() ViewName
	Show(m *UIMessage) (closeFn func(), err error)
}

// MiniTooltip（Qt トースト）
const tooltipMargin = 10

type MiniTooltipView struct {
	ws    *Workspace
	stack []*Tooltip
}

func (v *MiniTooltipView) Name() ViewName { return ViewMiniTooltip }

func (v *MiniTooltipView) Show(m *UIMessage) (func(), error) {
	// ウィジェット未準備時は split にフォールバック
	if v.ws == nil || v.ws.screen == nil || v.ws.screen.widget == nil || v.ws.screen.widget.Width() == 0 {
		alt := &SplitView{ws: v.ws}
		return alt.Show(m)
	}

	parent := v.ws.screen.widget
	tip := NewTooltip(parent, core.Qt__Widget)
	tip.ConnectPaintEvent(func(ev *gui.QPaintEvent) { tip.paint(ev) })

	tip.s = v.ws.screen
	tip.setPadding(20, 10)
	tip.setRadius(14, 10)
	tip.maximumWidth = int(float64(parent.Width()) * 0.85)
	tip.pathWidth = 2
	tip.setBgColor(v.ws.background)
	tip.setFont(v.ws.font)
	tip.fallbackfonts = v.ws.screen.fallbackfonts

	// Content → Tooltip
	for rowIdx, row := range m.Content {
		if rowIdx > 0 {
			attr := 0
			if len(row) > 0 {
				attr = row[0].AttrID
			}
			hl := v.ws.screen.hlAttrDef[attr]
			if hl == nil {
				hl = v.ws.screen.hlAttrDef[0]
			}
			if hl != nil {
				tip.updateText(hl, "\n", 0, tip.font.qfont)
			}
		}
		for _, ch := range row {
			hl := v.ws.screen.hlAttrDef[ch.AttrID]
			if hl == nil {
				hl = v.ws.screen.hlAttrDef[0]
			}
			if hl == nil {
				continue
			}
			qf := resolveFontFallback(v.ws.screen.font, v.ws.screen.fallbackfonts, ch.Text).qfont
			tip.updateText(hl, ch.Text, float64(v.ws.screen.font.letterSpace), qf)
		}
	}

	tip.update()
	tip.show()
	tip.SetGraphicsEffect(util.DropShadow(-1, 16, 130, 180))

	// 右上にスタック配置
	stackedHeight := 0
	for _, t := range v.stack {
		if t.IsVisible() {
			stackedHeight += t.Height() + tooltipMargin
		}
	}
	tip.Move2(parent.Width()-tip.Width()-tooltipMargin, stackedHeight)
	v.stack = append(v.stack, tip)
	if v.ws != nil && v.ws.messages != nil {
		v.ws.messages.msgs = append(v.ws.messages.msgs, tip)
	}
	return func() { tip.Hide() }, nil
}

// ─────────────────────────────────────────────────────────────
// span（バイトオフセット & hl_id保持）

type span struct {
	AttrID   int
	HLID     int
	ColStart int // byte offset (0-based)
	ColEnd   int // byte offset (exclusive)
}

// flattenWithSpansBytes: 1行ごとテキスト + 各チャンクの [byte cols]
func flattenWithSpansBytes(blocks [][]MsgChunk) (lines []string, lineSpans [][]span) {
	lines = []string{}
	lineSpans = [][]span{}
	for _, row := range blocks {
		var b strings.Builder
		colBytes := 0
		rowSpans := []span{}
		for _, ch := range row {
			start := colBytes
			part := ch.Text
			b.WriteString(part)
			l := len([]byte(part))
			if !utf8.ValidString(part) {
				runes := []rune(part)
				l = len([]byte(string(runes)))
			}
			colBytes += l
			rowSpans = append(rowSpans, span{
				AttrID:   ch.AttrID,
				HLID:     ch.HLID,
				ColStart: start,
				ColEnd:   colBytes,
			})
		}
		lines = append(lines, b.String())
		lineSpans = append(lineSpans, rowSpans)
	}
	if len(lines) == 0 {
		lines = []string{""}
		lineSpans = [][]span{{}}
	}
	return
}

// ─────────────────────────────────────────────────────────────
// Split（ビュー再利用 + ハイライト一括適用 + ヘッダ行）

// ヘッダ用“擬似”属性ID（負数を特別扱い）
const (
	attrHeaderTime = -1
	attrHeaderCmd  = -2
)

type SplitView struct {
	ws      *Workspace
	win     nvim.Window
	buf     nvim.Buffer
	nsID    int
	defined map[int]bool // attr_id -> defined (>=0 の通常属性のみ)
	// ヘッダHLは別フラグ
	headerHLDefined bool
}

func (v *SplitView) Name() ViewName { return ViewSplit }

// Window/Buffer/Namespace の確保（typed未提供は Call で代替）
func (v *SplitView) ensureWinBuf() error {
	n := v.ws.nvim

	// 既存ウィンドウ再利用
	if v.win != 0 {
		var ok bool
		_ = n.Call("nvim_win_is_valid", &ok, v.win)
		if ok {
			// バッファ有効性
			if v.buf != 0 {
				var bok bool
				_ = n.Call("nvim_buf_is_valid", &bok, v.buf)
				if bok {
					// そのまま利用
					return nil
				}
			}
			// 新規バッファに差し替え
			buf, err := n.CreateBuffer(false, true)
			if err != nil {
				return err
			}
			v.buf = buf
			_ = n.Call("nvim_win_set_buf", nil, v.win, v.buf)
			return nil
		}
		// 破棄
		v.win = 0
		v.buf = 0
	}

	// 新規 split
	c := make(chan error, 1)
	var win nvim.Window
	var buf nvim.Buffer
	go func() {
		err := n.Command("botright new")
		if err != nil {
			c <- err
		}
		win, err = n.CurrentWindow()
		if err != nil {
			c <- err
		}
		buf, err = n.CreateBuffer(false, true)
		if err != nil {
			c <- err
		}
		_ = n.Call("nvim_win_set_buf", nil, win, buf)

		// window/buf options
		_ = n.SetWindowOption(win, "number", false)
		_ = n.SetWindowOption(win, "relativenumber", false)
		_ = n.SetWindowOption(win, "wrap", true)
		_ = n.SetWindowOption(win, "cursorline", false)
		_ = n.SetWindowOption(win, "signcolumn", "no")
		_ = n.SetWindowOption(win, "winhighlight", "")

		_ = n.SetBufferOption(buf, "buftype", "nofile")
		_ = n.SetBufferOption(buf, "bufhidden", "wipe")
		_ = n.SetBufferOption(buf, "swapfile", false)

		opts := map[string]bool{"noremap": true, "silent": true, "nowait": true}
		_ = n.SetBufferKeyMap(buf, "n", "q", "<cmd>close<CR>", opts)

		c <- err
	}()

	var err error
	select {
	case err = <-c:
	case <-time.After(time.Duration(100) * time.Millisecond):
	}

	if err != nil {
		return err
	}

	v.win = win
	v.buf = buf
	return nil
}

func (v *SplitView) ensureNS() {
	if v.nsID != 0 {
		return
	}

	c := make(chan bool, 1)
	go func() {
		_ = v.ws.nvim.Call("nvim_create_namespace", &v.nsID, "goneo_messages_split_marks")
		c <- true
	}()

	select {
	case <-c:
	case <-time.After(time.Duration(100) * time.Millisecond):
	}
}

func (v *SplitView) defineHeaderHLOnce() {
	if v.headerHLDefined {
		return
	}

	c := make(chan bool, 1)
	go func() {
		_ = v.ws.nvim.Call("nvim_set_hl", nil, 0, "GoneoMsgHeaderTime", map[string]interface{}{"link": "Comment"})
		_ = v.ws.nvim.Call("nvim_set_hl", nil, 0, "GoneoMsgHeaderCmd", map[string]interface{}{"link": "Title"})

		c <- true
	}()

	select {
	case <-c:
	case <-time.After(time.Duration(100) * time.Millisecond):
	}

	v.headerHLDefined = true
}

func (v *SplitView) Show(m *UIMessage) (func(), error) {
	if err := v.ensureWinBuf(); err != nil {
		return nil, err
	}
	v.ensureNS()
	if v.defined == nil {
		v.defined = make(map[int]bool)
	}
	v.defineHeaderHLOnce()

	n := v.ws.nvim

	// 行 & スパン（UTF-8バイトオフセット）
	linesFlat, spans := flattenWithSpansBytes(m.Content)

	// 先頭に「実行日時 + コマンド」を差し込む（空行は挟まない）
	if hdrLine, hdrSpans := v.consumeHeaderDecorated(); hdrLine != "" {
		linesFlat = append([]string{hdrLine}, linesFlat...)
		spans = append([][]span{hdrSpans}, spans...)
	}

	// 高さ調整
	height := clamp(len(linesFlat), 3, 20)

	c := make(chan bool, 1)
	go func() {
		_ = n.Command(fmt.Sprintf("resize %d", height))

		// 行更新
		_ = n.SetCurrentBuffer(v.buf)
		_ = n.SetBufferOption(v.buf, "modifiable", true)
		_ = n.SetBufferLines(v.buf, 0, -1, true, toByteLines(linesFlat))

		// 既存ハイライトを一括クリア
		_ = n.Call("nvim_buf_clear_namespace", nil, v.buf, v.nsID, 0, -1)

		c <- true
	}()

	select {
	case <-c:
	case <-time.After(time.Duration(100) * time.Millisecond):
	}

	// 未定義 attr_id をまとめて定義（>=0のみ）
	for _, spansOnLine := range spans {
		for _, sp := range spansOnLine {
			if sp.AttrID < 0 {
				continue // ヘッダは別リンク名を使用
			}
			if v.defined[sp.AttrID] {
				continue
			}
			hl := v.ws.screen.hlAttrDef[sp.AttrID]
			if hl == nil {
				hl = v.ws.screen.hlAttrDef[0]
			}
			if hl == nil {
				continue
			}
			name := fmt.Sprintf("GoneoMsgA_%d", sp.AttrID)
			opts := map[string]interface{}{}
			if fg, ok := rgbaToNvimInt(hl.fg()); ok && fg != 0 {
				opts["fg"] = fg
			}
			if bg, ok := rgbaToNvimInt(hl.bg()); ok && bg != 0 {
				opts["bg"] = bg
			}
			if spc, ok := rgbaToNvimInt(hl.special); ok && spc != 0 {
				opts["sp"] = spc
			}
			if hl.bold {
				opts["bold"] = true
			}
			if hl.italic {
				opts["italic"] = true
			}
			if hl.underline {
				opts["underline"] = true
			}
			if hl.undercurl {
				opts["undercurl"] = true
			}
			if hl.underdouble {
				opts["underdouble"] = true
			}
			if hl.underdotted {
				opts["underdotted"] = true
			}
			if hl.underdashed {
				opts["underdashed"] = true
			}
			if hl.strikethrough {
				opts["strikethrough"] = true
			}
			if hl.reverse {
				opts["reverse"] = true
			}
			_ = n.Call("nvim_set_hl", nil, 0, name, opts)
			v.defined[sp.AttrID] = true
		}
	}

	// すべてのハイライトを一括適用
	for li, spansOnLine := range spans {
		if len(spansOnLine) == 0 {
			continue
		}
		lineLenBytes := len([]byte(linesFlat[li]))
		for _, spn := range spansOnLine {
			s := spn.ColStart
			e := spn.ColEnd
			if s < 0 {
				s = 0
			}
			if e < 0 {
				e = 0
			}
			if s > lineLenBytes {
				s = lineLenBytes
			}
			if e > lineLenBytes {
				e = lineLenBytes
			}
			if e <= s {
				continue
			}

			group := ""
			if spn.AttrID >= 0 {
				group = fmt.Sprintf("GoneoMsgA_%d", spn.AttrID)
			} else {
				// ヘッダ用
				if spn.AttrID == attrHeaderTime {
					group = "GoneoMsgHeaderTime"
				} else {
					group = "GoneoMsgHeaderCmd"
				}
			}

			c := make(chan bool, 1)
			go func() {
				_ = n.Call("nvim_buf_add_highlight", nil, v.buf, v.nsID, group, li, s, e)
				c <- true
			}()

			select {
			case <-c:
			case <-time.After(time.Duration(100) * time.Millisecond):
			}
		}
	}

	result := make(chan bool, 1)
	go func() {
		_ = n.SetBufferOption(v.buf, "modifiable", false)
		result <- true
	}()

	select {
	case <-result:
	case <-time.After(time.Duration(100) * time.Millisecond):
	}

	closeFn := func() {
		_ = n.Call("nvim_win_close", nil, v.win, true)
		v.win = 0
		v.buf = 0
	}
	return closeFn, nil
}

// ヘッダを 1 回だけ消費して返す（行文字列 & spans）
func (v *SplitView) consumeHeaderDecorated() (string, []span) {
	ms := v.ws.messages
	if ms == nil {
		return "", nil
	}
	name := strings.TrimSpace(ms.pendingCmdName)
	if name == "" || ms.pendingCmdStart.IsZero() {
		return "", nil
	}
	// 実行“日時”を表示（ローカルタイム）
	tstamp := ms.pendingCmdStart.Local().Format("2006-01-02 15:04:05")

	// 使ったらクリア
	ms.pendingCmdName = ""
	ms.pendingCmdStart = time.Time{}

	// 例: "2025-10-05 16:22:46  :digraphs"
	line := fmt.Sprintf("%s  %s", tstamp, name)

	// spans: [time] [spaces] [cmd]
	timeBytes := len([]byte(tstamp))
	spaceBytes := len([]byte("  "))
	return line, []span{
		{AttrID: attrHeaderTime, HLID: 0, ColStart: 0, ColEnd: timeBytes},
		{AttrID: attrHeaderCmd, HLID: 0, ColStart: timeBytes + spaceBytes, ColEnd: len([]byte(line))},
	}
}

// ─────────────────────────────────────────────────────────────
// Router

type RouteFilter struct {
	Event     MsgEvent
	KindIn    []string
	MinHeight int
	MinWidth  int
	Find      *regexp.Regexp
	Append    *bool
}

type Route struct {
	Filter RouteFilter
	View   ViewName
}

type Router struct {
	Routes []Route
}

func (r *Router) Resolve(m *UIMessage, measure func(*UIMessage) (h, w int)) ViewName {
	h, w := measure(m)
	for _, rt := range r.Routes {
		f := rt.Filter
		if f.Event != "" && m.Event != f.Event {
			continue
		}
		if len(f.KindIn) > 0 && !contains(f.KindIn, m.Kind) {
			continue
		}
		if f.MinHeight > 0 && h < f.MinHeight {
			continue
		}
		if f.MinWidth > 0 && w < f.MinWidth {
			continue
		}
		if f.Append != nil && m.Append != *f.Append {
			continue
		}
		if f.Find != nil && !f.Find.MatchString(joinText(m)) {
			continue
		}
		return rt.View
	}
	return ViewMiniTooltip
}

// ─────────────────────────────────────────────────────────────
// MsgManager

type MsgManager struct {
	ws        *Workspace
	router    *Router
	views     map[ViewName]View
	active    map[string]func()
	lastStack []string
}

func NewMsgManager(ws *Workspace, r *Router, mini *MiniTooltipView, split *SplitView) *MsgManager {
	return &MsgManager{
		ws:     ws,
		router: r,
		views: map[ViewName]View{
			ViewMiniTooltip: mini,
			ViewSplit:       split,
		},
		active:    map[string]func(){},
		lastStack: []string{},
	}
}

func (mgr *MsgManager) Add(m *UIMessage) error {
	if m.ReplaceLast && len(mgr.lastStack) > 0 {
		lastID := mgr.lastStack[len(mgr.lastStack)-1]
		if closer, ok := mgr.active[lastID]; ok {
			closer()
			delete(mgr.active, lastID)
			mgr.lastStack = mgr.lastStack[:len(mgr.lastStack)-1]
		}
	}
	if m.MsgID != "" {
		if closer, ok := mgr.active[m.MsgID]; ok {
			closer()
			delete(mgr.active, m.MsgID)
		}
	}

	viewName := mgr.router.Resolve(m, mgr.measure)
	view := mgr.views[viewName]

	closeFn, err := view.Show(m)
	if err != nil {
		return err
	}

	key := m.MsgID
	if key == "" {
		key = "anon:" + strconv.Itoa(len(mgr.active)+1)
	}
	mgr.active[key] = closeFn
	mgr.lastStack = append(mgr.lastStack, key)
	return nil
}

func (mgr *MsgManager) ClearAll() {
	for k, c := range mgr.active {
		c()
		delete(mgr.active, k)
	}
	mgr.lastStack = nil
}

func (mgr *MsgManager) measure(m *UIMessage) (height, width int) {
	h, w := 0, 0
	for _, row := range m.Content {
		line := ""
		for _, ch := range row {
			line += ch.Text
		}
		parts := strings.Split(line, "\n")
		for _, p := range parts {
			h++
			if l := len([]rune(p)); l > w {
				w = l
			}
		}
	}
	if h == 0 {
		h = 1
	}
	return h, w
}

// ─────────────────────────────────────────────────────────────
// Messages (ext_messages entrypoints + cmdline 連携)

type Messages struct {
	ws      *Workspace
	msgs    []*Tooltip
	manager *MsgManager

	// cmdline 連携（次の split 出力に 1 回だけ載せる）
	cmdlinePreview  string    // 直近に見えている cmdline 文字列
	pendingCmdName  string    // Enter確定直後のコマンド
	pendingCmdStart time.Time // 実行開始時刻（cmdline_hide時）
}

func initMessages() *Messages { return &Messages{} }

func (m *Messages) ensureManager() bool {
	if m.manager != nil {
		return true
	}
	if m.ws == nil {
		return false
	}
	mini := &MiniTooltipView{ws: m.ws}
	split := &SplitView{ws: m.ws}
	router := &Router{
		Routes: []Route{
			{Filter: RouteFilter{Event: MsgHistoryShow}, View: ViewSplit},                                                                       // 履歴は split
			{Filter: RouteFilter{Event: MsgShow, MinHeight: 10}, View: ViewSplit},                                                               // 長文は split
			{Filter: RouteFilter{Event: MsgShow, KindIn: []string{"emsg", "echoerr", "lua_error", "rpc_error"}, MinWidth: 50}, View: ViewSplit}, // 横長エラーも split
		},
	}
	m.manager = NewMsgManager(m.ws, router, mini, split)
	return true
}

func (m *Messages) AttachDefaultManager() { m.ensureManager() }
func (m *Messages) AttachManager(router *Router) {
	mini := &MiniTooltipView{ws: m.ws}
	split := &SplitView{ws: m.ws}
	m.manager = NewMsgManager(m.ws, router, mini, split)
}

func (m *Messages) msgClear() {
	if !m.ensureManager() {
		return
	}
	for _, t := range m.msgs {
		if t != nil {
			t.Hide()
		}
	}
	m.msgs = nil
	m.manager.ClearAll()
}

// ---- ui-cmdline 連携（既存UI側から呼ぶ） ----

func (m *Messages) CmdlineOnShow(content string, firstc string) {
	var b strings.Builder
	b.WriteString(firstc)
	b.WriteString(content)
	m.cmdlinePreview = b.String()
}

func (m *Messages) CmdlineOnHide() {
	if strings.TrimSpace(m.cmdlinePreview) == "" {
		m.pendingCmdName = ""
		m.pendingCmdStart = time.Time{}
		return
	}
	m.pendingCmdName = m.cmdlinePreview
	m.pendingCmdStart = time.Now()
}

// ext_messages: msg_show
func (m *Messages) msgShow(args []interface{}, _bulk bool) {
	if !m.ensureManager() || len(args) == 0 {
		return
	}
	// 単数
	if _, isString := args[0].(string); isString {
		if um := decodeMsgShow(args); um != nil {
			_ = m.manager.Add(um)
		}
		return
	}
	// 複数
	for _, item := range args {
		tuple, ok := item.([]interface{})
		if !ok || len(tuple) == 0 {
			continue
		}
		if um := decodeMsgShowOne(tuple); um != nil {
			_ = m.manager.Add(um)
		}
	}
}

// ext_messages: msg_history_show（安全版）
func (m *Messages) msgHistoryShow(args []interface{}) {
	if !m.ensureManager() {
		return
	}
	if len(args) == 0 {
		return
	}
	// 仕様: ["msg_history_show", entries, prev_cmd]
	var entries []interface{}
	if a0, ok := args[0].([]interface{}); ok {
		entries = a0
	} else {
		entries = args
	}

	all := &UIMessage{Event: MsgHistoryShow, Kind: "history"}

	for _, e := range entries {
		t, ok := e.([]interface{})
		if !ok || len(t) == 0 {
			continue
		}
		// [kind, content, append] or [content]
		var contentRaw []interface{}
		if len(t) >= 2 {
			if cr, ok := t[1].([]interface{}); ok {
				contentRaw = cr
			}
		} else if cr, ok := t[0].([]interface{}); ok {
			contentRaw = cr
		}
		if contentRaw == nil {
			continue
		}

		row := []MsgChunk{}
		for _, c := range contentRaw {
			cc, ok := c.([]interface{})
			if !ok || len(cc) < 2 {
				continue
			}
			attr := util.ReflectToInt(cc[0])
			text, _ := cc[1].(string)
			hl := 0
			if len(cc) >= 3 {
				hl = util.ReflectToInt(cc[2])
			}
			row = append(row, MsgChunk{AttrID: attr, Text: text, HLID: hl})
		}
		all.Content = append(all.Content, row)
	}

	_ = m.manager.Add(all)
}

// ─────────────────────────────────────────────────────────────
// Decode & helpers

func decodeMsgShowOne(tuple []interface{}) *UIMessage {
	if len(tuple) < 2 {
		return nil
	}
	kind, _ := tuple[0].(string)
	contentRaw, _ := tuple[1].([]interface{})

	msg := &UIMessage{Event: MsgShow, Kind: kind}

	if len(tuple) > 2 {
		if b, ok := tuple[2].(bool); ok {
			msg.ReplaceLast = b
		}
	}
	if len(tuple) > 3 {
		if b, ok := tuple[3].(bool); ok {
			msg.History = b
		}
	}
	if len(tuple) > 4 {
		if b, ok := tuple[4].(bool); ok {
			msg.Append = b
		}
	}
	if len(tuple) > 5 {
		switch v := tuple[5].(type) {
		case int:
			msg.MsgID = fmt.Sprintf("%d", v)
		case int64:
			msg.MsgID = fmt.Sprintf("%d", v)
		case string:
			msg.MsgID = v
		}
	}

	rows := [][]MsgChunk{{}}
	for _, c := range contentRaw {
		t, _ := c.([]interface{})
		attr := util.ReflectToInt(t[0])
		text, _ := t[1].(string)

		hl := 0
		if len(t) >= 3 {
			hl = util.ReflectToInt(t[2])
		}

		parts := strings.Split(text, "\n")
		for i, part := range parts {
			rows[len(rows)-1] = append(rows[len(rows)-1], MsgChunk{AttrID: attr, Text: part, HLID: hl})
			if i < len(parts)-1 {
				rows = append(rows, []MsgChunk{})
			}
		}
	}
	msg.Content = rows

	switch msg.Kind {
	case "emsg", "echoerr", "lua_error", "rpc_error":
		msg.Level = "error"
	case "wmsg":
		msg.Level = "warn"
	default:
		msg.Level = "info"
	}
	return msg
}

func decodeMsgShow(args []interface{}) *UIMessage { return decodeMsgShowOne(args) }

func toByteLines(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// ─────────────────────────────────────────────────────────────
// utils

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func joinText(m *UIMessage) string {
	var b strings.Builder
	for _, row := range m.Content {
		for _, ch := range row {
			b.WriteString(ch.Text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// Neovim の nvim_set_hl 用: 0xRRGGBB の整数を返す
func rgbaToNvimInt(c *RGBA) (int, bool) {
	if c == nil {
		return 0, false
	}
	return int(c.R)<<16 | int(c.G)<<8 | int(c.B), true
}

// 互換：フォント更新・emmit ダミー
func (m *Messages) updateFont() {}
func (t *Tooltip) emmit()       { t.show() }

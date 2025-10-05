// editor/message2.go
// ext_messages -> MiniTooltip(トースト) / Split(下部スプリット)
// ヘッダと本文の間を空けない & ヘッダ行に見やすいハイライトを適用（noice風）

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
			if ch.Text == "" {
				continue
			}
			start := colBytes
			b.WriteString(ch.Text)
			l := len([]byte(ch.Text))
			if !utf8.ValidString(ch.Text) {
				runes := []rune(ch.Text)
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
// Split（ビュー再利用 + set_hl/ハイライト一括適用 + ヘッダ行）

type SplitView struct {
	ws      *Workspace
	win     nvim.Window
	buf     nvim.Buffer
	nsID    int
	defined map[int]bool // attr_id -> defined
}

func (v *SplitView) Name() ViewName { return ViewSplit }

func (v *SplitView) ensureWinBuf() error {
	n := v.ws.nvim

	// 再利用
	if v.win != 0 {
		var ok bool
		_ = n.ExecLua(`return vim.api.nvim_win_is_valid(...)`, []interface{}{v.win}, &ok)
		if ok {
			if v.buf != 0 {
				var bok bool
				_ = n.ExecLua(`return vim.api.nvim_buf_is_valid(...)`, []interface{}{v.buf}, &bok)
				if bok {
					return nil
				}
			}
			buf, err := n.CreateBuffer(false, true)
			if err != nil {
				return err
			}
			v.buf = buf
			_ = n.ExecLua(`pcall(vim.api.nvim_win_set_buf, ..., ...)`, []interface{}{v.win, v.buf}, nil)
			return nil
		}
		v.win = 0
		v.buf = 0
	}

	// 新規 split
	if err := n.Command("botright new"); err != nil {
		return err
	}
	win, err := n.CurrentWindow()
	if err != nil {
		return err
	}
	buf, err := n.CreateBuffer(false, true)
	if err != nil {
		return err
	}
	_ = n.ExecLua(`pcall(vim.api.nvim_win_set_buf, ..., ...)`, []interface{}{win, buf}, nil)

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

	v.win = win
	v.buf = buf
	return nil
}

func (v *SplitView) ensureNS() {
	if v.nsID != 0 {
		return
	}
	const nsName = "goneo_messages_split_marks"
	_ = v.ws.nvim.ExecLua(`return vim.api.nvim_create_namespace(...)`, []interface{}{nsName}, &v.nsID)
}

// ヘッダ用 highlight を（1度だけでも）定義
func (v *SplitView) ensureHeaderHL() {
	lua := `
    pcall(vim.api.nvim_set_hl, 0, "GoneoMsgHdrTime", { link = "Title" })
    pcall(vim.api.nvim_set_hl, 0, "GoneoMsgHdrMeta", { link = "Comment" })
    pcall(vim.api.nvim_set_hl, 0, "GoneoMsgHdrIcon", { link = "Special" })
    pcall(vim.api.nvim_set_hl, 0, "GoneoMsgHdrCmd",  { link = "Identifier" })
  `
	_ = v.ws.nvim.ExecLua(lua, nil, nil)
}

// HTMLタグを削る（cmdline由来の文字列対策）
var reHTMLTag = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	if s == "" {
		return s
	}
	// エンティティはここでは気にしない（palette 側は見た目用）
	return reHTMLTag.ReplaceAllString(s, "")
}

func (v *SplitView) Show(m *UIMessage) (func(), error) {
	if err := v.ensureWinBuf(); err != nil {
		return nil, err
	}
	v.ensureNS()
	v.ensureHeaderHL()
	if v.defined == nil {
		v.defined = make(map[int]bool)
	}

	n := v.ws.nvim

	// 行 & スパン（UTF-8バイトオフセット）
	linesFlat, spans := flattenWithSpansBytes(m.Content)

	// ==== ヘッダ生成（行を空けない） ====
	if hdr, segs := v.buildHeaderLine(m); hdr != "" {
		// 既存先頭が空行なら落として、間を詰める
		if len(linesFlat) > 0 && strings.TrimSpace(strings.TrimRight(linesFlat[0], "\r")) == "" {
			linesFlat = linesFlat[1:]
			if len(spans) > 0 {
				spans = spans[1:]
			}
		}
		// 先頭にヘッダ行を追加（spans 側は空配列を1つ差し込む）
		linesFlat = append([]string{hdr}, linesFlat...)
		spans = append([][]span{{}}, spans...)

		// 後で add_highlight するため、ヘッダセグメントを保持しておく
		// セグメントはこの関数内のローカル扱いでOK。適用は buffer 書き込み後すぐ。
		// 下で callsHeader に積んで一括実行する。
		var callsHeader []string
		for _, sg := range segs {
			callsHeader = append(callsHeader,
				fmt.Sprintf(`pcall(vim.api.nvim_buf_add_highlight,%d,%d,%q,%d,%d,%d)`, int(v.buf), v.nsID, sg.Group, 0, sg.ColStart, sg.ColEnd))
		}

		// ==== 行更新 & ヘッダHL適用 ====
		height := clamp(len(linesFlat), 3, 20)
		_ = n.Command(fmt.Sprintf("resize %d", height))
		_ = n.SetCurrentBuffer(v.buf)
		_ = n.SetBufferOption(v.buf, "modifiable", true)
		_ = n.SetBufferLines(v.buf, 0, -1, true, toByteLines(linesFlat))
		_ = n.ExecLua(fmt.Sprintf(`pcall(vim.api.nvim_buf_clear_namespace, %d, %d, 0, -1)`, int(v.buf), v.nsID), nil, nil)
		if len(callsHeader) > 0 {
			_ = n.ExecLua(strings.Join(callsHeader, "\n"), nil, nil)
		}
	} else {
		// ヘッダなし通常パス：まず行更新
		height := clamp(len(linesFlat), 3, 20)
		_ = n.Command(fmt.Sprintf("resize %d", height))
		_ = n.SetCurrentBuffer(v.buf)
		_ = n.SetBufferOption(v.buf, "modifiable", true)
		_ = n.SetBufferLines(v.buf, 0, -1, true, toByteLines(linesFlat))
		_ = n.ExecLua(fmt.Sprintf(`pcall(vim.api.nvim_buf_clear_namespace, %d, %d, 0, -1)`, int(v.buf), v.nsID), nil, nil)
	}

	// ==== 本文ハイライト（spans）適用 ====
	// 未定義 attr_id をまとめて定義
	var needDefs []string
	for _, spansOnLine := range spans {
		for _, sp := range spansOnLine {
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
			var opts []string
			if fg, ok := rgbaToNvimInt(hl.fg()); ok && fg != 0 {
				opts = append(opts, fmt.Sprintf("opts.fg=%d", fg))
			}
			if bg, ok := rgbaToNvimInt(hl.bg()); ok && bg != 0 {
				opts = append(opts, fmt.Sprintf("opts.bg=%d", bg))
			}
			if spc, ok := rgbaToNvimInt(hl.special); ok && spc != 0 {
				opts = append(opts, fmt.Sprintf("opts.sp=%d", spc))
			}
			if hl.bold {
				opts = append(opts, "opts.bold=true")
			}
			if hl.italic {
				opts = append(opts, "opts.italic=true")
			}
			if hl.underline {
				opts = append(opts, "opts.underline=true")
			}
			if hl.undercurl {
				opts = append(opts, "opts.undercurl=true")
			}
			if hl.underdouble {
				opts = append(opts, "opts.underdouble=true")
			}
			if hl.underdotted {
				opts = append(opts, "opts.underdotted=true")
			}
			if hl.underdashed {
				opts = append(opts, "opts.underdashed=true")
			}
			if hl.strikethrough {
				opts = append(opts, "opts.strikethrough=true")
			}
			if hl.reverse {
				opts = append(opts, "opts.reverse=true")
			}
			needDefs = append(needDefs, fmt.Sprintf(`
        do local name=%q; local opts={}; %s; pcall(vim.api.nvim_set_hl,0,name,opts); end
      `, name, strings.Join(opts, " ")))
			v.defined[sp.AttrID] = true
		}
	}
	if len(needDefs) > 0 {
		_ = n.ExecLua(strings.Join(needDefs, "\n"), nil, nil)
	}

	// すべてのハイライトを一括適用
	var calls []string
	for li, spansOnLine := range spans {
		if len(spansOnLine) == 0 {
			continue
		}
		lineLenBytes := len([]byte(linesFlat[li]))
		for _, spn := range spansOnLine {
			s := spn.ColStart
			if s < 0 {
				s = 0
			}
			if s > lineLenBytes {
				s = lineLenBytes
			}
			e := spn.ColEnd
			if e < 0 {
				e = 0
			}
			if e > lineLenBytes {
				e = lineLenBytes
			}
			if e <= s {
				continue
			}
			name := fmt.Sprintf("GoneoMsgA_%d", spn.AttrID)
			calls = append(calls, fmt.Sprintf(`pcall(vim.api.nvim_buf_add_highlight,%d,%d,%q,%d,%d,%d)`, int(v.buf), v.nsID, name, li, s, e))
		}
	}
	if len(calls) > 0 {
		_ = n.ExecLua(strings.Join(calls, "\n"), nil, nil)
	}

	_ = n.SetBufferOption(v.buf, "modifiable", false)

	closeFn := func() {
		_ = n.ExecLua(`pcall(vim.api.nvim_win_close, ..., true)`, []interface{}{v.win}, nil)
		v.win = 0
		v.buf = 0
	}
	return closeFn, nil
}

// ヘッダ行を生成し、適用するハイライト範囲も返す
type headerSeg struct {
	Group    string
	ColStart int
	ColEnd   int
}

func (v *SplitView) buildHeaderLine(m *UIMessage) (string, []headerSeg) {
	ms := v.ws.messages
	if ms == nil {
		return "", nil
	}
	name := strings.TrimSpace(ms.pendingCmdName)
	if name == "" || ms.pendingCmdTime.IsZero() {
		return "", nil
	}
	// 使ったらクリア（次回には出さない）
	defer func() {
		ms.pendingCmdName = ""
		ms.pendingCmdTime = time.Time{}
	}()

	// プレーン化（paletteのHTML混入対策）
	cmd := stripHTMLTags(name)

	// kind 表示（例: msg_show.list_cmd / msg_history_show）
	src := "msg_show." + m.Kind
	if m.Event == MsgHistoryShow {
		src = "msg_history_show"
	}

	// "HH:MM:SS  msg_show.kind   cmd"
	clock := ms.pendingCmdTime.Local().Format("15:04:05")
	icon := ""
	sep := "  "

	// 結合と各パートの byte offset 計算
	var b strings.Builder
	b.WriteString(clock)
	b.WriteString(sep)
	clockEnd := len([]byte(b.String()))
	b.WriteString(src)
	b.WriteString(sep)
	srcEnd := len([]byte(b.String()))
	b.WriteString(icon)
	b.WriteString(" ")
	iconEnd := len([]byte(b.String()))
	b.WriteString(cmd)
	line := b.String()

	segs := []headerSeg{
		{Group: "GoneoMsgHdrTime", ColStart: 0, ColEnd: clockEnd - len([]byte(sep))},      // clock 部分
		{Group: "GoneoMsgHdrMeta", ColStart: clockEnd, ColEnd: srcEnd - len([]byte(sep))}, // src 部分
		{Group: "GoneoMsgHdrIcon", ColStart: srcEnd, ColEnd: iconEnd - 1},                 // アイコンのみ
		{Group: "GoneoMsgHdrCmd", ColStart: iconEnd, ColEnd: len([]byte(line))},           // コマンド名
	}
	return line, segs
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
	cmdlinePreview string    // 最新に見えている cmdline テキスト（:h ui-cmdline content から組み立て）
	pendingCmdName string    // Enter 確定直後に確定されたコマンド名（次の split 出力に載せる）
	pendingCmdTime time.Time // 実行した“日時”（cmdline_hide を受けた時刻）
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

// ---- ui-cmdline 連携 ----

// CmdlineOnShow : ["cmdline_show", content, pos, firstc, prompt, indent, level]
func (m *Messages) CmdlineOnShow(content string, firstc string) {
	var b strings.Builder
	b.WriteString(firstc)
	b.WriteString(content)
	m.cmdlinePreview = b.String()
}

// CmdlineOnHide : ["cmdline_hide", level]（Enter 確定直後に飛ぶ）
// ここで“実行した日時”を記録する
func (m *Messages) CmdlineOnHide() {
	if strings.TrimSpace(m.cmdlinePreview) == "" {
		m.pendingCmdName = ""
		m.pendingCmdTime = time.Time{}
		return
	}
	m.pendingCmdName = m.cmdlinePreview
	m.pendingCmdTime = time.Now()
}

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

func (m *Messages) msgHistoryShow(args []interface{}) {
	if !m.ensureManager() {
		return
	}
	if len(args) == 0 {
		return
	}

	// 仕様: ["msg_history_show", entries, prev_cmd]
	// 呼び出し側によっては entries だけが渡されることもあるので両方に対応
	var entries []interface{}
	if a0, ok := args[0].([]interface{}); ok {
		entries = a0
	} else {
		// フォールバック: 既存呼び出しが entries をそのまま渡してくる場合
		entries = args
	}

	all := &UIMessage{Event: MsgHistoryShow, Kind: "history"}

	for _, e := range entries {
		t, ok := e.([]interface{})
		if !ok || len(t) == 0 {
			continue
		}

		// entry は通常 [kind, content, append]。
		// まれに [content] だけのケースにも耐える。
		var contentRaw []interface{}
		if len(t) >= 2 {
			// t[0]=kind (string), t[1]=content ([]tuple)
			if cr, ok := t[1].([]interface{}); ok {
				contentRaw = cr
			}
		} else {
			// フォールバック: t[0] がそのまま content の場合
			if cr, ok := t[0].([]interface{}); ok {
				contentRaw = cr
			}
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

		// 行として追加。エントリ間は空行で区切る（既存挙動を踏襲）
		all.Content = append(all.Content, row)
		all.Content = append(all.Content, []MsgChunk{{Text: ""}})
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

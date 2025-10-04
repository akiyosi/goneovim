// editor/message2.go
// Single-file, buildable MsgManager/Router/View pipeline for ext_messages in goneovim,
// integrated with your Tooltip implementation for “short” messages.
// 2025-10-04 修正版 (lazy attach & robust fallback + Println デバッグ + paint接続):
//  - ensureManager(): ws があればその場で AttachDefaultManager して自己初期化
//  - msg_show 単数/複数（バッチ）両対応
//  - MiniTooltipView: hl フォールバック、親 QWidget 未準備時は PopupFloatView へフォールバック
//  - Manager → MsgManager
//  - Messages.msgs 互換フィールド
//  - clamp 実装、Tooltip.emmit() 互換ラッパ
//  - NewTooltip(parent, core.Qt__Widget)
//  - dprintf() による Println ログ（[msg2] プレフィックス）
//  - ★ Tooltip.ConnectPaintEvent を追加（描画されない問題の修正）

package editor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui" // ★ 追加：paint コールバック用
	"github.com/neovim/go-client/nvim"
)

//
// ──────────────────────────────────────────────────────────────────────────────
// DEBUG
//

var debugMsg = true

func dprintf(format string, a ...interface{}) {
	if debugMsg {
		fmt.Println("[msg2] " + fmt.Sprintf(format, a...))
	}
}

//
// ──────────────────────────────────────────────────────────────────────────────
// Model
//

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

//
// ──────────────────────────────────────────────────────────────────────────────
// Views
//

type ViewName string

const (
	ViewMiniTooltip ViewName = "mini_tooltip"
	ViewPopupFloat  ViewName = "popup_float"
)

type View interface {
	Name() ViewName
	Show(m *UIMessage) (closeFn func(), err error)
}

const tooltipMargin = 10

type MiniTooltipView struct {
	ws    *Workspace
	stack []*Tooltip
}

func (v *MiniTooltipView) Name() ViewName { return ViewMiniTooltip }

func (v *MiniTooltipView) Show(m *UIMessage) (func(), error) {
	dprintf("MiniTooltipView.Show: enter kind=%q lines=%d", m.Kind, len(m.Content))

	// widget 未準備時はフロートへフォールバック
	if v.ws == nil || v.ws.screen == nil || v.ws.screen.widget == nil || v.ws.screen.widget.Width() == 0 {
		dprintf("MiniTooltipView.Show: FALLBACK -> PopupFloat (widget nil or width=0)")
		alt := &PopupFloatView{ws: v.ws}
		return alt.Show(m)
	}

	parent := v.ws.screen.widget
	dprintf("MiniTooltipView.Show: parent widget dims: w=%d h=%d", parent.Width(), parent.Height())

	// Tooltip 生成
	tip := NewTooltip(parent, core.Qt__Widget)
	// ★ paint イベント接続（ログ付きでラップ）
	tip.ConnectPaintEvent(func(ev *gui.QPaintEvent) {
		dprintf("MiniTooltipView.Show: tip.paint invoked")
		tip.paint(ev)
	})

	tip.s = v.ws.screen
	tip.setPadding(10, 5)
	tip.setRadius(5, 5)
	tip.maximumWidth = int(float64(parent.Width()) * 0.85)
	tip.pathWidth = 2
	tip.setBgColor(v.ws.background)
	tip.setFont(v.ws.font)
	tip.fallbackfonts = v.ws.screen.fallbackfonts

	// Content → Tooltip
	totalRunes := 0
	for rowIdx, row := range m.Content {
		if rowIdx > 0 {
			attr := 0
			if len(row) > 0 {
				attr = row[0].AttrID
			}
			hl, ok := v.ws.screen.hlAttrDef[attr]
			if !ok {
				if def, ok2 := v.ws.screen.hlAttrDef[0]; ok2 {
					hl, ok = def, true
					dprintf("MiniTooltipView.Show: hl fallback for newline: attr=%d -> 0", attr)
				} else {
					dprintf("MiniTooltipView.Show: hl missing for newline and no fallback 0; skip newline")
				}
			}
			if ok {
				tip.updateText(hl, "\n", 0, tip.font.qfont)
			}
		}
		for _, ch := range row {
			hl, ok := v.ws.screen.hlAttrDef[ch.AttrID]
			if !ok {
				if def, ok2 := v.ws.screen.hlAttrDef[0]; ok2 {
					hl, ok = def, true
					dprintf("MiniTooltipView.Show: hl fallback: attr=%d -> 0", ch.AttrID)
				}
			}
			if !ok {
				dprintf("MiniTooltipView.Show: hl not found and no fallback, skip chunk text=%q", ch.Text)
				continue
			}
			runes := []rune(ch.Text)
			totalRunes += len(runes)
			qf := resolveFontFallback(v.ws.screen.font, v.ws.screen.fallbackfonts, ch.Text).qfont
			tip.updateText(hl, ch.Text, float64(v.ws.screen.font.letterSpace), qf)
		}
	}
	dprintf("MiniTooltipView.Show: content aggregated, total runes=%d, flatChunks=%d lines=%d",
		totalRunes, len(flattenContentToLines(m.Content)), len(m.Content))

	// 表示
	tip.update()
	dprintf("MiniTooltipView.Show: after tip.update size=%dx%d textWidth=%.1f msgHeight=%.1f maxW=%d",
		tip.Width(), tip.Height(), tip.textWidth, tip.msgHeight, tip.maximumWidth)

	tip.show()
	dprintf("MiniTooltipView.Show: after tip.show visible=%v", tip.IsVisible())
	tip.SetGraphicsEffect(util.DropShadow(-1, 16, 130, 180))

	// 右上スタック配置
	stackedHeight := 0
	for _, t := range v.stack {
		if t.IsVisible() {
			stackedHeight += t.Height() + tooltipMargin
		}
	}
	targetX := parent.Width() - tip.Width() - tooltipMargin
	targetY := stackedHeight
	dprintf("MiniTooltipView.Show: Move2 to x=%d y=%d (stackedHeight=%d stackLen=%d)", targetX, targetY, stackedHeight, len(v.stack))
	tip.Move2(targetX, targetY)

	// スタック追加 + 互換配列更新
	v.stack = append(v.stack, tip)
	if v.ws != nil && v.ws.messages != nil {
		v.ws.messages.msgs = append(v.ws.messages.msgs, tip)
	}
	dprintf("MiniTooltipView.Show: stack now len=%d, msgs len=%d", len(v.stack), len(v.ws.messages.msgs))

	closeFn := func() {
		dprintf("MiniTooltipView.Show: closeFn called -> Hide()")
		tip.Hide()
	}
	return closeFn, nil
}

type PopupFloatView struct{ ws *Workspace }

func (v *PopupFloatView) Name() ViewName { return ViewPopupFloat }

func (v *PopupFloatView) Show(m *UIMessage) (func(), error) {
	dprintf("PopupFloatView.Show: enter kind=%q lines=%d", m.Kind, len(m.Content))
	n := v.ws.nvim

	buf, err := n.CreateBuffer(false, true)
	if err != nil {
		dprintf("PopupFloatView.Show: CreateBuffer error: %v", err)
		return nil, err
	}

	lines := flattenContentToLines(m.Content)
	data := toByteLines(lines)
	height := clamp(len(lines), 3, 20)
	dprintf("PopupFloatView.Show: open win height=%d cols=%d rows=%d", height, v.ws.cols, v.ws.rows)

	win, err := n.OpenWindow(buf, true, &nvim.WindowConfig{
		Relative:  "editor",
		Anchor:    "NW",
		Width:     v.ws.cols,
		Height:    height,
		Row:       float64(v.ws.rows - height),
		Col:       0,
		Style:     "minimal",
		ZIndex:    60,
		Focusable: true,
	})
	if err != nil {
		dprintf("PopupFloatView.Show: OpenWindow error: %v", err)
		return nil, err
	}

	if err := n.SetBufferLines(buf, 0, -1, true, data); err != nil {
		dprintf("PopupFloatView.Show: SetBufferLines error: %v", err)
		return nil, err
	}
	dprintf("PopupFloatView.Show: buffer lines set: %d", len(data))

	_ = n.SetBufferOption(buf, "buftype", "nofile")
	_ = n.SetBufferOption(buf, "bufhidden", "wipe")
	_ = n.SetBufferOption(buf, "swapfile", false)
	_ = n.SetBufferOption(buf, "modifiable", false)
	_ = n.SetWindowOption(win, "wrap", true)
	_ = n.SetWindowOption(win, "cursorline", false)

	opts := map[string]bool{"noremap": true, "silent": true, "nowait": true}
	if err := n.SetBufferKeyMap(buf, "n", "q", "<cmd>lua vim.api.nvim_win_close(0, true)<CR>", opts); err != nil {
		dprintf("PopupFloatView.Show: map q error: %v", err)
	}

	closeFn := func() {
		dprintf("PopupFloatView.Show: closeFn -> nvim_win_close(%d)", win)
		_ = n.ExecLua(`vim.api.nvim_win_close(..., true)`, []interface{}{win}, nil)
	}
	return closeFn, nil
}

//
// ──────────────────────────────────────────────────────────────────────────────
// Router
//

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

//
// ──────────────────────────────────────────────────────────────────────────────
// MsgManager
//

type MsgManager struct {
	ws        *Workspace
	router    *Router
	views     map[ViewName]View
	active    map[string]func()
	lastStack []string
}

func NewMsgManager(ws *Workspace, r *Router, mini *MiniTooltipView, pop *PopupFloatView) *MsgManager {
	return &MsgManager{
		ws:     ws,
		router: r,
		views: map[ViewName]View{
			ViewMiniTooltip: mini,
			ViewPopupFloat:  pop,
		},
		active:    map[string]func(){},
		lastStack: []string{},
	}
}

func (mgr *MsgManager) Add(m *UIMessage) error {
	dprintf("MsgManager.Add: kind=%q id=%q replace_last=%v lines=%d", m.Kind, m.MsgID, m.ReplaceLast, len(m.Content))

	if m.ReplaceLast && len(mgr.lastStack) > 0 {
		lastID := mgr.lastStack[len(mgr.lastStack)-1]
		dprintf("MsgManager.Add: replace_last -> closing lastID=%q", lastID)
		if closer, ok := mgr.active[lastID]; ok {
			closer()
			delete(mgr.active, lastID)
			mgr.lastStack = mgr.lastStack[:len(mgr.lastStack)-1]
		}
	}
	if m.MsgID != "" {
		if closer, ok := mgr.active[m.MsgID]; ok {
			dprintf("MsgManager.Add: same msg_id -> closing existing id=%q", m.MsgID)
			closer()
			delete(mgr.active, m.MsgID)
		}
	}

	viewName := mgr.router.Resolve(m, mgr.measure)
	dprintf("MsgManager.Add: resolved view=%s", viewName)
	view := mgr.views[viewName]

	closeFn, err := view.Show(m)
	if err != nil {
		dprintf("MsgManager.Add: view.Show error: %v", err)
		return err
	}

	key := m.MsgID
	if key == "" {
		key = "anon:" + strconv.Itoa(len(mgr.active)+1)
	}
	mgr.active[key] = closeFn
	mgr.lastStack = append(mgr.lastStack, key)
	dprintf("MsgManager.Add: active+1 key=%q active_len=%d lastStack_len=%d", key, len(mgr.active), len(mgr.lastStack))
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
			if l := visibleWidth(p); l > w {
				w = l
			}
		}
	}
	if h == 0 {
		h = 1
	}
	return h, w
}

//
// ──────────────────────────────────────────────────────────────────────────────
// Messages (ext_messages entrypoints)
//

type Messages struct {
	ws      *Workspace
	msgs    []*Tooltip
	manager *MsgManager
}

func initMessages() *Messages { return &Messages{} }

func (m *Messages) ensureManager() bool {
	dprintf("ensureManager: manager=%v ws=%p", m.manager != nil, m.ws)
	if m.manager != nil {
		return true
	}
	if m.ws == nil {
		dprintf("ensureManager: ws=nil -> cannot attach yet")
		return false
	}
	mini := &MiniTooltipView{ws: m.ws}
	pop := &PopupFloatView{ws: m.ws}
	router := &Router{
		Routes: []Route{
			{Filter: RouteFilter{Event: MsgHistoryShow}, View: ViewPopupFloat},
			{Filter: RouteFilter{Event: MsgShow, MinHeight: 10}, View: ViewPopupFloat},
			{Filter: RouteFilter{Event: MsgShow, KindIn: []string{"emsg", "echoerr", "lua_error", "rpc_error"}, MinWidth: 50}, View: ViewPopupFloat},
		},
	}
	m.manager = NewMsgManager(m.ws, router, mini, pop)
	dprintf("ensureManager: attached default manager")
	return true
}

func (m *Messages) AttachDefaultManager() { m.ensureManager() }

func (m *Messages) AttachManager(router *Router) {
	mini := &MiniTooltipView{ws: m.ws}
	pop := &PopupFloatView{ws: m.ws}
	m.manager = NewMsgManager(m.ws, router, mini, pop)
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

func (m *Messages) msgShow(args []interface{}, _bulk bool) {
	dprintf("msgShow: enter len(args)=%d", len(args))
	if !m.ensureManager() || len(args) == 0 {
		dprintf("msgShow: early return (manager=%v len(args)=%d)", m.manager != nil, len(args))
		return
	}

	// 単数形
	if _, isString := args[0].(string); isString {
		dprintf("msgShow: single tuple path type(args[0])=%T", args[0])
		if um := decodeMsgShow(args); um != nil {
			dprintf("msgShow: decoded single: kind=%q lines=%d", um.Kind, len(um.Content))
			_ = m.manager.Add(um)
		} else {
			dprintf("msgShow: decode returned nil (single)")
		}
		return
	}

	// 複数形
	dprintf("msgShow: batch path items=%d", len(args))
	for i, item := range args {
		tuple, ok := item.([]interface{})
		if !ok || len(tuple) == 0 {
			dprintf("msgShow: batch[%d] skip (type=%T len=%d)", i, item, len(tuple))
			continue
		}
		if um := decodeMsgShowOne(tuple); um != nil {
			dprintf("msgShow: batch[%d] decoded: kind=%q lines=%d", i, um.Kind, len(um.Content))
			_ = m.manager.Add(um)
		} else {
			dprintf("msgShow: batch[%d] decode returned nil", i)
		}
	}
}

func (m *Messages) msgHistoryShow(entries []interface{}) {
	if !m.ensureManager() {
		return
	}
	all := &UIMessage{Event: MsgHistoryShow, Kind: "history"}
	for _, e := range entries {
		t := e.([]interface{})
		contentRaw, _ := t[1].([]interface{})
		row := []MsgChunk{}
		for _, c := range contentRaw {
			cc := c.([]interface{})
			attr := util.ReflectToInt(cc[0])
			text, _ := cc[1].(string)
			row = append(row, MsgChunk{AttrID: attr, Text: text})
		}
		all.Content = append(all.Content, row)
		all.Content = append(all.Content, []MsgChunk{{Text: ""}})
	}
	_ = m.manager.Add(all)
}

//
// ──────────────────────────────────────────────────────────────────────────────
// Decode & helpers
//

func decodeMsgShowOne(tuple []interface{}) *UIMessage {
	if len(tuple) < 2 {
		dprintf("decodeMsgShowOne: len(tuple)=%d < 2", len(tuple))
		return nil
	}
	kind, _ := tuple[0].(string)
	contentRaw, _ := tuple[1].([]interface{})
	dprintf("decodeMsgShowOne: kind=%q rawLen=%d", kind, len(contentRaw))

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
	for idx, c := range contentRaw {
		t, ok := c.([]interface{})
		if !ok || len(t) == 0 {
			dprintf("decodeMsgShowOne: content[%d] skip (type=%T len=%d)", idx, c, len(t))
			continue
		}
		attr := util.ReflectToInt(t[0])
		text, _ := t[1].(string)
		hl := 0
		if len(t) >= 3 {
			if id, ok := t[2].(int); ok {
				hl = id
			}
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
	dprintf("decodeMsgShowOne: built rows=%d", len(rows))

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

func flattenContentToLines(blocks [][]MsgChunk) []string {
	var out []string
	for _, row := range blocks {
		var b strings.Builder
		for _, ch := range row {
			b.WriteString(ch.Text)
		}
		out = append(out, b.String())
	}
	var flat []string
	for _, l := range out {
		flat = append(flat, strings.Split(l, "\n")...)
	}
	if len(flat) == 0 {
		flat = []string{""}
	}
	return flat
}

func toByteLines(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func visibleWidth(s string) int { return len([]rune(s)) }

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

//
// ──────────────────────────────────────────────────────────────────────────────
// Compatibility shims
//

func (m *Messages) updateFont() {}

func (t *Tooltip) emmit() { t.show() }

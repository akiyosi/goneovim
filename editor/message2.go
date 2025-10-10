package editor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"unsafe"

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
// グローバル: アプリ終了検知

var (
	aboutOnce  sync.Once
	uiQuitting atomic.Bool
)

func ensureAboutQuitHook() {
	aboutOnce.Do(func() {
		if app := core.QCoreApplication_Instance(); app != nil {
			app.ConnectAboutToQuit(func() { uiQuitting.Store(true) })
		}
	})
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

// ─────────────────────────────────────────────────────────────
// GUI キュー投げ（引数なしメソッド専用）。QTimer は使わない。

func invokeQueued(obj core.QObject_ITF, method string) {
	if obj == nil || uiQuitting.Load() {
		return
	}
	var null unsafe.Pointer
	core.QMetaObject_InvokeMethod(
		obj, method, core.Qt__QueuedConnection,
		core.NewQGenericReturnArgument("", null), // return
		core.NewQGenericArgument("", null), core.NewQGenericArgument("", null),
		core.NewQGenericArgument("", null), core.NewQGenericArgument("", null),
		core.NewQGenericArgument("", null), core.NewQGenericArgument("", null),
		core.NewQGenericArgument("", null), core.NewQGenericArgument("", null),
		core.NewQGenericArgument("", null), core.NewQGenericArgument("", null),
	)
}

// ─────────────────────────────────────────────────────────────
// MiniTooltip（Qt トースト）— TTL は Go の time.Timer, GUI 操作は invokeQueued

const tooltipMargin = 10
const defaultMiniTTLMs = 1800 // 1.8s

type MiniTooltipView struct {
	ws         *Workspace
	stack      []*Tooltip
	autoHideMs int

	mu      sync.Mutex
	closing map[*Tooltip]bool // まもなく閉じる=高さ計算から除外
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
	tip.setPadding(10, 5)
	tip.setRadius(7, 5)
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

	// 右上にスタック配置（closing中や不可視は高さ計算から除外＆ついでに圧縮）
	v.mu.Lock()
	if v.closing == nil {
		v.closing = make(map[*Tooltip]bool)
	}
	stackedHeight := 0
	compact := v.stack[:0]
	for _, t := range v.stack {
		if t == nil {
			continue
		}
		if v.closing[t] || !t.IsVisible() {
			// 閉じ待ち/不可視はここでドロップ（圧縮）
			continue
		}
		stackedHeight += t.Height() + tooltipMargin
		compact = append(compact, t)
	}
	v.stack = compact
	v.mu.Unlock()

	tip.Move2(parent.Width()-tip.Width()-tooltipMargin, stackedHeight)

	// スタックに追加
	v.mu.Lock()
	v.stack = append(v.stack, tip)
	v.mu.Unlock()

	if v.ws != nil && v.ws.messages != nil {
		v.ws.messages.msgs = append(v.ws.messages.msgs, tip)
	}

	// TTL（Goタイマー）。GUI は invokeQueued で破棄。
	ttl := v.autoHideMs
	if ttl <= 0 {
		ttl = defaultMiniTTLMs
	}
	go func(tipRef *Tooltip, ms int) {
		timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
		defer timer.Stop()
		<-timer.C
		if uiQuitting.Load() {
			return
		}

		// 閉じ始めをマーク（以降の配置から除外）
		v.mu.Lock()
		if v.closing == nil {
			v.closing = make(map[*Tooltip]bool)
		}
		v.closing[tipRef] = true
		v.mu.Unlock()

		invokeQueued(tipRef, "hide")
		invokeQueued(tipRef, "close")
		invokeQueued(tipRef, "deleteLater")
	}(tip, ttl)

	// 破棄時に stack と closing からも確実に除去
	tip.ConnectDestroyed(func(*core.QObject) {
		v.mu.Lock()
		for i, it := range v.stack {
			if it == tip {
				v.stack = append(v.stack[:i], v.stack[i+1:]...)
				break
			}
		}
		delete(v.closing, tip)
		v.mu.Unlock()

		// Messages 側の参照も後片付け
		if v.ws != nil && v.ws.messages != nil {
			msgs := v.ws.messages
			for i, it := range msgs.msgs {
				if it == tip {
					msgs.msgs = append(msgs.msgs[:i], msgs.msgs[i+1:]...)
					break
				}
			}
		}
	})

	// 手動クローズ（GUIスレッドへQueued）
	return func() {
		if uiQuitting.Load() {
			return
		}
		// 手動クローズでも即 closing マーク
		v.mu.Lock()
		if v.closing == nil {
			v.closing = make(map[*Tooltip]bool)
		}
		v.closing[tip] = true
		v.mu.Unlock()

		invokeQueued(tip, "hide")
		invokeQueued(tip, "close")
		invokeQueued(tip, "deleteLater")
	}, nil
}

func (v *MiniTooltipView) restack() {
	if v.ws == nil || v.ws.screen == nil || v.ws.screen.widget == nil {
		return
	}
	parent := v.ws.screen.widget
	v.mu.Lock()
	defer v.mu.Unlock()
	stackedHeight := 0
	for _, t := range v.stack {
		if t != nil && t.IsVisible() && !v.closing[t] {
			t.Move2(parent.Width()-t.Width()-tooltipMargin, stackedHeight)
			stackedHeight += t.Height() + tooltipMargin
		}
	}
}

// ─────────────────────────────────────────────────────────────
// span（バイトオフセット & hl_id保持）

type span struct {
	AttrID   int
	HLID     int
	ColStart int // byte offset (0-based)
	ColEnd   int // byte offset (exclusive)
}

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
// Split（再利用 + Mutex 保護 + HL 一括適用 + ヘッダ行）

const (
	attrHeaderTime = -1
	attrHeaderCmd  = -2
)

type SplitView struct {
	ws *Workspace

	mu   sync.Mutex
	win  nvim.Window
	buf  nvim.Buffer
	nsID int

	defined         map[int]bool // attr_id -> defined
	headerHLDefined bool
}

func (v *SplitView) Name() ViewName { return ViewSplit }

// 先頭ヘッダを 1 回だけ消費（実行日時 + コマンド名）。空行は挟まない。
func (v *SplitView) consumeHeaderDecorated() (string, []span) {
	ms := v.ws.messages
	if ms == nil {
		return "", nil
	}
	name := strings.TrimSpace(ms.pendingCmdName)
	if name == "" || ms.pendingCmdStart.IsZero() {
		return "", nil
	}
	tstamp := ms.pendingCmdStart.Local().Format("2006-01-02 15:04:05")

	// 使ったらクリア
	ms.pendingCmdName = ""
	ms.pendingCmdStart = time.Time{}

	// "YYYY-MM-DD HH:MM:SS  :cmd"
	line := fmt.Sprintf("%s  %s", tstamp, name)

	timeBytes := len([]byte(tstamp))
	spaceBytes := len([]byte("  "))
	return line, []span{
		{AttrID: attrHeaderTime, ColStart: 0, ColEnd: timeBytes},
		{AttrID: attrHeaderCmd, ColStart: timeBytes + spaceBytes, ColEnd: len([]byte(line))},
	}
}

func (v *SplitView) ensureMaps() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.defined == nil {
		v.defined = make(map[int]bool)
	}
}

// ─────────────────────────────────────────────────────────────
// Messages（Neovim RPC ワーカー／ルータ／エントリポイント）

type Messages struct {
	ws      *Workspace
	msgs    []*Tooltip
	manager *MsgManager

	// cmdline 連携（次の split 出力に 1 回だけ載せる）
	cmdlinePreview  string
	pendingCmdName  string
	pendingCmdStart time.Time

	// RPC worker
	once sync.Once
	rpcq chan func(n *nvim.Nvim)
}

func initMessages() *Messages { return &Messages{} }

func (m *Messages) ensureWorker() {
	m.once.Do(func() {
		m.rpcq = make(chan func(n *nvim.Nvim), 128)
		go func() {
			for fn := range m.rpcq {
				func() {
					defer func() { _ = recover() }()
					fn(m.ws.nvim)
				}()
			}
		}()
	})
}

func (m *Messages) enqueue(job func(n *nvim.Nvim)) {
	m.ensureWorker()
	select {
	case m.rpcq <- job:
	default:
		go func() { m.rpcq <- job }()
	}
}

// ─────────────────────────────────────────────────────────────

func (v *SplitView) Show(m *UIMessage) (func(), error) {
	// 行とスパンを先に計算（UIスレッド側で安価に実施）
	linesFlat, spans := flattenWithSpansBytes(m.Content)

	// ヘッダ（実行日時+コマンド）差し込み（空行なし）
	if hdrLine, hdrSpans := v.consumeHeaderDecorated(); hdrLine != "" {
		linesFlat = append([]string{hdrLine}, linesFlat...)
		spans = append([][]span{hdrSpans}, spans...)
	}

	// マップ初期化
	v.ensureMaps()

	// 実作業は RPC ワーカーに投げる
	ws := v.ws
	ws.messages.enqueue(func(n *nvim.Nvim) {
		// Namespace
		v.mu.Lock()
		ns := v.nsID
		v.mu.Unlock()
		if ns == 0 {
			if id, err := n.CreateNamespace("goneo_messages_split_ns"); err == nil {
				v.mu.Lock()
				v.nsID = id
				ns = id
				v.mu.Unlock()
			}
		}

		// Window/Buffer 再利用 or 作成
		v.mu.Lock()
		win := v.win
		buf := v.buf
		v.mu.Unlock()

		var ok bool
		if win != 0 {
			_ = n.Call("nvim_win_is_valid", &ok, win)
		}
		if win == 0 || !ok {
			// 新規 split
			_ = n.Command("botright new")
			w, _ := n.CurrentWindow()
			b, _ := n.CreateBuffer(false, true)
			_ = n.Call("nvim_win_set_buf", nil, w, b)

			_ = n.SetWindowOption(w, "number", false)
			_ = n.SetWindowOption(w, "relativenumber", false)
			_ = n.SetWindowOption(w, "wrap", true)
			_ = n.SetWindowOption(w, "cursorline", false)
			_ = n.SetWindowOption(w, "signcolumn", "no")
			_ = n.SetWindowOption(w, "winhighlight", "")

			_ = n.SetBufferOption(b, "buftype", "nofile")
			_ = n.SetBufferOption(b, "bufhidden", "wipe")
			_ = n.SetBufferOption(b, "swapfile", false)

			opts := map[string]bool{"noremap": true, "silent": true, "nowait": true}
			_ = n.SetBufferKeyMap(b, "n", "q", "<cmd>close<CR>", opts)

			// 24bit 色（必要なら）
			_ = n.SetOption("termguicolors", true)

			v.mu.Lock()
			v.win = w
			v.buf = b
			win = w
			buf = b
			v.mu.Unlock()
		} else {
			var bok bool
			if buf != 0 {
				_ = n.Call("nvim_buf_is_valid", &bok, buf)
			}
			if buf == 0 || !bok {
				b, _ := n.CreateBuffer(false, true)
				_ = n.Call("nvim_win_set_buf", nil, win, b)
				_ = n.SetBufferOption(b, "buftype", "nofile")
				_ = n.SetBufferOption(b, "bufhidden", "wipe")
				_ = n.SetBufferOption(b, "swapfile", false)
				opts := map[string]bool{"noremap": true, "silent": true, "nowait": true}
				_ = n.SetBufferKeyMap(b, "n", "q", "<cmd>close<CR>", opts)

				v.mu.Lock()
				v.buf = b
				buf = b
				v.mu.Unlock()
			}
		}

		// 高さ（Ex コマンドは使わない）
		h := clamp(len(linesFlat), 3, 20)
		_ = n.Call("nvim_win_set_height", nil, win, h)

		// 行更新
		_ = n.SetBufferOption(buf, "modifiable", true)
		_ = n.SetBufferLines(buf, 0, -1, true, toByteLines(linesFlat))

		// 既存 HL クリア
		_ = n.Call("nvim_buf_clear_namespace", nil, buf, ns, 0, -1)

		// ヘッダ HL を一度だけ定義
		v.mu.Lock()
		needHeader := !v.headerHLDefined
		v.mu.Unlock()
		if needHeader {
			_ = n.Call("nvim_set_hl", nil, 0, "GoneoMsgHeaderTime", map[string]interface{}{"link": "Comment"})
			_ = n.Call("nvim_set_hl", nil, 0, "GoneoMsgHeaderCmd", map[string]interface{}{"link": "Title"})
			v.mu.Lock()
			v.headerHLDefined = true
			v.mu.Unlock()
		}

		// 未定義 attr_id を定義（>=0 の通常属性のみ）
		type hlDef struct {
			attrID int
			name   string
			opts   map[string]interface{}
		}
		var defs []hlDef
		seen := map[int]bool{}

		for _, spansOnLine := range spans {
			for _, sp := range spansOnLine {
				if sp.AttrID < 0 {
					continue
				}
				if seen[sp.AttrID] {
					continue
				}
				seen[sp.AttrID] = true

				v.mu.Lock()
				already := v.defined[sp.AttrID]
				v.mu.Unlock()
				if already {
					continue
				}

				hl := ws.screen.hlAttrDef[sp.AttrID]
				if hl == nil {
					hl = ws.screen.hlAttrDef[0]
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
				defs = append(defs, hlDef{attrID: sp.AttrID, name: name, opts: opts})
			}
		}
		for _, d := range defs {
			_ = n.Call("nvim_set_hl", nil, 0, d.name, d.opts)
			v.mu.Lock()
			v.defined[d.attrID] = true
			v.mu.Unlock()
		}

		// HL 適用
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
				} else if spn.AttrID == attrHeaderTime {
					group = "GoneoMsgHeaderTime"
				} else {
					group = "GoneoMsgHeaderCmd"
				}
				_ = n.Call("nvim_buf_add_highlight", nil, buf, ns, group, li, s, e)
			}
		}

		_ = n.SetBufferOption(buf, "modifiable", false)
	})

	// 閉じる関数は「閉じるジョブ」を enqueue するだけ（即 return）
	closeFn := func() {
		ws.messages.enqueue(func(n *nvim.Nvim) {
			v.mu.Lock()
			w := v.win
			v.win = 0
			v.buf = 0
			v.mu.Unlock()
			if w != 0 {
				_ = n.Call("nvim_win_close", nil, w, true)
			}
		})
	}
	return closeFn, nil
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
	// replace_last
	if m.ReplaceLast && len(mgr.lastStack) > 0 {
		lastID := mgr.lastStack[len(mgr.lastStack)-1]
		if closer, ok := mgr.active[lastID]; ok {
			closer()
			delete(mgr.active, lastID)
			mgr.lastStack = mgr.lastStack[:len(mgr.lastStack)-1]
		}
	}
	// 同一 id の置き換え
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

func (m *Messages) ensureManager() bool {
	if m.manager != nil {
		return true
	}
	if m.ws == nil {
		return false
	}
	ensureAboutQuitHook()
	m.ensureWorker()
	mini := &MiniTooltipView{
		ws:         m.ws,
		autoHideMs: defaultMiniTTLMs,
	}
	split := &SplitView{ws: m.ws}
	router := &Router{
		Routes: []Route{
			{Filter: RouteFilter{Event: MsgHistoryShow}, View: ViewSplit},
			{Filter: RouteFilter{Event: MsgShow, MinHeight: 10}, View: ViewSplit},
			{Filter: RouteFilter{Event: MsgShow, KindIn: []string{"emsg", "echoerr", "lua_error", "rpc_error"}, MinWidth: 50}, View: ViewSplit},
		},
	}
	m.manager = NewMsgManager(m.ws, router, mini, split)
	return true
}

func (m *Messages) AttachDefaultManager() { m.ensureManager() }
func (m *Messages) AttachManager(router *Router) {
	ensureAboutQuitHook()
	m.ensureWorker()
	mini := &MiniTooltipView{ws: m.ws, autoHideMs: defaultMiniTTLMs}
	split := &SplitView{ws: m.ws}
	m.manager = NewMsgManager(m.ws, router, mini, split)
}

func (m *Messages) msgClear() {
	if !m.ensureManager() {
		return
	}
	// Tooltip は GUI スレッドで安全に破棄（Qt タイマーは使わない）
	for _, t := range m.msgs {
		if t != nil {
			invokeQueued(t, "hide")
			invokeQueued(t, "close")
			invokeQueued(t, "deleteLater")
		}
	}
	m.msgs = nil
	m.manager.ClearAll()
}

// ---- ui-cmdline 連携 ----

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

// ext_messages: msg_history_show（堅牢版）
func (m *Messages) msgHistoryShow(entries []interface{}) {
	if !m.ensureManager() {
		return
	}
	all := &UIMessage{Event: MsgHistoryShow, Kind: "history"}

	for _, e := range entries {
		t, ok := e.([]interface{})
		if !ok || len(t) == 0 {
			continue
		}
		var contentRaw []interface{}
		if len(t) >= 2 {
			if cr, ok := t[1].([]interface{}); ok {
				contentRaw = cr
			}
		}
		if contentRaw == nil {
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
		if len(t) < 2 {
			continue
		}
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

// 旧コード互換（現実装では no-op）
func (m *Messages) updateFont() {}

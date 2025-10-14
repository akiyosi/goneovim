package editor

import (
	"bytes"
	"math"
	"strings"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
)

type ColorStr struct {
	hl        *Highlight
	str       string
	width     float64
	cellwidth float64
}

// ---- 尾の向き（内部用: 既存I/Fは不変） ----
type tooltipTailPos int

const (
	tailNone tooltipTailPos = iota
	tailTop
	tailBottom
	tailLeft
	tailRight
)

// Tooltip is the tooltip
type Tooltip struct {
	widgets.QWidget

	s             *Screen
	text          []*ColorStr
	font          *Font
	fallbackfonts []*Font
	widthSlice    []float64

	textWidth  float64
	msgHeight  float64
	background *RGBA

	pathWidth float64
	xPadding  float64
	yPadding  float64
	xRadius   float64
	yRadius   float64

	maximumWidth int
	fixedWidth   bool

	backgrond *RGBA

	// 吹き出し尾（I/F不変: 全て非公開・デフォルトOFF）
	tailPos    tooltipTailPos
	tailWidth  float64
	tailLength float64
	tailOffset float64
}

func (t *Tooltip) drawContent(p *gui.QPainter, getFont func() *Font) {
	if t.text == nil {
		return
	}

	screenWidth := t.s.widget.Width()
	if t.maximumWidth == 0 {
		t.maximumWidth = screenWidth
	}
	height := float64(t.font.height) * 1.1
	lineHeight := float64(t.font.lineHeight)
	if height > float64(lineHeight) {
		height = float64(lineHeight)
	}
	shift := t.font.shift
	var x float64
	var y float64 = float64(lineHeight-height) / 2

	color := t.background
	if color == nil && t.s != nil {
		color = t.s.ws.background
	}

	// アンチエイリアス
	p.SetRenderHint(gui.QPainter__Antialiasing, true)

	// 内容原点（Left/Top尾で押し出しに使う）
	var contentOriginX float64
	var contentOriginY float64

	if t.xRadius == 0 && t.yRadius == 0 {
		// 角丸なしモードのときは従来どおり（尾は付けない）
		p.FillRect4(core.NewQRectF4(
			0, 0,
			t.textWidth+t.xPadding*2.0,
			t.msgHeight+t.yPadding*2.0,
		), color.QColor())
	} else {
		// アウトライン色
		outlineColor := warpColor(color, -20)
		pen := gui.NewQPen4(
			gui.NewQBrush3(gui.NewQColor3(outlineColor.R, outlineColor.G, outlineColor.B, 255), core.Qt__BrushStyle(1)),
			t.pathWidth,
			core.Qt__SolidLine,
			core.Qt__RoundCap,
			core.Qt__RoundJoin,
		)
		p.SetPen(pen)

		width := t.textWidth + t.xPadding*2.0
		if width > float64(t.maximumWidth)-t.pathWidth*2 {
			width = float64(t.maximumWidth) - t.pathWidth*2
		}
		if t.fixedWidth {
			width = float64(t.maximumWidth) - t.pathWidth*2
		}

		// Left/Top 尾はベース矩形を押し出す
		shiftX := 0.0
		shiftY := 0.0
		if t.tailPos != tailNone && t.tailWidth > 0 && t.tailLength > 0 {
			if t.tailPos == tailLeft {
				shiftX = t.tailLength
			}
			if t.tailPos == tailTop {
				shiftY = t.tailLength
			}
		}

		baseRect := core.NewQRectF4(
			t.pathWidth+shiftX,
			t.pathWidth+shiftY,
			width,
			t.msgHeight+t.yPadding*2.0,
		)

		// 吹き出し全体を1本のパスで構築（角丸＋尾込）
		bubblePath := t.buildBalloonPath(baseRect)

		// 塗り → ストローク（単一パスなので接合線は存在しない）
		p.FillPath(bubblePath, gui.NewQBrush3(gui.NewQColor3(color.R, color.G, color.B, 255), core.Qt__BrushStyle(1)))
		p.DrawPath(bubblePath)

		contentOriginX = baseRect.X()
		contentOriginY = baseRect.Y()
	}

	// ====== テキスト描画（content origin 補正） ======
	for _, chunk := range t.text {
		fg := chunk.hl.fg()
		bg := chunk.hl.bg()
		sp := chunk.hl.special
		if sp == nil {
			sp = fg
		}

		bold := chunk.hl.bold
		italic := chunk.hl.italic
		underline := chunk.hl.underline
		undercurl := chunk.hl.undercurl
		underdouble := chunk.hl.underdouble
		underdotted := chunk.hl.underdotted
		underdashed := chunk.hl.underdashed
		strikethrough := chunk.hl.strikethrough

		for i, s := range strings.Split(chunk.str, "\n") {
			if i > 0 {
				x = 0
				y += lineHeight
			}

			p.SetPen2(fg.QColor())
			r := []rune(s)

			for _, rr := range r {

				if x+t.xPadding*2 >= float64(t.maximumWidth) {
					x = 0
					y += lineHeight
				}

				p.SetFont(resolveFontFallback(getFont(), t.fallbackfonts, string(rr)).qfont)
				font := p.Font()
				cwidth := gui.NewQFontMetricsF(t.font.qfont).HorizontalAdvance(string(rr), -1)
				w := chunk.cellwidth
				for {
					if cwidth <= w {
						break
					}
					w += chunk.cellwidth
				}

				// 背景
				p.FillRect4(
					core.NewQRectF4(
						contentOriginX+x+t.xPadding,
						contentOriginY+y+t.yPadding,
						w,
						height,
					),
					bg.QColor(),
				)

				// フォント装飾
				font.SetItalic(italic)
				font.SetBold(bold)

				// 文字
				p.DrawText(
					core.NewQPointF3(
						contentOriginX+x+t.xPadding,
						contentOriginY+y+float64(shift)+t.yPadding,
					),
					string(rr),
				)

				// 装飾線（末尾は int 0,0 に固定）
				if strikethrough {
					drawStrikethrough(p, t.font, sp.QColor(), 0, contentOriginX+x, contentOriginX+x+chunk.width, 0, 0)
				}
				if underline {
					drawUnderline(p, t.font, sp.QColor(), 0, contentOriginX+x, contentOriginX+x+chunk.width, 0, 0)
				}
				if undercurl {
					drawUndercurl(p, t.font, sp.QColor(), 0, contentOriginX+x, contentOriginX+x+chunk.width, 0, 0)
				}
				if underdouble {
					drawUnderdouble(p, t.font, sp.QColor(), 0, contentOriginX+x, contentOriginX+x+chunk.width, 0, 0)
				}
				if underdotted {
					drawUnderdotted(p, t.font, sp.QColor(), 0, contentOriginX+x, contentOriginX+x+chunk.width, 0, 0)
				}
				if underdashed {
					drawUnderdashed(p, t.font, sp.QColor(), 0, contentOriginX+x, contentOriginX+x+chunk.width, 0, 0)
				}

				x += w
			}
		}
	}
}

func (t *Tooltip) getFont() *Font {
	return t.font
}

func (t *Tooltip) setBgColor(color *RGBA) {
	t.background = color
}

func (t *Tooltip) setPadding(xv, yv float64) {
	t.xPadding = xv
	t.yPadding = yv
}

func (t *Tooltip) setRadius(xr, yr float64) {
	t.xRadius = xr
	t.yRadius = yr
}

func (t *Tooltip) setQpainterFont(p *gui.QPainter) {
	p.SetFont(t.font.qfont)
}

func (t *Tooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(t)
	t.drawContent(p, t.getFont)
	p.DestroyQPainter()
}

func (t *Tooltip) setFont(font *Font) {
	t.font = font
}

func (t *Tooltip) clearText() {
	var newText []*ColorStr
	t.text = newText
}

func (t *Tooltip) updateText(hl *Highlight, str string, letterspace float64, font *gui.QFont) {
	if t.font == nil {
		return
	}

	fontMetrics := gui.NewQFontMetricsF(font)
	cellwidth := fontMetrics.HorizontalAdvance("w", -1) + letterspace

	// rune text
	r := []rune(str)

	var width float64
	var maxWidth float64
	var widthList []float64
	var buffer bytes.Buffer
	for _, rr := range r {
		buffer.WriteString(string(rr))

		// detect char width based cell width
		w := cellwidth
		cwidth := fontMetrics.HorizontalAdvance(string(rr), -1)

		if string(rr) == "\n" {
			widthList = append(widthList, width)
			width = 0
			continue
		}

		for {
			if cwidth <= w {
				break
			}
			w += cellwidth
		}

		width += w
	}
	widthList = append(widthList, width)

	for _, v := range widthList {
		if maxWidth < v {
			maxWidth = v
		}
	}

	t.text = append(t.text, &ColorStr{
		hl:        hl,
		str:       buffer.String(),
		width:     maxWidth,
		cellwidth: cellwidth,
	})
}

func (t *Tooltip) show() {
	if t == nil {
		return
	}
	t.Show()
	t.Raise()
}

func (t *Tooltip) update() {
	// detect width
	var textWidth float64
	var msgHeight float64
	screenWidth := t.s.widget.Width()
	screenHeight := t.s.widget.Height()

	if t.maximumWidth == 0 {
		t.maximumWidth = screenWidth
	}

	if len(t.text) == 0 {
		return
	}
	var width float64
	var height int
	for i, chunk := range t.text {
		if i == 0 {
			msgHeight += float64(t.font.lineHeight)
		}

		width += chunk.width
		for j, s := range strings.Split(chunk.str, "\n") {
			if j > 0 {
				msgHeight += float64(t.font.lineHeight)
				width = 0
			}
			r := []rune(s)
			for k := range r {
				if width > float64(t.maximumWidth)-t.xPadding*2-t.pathWidth*2 {
					height = int(math.Floor(width)) / int(math.Ceil((float64(t.maximumWidth) - t.xPadding*2 - t.pathWidth*2)))
					residue := int(math.Floor(width)) % int(math.Ceil((float64(t.maximumWidth) - t.xPadding*2 - t.pathWidth*2)))
					if height > 0 && residue == 0 {
						height -= 1
					}
				} else if textWidth < width {
					textWidth = width
				}

				if k == len(r)-1 && textWidth < width {
					textWidth = width
				}
			}
		}
	}
	if height > 0 {
		msgHeight += float64(height * t.font.lineHeight)
	}

	t.textWidth = textWidth
	t.msgHeight = msgHeight

	var tooltipwidth float64
	if t.fixedWidth {
		tooltipwidth = float64(t.maximumWidth) + t.pathWidth*2
	} else {
		tooltipwidth = t.textWidth + t.xPadding*2.0 + t.pathWidth*2
		if tooltipwidth > float64(t.maximumWidth) {
			tooltipwidth = float64(t.maximumWidth) + t.pathWidth*2
		}
	}

	tooltipheight := t.msgHeight + t.yPadding*2.0 + t.pathWidth*2

	// 尾の張り出しぶんを加算
	twExtra := 0.0
	thExtra := 0.0
	if t.tailPos != tailNone && t.tailWidth > 0 && t.tailLength > 0 {
		switch t.tailPos {
		case tailTop, tailBottom:
			thExtra = t.tailLength
		case tailLeft, tailRight:
			twExtra = t.tailLength
		}
	}
	tooltipwidth += twExtra
	tooltipheight += thExtra

	if tooltipwidth > float64(screenWidth) {
		tooltipwidth = float64(screenWidth)
	}
	if tooltipheight > float64(screenHeight) {
		tooltipheight = float64(screenHeight)
	}

	t.SetFixedSize2(int(tooltipwidth), int(tooltipheight))
	t.Update()
}

// ===== ここから：吹き出し形状の自前生成（1本パス） =====

// 角丸 + 尾（任意辺）を1本の輪郭で作る
func (t *Tooltip) buildBalloonPath(rect *core.QRectF) *gui.QPainterPath {
	x := rect.X()
	y := rect.Y()
	w := rect.Width()
	h := rect.Height()

	rx := t.xRadius
	ry := t.yRadius
	if rx < 0 {
		rx = 0
	}
	if ry < 0 {
		ry = 0
	}
	// 半径クランプ
	maxRx := w / 2
	maxRy := h / 2
	if rx > maxRx {
		rx = maxRx
	}
	if ry > maxRy {
		ry = maxRy
	}

	// 尾パラメータ
	pos := t.tailPos
	tw := t.tailWidth
	tl := t.tailLength
	toff := t.tailOffset
	if tw < 0 {
		tw = 0
	}
	if tl < 0 {
		tl = 0
	}

	// 尾の位置のクランプ（角丸に食い込まない）
	cx := x + w/2
	cy := y + h/2
	switch pos {
	case tailTop, tailBottom:
		minX := x + rx + tw/2 + 1
		maxX := x + w - rx - tw/2 - 1
		cx = clampF(cx+toff, minX, maxX)
	case tailLeft, tailRight:
		minY := y + ry + tw/2 + 1
		maxY := y + h - ry - tw/2 - 1
		cy = clampF(cy+toff, minY, maxY)
	}

	path := gui.NewQPainterPath()

	// 開始点：上辺の左端内側（x+rx, y）
	path.MoveTo2(x+rx, y)

	// 上辺（尾が上の場合は張り出し）
	if pos == tailTop && tw > 0 && tl > 0 {
		path.LineTo2(cx-tw/2, y)
		path.LineTo2(cx, y-tl)
		path.LineTo2(cx+tw/2, y)
	}
	path.LineTo2(x+w-rx, y)         // 上辺右端手前
	path.QuadTo2(x+w, y, x+w, y+ry) // 右上角
	// 右辺（尾が右の場合は張り出し）
	if pos == tailRight && tw > 0 && tl > 0 {
		path.LineTo2(x+w, cy-tw/2)
		path.LineTo2(x+w+tl, cy)
		path.LineTo2(x+w, cy+tw/2)
	}
	path.LineTo2(x+w, y+h-ry)           // 右下角手前
	path.QuadTo2(x+w, y+h, x+w-rx, y+h) // 右下角
	// 下辺（尾が下の場合は張り出し）
	if pos == tailBottom && tw > 0 && tl > 0 {
		path.LineTo2(cx+tw/2, y+h)
		path.LineTo2(cx, y+h+tl)
		path.LineTo2(cx-tw/2, y+h)
	}
	path.LineTo2(x+rx, y+h)         // 下辺左端手前
	path.QuadTo2(x, y+h, x, y+h-ry) // 左下角
	// 左辺（尾が左の場合は張り出し）
	if pos == tailLeft && tw > 0 && tl > 0 {
		path.LineTo2(x, cy+tw/2)
		path.LineTo2(x-tl, cy)
		path.LineTo2(x, cy-tw/2)
	}
	path.LineTo2(x, y+ry)       // 左上角手前
	path.QuadTo2(x, y, x+rx, y) // 左上角
	path.CloseSubpath()
	return path
}

// --- helpers ---

// 名前衝突回避のため float64 版のクランプは clampF を使用
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

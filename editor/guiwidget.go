package editor

import (
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"time"

	"github.com/akiyosi/goneovim/util"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
)

type Guiwidget struct {
	Tooltip

	raw    string
	data   *core.QByteArray
	mime   string
	width  int
	height int
	id     int
	markID int
	winid  nvim.Window
}

func newGuiwidget() *Guiwidget {
	guiwidget := NewGuiwidget(nil, 0)
	guiwidget.data = nil
	guiwidget.width = 0
	guiwidget.height = 0
	guiwidget.mime = ""
	guiwidget.id = 0
	guiwidget.ConnectPaintEvent(guiwidget.paint)

	return guiwidget
}

// GuiWidgetPut pushes visual resource data to the front-end
func (w *Workspace) handleRPCGuiwidgetput(updates []interface{}) {
	fmt.Println("put guiwidgets")
	for _, update := range updates {
		a := update.(map[string]interface{})

		idITF, ok := a["id"]
		if !ok {
			fmt.Println("debug 7:: continue")
			continue
		}
		id := util.ReflectToInt(idITF)

		var g *Guiwidget
		if g, ok = w.getGuiwidgetFromResID(id); !ok {
			g = newGuiwidget()
			w.storeGuiwidget(id, g)
			g.s = w.screen
			g.setFont(g.s.font)
		}

		g.id = id

		mime, ok := a["mime"]
		if ok {
			g.mime = mime.(string)
		}

		data, ok := a["data"]
		if ok {
			s := data.(string)

			switch mime {
			case "text/plain":
				g.updateText(g.s.hlAttrDef[0], s)

			case "image/svg",
				"image/svg+xml",
				"image/png",
				"image/gif",
				"image/jpeg",
				"image/*":
				g.raw = s
			default:
			}
		}
	}
}

// GuiWidgetUpdateView sends a list of "placements".
// A placement associates an extmark with a resource id, and provides
// display options for a widget (width, height, mouse events etc.).
func (w *Workspace) handleRPCGuiwidgetview(updates []interface{}) {
	fmt.Println("view guiwidgets")
	var markid, resid, width, height int
	for _, update := range updates {
		a := update.(map[string]interface{})

		buf, ok := a["buf"]
		if !ok {
			fmt.Println("debug 10:: continue")
			continue
		}

		errChan := make(chan error, 60)
		var err error
		var outstr string
		go func() {
			outstr, err = w.nvim.CommandOutput(
				fmt.Sprintf("echo bufwinid(%d)", util.ReflectToInt(buf)),
			)
			errChan <- err
		}()
		select {
		case <-errChan:
		case <-time.After(40 * time.Millisecond):
		}

		out, _ := strconv.Atoi(outstr)
		winid := (nvim.Window)(out)

		widgets, ok := a["widgets"]
		if !ok {
			fmt.Println("debug 11:: continue")
			continue
		}
		for _, e := range widgets.([]interface{}) {
			for k, ee := range e.([]interface{}) {
				if k == 0 {
					markid = util.ReflectToInt(ee)
				} else if k == 1 {
					resid = util.ReflectToInt(ee)
				} else if k == 2 {
					width = util.ReflectToInt(ee)
				} else if k == 3 {
					height = util.ReflectToInt(ee)
				}
				if k >= 4 {
				}
			}

			var g *Guiwidget
			if g, ok = w.getGuiwidgetFromResID(resid); !ok {
				g = newGuiwidget()
				w.storeGuiwidget(resid, g)
				g.s = w.screen
				g.setFont(g.s.font)
			}

			g.winid = winid
			g.markID = markid
			g.width = width
			g.height = height

			switch g.mime {
			case "text/plain":
				baseFont := g.s.ws.font
				g.font = initFontNew(
					baseFont.fontNew.Family(),
					float64(g.height*baseFont.height)*0.8,
					0,
					0,
				)
			default:
			}

		}

	}

}

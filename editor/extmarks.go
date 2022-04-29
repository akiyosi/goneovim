package editor

import (
	"fmt"

	"github.com/akiyosi/goneovim/util"
	"github.com/neovim/go-client/nvim"
)

// windowExtmarks is
// ["win_extmark", grid, win, ns_id, mark_id, row, col]
// 	Updates the position of an extmark which is currently visible in a
// 	window. Only emitted if the mark has the `ui_watched` attribute.
func (ws *Workspace) windowExtmarks(args []interface{}) {
	fmt.Println("win_extmark event")
	for _, e := range args {
		arg := e.([]interface{})
		grid := util.ReflectToInt(arg[0])
		winid := (arg[1]).(nvim.Window)
		_ = util.ReflectToInt(arg[2])
		markid := util.ReflectToInt(arg[3])
		row := util.ReflectToInt(arg[4])
		col := util.ReflectToInt(arg[5])

		win, ok := ws.screen.getWindow(grid)
		if !ok {
			fmt.Println("debug 21:: continue")
			return
		}

		var gw *Guiwidget
		ws.guiWidgets.Range(func(_, gITF interface{}) bool {
			g := gITF.(*Guiwidget)
			if g == nil {
				return true
			}
			if markid == g.markID && g.winid == winid {
				gw = g
				return false
			}
			return true
		})
		if gw == nil {
			fmt.Println("debug 22:: continue")
			continue
		}

		win.storeGuiwidget(gw.id, gw)
		cell := win.content[row][col]
		if cell != nil {
			cell.decal = &Decal{
				exists: true,
				markid: markid,
			}
		}

		win.hasExtmarks = true
		// if gw.width == 0 && gw.height == 0 {
		// 	continue
		// }
		// fmt.Println("debug::", "gridid", win.grid, gw.markID, "text", gw.text, "row,col:", row, col, "width, height:", gw.width, gw.height)
		// win.queueRedraw(row, col, gw.width+1, gw.height)
		// for i := row; i <= row+gw.height; i++ {
		// 	if i >= len(win.contentMask) {
		// 		continue
		// 	}
		// 	for j := col; j <= col+gw.height; j++ {
		// 		if j >= len(win.contentMask[i]) {
		// 			continue
		// 		}
		// 		win.contentMask[i][j] = true
		// 	}
		// }
		// win.update()
	}
}

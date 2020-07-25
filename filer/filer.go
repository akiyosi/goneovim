package filer

import (
	"fmt"
	"strings"

	"github.com/akiyosi/goneovim/util"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/widgets"
)

type Filer struct {
	Widget    *widgets.QListWidget
	nvim      *nvim.Nvim
	selectnum int
	items     [](map[string]string)
}

// RegisterPlugin registers this remote plugin
func RegisterPlugin(nvim *nvim.Nvim) {
	nvim.Subscribe("GonvimFiler")

	shim := &Filer{
		nvim: nvim,
	}
	nvim.RegisterHandler("GonvimFiler", func(args ...interface{}) {
		go shim.handle(args...)
	})
	finderFunction := `
	aug GonvimAuFiler | au! | aug END
	    au GonvimAuFiler DirChanged * call rpcnotify(0, "Gui", "filer_update")
	command! GonvimFilerOpen call Gonvim_filer_run()
	function! Gonvim_filer_run() abort
	    call rpcnotify(0, "GonvimFiler", "open")
	    let l:keymaps = { "\<Esc>": "cancel", "\<C-c>": "cancel", "\<Enter>": "right", "h": "left", "j": "down", "k": "up", "l": "right", "/": "search", }
	
	    while v:true
	        let l:input = getchar()
	        let l:char = nr2char(l:input)
	    
	        let event = get(l:keymaps, l:char, "noevent")
	        if (l:input is# "\<BS>")
	            call rpcnotify(0, "GonvimFiler", "up")
	        elseif (l:input is# "\<DEL>")
	            call rpcnotify(0, "GonvimFiler", "up")
	        elseif (event == "noevent")
	            call rpcnotify(0, "GonvimFiler", "char", l:char)
	        else
	            call rpcnotify(0, "GonvimFiler", event)
	        endif
	    
	        if (event == "cancel")
	            call rpcnotify(0, "GonvimFiler", event)
	            return
	        endif
	    endwhile
	endfunction
	`

	registerFunction := fmt.Sprintf(
		`call execute(%s)`,
		util.SplitVimscript(finderFunction),
	)
	nvim.Command(registerFunction)
}

func (f *Filer) handle(args ...interface{}) {
	if len(args) < 1 {
		return
	}
	event, ok := args[0].(string)
	if !ok {
		return
	}
	switch event {
	case "cancel":
		f.cancel()
	case "open":
		f.open()
	case "redraw":
		f.redraw()
	case "left":
		f.left()
	case "right":
		f.right()
	case "up":
		f.up()
	case "down":
		f.down()
	case "search":
		f.search()
	default:
		fmt.Println("unhandleld filer event", event)
	}
}

func (f *Filer) open() {
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "side_open")
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_open")
	f.redraw()
}

func (f *Filer) redraw() {
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_clear")
	var files string
	var err error
	var api5 int
	f.nvim.Eval(`has('nvim-0.5')`, &api5)
	if api5 == 1 {
		err = f.nvim.ExecLua(`
			local uv = vim and vim.loop or require 'luv'
			local path = vim.fn.expand(vim.fn.getcwd())
			local h = uv.fs_scandir(path)
			local result = ""
			while true do
			    local name, type = uv.fs_scandir_next(h)
			    if not name then
			        break
			    end
			    if type == "directory" then
			        result = result .. "\n" .. name .. "/"
			    else
			        result = result .. "\n" .. name
			    end
			end
			return result
		`, &files)
	} else {
		files, err = f.nvim.CommandOutput(`lua 
			-- Ref: https://gitter.im/neovim/neovim?at=5dcf9e5b5eb2e813db330dc8
			local uv = vim and vim.loop or require 'luv'
			local path = vim.api.nvim_eval('expand(getcwd())')
			local h = uv.fs_scandir(path)
			while true do
			    local name, type = uv.fs_scandir_next(h)
			    if not name then
			        break
			    end
			    if type == "directory" then
			        print(name .. "/")
			    else
			        print(name)
			    end
			end
		`)
	}

	if err != nil {
		return
	}

	var items []map[string]string
	for _, file := range strings.Split(files, "\n") {
		if file == "" {
			continue
		}
		// Skip './' and '../'
		if file == "./" || file == "../" {
			continue
		}

		// Remove current dir string
		parts := strings.SplitN(file, ".", -1)
		filetype := ""
		if len(parts) > 1 {
			filetype = parts[len(parts)-1]
		}

		// If it is directory
		if file[len(file)-1] == '/' {
			filetype = string("/")
		}

		f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_add", file, filetype)
		item := make(map[string]string)
		item["filename"] = file
		item["filetype"] = filetype
		items = append(items, item)
	}
	f.items = items
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_resize")
	if f.selectnum >= len(f.items) {
		f.selectnum = len(f.items) - 1
	}
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_select", f.selectnum)
}

func (f *Filer) up() {
	f.selectnum--
	if f.selectnum < 0 {
		f.selectnum = 0
	}
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_select", f.selectnum)
}

func (f *Filer) down() {
	f.selectnum++
	if f.selectnum >= len(f.items) {
		f.selectnum = len(f.items) - 1
	}
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_select", f.selectnum)
}

func (f *Filer) left() {
	go func() {
		f.nvim.Command("silent :tchdir ..")
	}()
}

func (f *Filer) right() {
	if f.selectnum >= len(f.items) {
		return
	}
	item := f.items[f.selectnum]
	filename, ok := item["filename"]
	if !ok {
		return
	}
	filetype, ok := item["filetype"]
	if !ok {
		return
	}

	command := ""
	cdCommand := ":tchdir"
	editCommand := ":e"
	switch filetype {
	case "/":
		command = "silent " + cdCommand + " " + filename + " | <CR><CR> | :redraw!"
	default:
		command = "silent " + editCommand + " " + filename + " | <CR><CR> | :redraw!"
	}
	go func() {
		f.nvim.Command(command)
		if filetype != "/" {
			f.nvim.Input("<Esc>")
			// f.nvim.Command("GonvimFilerOpen")
		}
	}()
}

func (f *Filer) cancel() {
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "side_close")
}

// TODO
func (f *Filer) search() {
}

package filer

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/akiyosi/goneovim/util"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/widgets"
)

type Filer struct {
	Widget    *widgets.QListWidget
	nvim      *nvim.Nvim
	isOpen    bool
	cancelled bool

	cwd       string
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
        au GonvimAuFiler BufEnter,TabEnter,DirChanged,TermOpen,TermClose * call rpcnotify(0, "Gui", "filer_update")
	command! GonvimFilerOpen call Gonvim_filer_run()
	function! Gonvim_filer_run()
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
	pwd := ""
	f.nvim.Eval(`expand(getcwd())`, &pwd)

	if f.cwd != pwd {
		f.selectnum = 0
	}
	f.cwd = pwd
	pwdlen := len(pwd)
	if runtime.GOOS != "windows" {
		if pwd != string(`/`) {
			pwdlen++
		}
	} else {
		if len(pwd) != 3 { // it means that Windows root is 'C:\', 'D:\', etc
			pwdlen++
		}
	}

	command := "globpath(expand(getcwd()), '{,.}*', 1, 0)"
	files := ""
	f.nvim.Eval(command, &files)
	if len(files) <= pwdlen {
		return
	}

	// In windows, we need to detect file or directory
	var directories []string
	if runtime.GOOS == "windows" {
		command := "let dir = globpath(expand(getcwd()), '*', 0, 1) | echo filter(dir, 'isdirectory(v:val)')"
		dirstring, err := f.nvim.CommandOutput(command)
		if err == nil && len(dirstring) > 2 {
			dirstring = dirstring[2 : len(dirstring)-2]
			for _, dir := range strings.Split(dirstring, `', '`) {
				dir = dir[pwdlen:]
				directories = append(directories, dir)
			}
		}
	}

	var items []map[string]string
	for _, file := range strings.Split(files, "\n") {
		file = file[pwdlen:]
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
		if runtime.GOOS == "windows" {
			for _, dir := range directories {
				if file == dir {
					filetype = string("/")
				}
			}
		} else {
			if file[len(file)-1] == '/' {
				filetype = string("/")
			}
		}

		f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_add", file, filetype)
		item := make(map[string]string)
		item["filename"] = file
		item["filetype"] = filetype
		items = append(items, item)
	}
	f.items = items
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
	filename := f.items[f.selectnum]["filename"]
	filetype := f.items[f.selectnum]["filetype"]
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

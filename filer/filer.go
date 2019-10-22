package filer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	// "github.com/akiyosi/gonvim/util"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/widgets"
)

type Filer struct {
	Widget   *widgets.QListWidget
	nvim     *nvim.Nvim
	isOpen   bool
	cancelled          bool

	cwd         string
	selectnum   int
	items       [](map[string]string)
}

// RegisterPlugin registers this remote plugin
func RegisterPlugin(nvim *nvim.Nvim) {
	nvim.Subscribe("GonvimFiler")

	shim := &Filer{
		nvim:               nvim,
	}
	nvim.RegisterHandler("GonvimFiler", func(args ...interface{}) {
		go shim.handle(args...)
	})
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
	case "left":
		f.left()
	case "right":
		f.right()
	case "up":
		f.up()
	case "down":
		f.down()
	case "search":
		f.search(args[1:])
	default:
		fmt.Println("unhandleld filer event", event)
	}
}


func (f *Filer) open() {
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "side_show")
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_clear")
	pwd := ""
	f.nvim.Eval(`expand(getcwd())`, &pwd)

	if f.cwd != pwd {
		f.selectnum = 0
	}
	f.cwd = pwd
	pwdlen := utf8.RuneCountInString(pwd)+1

	command := fmt.Sprintf("globpath(expand(getcwd()), '{,.}*', 1, 0)")
	files := ""
	f.nvim.Eval(command, &files)

	var items []map[string]string
	for _, file := range strings.Split(files, "\n") {
		file = file[pwdlen:]
		// Skip './' and '../'
		if file[len(file)-2:] == "./" || file[len(file)-3:] == "../" {
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
			filetype = `/`
		}

		f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_add", file, filetype)
		item :=  make(map[string]string)
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
		f.selectnum = len(f.items)-1
	}
	f.nvim.Call("rpcnotify", nil, 0, "Gui", "filer_item_select", f.selectnum)
}

func (f *Filer) left() {
	go func() {
		f.nvim.Command("cd ..")
		f.open()
	}()
}

func (f *Filer) right() {
	filename := f.items[f.selectnum]["filename"]
	filetype := f.items[f.selectnum]["filetype"]
	openCommand := ""
	switch filetype {
	case "/":
		openCommand = ":cd " + filename
	default:
		openCommand = ":e " + filename
	}
	fmt.Println(openCommand)
	go func() {
		f.nvim.Command(openCommand)
		f.open()
		f.nvim.Input("<Esc>")
	}()

}


func (f *Filer) search(args []interface{}) {
}

func (f *Filer) cancel() {

}




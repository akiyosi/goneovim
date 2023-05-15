package editor

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/akiyosi/goneovim/util"
	"github.com/atotto/clipboard"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)

func newNvim(cols, rows int, ctx context.Context) (signal *neovimSignal, redrawUpdates chan [][]interface{}, guiUpdates chan []interface{}, nvimCh chan *nvim.Nvim, uiRemoteAttachedCh chan bool, errCh chan error) {
	signal = NewNeovimSignal(nil)
	redrawUpdates = make(chan [][]interface{}, 1000)
	guiUpdates = make(chan []interface{}, 1000)
	nvimCh = make(chan *nvim.Nvim, 2)
	uiRemoteAttachedCh = make(chan bool, 2)
	errCh = make(chan error, 2)
	var neovim *nvim.Nvim
	// var uiAttached, uiRemoteAttached bool
	var uiRemoteAttached bool
	var err error

	// // for debug signal
	// z := 1
	// go func() {
	// 	for {
	// 		editor.putLog("emmit test event", z)
	// 		redrawUpdates <- [][]interface{}{[]interface{}{"test event " + fmt.Sprintf("%d", z)}}
	// 		signal.RedrawSignal()
	// 		z++
	// 		time.Sleep(time.Millisecond * 10)
	// 	}
	// }()

	go func() {
		neovim, uiRemoteAttached, err = startNvim(signal, ctx)
		if err != nil {
			errCh <- err
			return
		} else {
			errCh <- nil
		}
		initGui(neovim)
		registerHandler(neovim, signal, redrawUpdates, guiUpdates)
		attachUI(neovim, cols, rows)
		setVar(neovim)

		nvimCh <- neovim
		uiRemoteAttachedCh <- uiRemoteAttached
	}()

	// // Suppress the problem that cmd.Start() hangs on MacOS.
	// time.Sleep(3 * time.Millisecond)

	return
}

func startNvim(signal *neovimSignal, ctx context.Context) (neovim *nvim.Nvim, uiRemoteAttached bool, err error) {
	editor.putLog("starting nvim")

	option := []string{
		"--cmd",
		"let g:gonvim_running=1",
		"--cmd",
		"let g:goneovim=1",
		"--cmd",
		"set termguicolors",
		"--embed",
	}

	childProcessArgs := nvim.ChildProcessArgs(
		append(option, editor.args...)...,
	)
	childProcessServe := nvim.ChildProcessServe(false)
	childProcessContext := nvim.ChildProcessContext(ctx)

	useWSL := editor.opts.Wsl != nil || editor.config.Editor.UseWSL
	if runtime.GOOS != "windows" {
		useWSL = false
	}

	if editor.opts.Server != "" {
		// Attaching to remote nvim session
		dialServe := nvim.DialServe(false)
		dialContext := nvim.DialContext(ctx)
		neovim, err = nvim.Dial(editor.opts.Server, dialServe, dialContext)
		uiRemoteAttached = true

	} else if editor.opts.Nvim != "" {
		// Attaching to /path/to/nvim
		childProcessCmd := nvim.ChildProcessCommand(editor.opts.Nvim)
		neovim, err = nvim.NewChildProcess(childProcessArgs, childProcessCmd, childProcessServe, childProcessContext)
	} else if useWSL {
		// Attaching remote nvim via wsl
		uiRemoteAttached = true
		neovim, err = newWslProcess()
	} else if editor.opts.Ssh != "" {
		// Attaching remote nvim via ssh
		uiRemoteAttached = true
		neovim, err = newRemoteChildProcess()
	} else {
		// Attaching to nvim normally
		neovim, err = nvim.NewChildProcess(childProcessArgs, childProcessServe, childProcessContext)
	}
	if err != nil {
		editor.putLog(err)
		return nil, false, err
	}

	go func() {
		err := neovim.Serve()
		if err != nil {
			editor.putLog(err)
		}
		signal.StopSignal()
	}()

	editor.putLog("done starting nvim")

	return neovim, uiRemoteAttached, nil
}

func registerHandler(neovim *nvim.Nvim, signal *neovimSignal, redrawUpdates chan [][]interface{}, guiUpdates chan []interface{}) {
	handleRequest(neovim)
	handleNotification(neovim, signal, redrawUpdates, guiUpdates)
}

func handleRequest(neovim *nvim.Nvim) {
	neovim.RegisterHandler("goneovim.set_clipboard", func(args interface{}) {
		if !editor.isUiPrepared {
			select {
			case <-editor.chUiPrepared:
				editor.isUiPrepared = true
			}
		}

		editor.putLog("goneovim.set_clipboard:: start")

		var endline string
		if runtime.GOOS == "windows" {
			endline = "\r\n"
		} else {
			endline = "\n"
		}

		newlines := []string{}
		for _, line := range args.([]interface{}) {
			newlines = append(newlines, strings.Replace(line.(string), "\r", "", -1))
		}
		str := strings.Join(newlines, endline)
		editor.putLog("goneovim.set_clipboard:: copy text is:", str)
		if runtime.GOOS == "darwin" {
			editor.app.Clipboard().SetText(str, gui.QClipboard__Clipboard)
		} else {
			clipboard.WriteAll(str)
		}
		editor.putLog("goneovim.set_clipboard:: finished")
	})

	neovim.RegisterHandler("goneovim.get_clipboard", func(args interface{}) ([]interface{}, error) {
		if !editor.isUiPrepared {
			select {
			case <-editor.chUiPrepared:
				editor.isUiPrepared = true
			}
		}

		text := editor.app.Clipboard().Text(gui.QClipboard__Clipboard)

		str := strings.Replace(text, "\r", "", -1)
		isLinePaste := strings.HasSuffix(str, "\n")
		var regType string
		if isLinePaste {
			regType = "V"
		} else {
			regType = "v"
		}

		// c := make(chan string, 10)
		// go func() {
		// 	ff, _ := neovim.CommandOutput("echo &ff")
		// 	c <- ff
		// }()
		// var endLine string
		// select {
		// case endLine = <-c:
		// case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
		// }

		// var s string
		// if endLine == "dos" {
		// 	s = strings.Replace(str, "\n", "\r\n", -1)
		// } else {
		// 	s = str
		// }

		lines := strings.Split(str, "\n")
		linesITF := make([]interface{}, len(lines))
		for i, line := range lines {
			var lineITF interface{}
			lineITF = line
			linesITF[i] = lineITF
		}

		return []interface{}{linesITF, regType}, nil
	})
}

func handleNotification(neovim *nvim.Nvim, signal *neovimSignal, redrawUpdates chan [][]interface{}, guiUpdates chan []interface{}) {
	neovim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		if !editor.isUiPrepared {
			select {
			case <-editor.chUiPrepared:
				editor.isUiPrepared = true
			}
		}

		redrawUpdates <- updates
		signal.RedrawSignal()
	})

	neovim.Subscribe("Gui")
	neovim.RegisterHandler("Gui", func(updates ...interface{}) {
		if !editor.isUiPrepared {
			select {
			case <-editor.chUiPrepared:
				editor.isUiPrepared = true
			}
		}
		guiUpdates <- updates
		signal.GuiSignal()
	})
}

func newRemoteChildProcess() (*nvim.Nvim, error) {
	logf := log.Printf
	command := "ssh"
	if runtime.GOOS == "windows" {
		command = `C:\windows\system32\OpenSSH\ssh.exe`
	}
	ctx := context.Background()

	var hostname string = ""
	var portno string = "22"
	var err error
	hostname, portno, err = net.SplitHostPort(editor.opts.Ssh)

	nvimargs := `"nvim --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed `
	for _, s := range editor.args {
		nvimargs += s + " "
	}
	nvimargs += `"`
	sshargs := []string{
		hostname,
		"-p", portno,
		"$SHELL",
		"--login",
		"-c",
		nvimargs,
	}
	cmd := exec.CommandContext(
		ctx,
		command,
		sshargs...,
	)

	inw, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	outr, err := cmd.StdoutPipe()
	if err != nil {
		inw.Close()
		return nil, err
	}
	cmd.Start()

	v, err := nvim.New(outr, inw, inw, logf)
	if err != nil {
		editor.putLog("error:", err)
		return nil, err
	}

	return v, nil
}

func newWslProcess() (*nvim.Nvim, error) {
	editor.putLog("Attaching nvim on wsl")

	logf := log.Printf
	command := "wsl"
	ctx := context.Background()

	nvimargs := ""
	if editor.config.Editor.NvimInWsl == "" {
		nvimargs = `nvim --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed `
	} else {
		nvimargs += fmt.Sprintf("%s --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed ", editor.config.Editor.NvimInWsl)
	}
	for _, s := range editor.args {
		nvimargs += s + " "
	}

	wslArgs := []string{
		"$SHELL",
		"-lc",
		nvimargs,
	}
	wslDist := ""
	if editor.opts.Wsl != nil {
		wslDist = *editor.opts.Wsl
	}
	if wslDist == "" {
		wslDist = editor.config.Editor.WSLDist
	}
	if wslDist != "" {
		wslArgs = append([]string{"-d", wslDist}, wslArgs...)
	}

	cmd := exec.CommandContext(
		ctx,
		command,
		wslArgs...,
	)
	util.PrepareRunProc(cmd)

	// NOTE: cmd.String() was added in Go1.13, which cannot be used in MSVC builds based on Go1.10 builds.
	// editor.putLog("exec command:", cmd.String())

	inw, err := cmd.StdinPipe()
	if err != nil {
		editor.putLog("stdin pipe error:", err)
		return nil, err
	}

	outr, err := cmd.StdoutPipe()
	if err != nil {
		inw.Close()
		editor.putLog("stdin pipe error:", err)
		return nil, err
	}
	cmd.Start()

	v, err := nvim.New(outr, inw, inw, logf)
	if err != nil {
		editor.putLog("error:", err)
		return nil, err
	}

	return v, nil
}

func attachUI(neovim *nvim.Nvim, cols, rows int) error {
	editor.putLog("attaching UI")

	_, o := attachUIOption(neovim)
	err := neovim.AttachUI(cols, rows, o)
	if err != nil {
		return err
	}

	editor.putLog("done attaching UI")

	return nil
}

func attachUIOption(nvim *nvim.Nvim) (int, map[string]interface{}) {
	o := make(map[string]interface{})
	o["rgb"] = true
	o["ext_multigrid"] = true
	o["ext_hlstate"] = true

	apiInfo, err := nvim.APIInfo()
	var channel int
	var item interface{}
	if err == nil {
		for channel, item = range apiInfo {
			i, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range i {
				if k != "ui_events" {
					continue
				}
				events, ok := v.([]interface{})
				if !ok {
					continue
				}
				for _, event := range events {
					function, ok := event.(map[string]interface{})
					if !ok {
						continue
					}
					name, ok := function["name"]
					if !ok {
						continue
					}

					switch name {
					// case "wildmenu_show" :
					// 	o["ext_wildmenu"] = editor.config.Editor.ExtCmdline
					case "cmdline_show":
						o["ext_cmdline"] = editor.config.Editor.ExtCmdline
					case "msg_show":
						o["ext_messages"] = editor.config.Editor.ExtMessages
					case "popupmenu_show":
						o["ext_popupmenu"] = editor.config.Editor.ExtPopupmenu
					case "tabline_update":
						o["ext_tabline"] = editor.config.Editor.ExtTabline
					}
				}
			}
		}
	}

	return channel, o
}

func initGui(neovim *nvim.Nvim) {
	// autocmds that goneovim uses
	gonvimAutoCmds := `
	aug GoneovimCore | au! | aug END
	au GoneovimCore OptionSet * if &ro != 1 | silent! call rpcnotify(0, "Gui", "gonvim_optionset", expand("<amatch>"), v:option_new, v:option_old, win_getid()) | endif
	au GoneovimCore BufEnter * call rpcnotify(0, "Gui", "gonvim_bufenter", win_getid())
	aug Goneovim | au! | aug END
	au Goneovim DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd", v:event)
	au Goneovim BufEnter,TabEnter,DirChanged,TermOpen,TermClose * silent call rpcnotify(0, "Gui", "gonvim_workspace_filepath", expand("%:p"))
	`
	if editor.opts.Server == "" && !editor.config.MiniMap.Disable {
		gonvimAutoCmds = gonvimAutoCmds + `
		au Goneovim BufEnter,BufWrite * call rpcnotify(0, "Gui", "gonvim_minimap_update")
		au Goneovim TextChanged,TextChangedI * call rpcnotify(0, "Gui", "gonvim_minimap_sync")
		au Goneovim ColorScheme * call rpcnotify(0, "Gui", "gonvim_colorscheme")
		`
	}
	if editor.config.Statusline.Visible {
		gonvimAutoCmds = gonvimAutoCmds + `
		au Goneovim BufEnter,TermOpen,TermClose * call rpcnotify(0, "statusline", "bufenter", &filetype, &fileencoding, &fileformat, &ro)
	`
	}
	registerScripts := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimAutoCmds))
	neovim.Command(registerScripts)

	// Definition of the commands that goneovim provides
	gonvimCommands := fmt.Sprintf(`
	command! -nargs=1 GonvimResize call rpcnotify(0, "Gui", "gonvim_resize", <args>)
	command! GonvimSidebarShow call rpcnotify(0, "Gui", "side_open")
	command! GonvimSidebarToggle call rpcnotify(0, "Gui", "side_toggle")
	command! GonvimVersion echo "%s"`, editor.version)
	if editor.opts.Server == "" {
		if !editor.config.MiniMap.Disable {
			gonvimCommands = gonvimCommands + `
			command! GonvimMiniMap call rpcnotify(0, "Gui", "gonvim_minimap_toggle")
		`
		}
		gonvimCommands = gonvimCommands + `
		command! GonvimWorkspaceNew call rpcnotify(0, "Gui", "gonvim_workspace_new")
		command! GonvimWorkspaceNext call rpcnotify(0, "Gui", "gonvim_workspace_next")
		command! GonvimWorkspacePrevious call rpcnotify(0, "Gui", "gonvim_workspace_previous")
		command! -nargs=1 GonvimWorkspaceSwitch call rpcnotify(0, "Gui", "gonvim_workspace_switch", <args>)
		`
	}
	gonvimCommands = gonvimCommands + `
	command! -nargs=1 GonvimGridFont call rpcnotify(0, "Gui", "gonvim_grid_font", <args>)
	command! -nargs=1 GonvimLetterSpacing call rpcnotify(0, "Gui", "gonvim_letter_spacing", <args>)
	command! -nargs=1 GuiMacmeta call rpcnotify(0, "Gui", "gonvim_macmeta", <args>)
	command! -nargs=? GonvimMaximize call rpcnotify(0, "Gui", "gonvim_maximize", <args>)
	command! -nargs=? GonvimFullscreen call rpcnotify(0, "Gui", "gonvim_fullscreen", <args>)
	command! -nargs=+ GonvimWinpos call rpcnotify(0, "Gui", "gonvim_winpos", <f-args>)
	command! GonvimToggleHorizontalScroll call rpcnotify(0, "Gui", "gonvim_toggle_horizontal_scroll")
	command! GonvimLigatures call rpcnotify(0, "Gui", "gonvim_ligatures")
	command! GonvimSmoothScroll call rpcnotify(0, "Gui", "gonvim_smoothscroll")
	command! GonvimSmoothCursor call rpcnotify(0, "Gui", "gonvim_smoothcursor")
	command! GonvimIndentguide call rpcnotify(0, "Gui", "gonvim_indentguide")
	command! -nargs=? GonvimMousescrollUnit call rpcnotify(0, "Gui", "gonvim_mousescroll_unit", <args>)
	`
	registerScripts = fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimCommands))
	neovim.Command(registerScripts)

	if editor.config.Statusline.Visible {
		gonvimInitNotify := `
		call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding, &fileformat, &ro)
		`
		initialNotify := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimInitNotify))
		neovim.Command(initialNotify)
	}

	code := `
    local function set_clipboard(register)
        return function(lines, regtype)
            vim.rpcrequest(vim.g.goneovim_channel_id, 'goneovim.set_clipboard', lines)
        end
    end

    local function get_clipboard(register)
        return function()
            return vim.rpcrequest(vim.g.goneovim_channel_id, 'goneovim.get_clipboard', register)
        end
    end

    vim.g.clipboard = {
        name = 'goneovim',
        copy = {
            ['+'] = set_clipboard('+'),
            ['*'] = set_clipboard('*'),
        },
        paste = {
            ['+'] = get_clipboard('+'),
            ['*'] = get_clipboard('*'),
        },
        cache_enabled = 0
    }`
	var result, args interface{}
	if editor.config.Editor.Clipboard {
		neovim.ExecLua(
			code,
			&result,
			args,
		)
	}
}

func setVar(neovim *nvim.Nvim) {
	go neovim.SetVar("goneovim_channel_id", neovim.ChannelID())
}

func source(neovim *nvim.Nvim, file string) {
	if file == "" {
		return
	}

	go neovim.Command("so " + file)
}

func loadGinitVim(neovim *nvim.Nvim) {
	if editor.config.Editor.GinitVim == "" {
		return
	}

	var result bool
	_, err := neovim.Exec(editor.config.Editor.GinitVim, result)
	if err != nil {
		editor.pushNotification(
			NotifyWarn,
			0,
			"An error occurs while processing Vimscript in Ginitvim.\n"+err.Error(),
			notifyOptionArg([]*NotifyButton{}),
		)
	}
}

func loadHelpDoc(neovim *nvim.Nvim) {
	// Add runtimepath
	runtimepath := getResourcePath() + "/runtime/"
	cmd := fmt.Sprintf("let &rtp.=',%s'", runtimepath)

	var result bool
	// resultCh := make(chan bool, 5)

	// go func() {
	// 	neovim.Exec(cmd, result)
	// 	resultCh <- result
	// }()
	// select {
	// case <-time.After(80 * time.Millisecond):
	// }
	neovim.Exec(cmd, result)

	// Generate goneovim helpdoc tag
	helpdocpath := getResourcePath() + "/runtime/doc"
	cmd = fmt.Sprintf(`try | helptags %s | catch /^Vim\%%((\a\+)\)\=:E/ | endtry`, helpdocpath)

	// go func() {
	// 	neovim.Exec(cmd, result)
	// 	resultCh <- result
	// }()
	// select {
	// case <-time.After(80 * time.Millisecond):
	// }
	neovim.Exec(cmd, result)
}

func getResourcePath() string {
	p := ""
	if runtime.GOOS == "darwin" {
		p = core.QCoreApplication_ApplicationDirPath() + "/../Resources"
	} else if runtime.GOOS == "linux" {
		p = core.QCoreApplication_ApplicationDirPath()
	} else if runtime.GOOS == "windows" {
		p = core.QCoreApplication_ApplicationDirPath()
	}

	return p
}

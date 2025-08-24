package editor

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/atotto/clipboard"
	"github.com/neovim/go-client/nvim"
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
		setVar(neovim)
		setupGoneovim(neovim)
		setupGoneovimCommands(neovim)
		registerHandler(neovim, signal, redrawUpdates, guiUpdates)
		attachUI(neovim, cols, rows)

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

func setupGoneovim(neovim *nvim.Nvim) {
	// autocmds that goneovim uses
	gonvimAutoCmds := `
	aug GoneovimCore | au! | aug END
	au GoneovimCore VimEnter * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_vimenter")
	au GoneovimCore OptionSet * if &ro != 1 | silent! call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_optionset", expand("<amatch>"), v:option_new, v:option_old, win_getid()) | endif
	au GoneovimCore BufEnter * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_bufenter", win_getid())
	au GoneovimCore TermEnter * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_termenter")
	au GoneovimCore TermLeave * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_termleave")
	aug Goneovim | au! | aug END
	au Goneovim BufEnter,TabEnter,TermOpen,TermClose * silent call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_workspace_filepath", expand("%:p"))
	`
	if editor.opts.Server == "" && !editor.config.MiniMap.Disable {
		gonvimAutoCmds = gonvimAutoCmds + `
		au Goneovim BufEnter,BufWrite * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_minimap_update")
		au Goneovim TextChanged,TextChangedI * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_minimap_sync")
		au Goneovim ColorScheme * call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_colorscheme")
		`
	}
	registerScripts := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimAutoCmds))
	neovim.Command(registerScripts)
}

func setupGoneovimCommands(neovim *nvim.Nvim) {
	// Definition of the commands that goneovim provides
	gonvimCommands := fmt.Sprintf(`
	command! -nargs=1 GonvimResize call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_resize", <args>)
	command! GonvimSidebarShow call rpcnotify(g:goneovim_channel_id, "Gui", "side_open")
	command! GonvimSidebarToggle call rpcnotify(g:goneovim_channel_id, "Gui", "side_toggle")
	command! GonvimVersion echo "%s"`, editor.version)
	if editor.opts.Server == "" {
		if !editor.config.MiniMap.Disable {
			gonvimCommands = gonvimCommands + `
			command! GonvimMiniMap call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_minimap_toggle")
		`
		}
		gonvimCommands = gonvimCommands + `
		command! GonvimWorkspaceNew call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_workspace_new")
		command! GonvimWorkspaceNext call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_workspace_next")
		command! GonvimWorkspacePrevious call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_workspace_previous")
		command! -nargs=1 GonvimWorkspaceSwitch call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_workspace_switch", <args>)
		`
	}
	gonvimCommands = gonvimCommands + `
	command! -nargs=1 GonvimGridFont call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_grid_font", <args>)
	command! -nargs=1 GonvimGridFontAutomaticHeight call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_grid_font_automatic_height", <args>)
	command! -nargs=1 GonvimLetterSpacing call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_letter_spacing", <args>)
	command! -nargs=1 GuiMacmeta call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_macmeta", <args>)
	command! -nargs=? GonvimMaximize call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_maximize", <args>)
	command! -nargs=? GonvimFullscreen call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_fullscreen", <args>)
	command! -nargs=+ GonvimWinpos call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_winpos", <f-args>)
	command! GonvimToggleHorizontalScroll call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_toggle_horizontal_scroll")
	command! GonvimLigatures call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_ligatures")
	command! GonvimSmoothScroll call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_smoothscroll")
	command! GonvimSmoothCursor call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_smoothcursor")
	command! GonvimIndentguide call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_indentguide")
	command! -nargs=? GonvimMousescrollUnit call rpcnotify(g:goneovim_channel_id, "Gui", "gonvim_mousescroll_unit", <args>)
	`
	registerScripts := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimCommands))
	neovim.Command(registerScripts)
}

func setupGoneovimClipBoard(neovim *nvim.Nvim) {
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

	o := make(map[string]interface{})
	o["output"] = true
	errCh := make(chan error, 5)

	cmd := editor.config.Editor.GinitVim

	go func() {
		_, err := neovim.Exec(cmd, o)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		// if err is not nil
		if err != nil {
			editor.pushNotification(
				NotifyWarn,
				0,
				"An error occurs while processing Vimscript in Ginitvim.\n"+err.Error(),
				notifyOptionArg([]*NotifyButton{}),
			)
		}
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}

}

func loadHelpDoc(neovim *nvim.Nvim) {
	var helpdocpath, runtimepath, cmd string
	o := make(map[string]interface{})
	o["output"] = true

	// make register command
	runtimepath = filepath.Join(getResourcePath(), "runtime")
	if isFileExist(runtimepath) {
		cmd = fmt.Sprintf("let &rtp.=',%s'", runtimepath)
		neovim.Exec(cmd, o)
		helpdocpath = filepath.Join(runtimepath, "doc")
		cmd = fmt.Sprintf(`try | helptags %s | catch /^Vim\%%((\a\+)\)\=:E/ | endtry`, helpdocpath)
	} else {
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		helpdocpath = filepath.Join(xdgDataHome, "nvim", "site", "doc")
		cmd = fmt.Sprintf(`try | helptags %s | catch /^Vim\%%((\a\+)\)\=:E/ | endtry`, helpdocpath)
	}

	neovim.Exec(cmd, o)
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

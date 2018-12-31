package editor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	homedir "github.com/mitchellh/go-homedir"
)

// gonvimConfig is the following toml file
// # Gonvim config toml
// [editor]
// width = 1000  # >= 800
// height = 800  # >= 600
// fontFamily = "FuraCode Nerd Font Mono"
// fontsize = 18
// linespace = 10
// clipboard = true
// cursorBlink = true
// disableIMEinNormal = true
// startFullScreen = true
// ginitvim = '''
//   set guifont=FuraCode\ Nerd\ Font\ Mono:h14
//   if g:gonvim_running == 1
//     set laststatus=0
//   endif
// '''
//
// [statusLine]
// visible = true
// # textLabel / icon / background / none
// modeIndicatorType = "icon"
// normalModeColor = "#123456"
// commandModeColor = "#123456"
// insertModeColor = "#123456"
// replaceModeColor = "#123456"
// visualModeColor = "#123456"
// termnalModeColor = "#123456"
//
// [tabline]
// visible = true
//
// [lint]
// visible = true
//
// [scrollBar]
// visible = true
//
// [activityBar]
// visible = true
// dropshadow = true
//
// [miniMap]
// visible = true
//
// [sideBar]
// visible = false
// dropshadow = true
// width = 360
// accentColor = "#5596ea"
//
// [workspace]
// # Path style
// #   full: fullpath,
// #   name: directory name only,
// #   minimum: only the last directory is full name, middle directory is short form
// pathStyle = minimum
// FileExplorerOpenCmd = ":tabew"
//
// # restore the previous sessions if there are exists.
// restoreSession = false
//
// [dein]
// tomlFile
type gonvimConfig struct {
	Editor      editorConfig
	Statusline  statusLineConfig
	Tabline     tabLineConfig
	Lint        lintConfig
	ScrollBar   scrollBarConfig
	ActivityBar activityBarConfig
	MiniMap     miniMapConfig
	SideBar     sideBarConfig
	Workspace   workspaceConfig
	Dein        deinConfig
}

type editorConfig struct {
	Width              int
	Height             int
	FontFamily         string
	FontSize           int
	Linespace          int
	Clipboard          bool
	CursorBlink        bool
	DisableImeInNormal bool
	GinitVim           string
	StartFullscreen    bool
}

type statusLineConfig struct {
	Visible           bool
	ModeIndicatorType string
	NormalModeColor   string
	CommandModeColor  string
	InsertModeColor   string
	ReplaceModeColor  string
	VisualModeColor   string
	TerminalModeColor string
}

type tabLineConfig struct {
	Visible bool
}

type lintConfig struct {
	Visible bool
}

type miniMapConfig struct {
	Visible bool
}

type scrollBarConfig struct {
	Visible bool
}

type activityBarConfig struct {
	Visible    bool
	DropShadow bool
}

type sideBarConfig struct {
	Visible     bool
	DropShadow  bool
	Width       int
	AccentColor string
}

type workspaceConfig struct {
	RestoreSession bool
	PathStyle      string
	FileExplorerOpenCmd string
}

type deinConfig struct {
	TomlFile string
}

func newGonvimConfig(home string) gonvimConfig {
	var config gonvimConfig
	if _, err := toml.DecodeFile(filepath.Join(home, ".gonvim", "setting.toml"), &config); err != nil {
		config.Editor.FontSize = 14
		config.Editor.Width = 800
		config.Editor.Height = 600
		config.Statusline.Visible = true
		config.Statusline.ModeIndicatorType = "textLabel"
		config.Tabline.Visible = true
		config.Lint.Visible = true
		config.ActivityBar.Visible = true
		config.ScrollBar.Visible = true
		config.SideBar.Width = 300
		config.SideBar.AccentColor = "#5596ea"
		config.Workspace.PathStyle = "minimum"
		go func() {
			time.Sleep(2000 * time.Millisecond)
			editor.pushNotification(NotifyWarn, -1, "[Gonvim] Error detected while parsing setting.toml: "+fmt.Sprintf("%s", err))
		}()
	}

	if config.Editor.Width <= 800 {
		config.Editor.Width = 800
	}
	if config.Editor.Height <= 600 {
		config.Editor.Height = 600
	}
	if config.Statusline.ModeIndicatorType == "" {
		config.Statusline.ModeIndicatorType = "textLabel"
	}

	if config.Editor.FontFamily == "" {
		switch runtime.GOOS {
		case "windows":
			config.Editor.FontFamily = "Consolas"
		case "darwin":
			config.Editor.FontFamily = "Monaco"
		default:
			config.Editor.FontFamily = "Monospace"
		}
	}
	if config.Editor.FontSize <= 5 {
		config.Editor.FontSize = 13
	}
	if config.Editor.Linespace < 0 {
		config.Editor.Linespace = 6
	}

	if config.Statusline.NormalModeColor == "" {
		config.Statusline.NormalModeColor = newRGBA(60, 171, 235, 1).Hex()
	}
	if config.Statusline.CommandModeColor == "" {
		config.Statusline.CommandModeColor = newRGBA(82, 133, 184, 1).Hex()
	}
	if config.Statusline.InsertModeColor == "" {
		config.Statusline.InsertModeColor = newRGBA(42, 188, 180, 1).Hex()
	}
	if config.Statusline.VisualModeColor == "" {
		config.Statusline.VisualModeColor = newRGBA(153, 50, 204, 1).Hex()
	}
	if config.Statusline.ReplaceModeColor == "" {
		config.Statusline.ReplaceModeColor = newRGBA(255, 140, 10, 1).Hex()
	}
	if config.Statusline.TerminalModeColor == "" {
		config.Statusline.TerminalModeColor = newRGBA(119, 136, 153, 1).Hex()
	}

	if config.SideBar.Width == 0 {
		config.SideBar.Width = 300
	}
	if config.SideBar.AccentColor == "" {
		config.SideBar.AccentColor = "#5596ea"
	}
	if config.Workspace.PathStyle == "" {
		config.Workspace.PathStyle = "minimum"
	}

	return config
}

func outputGonvimConfig() {
	home, err := homedir.Dir()
	if err != nil {
		home = "~"
	}
	filepath := filepath.Join(home, ".gonvim", "setting.toml")
	if isFileExist(filepath) {
		return
	}
	buf := new(bytes.Buffer)
	toml.NewEncoder(buf).Encode(editor.config)
	err = ioutil.WriteFile(filepath, buf.Bytes(), 664)
	if err != nil {
		editor.pushNotification(NotifyWarn, -1, "[Gonvim] I can't write to setting.toml file at ~/.gonvim/setting.toml")
	}
}

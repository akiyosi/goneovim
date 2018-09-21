package editor

import (
	"fmt"
	homedir "github.com/mitchellh/go-homedir"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// gonvimConfig is the following toml file
// [editor]
// clipboard = true
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
// accentColor
//
// [workspace]
// # Path style
// #   full: fullpath,
// #   name: directory name only,
// #   minimum: only the last directory is full name, middle directory is short form
// pathStyle = minimum
//
// # restore the previous sessions if there are exists.
// restoreSession = false
//
// [dein]
// tomlFile
type gonvimConfig struct {
	Editor      editorConfig
	ScrollBar   scrollBarConfig
	ActivityBar activityBarConfig
	MiniMap     miniMapConfig
	SideBar     sideBarConfig
	Workspace   workspaceConfig
	Dein        deinConfig
}

type editorConfig struct {
	Clipboard bool
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
}

type deinConfig struct {
	TomlFile string
}

func newGonvimConfig(home string) gonvimConfig {
	var config gonvimConfig
	if _, err := toml.DecodeFile(filepath.Join(home, ".gonvim", "setting.toml"), &config); err != nil {
		config.ActivityBar.Visible = true
		config.ScrollBar.Visible = true
		config.SideBar.Width = 300
		config.SideBar.AccentColor = "#519aba"
		config.Workspace.PathStyle = "minimum"
	}

	if config.SideBar.Width == 0 {
		config.SideBar.Width = 300
	}
	if config.SideBar.AccentColor == "" {
		config.SideBar.AccentColor = "#519aba"
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
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		editor.pushNotification(NotifyWarn, -1, "[Gonvim] I can't Open the setting.toml file at ~/.gonvim/setting.toml")
	}
	defer file.Close()
	fmt.Fprintln(file, "")

	fmt.Fprint(file, "[editor]\n")
	fmt.Fprint(file, "clipboard = ", editor.config.Editor.Clipboard, "\n")
	fmt.Fprint(file, "[scrollBar]", "\n")
	fmt.Fprint(file, "visible = ", editor.config.ScrollBar.Visible, "\n")
	fmt.Fprint(file, "[activityBar]", "\n")
	fmt.Fprint(file, "visible = ", editor.config.ActivityBar.Visible, "\n")
	fmt.Fprint(file, "dropshadow = ", editor.config.ActivityBar.DropShadow, "\n")
	fmt.Fprint(file, "[miniMap]", "\n")
	fmt.Fprint(file, "visible = ", editor.config.MiniMap.Visible, "\n")
	fmt.Fprint(file, "[sideBar]", "\n")
	fmt.Fprint(file, "visible = ", editor.config.SideBar.Visible, "\n")
	fmt.Fprint(file, "dropshadow = ", editor.config.SideBar.DropShadow, "\n")
	fmt.Fprint(file, "width = ", editor.config.SideBar.Width, "\n")
	fmt.Fprint(file, `accentColor = "`, editor.config.SideBar.AccentColor, `"`, "\n")
	fmt.Fprint(file, "[workspace]", "\n")
	fmt.Fprint(file, `pathStyle = "`, editor.config.Workspace.PathStyle, `"`, "\n")
	fmt.Fprint(file, "restoreSession = ", editor.config.Workspace.RestoreSession, "\n")
	fmt.Fprint(file, "[dein]", "\n")
	fmt.Fprint(file, `tomlFile = '`, editor.config.Dein.TomlFile, `'`, "\n")
}

package editor

import (
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// gonvimConfig is the following toml file
// [editor]
// clipboard = true
//
// [activityBar]
// visible = true
// dropshadow = true
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
	ActivityBar activityBarConfig
	SideBar     sideBarConfig
	Workspace   workspaceConfig
	Dein        deinConfig
}

type editorConfig struct {
	Clipboard bool
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
		config.SideBar.Width = 300
		config.SideBar.AccentColor = "#519aba"
		config.Workspace.PathStyle = "minimum"
	}
	return config
}

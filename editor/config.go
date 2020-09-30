package editor

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

type gonvimConfig struct {
	Editor      editorConfig
	Palette     paletteConfig
	Message     messageConfig
	Statusline  statusLineConfig
	Tabline     tabLineConfig
	Lint        lintConfig
	Popupmenu   popupMenuConfig
	ScrollBar   scrollBarConfig
	MiniMap     miniMapConfig
	Markdown    markdownConfig
	SideBar     sideBarConfig
	Workspace   workspaceConfig
	FileExplore fileExploreConfig
}

type editorConfig struct {
	Width                    int
	Height                   int
	FontFamily               string
	FontSize                 int
	Linespace                int
	ExtCmdline               bool
	ExtPopupmenu             bool
	ExtTabline               bool
	ExtMessages              bool
	Clipboard                bool
	CachedDrawing            bool
	CacheSize                int
	DisableImeInNormal       bool
	GinitVim                 string
	StartFullscreen          bool
	StartMaximizedWindow     bool
	Macmeta                  bool
	Transparent              float64
	DrawBorder               bool
	DrawWindowSeparator      bool
	WindowSeparatorTheme     string
	WindowSeparatorGradient  bool
	WindowSeparatorColor     string
	SkipGlobalId             bool
	IndentGuide              bool
	IndentGuideIgnoreFtList  []string
	DrawBorderForFloatWindow bool
	DrawShadowForFloatWindow bool
	DesktopNotifications     bool
	DiffAddPattern           int
	DiffDeletePattern        int
	DiffChangePattern        int
	ClickEffect              bool
	BorderlessWindow         bool
	// ExtWildmenu            bool
	// ExtMultigrid           bool
}

type paletteConfig struct {
	AreaRatio              float64
	MaxNumberOfResultItems int
	Transparent            float64
}

type messageConfig struct {
	Transparent float64
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
	Left              []string
	Right             []string
}

type tabLineConfig struct {
	Visible  bool
	ShowIcon bool
}

type popupMenuConfig struct {
	ShowDetail  bool
	Total       int
	MenuWidth   int
	InfoWidth   int
	DetailWidth int
}

type lintConfig struct {
	Visible bool
}

type miniMapConfig struct {
	Visible bool
	Disable bool
	Width   int
}

type markdownConfig struct {
	CodeHlStyle         string
	CodeWithLineNumbers bool
}

type scrollBarConfig struct {
	Visible bool
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

type fileExploreConfig struct {
	OpenCmd         string
	MaxDisplayItems int
}

func newGonvimConfig(configDir string) gonvimConfig {
	var config gonvimConfig

	config.init()

	// Read toml
	_, err := toml.DecodeFile(filepath.Join(configDir, "setting.toml"), &config)
	if err != nil {
		fmt.Println(err)
	}

	// Setting ExtMessages to true should automatically set ExtCmdLine to true as well
	// Ref: https://github.com/akiyosi/goneovim/issues/162
	if config.Editor.ExtMessages {
		config.Editor.ExtCmdline = true
	}

	if config.Editor.Transparent < 1.0 {
		config.Editor.DrawWindowSeparator = true
		config.Editor.BorderlessWindow = true
	}

	if config.Editor.DiffAddPattern < 1 || config.Editor.DiffAddPattern > 24 {
		config.Editor.DiffAddPattern = 1
	}
	if config.Editor.DiffDeletePattern < 1 || config.Editor.DiffDeletePattern > 24 {
		config.Editor.DiffDeletePattern = 1
	}
	if config.Editor.DiffChangePattern < 1 || config.Editor.DiffChangePattern > 24 {
		config.Editor.DiffChangePattern = 1
	}

	if config.Editor.Width <= 400 {
		config.Editor.Width = 400
	}
	if config.Editor.Height <= 300 {
		config.Editor.Height = 300
	}
	if config.Editor.Transparent <= 0.1 {
		config.Editor.Transparent = 1.0
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
	if config.Editor.FontSize <= 3 {
		config.Editor.FontSize = 12
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
		config.SideBar.Width = 200
	}
	if config.SideBar.AccentColor == "" {
		config.SideBar.AccentColor = "#5596ea"
	}

	if config.FileExplore.MaxDisplayItems < 1 {
		config.FileExplore.MaxDisplayItems = 1
	}

	if config.Workspace.PathStyle == "" {
		config.Workspace.PathStyle = "minimum"
	}

	if config.MiniMap.Width == 0 || config.MiniMap.Width >= 250 {
		config.MiniMap.Width = 120
	}

	return config
}

func (c *gonvimConfig) init() {
	// Set default value
	c.Editor.Width = 800
	c.Editor.Height = 600
	c.Editor.Transparent = 1.0
	c.Editor.BorderlessWindow = false

	c.Editor.SkipGlobalId = false
	c.Editor.CachedDrawing = true
	c.Editor.CacheSize = 320

	c.Editor.ExtCmdline = false
	c.Editor.ExtPopupmenu = false
	c.Editor.ExtTabline = false
	c.Editor.ExtMessages = false
	c.Editor.DrawWindowSeparator = false
	c.Editor.WindowSeparatorTheme = "dark"
	c.Editor.WindowSeparatorColor = ""
	c.Editor.WindowSeparatorGradient = false

	switch runtime.GOOS {
	case "windows":
		c.Editor.FontFamily = "Consolas"
	case "darwin":
		c.Editor.FontFamily = "Monaco"
	default:
		c.Editor.FontFamily = "Monospace"
	}
	c.Editor.FontSize = 12
	c.Editor.Linespace = 6

	// Indent guide
	c.Editor.IndentGuide = true
	c.Editor.IndentGuideIgnoreFtList = []string{"markdown", "md", "txt", "text", "help", "json", "nerdtree"}

	// replace diff color drawing pattern
	c.Editor.DiffAddPattern = 1
	c.Editor.DiffDeletePattern = 1

	// palette size
	c.Palette.AreaRatio = 0.5
	c.Palette.MaxNumberOfResultItems = 30
	c.Palette.Transparent = 1.0

	c.Message.Transparent = 1.0

	c.Statusline.Visible = false
	c.Statusline.ModeIndicatorType = "textLabel"
	c.Statusline.Left = []string{"mode", "filename"}
	c.Statusline.Right = []string{"git", "filetype", "fileformat", "fileencoding", "curpos", "lint"}

	c.Tabline.Visible = true

	c.Lint.Visible = false

	c.Popupmenu.ShowDetail = true
	c.Popupmenu.Total = 20
	c.Popupmenu.MenuWidth = 400
	c.Popupmenu.InfoWidth = 1
	c.Popupmenu.DetailWidth = 250

	// c.ActivityBar.Visible = true

	c.ScrollBar.Visible = false

	c.MiniMap.Width = 120

	c.Markdown.CodeHlStyle = "github"

	c.SideBar.Width = 200
	c.SideBar.AccentColor = "#5596ea"

	c.FileExplore.MaxDisplayItems = 30

	c.Workspace.PathStyle = "minimum"
}

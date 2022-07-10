package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/BurntSushi/toml"
)

type gonvimConfig struct {
	Editor      editorConfig
	SideBar     sideBarConfig
	Workspace   workspaceConfig
	Statusline  statusLineConfig
	FileExplore fileExploreConfig
	Popupmenu   popupMenuConfig
	Palette     paletteConfig
	MiniMap     miniMapConfig
	Cursor      cursorConfig
	Message     messageConfig
	mu          sync.RWMutex
	Tabline     tabLineConfig
	ScrollBar   scrollBarConfig
	Lint        lintConfig
}

type editorConfig struct {
	DockmenuActions                         map[string]string
	OptionsToUseGuideWidth                  string
	FileOpenCmd                             string
	WindowSeparatorColor                    string
	FontFamily                              string
	GinitVim                                string
	WindowSeparatorTheme                    string
	NvimInWsl                               string
	ModeEnablingIME                         []string
	IndentGuideIgnoreFtList                 []string
	Transparent                             float64
	DiffDeletePattern                       int
	DiffAddPattern                          int
	LineToScroll                            int
	DiffChangePattern                       int
	CacheSize                               int
	Linespace                               int
	Letterspace                             float64
	FontSize                                int
	Margin                                  int
	Gap                                     int
	Height                                  int
	Width                                   int
	SmoothScrollDuration                    int
	DrawWindowSeparator                     bool
	Macmeta                                 bool
	DrawBorder                              bool
	DisableLigatures                        bool
	StartMaximizedWindow                    bool
	WindowSeparatorGradient                 bool
	StartFullscreen                         bool
	SkipGlobalId                            bool
	IndentGuide                             bool
	DisableImeInNormal                      bool
	CachedDrawing                           bool
	Clipboard                               bool
	ReversingScrollDirection                bool
	SmoothScroll                            bool
	DisableHorizontalScroll                 bool
	DrawBorderForFloatWindow                bool
	DrawShadowForFloatWindow                bool
	DesktopNotifications                    bool
	ExtMessages                             bool
	ExtTabline                              bool
	ExtPopupmenu                            bool
	ClickEffect                             bool
	BorderlessWindow                        bool
	RestoreWindowGeometry                   bool
	ExtCmdline                              bool
	WorkAroundNeovimIssue12985              bool
	NoFontMerge                             bool
	WindowGeometryBasedOnFontmetrics        bool
	IgnoreFirstMouseClickWhenAppInactivated bool
	HideTitlebar                            bool
}

type cursorConfig struct {
	SmoothMove bool
	Duration   int
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
	VisualModeColor   string
	ModeIndicatorType string
	NormalModeColor   string
	CommandModeColor  string
	InsertModeColor   string
	ReplaceModeColor  string
	TerminalModeColor string
	Left              []string
	Right             []string
	Visible           bool
}

type tabLineConfig struct {
	Visible  bool
	ShowIcon bool
}

type popupMenuConfig struct {
	Total       int
	MenuWidth   int
	InfoWidth   int
	DetailWidth int
	ShowDetail  bool
	ShowDigit   bool
}

type lintConfig struct {
	Visible bool
}

type miniMapConfig struct {
	Visible bool
	Disable bool
	Width   int
}

type scrollBarConfig struct {
	Visible bool
	Width   int
	Color   string
}

type sideBarConfig struct {
	AccentColor string
	Width       int
	Visible     bool
	DropShadow  bool
}

type workspaceConfig struct {
	PathStyle      string
	RestoreSession bool
}

type fileExploreConfig struct {
	OpenCmd         string
	MaxDisplayItems int
}

func newConfig(home string) (string, gonvimConfig) {

	// detect config dir
	var configDir string
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		configDir = filepath.Join(xdgConfigHome, "goneovim")
	} else {
		configDir = filepath.Join(home, ".config", "goneovim")
	}
	if !isFileExist(configDir) {
		configDir = filepath.Join(home, ".goneovim")
	}

	var config gonvimConfig

	config.init()

	// Read toml
	configFilePath := filepath.Join(configDir, "settings.toml")
	if !isFileExist(configFilePath) {
		configFilePath = filepath.Join(configDir, "setting.toml")
	}
	_, err := toml.DecodeFile(configFilePath, &config)
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
		config.MiniMap.Width = 100
	}

	return configDir, config
}

func (c *gonvimConfig) init() {
	// For debug
	c.Editor.SkipGlobalId = false

	// Set default value
	c.Editor.BorderlessWindow = false
	c.Editor.RestoreWindowGeometry = false
	c.Editor.WindowGeometryBasedOnFontmetrics = false

	c.Editor.Width = 800
	c.Editor.Height = 600
	c.Editor.Gap = 0

	c.Editor.FileOpenCmd = ":e"

	c.Editor.Transparent = 1.0

	switch runtime.GOOS {
	case "windows":
		c.Editor.FontFamily = "Consolas"
		c.Editor.Margin = 2
	case "darwin":
		c.Editor.FontFamily = "Monaco"
		c.Editor.Margin = 2
	default:
		c.Editor.FontFamily = "Monospace"
		c.Editor.Margin = 0
	}
	c.Editor.FontSize = 12
	c.Editor.Linespace = 6

	c.Editor.ExtCmdline = false
	c.Editor.ExtPopupmenu = false
	c.Editor.ExtTabline = false
	c.Editor.ExtMessages = false

	c.Editor.CachedDrawing = true
	c.Editor.CacheSize = 480

	c.Editor.DisableLigatures = false
	c.Editor.Clipboard = true
	c.Editor.Macmeta = false
	c.Editor.DisableImeInNormal = false

	c.Editor.DrawWindowSeparator = false
	c.Editor.WindowSeparatorTheme = "dark"
	c.Editor.WindowSeparatorColor = "#2222ff"
	c.Editor.WindowSeparatorGradient = false

	// Indent guide
	c.Editor.IndentGuide = false
	c.Editor.IndentGuideIgnoreFtList = []string{"markdown", "md", "txt", "text", "help", "json", "nerdtree"}
	c.Editor.OptionsToUseGuideWidth = "tabstop"

	c.Editor.LineToScroll = 1
	c.Editor.SmoothScroll = false
	c.Editor.SmoothScrollDuration = 300
	c.Editor.DisableHorizontalScroll = false

	c.Editor.DrawBorderForFloatWindow = false
	c.Editor.DrawShadowForFloatWindow = false

	c.Editor.DesktopNotifications = false
	c.Editor.ClickEffect = false

	// replace diff color drawing pattern
	c.Editor.DiffAddPattern = 1
	c.Editor.DiffDeletePattern = 1
	c.Editor.DiffChangePattern = 1

	c.Cursor.Duration = 55

	// ----

	// palette size
	c.Palette.AreaRatio = 0.5
	c.Palette.MaxNumberOfResultItems = 30
	c.Palette.Transparent = 1.0

	// ----

	c.Message.Transparent = 1.0

	// ----

	c.Statusline.Visible = false
	c.Statusline.ModeIndicatorType = "textLabel"
	c.Statusline.Left = []string{"mode", "filepath", "filename"}
	c.Statusline.Right = []string{"git", "filetype", "fileformat", "fileencoding", "curpos", "lint"}

	// ----

	c.Tabline.Visible = true
	c.Tabline.ShowIcon = true

	// ----

	c.Lint.Visible = false

	// ----

	c.Popupmenu.ShowDetail = true
	c.Popupmenu.Total = 20
	c.Popupmenu.MenuWidth = 400
	c.Popupmenu.InfoWidth = 1
	c.Popupmenu.DetailWidth = 250

	// ----

	c.ScrollBar.Visible = false
	c.ScrollBar.Width = 10

	// ----

	c.MiniMap.Width = 110

	// ----

	c.SideBar.Visible = false
	c.SideBar.Width = 200
	c.SideBar.AccentColor = "#5596ea"

	// ----

	c.FileExplore.MaxDisplayItems = 30

	// ----

	c.Workspace.PathStyle = "minimum"
	c.Workspace.RestoreSession = false
}

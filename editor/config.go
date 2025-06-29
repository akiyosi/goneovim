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
	FileExplore fileExploreConfig
	Popupmenu   popupMenuConfig
	Palette     paletteConfig
	MiniMap     miniMapConfig
	Cursor      cursorConfig
	Message     messageConfig
	mu          sync.RWMutex
	Tabline     tabLineConfig
	ScrollBar   scrollBarConfig
}

type editorConfig struct {
	DockmenuActions                         map[string]string
	MouseScrollingUnit                      string
	OptionsToUseGuideWidth                  string
	FileOpenCmd                             string
	WindowSeparatorColor                    string
	FontFamily                              string
	GinitVim                                string
	WindowSeparatorTheme                    string
	NvimInWsl                               string
	WSLDist                                 string
	FontWeight                              string
	ModeEnablingIME                         []string
	IndentGuideIgnoreFtList                 []string
	CharsScaledLineHeight                   []string
	Transparent                             float64
	EnableBackgroundBlur                    bool
	DiffDeletePattern                       int
	DiffAddPattern                          int
	LineToScroll                            int
	DiffChangePattern                       int
	CacheSize                               int
	Linespace                               int
	Letterspace                             int
	FontSize                                int
	FontStretch                             int
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
	ManualFontFallback                      bool
	WindowGeometryBasedOnFontmetrics        bool
	IgnoreFirstMouseClickWhenAppInactivated bool
	HideTitlebar                            bool
	HideMouseWhenTyping                     bool
	IgnoreSaveConfirmationWithCloseButton   bool
	UseWSL                                  bool
	ShowDiffDialogOnDrop                    bool
	ProportionalFontAlignGutter             bool
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
	Transparent           float64
	ShowMessageSeparators bool
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

func newConfig(home string, skipConfigLoading bool) (string, gonvimConfig) {

	// init
	var config gonvimConfig
	config.init()

	// detect configdir, configfile
	configDir, configFilePath := detectConfig(home)

	if !skipConfigLoading {
		// load toml
		_, err := toml.DecodeFile(configFilePath, &config)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Setting ExtMessages to true should automatically set ExtCmdLine to true as well
	// Ref: https://github.com/akiyosi/goneovim/issues/162
	if config.Editor.ExtMessages {
		config.Editor.ExtCmdline = true
	}

	if config.Editor.EnableBackgroundBlur {
		config.Editor.Transparent = 0.9
	}

	if config.Editor.Transparent < 1.0 {
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

	editor.putLog("reading config")

	return configDir, config
}

func detectConfig(home string) (configDir, configFilePath string) {
	// detect config dir
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	settingsfile := "settings.toml"
	if runtime.GOOS != "windows" {
		if xdgConfigHome != "" {
			configDir = filepath.Join(xdgConfigHome, "goneovim")
		} else {
			configDir = filepath.Join(home, ".config", "goneovim")
		}
		configFilePath = filepath.Join(configDir, settingsfile)

		return
	} else {
		if xdgConfigHome != "" {
			configDir = filepath.Join(xdgConfigHome, "goneovim")
			configFilePath = filepath.Join(xdgConfigHome, "goneovim", settingsfile)
		}
		if isFileExist(configFilePath) {
			return
		}

		localappdata := os.Getenv("LOCALAPPDATA")
		configDir = filepath.Join(localappdata, "goneovim")
		configFilePath = filepath.Join(localappdata, "goneovim", settingsfile)
		if isFileExist(configFilePath) {
			return
		}

		configDir = filepath.Join(home, ".config", "goneovim")
		configFilePath = filepath.Join(home, ".config", "goneovim", settingsfile)
		if isFileExist(configFilePath) {
			return
		}

		configDir = filepath.Join(home, ".goneovim")
		configFilePath = filepath.Join(home, ".goneovim", settingsfile)
		if isFileExist(configFilePath) {
			return
		}
	}

	// windows
	localappdata := os.Getenv("LOCALAPPDATA")
	configDir = filepath.Join(localappdata, "goneovim")
	configFilePath = filepath.Join(localappdata, "goneovim", settingsfile)

	return
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
		c.Editor.MouseScrollingUnit = "line"
	case "darwin":
		c.Editor.FontFamily = "Monaco"
		c.Editor.Margin = 2
		c.Editor.MouseScrollingUnit = "smart"
	default:
		c.Editor.FontFamily = "Monospace"
		c.Editor.Margin = 0
		c.Editor.MouseScrollingUnit = "line"
	}
	c.Editor.FontSize = 12
	c.Editor.FontWeight = "normal"
	// Horizontal stretch ratio
	c.Editor.FontStretch = 100
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
	c.Editor.CharsScaledLineHeight = []string{"", "", "", "", "", "", "", "", "", "", "│", "▎"}
	c.Editor.OptionsToUseGuideWidth = "tabstop"

	c.Editor.LineToScroll = 1
	c.Editor.SmoothScroll = false
	c.Editor.SmoothScrollDuration = 800
	c.Editor.DisableHorizontalScroll = false

	c.Editor.DrawBorderForFloatWindow = false
	c.Editor.DrawShadowForFloatWindow = false

	c.Editor.DesktopNotifications = false
	c.Editor.ClickEffect = false

	// replace diff color drawing pattern
	c.Editor.DiffAddPattern = 1
	c.Editor.DiffDeletePattern = 1
	c.Editor.DiffChangePattern = 1

	c.Cursor.Duration = 180

	// ----

	// palette size
	c.Palette.AreaRatio = 0.5
	c.Palette.MaxNumberOfResultItems = 30
	c.Palette.Transparent = 1.0

	// ----

	c.Message.Transparent = 1.0

	// ----

	c.Tabline.Visible = true
	c.Tabline.ShowIcon = true

	// ----

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

	c.MiniMap.Disable = true
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

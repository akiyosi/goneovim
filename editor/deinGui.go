package editor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/BurntSushi/toml"
	"github.com/akiyosi/tomlwriter"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/neovim/go-client/nvim"
	"gopkg.in/yaml.v2"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type deinsideSignal struct {
	core.QObject
	_ func() `signal:"searchSignal"`
	_ func() `signal:"deinInstallSignal"`
	_ func() `signal:"deinUpdateSignal"`
}

// DeinSide is the side bar witch is GUI for Shougo/dein.vim
type DeinSide struct {
	signal            *deinsideSignal
	searchUpdates     chan PluginSearchResults
	searchPageUpdates chan int
	deinInstall       chan string
	deinUpdate        chan string
	installing        bool

	widget       *widgets.QWidget
	layout       *widgets.QLayout
	header       *widgets.QWidget
	contextMenu  *widgets.QMenu
	scrollarea   *widgets.QScrollArea
	searchlayout *widgets.QBoxLayout
	// searchbox        *widgets.QLineEdit
	combobox           *SearchComboBox
	searchbox          *Searchbox
	progressbar        *widgets.QProgressBar
	plugincontent      *widgets.QStackedWidget
	searchresult       *Searchresult
	installedplugins   *InstalledPlugins
	configIcon         *svg.QSvgWidget
	preSearchKeyword   string
	preDisplayedReadme string
	deintoml           DeinTomlConfig
	deintomlbare       []byte
}

// SearchComboBox is the ComboBox widget for DeinSide
type SearchComboBox struct {
	widget   *widgets.QWidget
	layout   *widgets.QHBoxLayout
	comboBox *widgets.QComboBox
}

// Searchbox is the search box widget for DeinSide
type Searchbox struct {
	widget  *widgets.QWidget
	layout  *widgets.QHBoxLayout
	editBox *widgets.QLineEdit
}

// DeinPluginItem is the item structure witch is installed plugin of Shougo/dein.vim
type DeinPluginItem struct {
	widget          *widgets.QWidget
	updateLabel     *widgets.QStackedWidget
	updateLabelName *widgets.QLabel
	updateButton    *widgets.QWidget
	waitingLabel    *widgets.QWidget
	nameLabel       *widgets.QLabel
	contextMenu     *widgets.QMenu

	itemname       string
	lazy           bool
	path           string
	repo           string
	hookadd        string
	merged         bool
	normalizedName string
	pluginType     string
	rtp            string
	sourced        bool
	name           string
}

// InstalledPlugins is the widget witch displays installed plugins in DeinSide
type InstalledPlugins struct {
	widget *widgets.QWidget
	items  []*DeinPluginItem
}

func readDeinCache() (map[interface{}]interface{}, error) {
	w := editor.workspaces[editor.active]
	basePath, err := w.nvim.CommandOutput("echo g:dein#_base_path")
	if err != nil {
		return nil, err
	}

	if isFileExist(basePath+"/cache_nvim") == false {
		editor.workspaces[editor.active].nvim.Command("call dein#util#_save_cache(g:dein#_vimrcs, 1, 1)")
	}
	file, err := os.Open(basePath + "/cache_nvim")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if editor.config.Dein.TomlFile == "" {
		detectTomlFile(lines[0])
	}

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(lines[1]), &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func detectTomlFile(s string) {
	filesString := strings.Replace(s, `['`, "", 1)
	filesString = strings.Replace(filesString, `']`, "", 1)
	filesString = strings.Replace(filesString, `', '`, ",", -1)
	for i, file := range strings.Split(filesString, ",") {
		if strings.Contains(file[len(file)-5:], ".toml") && !strings.Contains(file, "lazy") {
			confirmDeinGUITomlFile(file)
			break
		}
		if i == len(strings.Split(filesString, ","))-1 {
			editor.pushNotification(NotifyWarn, -1, "[Gonvim] I can't find the dein.vim toml file. Please specify loading of toml file in dein.vim setting.")
		}
	}
}

func confirmDeinGUITomlFile(file string) {
	outputGonvimConfig()
	home, err := homedir.Dir()
	if err != nil {
		home = "~"
	}
	setting := filepath.Join(home, ".gonvim", "setting.toml")

	message := fmt.Sprintf("[Gonvim] Detect the dein.vim toml file: %s, Would you like to use this file with Dein.vim GUI?", file)
	opts := []*NotifyButton{}
	opt1 := &NotifyButton{
		action: func() {
			b, _ := ioutil.ReadFile(setting)
			b, _ = tomlwriter.WriteValue(`'`+file+`'`, b, "Dein", "TomlFile", "")
			editor.config.Dein.TomlFile = file
			err := ioutil.WriteFile(setting, b, 0664)
			if err != nil {
				fmt.Println(err)
			}
		},
		text: "Yes",
	}
	opts = append(opts, opt1)

	opt2 := &NotifyButton{
		action: func() {
			go editor.workspaces[editor.active].nvim.Command(fmt.Sprintf(":-1tabnew %s", setting))
		},
		text: "No, I will use another file",
	}
	opts = append(opts, opt2)

	editor.pushNotification(NotifyWarn, 0, message, notifyOptionArg(opts))
}

func loadDeinCashe() []*DeinPluginItem {
	labelColor := darkenHex(editor.config.SideBar.AccentColor)

	m, _ := readDeinCache()
	installedPlugins := []*DeinPluginItem{}
	// width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()
	width := editor.config.SideBar.Width

	var keys []string
	for k, _ := range m {
		keys = append(keys, k.(string))
	}
	sort.Strings(keys)

	// for name, item := range m {
	for _, k := range keys {
		item := m[k]
		s, _ := item.(map[interface{}]interface{})
		i := &DeinPluginItem{}
		i.itemname = k
		for key, value := range s {
			switch key {
			case "lazy":
				if value == 0 {
					i.lazy = false
				} else {
					i.lazy = true
				}
			case "path":
				i.path = value.(string)
			case "repo":
				i.repo = value.(string)
			case "hookadd":
				i.hookadd = value.(string)
			case "merged":
				if value == 0 {
					i.merged = false
				} else {
					i.merged = true
				}
			case "normalized_name":
				i.normalizedName = value.(string)
			case "type":
				i.pluginType = value.(string)
			case "rtp":
				i.rtp = value.(string)
			case "sourced":
				if value == 0 {
					i.sourced = false
				} else {
					i.sourced = true
				}
			case "name":
				i.name = value.(string)
			}
		}

		// width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()

		// make widgets
		installedPluginWidget := widgets.NewQWidget(nil, 0)
		installedPluginLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, installedPluginWidget)
		installedPluginLayout.SetContentsMargins(20, 5, 20, 5)
		installedPluginLayout.SetSpacing(3)
		installedPluginWidget.Hide()
		// installedPluginWidget.SetFixedWidth(editor.config.sideWidth)
		// installedPluginWidget.SetMaximumWidth(editor.config.sideWidth - 55)
		// installedPluginWidget.SetMinimumWidth(editor.config.sideWidth - 55)
		// installedPluginWidget.SetMaximumWidth(width)
		// installedPluginWidget.SetMinimumWidth(width)

		// plugin mame
		installedPluginName := widgets.NewQLabel(nil, 0)
		installedPluginName.SetText(i.name)
		installedPluginName.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		fg := editor.colors.fg
		installedPluginName.SetStyleSheet(fmt.Sprintf(" .QLabel { color: rgba(%d, %d, %d, 1);} ", fg.R, fg.G, fg.B))
		installedPluginName.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 3, false))
		installedPluginName.SetFixedWidth(width)

		i.nameLabel = installedPluginName

		// plugin desc
		installedPluginDescLabel := widgets.NewQLabel(nil, 0)
		installedPluginDescLabel.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		go func() {
			desc := getRepoDesc(strings.ToLower(strings.Split(i.repo, "/")[0]), i.name)
			installedPluginDescLabel.SetText(desc)
			installedPluginWidget.Show()
		}()
		installedPluginDescLabel.SetWordWrap(true)
		installedPluginDescLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
		installedPluginDesc := widgets.NewQWidget(nil, 0)
		installedPluginDescLayout := widgets.NewQHBoxLayout()
		installedPluginDescLayout.AddWidget(installedPluginDescLabel, 0, 0)
		installedPluginDescLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginDesc.SetLayout(installedPluginDescLayout)

		// * plugin install button
		updateButtonLabel := widgets.NewQLabel(nil, 0)
		updateButtonLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
		updateButtonLabel.SetFixedWidth(7 * editor.config.Editor.FontSize)
		updateButtonLabel.SetFixedHeight(2 * (editor.config.Editor.FontSize - 2))
		updateButtonLabel.SetContentsMargins(editor.config.Editor.FontSize/2, 0, editor.config.Editor.FontSize/2, 0)
		updateButtonLabel.SetAlignment(core.Qt__AlignCenter)
		updateButton := widgets.NewQWidget(nil, 0)
		updateButtonLayout := widgets.NewQHBoxLayout()
		updateButtonLayout.SetContentsMargins(0, 0, 0, 0)
		updateButtonLayout.AddWidget(updateButtonLabel, 0, 0)
		updateButton.SetLayout(updateButtonLayout)
		updateButton.SetObjectName("updatebutton")
		updateButtonLabel.SetText("Update")
		updateButton.SetStyleSheet(fmt.Sprintf(" #updatebutton QLabel { color: #ffffff; background: %s;} ", labelColor))

		updateButton.ConnectMousePressEvent(func(*gui.QMouseEvent) {
			go i.deinUpdatePre(i.name)
		})
		updateButton.ConnectEnterEvent(func(event *core.QEvent) {
			updateButton.SetStyleSheet(fmt.Sprintf(" #updatebutton QLabel { color: #ffffff; background: %s;} ", editor.config.SideBar.AccentColor))
		})
		updateButton.ConnectLeaveEvent(func(event *core.QEvent) {
			labelColor := darkenHex(editor.config.SideBar.AccentColor)
			updateButton.SetStyleSheet(fmt.Sprintf(" #updatebutton QLabel { color: #ffffff; background: %s;} ", labelColor))
		})
		i.updateButton = updateButton

		// waiting label while update pugin
		updateWaiting := widgets.NewQWidget(nil, 0)
		updateWaiting.SetFixedWidth(7 * editor.config.Editor.FontSize)
		updateWaiting.SetFixedHeight(2 * (editor.config.Editor.FontSize - 2))
		updateWaitingLayout := widgets.NewQHBoxLayout()
		updateWaitingLayout.SetContentsMargins(editor.config.Editor.FontSize/2, 0, editor.config.Editor.FontSize/2, 0)
		updateWaiting.SetLayout(updateWaitingLayout)
		waitingLabel := widgets.NewQProgressBar(nil)
		bg := editor.colors.bg
		waitingLabel.SetStyleSheet(fmt.Sprintf(" QProgressBar { border: 0px; background: %s; } QProgressBar::chunk { background-color: %s; } ", shiftHex(editor.config.SideBar.AccentColor, 25), editor.config.SideBar.AccentColor))
		waitingLabel.SetRange(0, 0)
		updateWaitingLayout.AddWidget(waitingLabel, 0, 0)

		i.waitingLabel = updateWaiting

		// stack installButton & waitingLabel
		updateLabel := widgets.NewQStackedWidget(nil)
		updateLabel.SetContentsMargins(0, 0, 0, 0)

		i.updateLabel = updateLabel

		// ** Lazy plugin icon
		installedPluginLazy := widgets.NewQWidget(nil, 0)
		installedPluginLazyLayout := widgets.NewQHBoxLayout()
		installedPluginLazyLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginLazyLayout.SetSpacing(1)
		installedPluginLazyIcon := svg.NewQSvgWidget(nil)
		iconSize := editor.iconSize
		installedPluginLazyIcon.SetFixedSize2(iconSize, iconSize)
		var svgLazyContent string
		if i.lazy == false {
			svgLazyContent = editor.getSvg("timer", editor.colors.sideBarBg)
		} else {
			svgLazyContent = editor.getSvg("timer", fg)
		}
		installedPluginLazyIcon.Load2(core.NewQByteArray2(svgLazyContent, len(svgLazyContent)))
		installedPluginLazyLayout.AddWidget(installedPluginLazyIcon, 0, 0)
		installedPluginLazy.SetLayout(installedPluginLazyLayout)

		// ** installedPlugin sourced
		installedPluginSourced := widgets.NewQWidget(nil, 0)
		installedPluginSourcedLayout := widgets.NewQHBoxLayout()
		installedPluginSourcedLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginSourcedLayout.SetSpacing(1)
		installedPluginSourcedIcon := svg.NewQSvgWidget(nil)
		installedPluginSourcedIcon.SetFixedSize2(iconSize-1, iconSize-1)
		var svgSourcedContent string
		if i.sourced == false {
			svgSourcedContent = editor.getSvg("puzzle", editor.colors.sideBarBg)
		} else {
			svgSourcedContent = editor.getSvg("puzzle", fg)
		}
		installedPluginSourcedIcon.Load2(core.NewQByteArray2(svgSourcedContent, len(svgSourcedContent)))
		installedPluginSourcedLayout.AddWidget(installedPluginSourcedIcon, 0, 0)
		installedPluginSourced.SetLayout(installedPluginSourcedLayout)

		// ** installedPlugin setting
		installedPluginSettings := widgets.NewQWidget(nil, 0)
		installedPluginSettingsLayout := widgets.NewQHBoxLayout()
		installedPluginSettingsLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginSettingsLayout.SetSpacing(1)
		installedPluginSettingsIcon := svg.NewQSvgWidget(nil)
		installedPluginSettingsIcon.SetFixedSize2(iconSize+1, iconSize+1)
		svgSettingsContent := editor.getSvg("settings", fg)
		installedPluginSettingsIcon.Load2(core.NewQByteArray2(svgSettingsContent, len(svgSettingsContent)))
		installedPluginSettingsLayout.AddWidget(installedPluginSettingsIcon, 0, 0)
		installedPluginSettings.SetLayout(installedPluginSettingsLayout)
		installedPluginSettings.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
			if i.contextMenu == nil {
				i.contextMenu = widgets.NewQMenu(installedPluginSettings)
				i.contextMenu.SetStyleSheet(fmt.Sprintf(" QMenu { padding: 5px 0px 5px 0px; border-radius:6px; background-color: %s;} QMenu::item { padding: 2px 20px 2px 20px; background-color: transparent; color: %s;} QMenu::item:selected { padding: 2px 20px 2px 20px; background-color: #257afd; color: #eeeeee; } ", fg.String(), bg.String()))
				i.contextMenu.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
				// Example menu and action
				// menuActionExit := i.contextMenu.AddAction("E&xit")
				// menuActionExit.SetShortcut(gui.NewQKeySequence2("Ctrl+X", gui.QKeySequence__NativeText))
				// menuActionExit.ConnectTriggered(func(checked bool) { fmt.Println("some action") })

				// menuActionDisable := i.contextMenu.AddAction("Disable")
				// menuActionDisable.ConnectTriggered(i.disable)
				menuActionEditToml := i.contextMenu.AddAction("Configure the toml file")
				menuActionRemove := i.contextMenu.AddAction("Remove this plugin")
				var invalidConfigure bool
				for _, pluginSetting := range editor.deinSide.deintoml.Plugins {
					if pluginSetting.Repo == i.repo {
						invalidConfigure = true
						break
					}
				}
				if !invalidConfigure {
					i.contextMenu.SetStyleSheet(fmt.Sprintf(" QMenu { padding: 5px 0px 5px 0px; border-radius:6px; background-color: %s;} QMenu::item { padding: 2px 20px 2px 20px; background-color: transparent; color: %s; } ", editor.colors.fg.String(), editor.colors.inactiveFg.String()))
				} else {
					menuActionRemove.ConnectTriggered(i.remove)
					menuActionEditToml.ConnectTriggered(i.editToml)
				}
			}
			p := event.Pos()
			i.contextMenu.Exec2(installedPluginSettings.MapToGlobal(p), nil)
		})
		installedPluginSettings.ConnectEnterEvent(func(event *core.QEvent) {
			svgSettingsContent := editor.getSvg("settings", hexToRGBA(editor.config.SideBar.AccentColor))
			installedPluginSettingsIcon.Load2(core.NewQByteArray2(svgSettingsContent, len(svgSettingsContent)))
		})
		installedPluginSettings.ConnectLeaveEvent(func(event *core.QEvent) {
			svgSettingsContent := editor.getSvg("settings", fg)
			installedPluginSettingsIcon.Load2(core.NewQByteArray2(svgSettingsContent, len(svgSettingsContent)))
		})

		// * installedPlugin name & some option icon
		installedPluginHead := widgets.NewQWidget(nil, 0)

		// spacing, padding, paddingtop, rightitemnum, width
		// installedPluginHeadLayout := newVFlowLayout(2, 2, 1, 1, 0)
		installedPluginHeadLayout := widgets.NewQHBoxLayout()
		installedPluginHeadLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginHead.SetLayout(installedPluginHeadLayout)

		installedPluginStatus := widgets.NewQWidget(nil, 0)
		installedPluginStatus.SetMaximumWidth(4 + 3*iconSize)
		installedPluginStatus.SetMinimumWidth(4 + 3*iconSize)
		installedPluginStatusLayout := widgets.NewQHBoxLayout2(nil)
		installedPluginStatus.SetLayout(installedPluginStatusLayout)
		installedPluginStatusLayout.SetSpacing(0)
		installedPluginStatusLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginStatusLayout.AddWidget(installedPluginLazy, 0, 0)
		installedPluginStatusLayout.AddWidget(installedPluginSourced, 0, 0)
		installedPluginStatusLayout.AddWidget(installedPluginSettings, 0, 0)

		// ** installedPlugin foot
		installedPluginFoot := widgets.NewQWidget(nil, 0)
		installedPluginFootLayout := widgets.NewQHBoxLayout()
		installedPluginFootLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginFootLayout.AddWidget(installedPluginDesc, 0, 0)
		installedPluginFootLayout.AddWidget(installedPluginStatus, 0, 0)
		installedPluginFoot.SetLayout(installedPluginFootLayout)

		installedPluginHeadLayout.AddWidget(installedPluginName, 0, 0)
		installedPluginHeadLayout.AddWidget(i.updateLabel, 0, 0)

		i.updateLabel.AddWidget(i.updateButton)
		i.updateLabel.AddWidget(i.waitingLabel)
		i.updateLabel.SetCurrentWidget(i.updateButton)

		installedPluginLayout.AddWidget(installedPluginHead, 0, 0)
		installedPluginLayout.AddWidget(installedPluginFoot, 0, 0)
		i.widget = installedPluginWidget

		i.widget.ConnectEnterEvent(i.enterWidget)
		i.widget.ConnectLeaveEvent(i.leaveWidget)

		installedPlugins = append(installedPlugins, i)
	}

	return installedPlugins
}

func getRepoDesc(owner string, name string) string {
	var results PluginSearchResults
	response, err := http.Get(fmt.Sprintf("http://vimawesome.com/api/plugins?query=%v", name))
	if err != nil {
		return ""
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&results); err != nil {
		fmt.Println("JSON decode error:", err)
		return ""
	}

	var text string
	for _, p := range results.Plugins {
		if strings.ToLower(p.GithubOwner) != owner {
			continue
		}
		if strings.ToLower(p.GithubOwner) == owner {
			text = p.ShortDesc
			break
		}
	}

	return text
}

type DeinTomlConfig struct {
	Plugins []DeinPlugin
}

type DeinPlugin struct {
	Repo           string
	AuGroup        string `toml:"augroup"`
	Build          string
	Depends        []string
	Frozen         int
	If             interface{}
	Lazy           int
	Marged         int
	Name           string
	NormalizedName string      `toml:"normalized_name"`
	OnCmd          interface{} `toml:"on_cmd"`
	OnEvent        interface{} `toml:"on_event"`
	OnFunc         interface{} `toml:"on_func"`
	OnFt           interface{} `toml:"on_ft"`
	OnI            int         `toml:"on_i"`
	OnIdle         int         `toml:"on_idle"`
	OnIf           string      `toml:"on_if"`
	OnMap          interface{} `toml:"on_map"`
	OnPath         interface{} `toml:"on_path"`
	OnSource       interface{} `toml:"on_source"`
	Path           string
	Rev            string
	Rtp            string
	ScriptType     string `toml:"script_type"`
	Timeout        int
	Trusted        int
	Type           string
	TypeDepth      int         `toml:"type__depth"`
	HookAdd        interface{} `toml:"hook_add"`
	HookDoneUpdate interface{} `toml:"hook_done_update"`
	HookPostSource interface{} `toml:"hook_post_source"`
	HookPostUpdate interface{} `toml:"hook_post_update"`
	HookSource     interface{} `toml:"hook_source"`
}

func newDeinSide() *DeinSide {
	fg := editor.colors.fg
	bg := editor.colors.bg
	sbg := editor.colors.sideBarSelectedItemBg

	layout := newHFlowLayout(0, 0, 0, 0, 20)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	headerWidget := widgets.NewQWidget(nil, 0)
	headerLayout := widgets.NewQHBoxLayout()
	headerLayout.SetContentsMargins(20, 15, 20, 5)
	header := widgets.NewQLabel(nil, 0)
	//header.SetContentsMargins(20, 15, 20, 5)
	header.SetContentsMargins(0, 0, 0, 0)
	header.SetText("DEIN.VIM")
	configIcon := svg.NewQSvgWidget(nil)
	configIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	svgConfigContent := editor.getSvg("configfile", editor.colors.inactiveFg)
	configIcon.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
	headerLayout.AddWidget(header, 0, 0)
	headerLayout.AddWidget(configIcon, 0, 0)
	headerWidget.SetLayout(headerLayout)
	header.SetStyleSheet(fmt.Sprintf(" .QLabel{ color: %s;} ", fg.String()))
	header.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))

	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)

	searchWidget := widgets.NewQWidget(nil, 0)
	searchWidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Minimum)
	searchLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, searchWidget)
	searchLayout.SetContentsMargins(0, 5, 0, 5)

	comboBoxLayout := widgets.NewQHBoxLayout()
	comboBoxLayout.SetContentsMargins(20, 5, 20, 0)
	comboBoxLayout.SetSpacing(0)
	comboBoxMenu := widgets.NewQComboBox(nil)
	comboBoxMenu.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	comboBoxMenu.SetMinimumHeight(editor.config.Editor.FontSize + 3)
	comboBoxMenu.AddItems([]string{"ALL", "Language", "Completion", "Code-display", "Integrations", "Interface", "Commands", "Other"})
	comboBoxMenu.SetFocusPolicy(core.Qt__ClickFocus)
	comboBoxMenu.SetStyleSheet(fmt.Sprintf(" * { padding-top: 1px; padding-left: 2px; border: 1px solid %s; border-radius: 1; selection-background-color: %s; background-color: %s; color: %s;} ", editor.config.SideBar.AccentColor, sbg, bg, fg))
	comboBoxLayout.AddWidget(comboBoxMenu, 0, 0)
	comboBoxWidget := widgets.NewQWidget(nil, 0)
	comboBoxWidget.SetLayout(comboBoxLayout)

	combobox := &SearchComboBox{
		widget:   comboBoxWidget,
		layout:   comboBoxLayout,
		comboBox: comboBoxMenu,
	}

	searchBoxLayout := widgets.NewQHBoxLayout()
	searchBoxLayout.SetContentsMargins(20, 0, 20, 0)
	searchBoxLayout.SetSpacing(0)
	searchboxEdit := widgets.NewQLineEdit(nil)
	searchboxEdit.SetMinimumHeight(editor.config.Editor.FontSize + 3)
	searchboxEdit.SetPlaceholderText("Search Plugins in VimAwesome")
	searchboxEdit.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	searchboxEdit.SetFocusPolicy(core.Qt__ClickFocus)
	// searchboxEdit.SetFixedWidth(editor.config.sideWidth - (20 + 20))
	// searchboxEdit.SetFixedWidth(width - (20 + 20))
	searchBoxLayout.AddWidget(searchboxEdit, 0, 0)
	searchBoxWidget := widgets.NewQWidget(nil, 0)
	searchBoxWidget.SetLayout(searchBoxLayout)

	searchboxEdit.ConnectReturnPressed(func() {
		doPluginSearch()
	})

	searchbox := &Searchbox{
		widget:  searchBoxWidget,
		layout:  searchBoxLayout,
		editBox: searchboxEdit,
	}

	searchLayout.AddWidget(combobox.widget, 0, 0)
	searchLayout.AddWidget(searchbox.widget, 0, 0)

	waitingWidget := widgets.NewQWidget(nil, 0)
	// waitingWidget.SetMaximumWidth(100)
	// waitingWidget.SetMinimumWidth(100)
	waitingLayout := widgets.NewQHBoxLayout()
	waitingLayout.SetContentsMargins(20, 0, 20, 5)
	waitingWidget.SetLayout(waitingLayout)
	pbar := widgets.NewQProgressBar(nil)
	pbar.SetStyleSheet(fmt.Sprintf(" QProgressBar { padding: 20 1 0 1; height: 1px; border: 0px; background: %s; } QProgressBar::chunk { background-color: %s; } ", editor.colors.sideBarBg.String(), editor.config.SideBar.AccentColor))
	pbar.SetRange(0, 0)
	waitingLayout.AddWidget(pbar, 0, 0)
	pbar.Hide()

	content := widgets.NewQStackedWidget(nil)
	content.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Minimum)

	layout.AddWidget(headerWidget)
	layout.AddWidget(searchWidget)
	layout.AddWidget(waitingWidget)
	layout.AddWidget(content)

	installed := newInstalledPlugins()
	searchresultWidget := widgets.NewQWidget(nil, 0)
	searchresult := &Searchresult{
		widget: searchresultWidget,
	}

	deinTomlBare, deinToml := readDeinToml()

	// make DeinSide
	side := &DeinSide{
		signal:            NewDeinsideSignal(nil),
		searchUpdates:     make(chan PluginSearchResults, 1000),
		searchPageUpdates: make(chan int, 1000),
		deinInstall:       make(chan string, 20000),
		deinUpdate:        make(chan string, 20000),

		widget:           widget,
		layout:           layout,
		header:           headerWidget,
		combobox:         combobox,
		searchbox:        searchbox,
		searchlayout:     searchLayout,
		progressbar:      pbar,
		plugincontent:    content,
		searchresult:     searchresult,
		installedplugins: installed,
		configIcon:       configIcon,
		deintoml:         deinToml,
		deintomlbare:     deinTomlBare,
	}
	side.signal.ConnectSearchSignal(func() {
		updates := <-side.searchUpdates
		pagenum := <-side.searchPageUpdates
		drawSearchresults(updates, pagenum)
	})
	side.signal.ConnectDeinInstallSignal(func() {
		result := <-side.deinInstall
		deinInstallPost(result)
	})
	side.signal.ConnectDeinUpdateSignal(func() {
		result := <-side.deinUpdate
		deinUpdatePost(result)
	})
	content.AddWidget(side.searchresult.widget)
	content.AddWidget(side.installedplugins.widget)
	content.SetCurrentWidget(side.installedplugins.widget)

	configIcon.ConnectEnterEvent(side.enterConfigIcon)
	configIcon.ConnectLeaveEvent(side.leaveConfigIcon)
	configIcon.ConnectMousePressEvent(side.pressConfigIcon)

	deinSideStyle := fmt.Sprintf("QWidget {	color: %s; border-right: 0px solid; }", editor.colors.comment.String())
	side.widget.SetStyleSheet(fmt.Sprintf(".QWidget {padding-top: 5px; background-color: %s; }", editor.colors.sideBarBg.String()) + deinSideStyle)
	side.searchbox.editBox.SetStyleSheet(fmt.Sprintf(".QLineEdit { border: 1px solid %s; border-radius: 1px; background: %s; selection-background-color: %s; color: %s; }", editor.config.SideBar.AccentColor, editor.colors.sideBarBg.String(), editor.colors.sideBarSelectedItemBg.String(), editor.colors.fg.String()) + deinSideStyle)

	return side
}

func readDeinToml() ([]byte, DeinTomlConfig) {
	var deinToml DeinTomlConfig
	var deinTomlBare []byte
	if editor.config.Dein.TomlFile != "" {
		_, err := toml.DecodeFile(editor.config.Dein.TomlFile, &deinToml)
		if err != nil {
			editor.pushNotification(NotifyInfo, -1, "Something is wrong with the dein toml file.")
		} else {
			deinTomlBare, _ = ioutil.ReadFile(editor.config.Dein.TomlFile)
		}
	}

	return deinTomlBare, deinToml
}

func newInstalledPlugins() *InstalledPlugins {
	fg := editor.colors.fg.String()
	bg := editor.colors.sideBarSelectedItemBg.String()

	installedWidget := widgets.NewQWidget(nil, 0)
	installedLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, installedWidget)
	installedLayout.SetSpacing(10)

	installedHeader := widgets.NewQLabel(nil, 0)
	installedHeader.SetContentsMargins(20, 2, 20, 2)
	installedHeader.SetText("INSTALLED")
	installedHeader.SetStyleSheet(fmt.Sprintf(" QLabel { margin-top: 11px; margin-bottom: 0px; background: %s; color: %s; } ", bg, fg))
	installedHeader.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	installedLayout.AddWidget(installedHeader, 0, 0)

	cache := loadDeinCashe()
	for _, c := range cache {
		installedLayout.AddWidget(c.widget, 0, 0)
	}
	installedLayout.SetContentsMargins(0, 0, 0, 12*len(cache))

	installedPlugins := &InstalledPlugins{
		widget: installedWidget,
		items:  cache,
	}

	return installedPlugins
}

// PluginSearchResults is the structure for storing json witch is the search result in vim-awesome
type PluginSearchResults struct {
	TotalResults   int `json:"total_results"`
	ResultsPerPage int `json:"results_per_page"`
	TotalPages     int `json:"total_pages"`
	Plugins        []struct {
		VimorgRating             int      `json:"vimorg_rating"`
		GithubHomepage           string   `json:"github_homepage"`
		UpdatedAt                int      `json:"updated_at"`
		GithubReadmeFilename     string   `json:"github_readme_filename"`
		VimorgShortDesc          string   `json:"vimorg_short_desc"`
		GithubVimScriptsStars    int      `json:"github_vim_scripts_stars"`
		VimorgType               string   `json:"vimorg_type"`
		NormalizedName           string   `json:"normalized_name"`
		Category                 string   `json:"category"`
		Author                   string   `json:"author"`
		PluginManagerUsers       int      `json:"plugin_manager_users"`
		ShortDesc                string   `json:"short_desc"`
		VimorgAuthor             string   `json:"vimorg_author"`
		VimorgNumRaters          int      `json:"vimorg_num_raters"`
		VimorgURL                string   `json:"vimorg_url"`
		GithubVimScriptsBundles  int      `json:"github_vim_scripts_bundles"`
		GithubRepoName           string   `json:"github_repo_name"`
		Tags                     []string `json:"tags"`
		GithubStars              int      `json:"github_stars"`
		GithubVimScriptsRepoName string   `json:"github_vim_scripts_repo_name"`
		VimorgDownloads          int      `json:"vimorg_downloads"`
		GithubRepoID             string   `json:"github_repo_id"`
		Slug                     string   `json:"slug"`
		VimorgID                 string   `json:"vimorg_id"`
		GithubOwner              string   `json:"github_owner"`
		Name                     string   `json:"name"`
		CreatedAt                int      `json:"created_at"`
		GithubShortDesc          string   `json:"github_short_desc"`
		VimorgName               string   `json:"vimorg_name"`
		GithubURL                string   `json:"github_url"`
		GithubBundles            int      `json:"github_bundles"`
		Keywords                 string   `json:"keywords"`
		GithubAuthor             string   `json:"github_author"`
	} `json:"plugins"`
}

// Pligin is the item structure witch is the search result in vim-awesome
type Plugin struct {
	repo      string
	readme    string
	installed bool

	widget            *widgets.QWidget
	nameLabel         *widgets.QLabel
	head              *widgets.QWidget
	desc              *widgets.QWidget
	info              *widgets.QWidget
	installLabel      *widgets.QStackedWidget
	installLabelName  *widgets.QLabel
	installButton     *widgets.QWidget
	installButtonIcon *svg.QSvgWidget
	waitingLabel      *widgets.QWidget
}

// Searchresult is the structure witch displays the search result of plugins in DeinSide
type Searchresult struct {
	widget   *widgets.QWidget
	layout   *widgets.QBoxLayout
	plugins  []*Plugin
	readmore *widgets.QPushButton
	pagenum  int
}

func doPluginSearch() {
	if len(editor.deinSide.searchbox.editBox.Text()) == 0 && editor.deinSide.preSearchKeyword == "" {
		return
	}
	if len(editor.deinSide.searchbox.editBox.Text()) == 0 {
		editor.deinSide.plugincontent.RemoveWidget(editor.deinSide.searchresult.widget)
		editor.deinSide.installedplugins = newInstalledPlugins()
		editor.deinSide.plugincontent.AddWidget(editor.deinSide.installedplugins.widget)
		editor.deinSide.plugincontent.SetCurrentWidget(editor.deinSide.installedplugins.widget)
		return
	}

	editor.deinSide.plugincontent.RemoveWidget(editor.deinSide.installedplugins.widget)
	editor.deinSide.plugincontent.RemoveWidget(editor.deinSide.searchresult.widget)

	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, widget)
	layout.SetSpacing(1)

	searchresult := &Searchresult{
		widget:  widget,
		layout:  layout,
		pagenum: 1,
	}
	editor.deinSide.searchresult = searchresult
	editor.deinSide.plugincontent.AddWidget(editor.deinSide.searchresult.widget)
	editor.deinSide.plugincontent.SetCurrentWidget(editor.deinSide.searchresult.widget)

	editor.deinSide.progressbar.Show()
	go editor.deinSide.doSearch(editor.deinSide.searchresult.pagenum)
}

func setSearchWord() string {
	category := editor.deinSide.combobox.comboBox.CurrentText()
	words := strings.Fields(editor.deinSide.searchbox.editBox.Text())
	var searchWord string
	for i, word := range words {
		if i == 0 {
			searchWord = word
		} else {
			searchWord += "+" + word
		}
	}
	if category != "ALL" {
		searchWord += "+cat:" + category
	}
	editor.deinSide.preSearchKeyword = searchWord

	return searchWord
}

func (side *DeinSide) doSearch(pagenum int) {
	var results PluginSearchResults

	// Search
	searchWord := setSearchWord()
	response, err := http.Get(fmt.Sprintf("http://vimawesome.com/api/plugins?page=%v&query=%v", pagenum, searchWord))
	if err != nil {
		return
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&results); err != nil {
		fmt.Println("JSON decode error:", err)
		return
	}

	side.searchUpdates <- results
	side.searchPageUpdates <- pagenum
	side.signal.SearchSignal()
}

func drawSearchresults(results PluginSearchResults, pagenum int) {
	fg := editor.colors.fg
	resultplugins := []*Plugin{}
	labelColor := darkenHex(editor.config.SideBar.AccentColor)
	width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()
	parentLayout := editor.deinSide.searchresult.layout

	for _, p := range results.Plugins {
		if p.GithubRepoName == "" {
			continue
		}
		pluginWidget := widgets.NewQWidget(nil, 0)
		pluginLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, pluginWidget)
		pluginLayout.SetContentsMargins(15, 10, 20, 10)
		pluginLayout.SetSpacing(1)
		pluginLayout.SetAlignment(pluginWidget, core.Qt__AlignTop)
		pluginWidget.SetMinimumWidth(width)
		pluginWidget.SetMaximumWidth(width)

		// * plugin name
		pluginName := widgets.NewQLabel(nil, 0)
		pluginName.SetFixedWidth(width)
		pluginName.SetText(p.Name)
		pluginName.SetStyleSheet(fmt.Sprintf(" .QLabel { color: rgba(%d, %d, %d, 1);} ", fg.R, fg.G, fg.B))
		pluginName.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 3, false))
		pluginName.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Fixed)

		// * plugin description
		var pluginDesc *widgets.QWidget
		var pluginDescLayout *widgets.QHBoxLayout
		var pluginDescLabel *widgets.QLabel
		if p.ShortDesc != "" {
			pluginDesc = widgets.NewQWidget(nil, 0)
			pluginDescLayout = widgets.NewQHBoxLayout()
			pluginDescLayout.SetContentsMargins(0, 0, 0, 0)
			pluginDescLabel = widgets.NewQLabel(nil, 0)
			pluginDescLabel.SetText(p.ShortDesc)
			pluginDescLabel.SetWordWrap(true)
			pluginDescLayout.AddWidget(pluginDescLabel, 0, 0)
			pluginDesc.SetLayout(pluginDescLayout)
			pluginDescLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
		}

		// * plugin info
		pluginInfo := widgets.NewQWidget(nil, 0)
		//pluginInfoLayout := newVFlowLayout(16, 10, 1, 2, 0)
		pluginInfoLayout := widgets.NewQHBoxLayout()
		pluginInfoLayout.SetContentsMargins(0, 0, 0, 0)
		pluginInfoLayout.SetSpacing(5)

		// ** plugin stars
		pluginStars := widgets.NewQWidget(nil, 0)
		pluginStarsLayout := widgets.NewQHBoxLayout()
		pluginStarsLayout.SetContentsMargins(0, 0, 0, 0)
		pluginStarsLayout.SetSpacing(1)

		pluginStarsIcon := svg.NewQSvgWidget(nil)
		pluginStarsIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
		svgStarContent := editor.getSvg("star", fg)
		pluginStarsIcon.Load2(core.NewQByteArray2(svgStarContent, len(svgStarContent)))

		pluginStarsNum := widgets.NewQLabel(nil, 0)
		pluginStarsNum.SetText(strconv.Itoa(p.GithubStars))
		pluginStarsNum.SetContentsMargins(0, 0, 0, 0)
		pluginStarsNum.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-2, 1, false))

		pluginStarsLayout.AddWidget(pluginStarsIcon, 0, 0)
		pluginStarsLayout.AddWidget(pluginStarsNum, 0, 0)

		pluginStars.SetLayout(pluginStarsLayout)

		// ** plugin downloadss
		pluginDownloads := widgets.NewQWidget(nil, 0)
		pluginDownloadsLayout := widgets.NewQHBoxLayout()
		pluginDownloadsLayout.SetContentsMargins(0, 0, 0, 0)
		pluginDownloadsLayout.SetSpacing(1)

		pluginDownloadsIcon := svg.NewQSvgWidget(nil)
		pluginDownloadsIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
		svgDownloadContent := editor.getSvg("downloaded", fg)
		pluginDownloadsIcon.Load2(core.NewQByteArray2(svgDownloadContent, len(svgDownloadContent)))

		pluginDownloadsNum := widgets.NewQLabel(nil, 0)
		pluginDownloadsNum.SetText(strconv.Itoa(p.PluginManagerUsers))
		pluginDownloadsNum.SetContentsMargins(0, 0, 0, 0)
		pluginDownloadsNum.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-2, 1, false))

		pluginDownloadsLayout.AddWidget(pluginDownloadsIcon, 0, 0)
		pluginDownloadsLayout.AddWidget(pluginDownloadsNum, 0, 0)

		pluginDownloads.SetLayout(pluginDownloadsLayout)

		// ** plugin author
		pluginAuthor := widgets.NewQWidget(nil, 0)
		pluginAuthorLayout := widgets.NewQHBoxLayout()
		pluginAuthorLayout.SetContentsMargins(0, 0, 0, 0)
		pluginAuthorLayout.SetSpacing(1)

		pluginAuthorIcon := svg.NewQSvgWidget(nil)
		pluginAuthorIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
		svgUserContent := editor.getSvg("user", fg)
		pluginAuthorIcon.Load2(core.NewQByteArray2(svgUserContent, len(svgUserContent)))

		pluginAuthorNum := widgets.NewQLabel(nil, 0)
		pluginAuthorNum.SetText(p.GithubAuthor)
		pluginAuthorNum.SetContentsMargins(0, 0, 0, 0)
		pluginAuthorNum.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-2, 1, false))

		pluginAuthorLayout.AddWidget(pluginAuthorIcon, 0, 0)
		pluginAuthorLayout.AddWidget(pluginAuthorNum, 0, 0)

		pluginAuthor.SetLayout(pluginAuthorLayout)

		// ** plugin update time
		pluginUpdated := widgets.NewQWidget(nil, 0)
		pluginUpdatedLayout := widgets.NewQHBoxLayout()
		pluginUpdatedLayout.SetContentsMargins(0, 0, 0, 0)
		pluginUpdatedLayout.SetSpacing(1)

		pluginUpdatedIcon := svg.NewQSvgWidget(nil)
		pluginUpdatedIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
		svgUpdatedContent := editor.getSvg("progress", fg)
		pluginUpdatedIcon.Load2(core.NewQByteArray2(svgUpdatedContent, len(svgUpdatedContent)))

		pluginUpdatedNum := widgets.NewQLabel(nil, 0)

		pluginUpdatedNum.SetText(fmt.Sprintf("%v", sinceUpdate(p.UpdatedAt)))
		pluginUpdatedNum.SetContentsMargins(0, 0, 0, 0)
		pluginUpdatedNum.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-2, 1, false))

		pluginUpdatedLayout.AddWidget(pluginUpdatedIcon, 0, 0)
		pluginUpdatedLayout.AddWidget(pluginUpdatedNum, 0, 0)

		pluginUpdated.SetLayout(pluginUpdatedLayout)

		// * plugin info
		pluginStars.AdjustSize()
		pluginDownloads.AdjustSize()
		pluginAuthor.AdjustSize()
		pluginUpdated.AdjustSize()
		pluginInfoLayout.AddWidget(pluginStars, 0, 0)
		pluginInfoLayout.AddWidget(pluginDownloads, 0, 0)
		pluginInfoLayout.AddWidget(pluginAuthor, 0, 0)
		pluginInfoLayout.AddWidget(pluginUpdated, 0, 0)
		pluginInfo.SetLayout(pluginInfoLayout)
		pluginInfo.SetFixedWidth(pluginStars.Width() + pluginDownloads.Width() + pluginAuthor.Width() + pluginUpdated.Width() + 2*10)

		var buttonWidth float64
		if runtime.GOOS == "darwin" {
			buttonWidth = 2.0
		} else {
			buttonWidth = 1.8
		}
		// * plugin install button
		pluginInstallLabel := widgets.NewQLabel(nil, 0)
		pluginInstallLabel.SetContentsMargins(0, 0, 5, 0)
		pluginInstallLabel.SetAlignment(core.Qt__AlignCenter)
		pluginInstallLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
		pluginInstall := widgets.NewQWidget(nil, 0)
		font := gui.NewQFontMetricsF(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
		fontwidth := int(font.HorizontalAdvance("install", -1))
		pluginInstall.SetFixedWidth(int(buttonWidth * float64(fontwidth)))
		pluginInstall.SetFixedHeight(int(2.0 * float64(editor.iconSize)))
		pluginInstallLayout := widgets.NewQHBoxLayout()
		pluginInstallLayout.SetContentsMargins(editor.config.Editor.FontSize, 0, 0, 0)
		pluginInstallLayout.SetSpacing(0)
		pluginInstall.SetLayout(pluginInstallLayout)
		pluginInstall.SetObjectName("installbutton")

		// waiting label while install pugin
		pluginWaiting := widgets.NewQWidget(nil, 0)
		pluginWaiting.SetFixedWidth(int(buttonWidth * float64(fontwidth)))
		pluginWaiting.SetFixedHeight(int(2.0 * float64(editor.iconSize)))
		pluginWaitingLayout := widgets.NewQHBoxLayout()
		pluginWaitingLayout.SetContentsMargins(5, 0, 5, 0)
		pluginWaiting.SetLayout(pluginWaitingLayout)
		waitingLabel := widgets.NewQProgressBar(nil)
		waitingLabel.SetStyleSheet(fmt.Sprintf(" QProgressBar { border: 0px; background: %s; } QProgressBar::chunk { background-color: %s; } ", shiftHex(editor.config.SideBar.AccentColor, 25), editor.config.SideBar.AccentColor))
		waitingLabel.SetRange(0, 0)
		pluginWaitingLayout.AddWidget(waitingLabel, 0, 0)

		// stack installButton & waitingLabel
		installLabel := widgets.NewQStackedWidget(nil)
		installLabel.SetContentsMargins(0, 0, 0, 0)

		// * plugin name & button
		pluginHead := widgets.NewQWidget(nil, 0)
		pluginHeadLayout := widgets.NewQHBoxLayout()
		pluginHeadLayout.SetContentsMargins(0, 0, 0, 0)
		pluginHeadLayout.SetSpacing(5)
		// pluginHead.SetFixedWidth(editor.config.sideWidth - 2)
		// pluginHead.SetMinimumWidth(editor.config.sideWidth)
		// pluginHead.SetMaximumWidth(editor.config.sideWidth)
		pluginHead.SetLayout(pluginHeadLayout)
		pluginHeadLayout.AddWidget(pluginName, 0, 0)
		pluginHeadLayout.AddWidget(installLabel, 0, 0)

		// make widget
		pluginLayout.AddWidget(pluginHead, 0, 0)
		if p.ShortDesc != "" {
			pluginLayout.AddWidget(pluginDesc, 0, 0)
		}
		pluginLayout.AddWidget(pluginInfo, 0, 0)

		plugin := &Plugin{
			widget:           pluginWidget,
			nameLabel:        pluginName,
			head:             pluginHead,
			desc:             pluginDesc,
			info:             pluginInfo,
			repo:             p.GithubOwner + "/" + p.GithubRepoName,
			readme:           p.GithubReadmeFilename,
			installLabel:     installLabel,
			installLabelName: pluginInstallLabel,
			installButton:    pluginInstall,
			waitingLabel:     pluginWaiting,
		}

		pluginInstallIcon := svg.NewQSvgWidget(nil)
		iconSize := editor.iconSize
		pluginInstallIcon.SetFixedSize2(iconSize, iconSize)
		svgContent := editor.getSvg("download", newRGBA(255, 255, 255, 1))
		pluginInstallIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		pluginInstallLayout.AddWidget(pluginInstallIcon, 0, 0)
		plugin.installButtonIcon = pluginInstallIcon
		pluginInstallLayout.AddWidget(pluginInstallLabel, 0, 0)

		installLabel.AddWidget(plugin.installButton)
		installLabel.AddWidget(plugin.waitingLabel)
		installLabel.SetCurrentWidget(plugin.installButton)

		for _, item := range editor.deinSide.installedplugins.items {
			if strings.ToLower(plugin.repo) == strings.ToLower(item.repo) {
				plugin.installed = true
				break
			}
		}

		if plugin.installed == true {
			plugin.installButtonIcon.Hide()
			pluginInstallLabel.SetText("Installed")
			bg := editor.colors.inactiveFg
			pluginInstall.SetStyleSheet(fmt.Sprintf(" #installbutton { background: %s;}  #installbutton QLabel { color: %s; }", bg.String(), fg.String()))
		} else {
			pluginInstallLabel.SetText("Install")
			pluginInstall.SetStyleSheet(fmt.Sprintf(" #installbutton { background: %s;}  #installbutton QLabel { color: #ffffff; }", labelColor))
			plugin.installButton.ConnectMousePressEvent(func(*gui.QMouseEvent) {
				go plugin.deinInstallPre(plugin.repo)
			})
		}

		plugin.widget.ConnectEnterEvent(plugin.enterWidget)
		plugin.widget.ConnectLeaveEvent(plugin.leaveWidget)
		plugin.widget.ConnectMousePressEvent(plugin.pressPluginWidget)

		plugin.installButton.ConnectEnterEvent(plugin.enterButton)
		plugin.installButton.ConnectLeaveEvent(plugin.leaveButton)

		resultplugins = append(resultplugins, plugin)

		parentLayout.AddWidget(pluginWidget, 0, 0)
	}
	parentLayout.Update()
	editor.deinSide.searchresult.plugins = append(editor.deinSide.searchresult.plugins, resultplugins...)
	editor.deinSide.searchresult.layout.Update()
	parentLayout.SetContentsMargins(0, 0, 0, 18*len(editor.deinSide.searchresult.plugins))

	if pagenum < results.TotalPages {
		readMoreButton := widgets.NewQPushButton2("read more", nil)
		editor.deinSide.searchresult.readmore = readMoreButton
		editor.deinSide.searchresult.readmore.SetFocusPolicy(core.Qt__NoFocus)
		parentLayout.AddWidget(readMoreButton, 0, 0)
		readMoreButton.ConnectPressed(func() {
			pos := editor.deinSide.scrollarea.VerticalScrollBar().Value()
			editor.deinSide.searchresult.readmore.DestroyQPushButton()
			editor.deinSide.searchresult.pagenum = editor.deinSide.searchresult.pagenum + 1
			editor.deinSide.doSearch(editor.deinSide.searchresult.pagenum)
			// It is workaround that scroll bar returns to the top, only the first load
			go func() {
				time.Sleep(10 * time.Millisecond)
				editor.deinSide.scrollarea.VerticalScrollBar().SetValue(pos)
			}()
		})
	}
	editor.deinSide.progressbar.Hide()
}

func sinceUpdate(t int) string {
	s := time.Since(time.Unix(int64(t), 0))
	years := s.Hours() / 24 / 30.41 / 12
	if years >= 1 {
		return fmt.Sprintf("%v years ago", math.Trunc(years))
	}
	months := s.Hours() / 24 / 30.41
	if months >= 1 {
		return fmt.Sprintf("%v months ago", math.Trunc(months))
	}
	days := s.Hours() / 24
	if days >= 1 {
		return fmt.Sprintf("%v days ago", math.Trunc(days))
	}
	hours := s.Hours()
	if hours >= 1 {
		return fmt.Sprintf("%v hours ago", math.Trunc(hours))
	}
	minutes := s.Minutes()
	if minutes >= 1 {
		return fmt.Sprintf("%v minutes ago", math.Trunc(minutes))
	}

	return fmt.Sprintf("%v seconds ago", math.Trunc(s.Seconds()))
}

func (d *DeinPluginItem) enterWidget(event *core.QEvent) {
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
	bg := editor.colors.selectedBg.String()
	d.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: %s;} ", bg))
}

func (d *DeinPluginItem) leaveWidget(event *core.QEvent) {
	bg := editor.colors.sideBarBg.String()
	d.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: %s;} ", bg))
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (p *Plugin) enterWidget(event *core.QEvent) {
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
	bg := editor.colors.selectedBg.String()
	p.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: %s;} ", bg))
}

func (p *Plugin) leaveWidget(event *core.QEvent) {
	p.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: %s;} ", editor.colors.sideBarBg.String()))
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (p *Plugin) enterButton(event *core.QEvent) {
	if p.installed == true {
		return
	}
	p.installButton.SetStyleSheet(fmt.Sprintf(" #installbutton { background: %s;} #installbutton QLabel { color: #ffffff; }", editor.config.SideBar.AccentColor))
	p.installButton.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
}

func (p *Plugin) leaveButton(event *core.QEvent) {
	if p.installed == true {
		return
	}
	labelColor := darkenHex(editor.config.SideBar.AccentColor)
	p.installButton.SetStyleSheet(fmt.Sprintf(" #installbutton { background: %s;} #installbutton QLabel { color: #ffffff; }", labelColor))
	p.installButton.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
}

func deinInstallPost(result string) {
	for _, message := range strings.Split(result, "\n") {
		if strings.Contains(message, "Not installed plugins") {
			continue
		}
		if strings.Contains(message, "check_install()") {
			continue
		}
		if strings.Contains(message, "install()") {
			continue
		}
		if !(strings.Contains(message, "[dein]") || strings.Contains(message, "[Gonvim]")) {
			continue
		}
		message = strings.TrimRight(message, ".")
		editor.pushNotification(NotifyInfo, -1, message)
	}
	editor.deinSide.installing = false
}

func (p *Plugin) deinInstallPre(reponame string) {
	if editor.deinSide.installing {
		return
	}
	p.installButton.DisconnectMousePressEvent()
	p.installLabel.SetCurrentWidget(p.waitingLabel)
	editor.deinSide.installing = true

	b := editor.deinSide.deintomlbare
	b, _ = tomlwriter.WriteValue(`'`+reponame+`'`, b, "[plugins]", "repo", nil)
	err := ioutil.WriteFile(editor.config.Dein.TomlFile, b, 0755)
	if err != nil {
		fmt.Println(err)
	}

	v, cleanup := newNvimProcess()
	defer cleanup()

	// Dein install
	v.Command(`sleep 1000m | if dein#check_install() | call dein#install() | endif`)

	var text string
	for !strings.Contains(text, "[dein] Done") {
		text, _ = v.CommandOutput("messages")
		time.Sleep(500 * time.Millisecond)
	}

	editor.deinSide.deinInstall <- text
	editor.deinSide.signal.DeinInstallSignal()

	fg := editor.colors.fg.String()
	bg := editor.colors.selectedBg.String()
	p.installButton.SetStyleSheet(fmt.Sprintf(" #installbutton { background: %s;} #installbutton QLabel { color: %s; }", bg, fg))
	p.installButtonIcon.Hide()

	p.installed = true
	p.installLabelName.SetText("Installed")
	deinTomlBare, _ := ioutil.ReadFile(editor.config.Dein.TomlFile)
	editor.deinSide.deintomlbare = deinTomlBare
	p.installLabel.SetCurrentWidget(p.installButton)

	time.Sleep(900 * time.Millisecond)
	editor.deinSide.deinInstall <- "[Gonvim] The added plugin can be used in a new workspace session."
	editor.deinSide.signal.DeinInstallSignal()
}

func deinUpdatePost(result string) {
	var isUpdateStarted, isUpdatePlugins, isUpdateChange, isUpdateURL, isUpdateDone bool
	for _, message := range strings.Split(result, "\n") {
		if !strings.Contains(message, "[dein]") {
			continue
		}
		if strings.Contains(message, "Update started:") && !isUpdateStarted {
			isUpdateStarted = true
			message = strings.TrimRight(message, ".")
			editor.pushNotification(NotifyInfo, -1, message)
		}
		if strings.Contains(message, "Update plugins:") && !isUpdatePlugins {
			isUpdatePlugins = true
			message = strings.TrimRight(message, ".")
			editor.pushNotification(NotifyInfo, -1, message)
		}
		if strings.Contains(message, " change)") && !isUpdateChange {
			isUpdateChange = true
			message = strings.TrimRight(message, ".")
			editor.pushNotification(NotifyInfo, -1, message)
		}
		if strings.Contains(message, "https://github") && !isUpdateURL {
			isUpdateURL = true
			message = strings.TrimRight(message, ".")
			editor.pushNotification(NotifyInfo, -1, message)
		}
		if strings.Contains(message, "Done: (") && !isUpdateDone {
			isUpdateDone = true
			message = strings.TrimRight(message, ".")
			editor.pushNotification(NotifyInfo, -1, message)
		}
	}
}

func (d *DeinPluginItem) deinUpdatePre(pluginName string) {
	d.updateButton.DisconnectMousePressEvent()
	d.updateLabel.SetCurrentWidget(d.waitingLabel)

	v, cleanup := newNvimProcess()
	defer cleanup()

	// Dein update
	v.Command(`call dein#update("` + pluginName + `")`)

	var text string
	for !strings.Contains(text, "[dein] Done") {
		text, _ = v.CommandOutput("messages")
		time.Sleep(500 * time.Millisecond)
	}

	d.updateLabel.SetCurrentWidget(d.updateButton)

	editor.deinSide.deinUpdate <- text
	editor.deinSide.signal.DeinUpdateSignal()
}

func newNvimProcess() (*nvim.Nvim, func()) {
	v, err := nvim.NewChildProcess(nvim.ChildProcessArgs("-n", "--embed", "--headless"))
	if err != nil {
		fmt.Println(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- v.Serve()
	}()

	return v, func() {
		v.Command(`qa!`)
		if err := v.Close(); err != nil {
			fmt.Println(err)
		}
	}
}

func containPluginsStr(s string) bool {
	return strings.Contains(s, "[[") && strings.Contains(s, "]]") && strings.Contains(s, "plugins")
}

func (d *DeinPluginItem) disablePluginInToml() {
	b := editor.deinSide.deintomlbare
	filestr := string(b)

	strings.NewReplacer("\r\n", "\n", "\r", "\n", "\n", "\n").Replace(filestr)
	lines := strings.Split(filestr, "\n")

	var pluginSnips []([]byte)
	var pluginSnip []byte
	var i int

	// make pluginSnips
	for j, line := range lines {
		// Output pluginSnip to pluginSnips
		if containPluginsStr(line) || j+1 == len(lines) {
			i++
			if containPluginsStr(string(pluginSnip)) {
				if i-1 >= 1 {
					pluginSnips = append(pluginSnips, pluginSnip)
				}
			}
			pluginSnip = []byte{}
		}

		line += "\n"
		// push lines to pluginSnip
		pluginSnip = append(pluginSnip, *(*[]byte)(unsafe.Pointer(&line))...)
	}

	var writebyte []byte
	for _, p := range pluginSnips {
		_, ln := tomlwriter.WriteValue("dummy", p, "[plugins]", "repo", d.repo)
		if ln != 0 {
			// var ifvalue interface{}
			// for _, pluginSetting := range editor.deinSide.deintoml.Plugins {
			// 	if pluginSetting.Repo == d.repo {
			// 		ifvalue = pluginSetting.If
			// 	}
			// }
			// p, _ = tomlwriter.WriteValue("0", p, "[plugins]", "if", ifvalue)
			continue
		}
		writebyte = append(writebyte, *(*[]byte)(unsafe.Pointer(&p))...)
	}
	_ = ioutil.WriteFile(editor.config.Dein.TomlFile, writebyte, 0755)
}

// func (d *DeinPluginItem) disable(dummy bool) {
// 	d.disablePluginInToml()
// 	editor.pushNotification(NotifyInfo, -1, "[Gonvim] Disable settings are applied in the new workspace session.")
// }

func (d *DeinPluginItem) remove(dummy bool) {
	editor.pushNotification(NotifyInfo, -1, "[Gonvim] Start removing plugin.")
	go func() {
		d.disablePluginInToml()

		v, cleanup := newNvimProcess()
		defer cleanup()

		v.Command(`call map(dein#check_clean(), "delete(v:val, 'rf')")`)
		time.Sleep(1000 * time.Millisecond)
		v.Command(`call dein#recache_runtimepath()`)
		editor.pushNotification(NotifyInfo, -1, "[Gonvim] Rmoving the plugin done.")
		editor.pushNotification(NotifyInfo, -1, "[Gonvim] New settings are applied in the new workspace session.")
		editor.deinSide.deintomlbare, editor.deinSide.deintoml = readDeinToml()
	}()
}

func (d *DeinPluginItem) editToml(dummy bool) {
	w := editor.workspaces[editor.active]
	b := editor.deinSide.deintomlbare
	_, ln := tomlwriter.WriteValue("dummy", b, "[plugins]", "repo", d.repo)

	path, _ := w.nvim.CommandOutput(`echo expand("%:p")`)
	path, _ = filepath.EvalSymlinks(path)
	tomlpath, _ := filepath.EvalSymlinks(editor.config.Dein.TomlFile)
	if path != tomlpath {
		w.nvim.Command(fmt.Sprintf("-1tabnew %s", editor.config.Dein.TomlFile))
	}
	w.nvim.Command(fmt.Sprintf("call cursor(%v, 0)", ln-1))
}

func (p *Plugin) pressPluginWidget(event *gui.QMouseEvent) {
	go editor.workspaces[editor.active].markdown.openReadme(p.repo, p.readme)
}

func (side *DeinSide) enterConfigIcon(event *core.QEvent) {
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
	svgConfigContent := editor.getSvg("configfile", nil)
	side.configIcon.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
}

func (side *DeinSide) leaveConfigIcon(event *core.QEvent) {
	svgConfigContent := editor.getSvg("configfile", editor.colors.inactiveFg)
	side.configIcon.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (side *DeinSide) pressConfigIcon(event *gui.QMouseEvent) {
	fg := editor.colors.fg
	bg := editor.colors.bg
	if side.contextMenu == nil {
		side.contextMenu = widgets.NewQMenu(side.widget)
		side.contextMenu.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
		side.contextMenu.SetStyleSheet(fmt.Sprintf(" QMenu { padding: 5px 0px 5px 0px; border-radius:6px; background-color: %s;} QMenu::item { padding: 2px 20px 2px 20px; background-color: transparent; color: %s;} QMenu::item:selected { padding: 2px 20px 2px 20px; background-color: #257afd; color: #eeeeee; } ", fg.String(), bg.String()))
		menuActionUpdateRemotePlugins := side.contextMenu.AddAction("UpdateRemotePlugins")
		menuActionUpdateRemotePlugins.ConnectTriggered(func(dummy bool) {
			go editor.workspaces[editor.active].nvim.Command(":UpdateRemotePlugins")
		})
		menuActionRecacheRuntmepath := side.contextMenu.AddAction("Recache runtimepath")
		menuActionRecacheRuntmepath.ConnectTriggered(func(dummy bool) {
			go editor.workspaces[editor.active].nvim.Command(":call dein#recache_runtimepath()")
		})
		menuActionOpenToml := side.contextMenu.AddAction("Open toml file")
		menuActionOpenToml.ConnectTriggered(func(dummy bool) {
			go editor.workspaces[editor.active].nvim.Command(":tabnew " + editor.config.Dein.TomlFile)
		})
	}
	p := event.Pos()
	side.contextMenu.Exec2(side.configIcon.MapToGlobal(p), nil)

}

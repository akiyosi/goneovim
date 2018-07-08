package editor

import (
	"bufio"
	"encoding/json"
	"fmt"
	// "io/ioutil"
	"net/http"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type DeinSide struct {
	widget       *widgets.QWidget
	layout       *widgets.QLayout
	title        *widgets.QLabel
	scrollarea   *widgets.QScrollArea
	searchlayout *widgets.QBoxLayout
	// searchbox        *widgets.QLineEdit
	searchbox        *Searchbox
	plugincontent    *widgets.QStackedWidget
	searchresult     *Searchresult
	installedplugins *InstalledPlugins
	config           *svg.QSvgWidget
}

type Searchbox struct {
	widget  *widgets.QWidget
	layout  *widgets.QHBoxLayout
	editBox *widgets.QLineEdit
}

type DeinPluginItem struct {
	widget *widgets.QWidget

	itemname       string
	lazy           bool
	path           string
	repo           string
	hook_add       string
	merged         bool
	normalizedName string
	pluginType     string
	rtp            string
	sourced        bool
	name           string
}

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

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(lines[1]), &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func loadDeinCashe() []*DeinPluginItem {
	w := editor.workspaces[editor.active]

	m, _ := readDeinCache()
	installedPlugins := []*DeinPluginItem{}

	for name, item := range m {
		s, _ := item.(map[interface{}]interface{})
		i := &DeinPluginItem{}
		i.itemname = name.(string)
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
			case "hook_add":
				i.hook_add = value.(string)
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

		// make widgets
		installedPluginWidget := widgets.NewQWidget(nil, 0)
		installedPluginLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, installedPluginWidget)
		installedPluginLayout.SetContentsMargins(20, 5, 20, 5)
		installedPluginLayout.SetSpacing(0)
		// installedPluginWidget.SetFixedWidth(editor.config.sideWidth)
		installedPluginWidget.SetMaximumWidth(editor.config.sideWidth - 55)
		installedPluginWidget.SetMinimumWidth(editor.config.sideWidth - 55)

		// plugin mame
		installedPluginName := widgets.NewQLabel(nil, 0)
		installedPluginName.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		installedPluginName.SetText(i.repo)
		fg := editor.fgcolor
		installedPluginName.SetStyleSheet(fmt.Sprintf(" .QLabel {font: bold; color: rgba(%d, %d, %d, 1);} ", fg.R, fg.G, fg.B))

		// ** Lazy plugin icon
		bg := editor.bgcolor
		installedPluginLazy := widgets.NewQWidget(nil, 0)
		installedPluginLazyLayout := widgets.NewQHBoxLayout()
		installedPluginLazyLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginLazyLayout.SetSpacing(1)
		installedPluginLazyIcon := svg.NewQSvgWidget(nil)
		installedPluginLazyIcon.SetFixedSize2(12, 12)
		var svgLazyContent string
		if i.lazy == false {
			svgLazyContent = w.getSvg("timer", shiftColor(bg, -5))
		} else {
			svgLazyContent = w.getSvg("timer", fg)
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
		installedPluginSourcedIcon.SetFixedSize2(12, 12)
		var svgSourcedContent string
		if i.sourced == false {
			svgSourcedContent = w.getSvg("puzzle", shiftColor(bg, -5))
		} else {
			svgSourcedContent = w.getSvg("puzzle", fg)
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
		installedPluginSettingsIcon.SetFixedSize2(12, 12)
		svgSettingsContent := w.getSvg("settings", fg)
		installedPluginSettingsIcon.Load2(core.NewQByteArray2(svgSettingsContent, len(svgSettingsContent)))
		installedPluginSettingsLayout.AddWidget(installedPluginSettingsIcon, 0, 0)
		installedPluginSettings.SetLayout(installedPluginSettingsLayout)

		// * installedPlugin name & some option icon
		installedPluginHead := widgets.NewQWidget(nil, 0)

		// spacing, padding, paddingtop, rightitemnum, width
		// installedPluginHeadLayout := newVFlowLayout(2, 2, 1, 1, 0)
		installedPluginHeadLayout := widgets.NewQHBoxLayout()
		installedPluginHeadLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginHead.SetLayout(installedPluginHeadLayout)

		installedPluginStatus := widgets.NewQWidget(nil, 0)
		installedPluginStatusLayout := widgets.NewQHBoxLayout2(nil)
		installedPluginStatus.SetLayout(installedPluginStatusLayout)
		installedPluginStatusLayout.SetSpacing(0)
		installedPluginStatusLayout.SetContentsMargins(0, 0, 0, 0)
		installedPluginStatusLayout.AddWidget(installedPluginLazy, 0, 0)
		installedPluginStatusLayout.AddWidget(installedPluginSourced, 0, 0)
		installedPluginStatusLayout.AddWidget(installedPluginSettings, 0, 0)

		installedPluginHeadLayout.AddWidget(installedPluginName, 0, 0)
		installedPluginHeadLayout.AddWidget(installedPluginStatus, 0, 0)

		installedPluginLayout.AddWidget(installedPluginHead, 0, 0)
		i.widget = installedPluginWidget

		installedPlugins = append(installedPlugins, i)
	}

	return installedPlugins
}

func newDeinSide() *DeinSide {
	w := editor.workspaces[editor.active]
	fg := editor.fgcolor
	bg := editor.bgcolor

	layout := newHFlowLayout(0, 0, 0, 0, 20)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	headerWidget := widgets.NewQWidget(nil, 0)
	headerLayout := widgets.NewQHBoxLayout()
	headerLayout.SetContentsMargins(20, 15, 20, 5)
	header := widgets.NewQLabel(nil, 0)
	//header.SetContentsMargins(20, 15, 20, 5)
	header.SetContentsMargins(0, 0, 0, 0)
	header.SetText("Dein.vim")
	configIcon := svg.NewQSvgWidget(nil)
	configIcon.SetFixedSize2(13, 13)
	svgConfigContent := w.getSvg("configfile", newRGBA(gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, 1))
	configIcon.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
	headerLayout.AddWidget(header, 0, 0)
	headerLayout.AddWidget(configIcon, 0, 0)
	headerWidget.SetLayout(headerLayout)

	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)

	searchWidget := widgets.NewQWidget(nil, 0)
	searchWidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Minimum)
	searchLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, searchWidget)
	searchLayout.SetContentsMargins(0, 5, 0, 5)

	searchBoxLayout := widgets.NewQHBoxLayout()
	searchBoxLayout.SetContentsMargins(20, 5, 20, 5)
	searchBoxLayout.SetSpacing(0)
	searchboxEdit := widgets.NewQLineEdit(nil)
	searchboxEdit.SetPlaceholderText("Search Plugins in VimAwesome")
	searchboxEdit.SetStyleSheet(" #LineEdit {font-size: 9px;} ")
	searchboxEdit.SetFocusPolicy(core.Qt__ClickFocus)
	searchboxEdit.SetFixedWidth(editor.config.sideWidth - (20 + 20))
	searchBoxLayout.AddWidget(searchboxEdit, 0, 0)
	searchBoxWidget := widgets.NewQWidget(nil, 0)
	searchBoxWidget.SetLayout(searchBoxLayout)

	// searchbox.ConnectReturnPressed(doPluginSearch)
	searchboxEdit.ConnectEditingFinished(doPluginSearch)

	searchbox := &Searchbox{
		widget:  searchBoxWidget,
		layout:  searchBoxLayout,
		editBox: searchboxEdit,
	}

	searchLayout.AddWidget(searchbox.widget, 0, 0)

	content := widgets.NewQStackedWidget(nil)
	content.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Minimum)

	layout.AddWidget(headerWidget)
	layout.AddWidget(searchWidget)
	layout.AddWidget(content)

	installed := newInstalledPlugins()
	searchresultWidget := widgets.NewQWidget(nil, 0)
	searchresult := &Searchresult{
		widget: searchresultWidget,
	}

	// make DeinSide
	side := &DeinSide{
		widget:           widget,
		layout:           layout,
		title:            header,
		searchlayout:     searchLayout,
		searchbox:        searchbox,
		plugincontent:    content,
		searchresult:     searchresult,
		installedplugins: installed,
		config:           configIcon,
	}
	content.AddWidget(side.searchresult.widget)
	content.AddWidget(side.installedplugins.widget)
	content.SetCurrentWidget(side.installedplugins.widget)

	configIcon.ConnectEnterEvent(side.enterConfigIcon)
	configIcon.ConnectLeaveEvent(side.leaveConfigIcon)
	configIcon.ConnectMousePressEvent(pressConfigIcon)

	deinSideStyle := fmt.Sprintf("QWidget {	color: rgba(%d, %d, %d, 1);		border-right: 0px solid;	}", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B)
	side.widget.SetStyleSheet(fmt.Sprintf(".QWidget {padding-top: 5px;	background-color: rgba(%d, %d, %d, 1);	}	", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B) + deinSideStyle)
	side.searchbox.editBox.SetStyleSheet(fmt.Sprintf(".QLineEdit { border: 1px solid	%s; border-radius: 1px; background: rgba(%d, %d, %d, 1); selection-background-color: rgba(%d, %d, %d, 1); }	", editor.config.accentColor, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B) + deinSideStyle)

	return side
}

func newInstalledPlugins() *InstalledPlugins {
	fg := editor.fgcolor
	bg := editor.bgcolor

	installedWidget := widgets.NewQWidget(nil, 0)
	installedLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, installedWidget)
	installedLayout.SetSpacing(0)

	installedHeader := widgets.NewQLabel(nil, 0)
	installedHeader.SetContentsMargins(20, 2, 20, 1)
	installedHeader.SetText("INSTALLED")
	installedHeader.SetStyleSheet(fmt.Sprintf(" QLabel { margin-top: 10px; margin-bottom: 10px; background: rgba(%d, %d, %d, 1); font-size: 11px; color: rgba(%d, %d, %d, 1); } ", gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, fg.R, fg.G, fg.B))
	installedLayout.AddWidget(installedHeader, 0, 0)

	cache := loadDeinCashe()
	for _, c := range cache {
		installedLayout.AddWidget(c.widget, 0, 0)
	}
	installedLayout.SetContentsMargins(0, 0, 0, 8*len(cache))

	installedPlugins := &InstalledPlugins{
		widget: installedWidget,
		items:  cache,
	}

	return installedPlugins
}

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

type Plugin struct {
	widget        *widgets.QWidget
	head          *widgets.QWidget
	desc          *widgets.QWidget
	info          *widgets.QWidget
	installButton *widgets.QWidget
	repo          string
}

type Searchresult struct {
	widget  *widgets.QWidget
	plugins []*Plugin
}

func doPluginSearch() {

	if len(editor.deinSide.searchbox.editBox.Text()) == 0 {
		editor.deinSide.plugincontent.RemoveWidget(editor.deinSide.searchresult.widget)
		editor.deinSide.installedplugins = newInstalledPlugins()
		editor.deinSide.plugincontent.AddWidget(editor.deinSide.installedplugins.widget)
		editor.deinSide.plugincontent.SetCurrentWidget(editor.deinSide.installedplugins.widget)
		return
	}

	w := editor.workspaces[editor.active]
	fg := editor.fgcolor

	var results PluginSearchResults
	response, _ := http.Get("http://vimawesome.com/api/plugins?query=" + editor.deinSide.searchbox.editBox.Text())
	defer response.Body.Close()

	// data, _ := ioutil.ReadAll(response.Body)
	// jsonBytes := ([]byte)(data)
	// if err := json.Unmarshal(jsonBytes, &results); err != nil {
	if err := json.NewDecoder(response.Body).Decode(&results); err != nil {
		fmt.Println("JSON Unmarshal error:", err)
		return
	}

	// if results has pages
	if results.TotalPages > 1 {
		for p := 1; p <= results.TotalPages; p++ {

			var r PluginSearchResults
			res, _ := http.Get(fmt.Sprintf("http://vimawesome.com/api/plugins?page=%v&query=%v", p, editor.deinSide.searchbox.editBox.Text()))
			defer res.Body.Close()
			if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
				fmt.Println("JSON Unmarshal error:", err)
				return
			}

			results.Plugins = append(results.Plugins, r.Plugins...)

		}
	}

	labelColor := darkenHex(editor.config.accentColor)

	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, widget)
	layout.SetContentsMargins(0, 0, 0, 18*len(results.Plugins))
	layout.SetSpacing(1)

	height := editor.workspaces[editor.active].font.height + 5

	resultplugins := []*Plugin{}
	for _, p := range results.Plugins {
		pluginWidget := widgets.NewQWidget(nil, 0)
		// pluginWidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Minimum)
		pluginLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, pluginWidget)
		pluginLayout.SetContentsMargins(20, 10, 20, 10)
		pluginLayout.SetSpacing(1)
		pluginLayout.SetAlignment(pluginWidget, core.Qt__AlignTop)
		// pluginWidget.SetFixedWidth(editor.config.sideWidth)
		pluginWidget.SetMinimumWidth(editor.config.sideWidth)
		pluginWidget.SetMaximumWidth(editor.config.sideWidth)

		// * plugin name
		pluginName := widgets.NewQLabel(nil, 0)
		pluginName.SetText(p.Name)
		pluginName.SetStyleSheet(fmt.Sprintf(" .QLabel {font: bold; color: rgba(%d, %d, %d, 1);} ", fg.R, fg.G, fg.B))
		pluginName.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Fixed)

		// * plugin description
		var pluginDesc *widgets.QWidget
		var pluginDescLayout *widgets.QHBoxLayout
		var pluginDescLabel *widgets.QLabel
		if p.ShortDesc != "" {
			pluginDesc = widgets.NewQWidget(nil, 0)
			pluginDescLayout = widgets.NewQHBoxLayout()
			pluginDescLabel = widgets.NewQLabel(nil, 0)
			pluginDescLabel.SetText(p.ShortDesc)
			pluginDescLabel.SetWordWrap(true)
			pluginDescLayout.AddWidget(pluginDescLabel, 0, 0)
			pluginDesc.SetLayout(pluginDescLayout)
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
		pluginStarsIcon.SetFixedSize2(11, 11)
		svgStarContent := w.getSvg("star", fg)
		pluginStarsIcon.Load2(core.NewQByteArray2(svgStarContent, len(svgStarContent)))

		pluginStarsNum := widgets.NewQLabel(nil, 0)
		pluginStarsNum.SetText(strconv.Itoa(p.GithubStars))
		pluginStarsNum.SetContentsMargins(0, 0, 0, 0)
		pluginStarsNum.SetStyleSheet(" .QLabel {font-size: 10px;} ")

		pluginStarsLayout.AddWidget(pluginStarsIcon, 0, 0)
		pluginStarsLayout.AddWidget(pluginStarsNum, 0, 0)

		pluginStars.SetLayout(pluginStarsLayout)

		// ** plugin downloadss
		pluginDownloads := widgets.NewQWidget(nil, 0)
		pluginDownloadsLayout := widgets.NewQHBoxLayout()
		pluginDownloadsLayout.SetContentsMargins(0, 0, 0, 0)
		pluginDownloadsLayout.SetSpacing(1)

		pluginDownloadsIcon := svg.NewQSvgWidget(nil)
		pluginDownloadsIcon.SetFixedSize2(11, 11)
		svgDownloadContent := w.getSvg("download", fg)
		pluginDownloadsIcon.Load2(core.NewQByteArray2(svgDownloadContent, len(svgDownloadContent)))

		pluginDownloadsNum := widgets.NewQLabel(nil, 0)
		pluginDownloadsNum.SetText(strconv.Itoa(p.PluginManagerUsers))
		pluginDownloadsNum.SetContentsMargins(0, 0, 0, 0)
		pluginDownloadsNum.SetStyleSheet(" .QLabel {font-size: 10px;} ")

		pluginDownloadsLayout.AddWidget(pluginDownloadsIcon, 0, 0)
		pluginDownloadsLayout.AddWidget(pluginDownloadsNum, 0, 0)

		pluginDownloads.SetLayout(pluginDownloadsLayout)

		// ** plugin author
		pluginAuthor := widgets.NewQWidget(nil, 0)
		pluginAuthorLayout := widgets.NewQHBoxLayout()
		pluginAuthorLayout.SetContentsMargins(0, 0, 0, 0)
		pluginAuthorLayout.SetSpacing(1)

		pluginAuthorIcon := svg.NewQSvgWidget(nil)
		pluginAuthorIcon.SetFixedSize2(11, 11)
		svgUserContent := w.getSvg("user", fg)
		pluginAuthorIcon.Load2(core.NewQByteArray2(svgUserContent, len(svgUserContent)))

		pluginAuthorNum := widgets.NewQLabel(nil, 0)
		pluginAuthorNum.SetText(p.GithubAuthor)
		pluginAuthorNum.SetContentsMargins(0, 0, 0, 0)
		pluginAuthorNum.SetStyleSheet(" .QLabel {font-size: 10px;} ")

		pluginAuthorLayout.AddWidget(pluginAuthorIcon, 0, 0)
		pluginAuthorLayout.AddWidget(pluginAuthorNum, 0, 0)

		pluginAuthor.SetLayout(pluginAuthorLayout)

		// * plugin info
		pluginStars.AdjustSize()
		pluginDownloads.AdjustSize()
		pluginAuthor.AdjustSize()
		pluginInfoLayout.AddWidget(pluginStars, 0, 0)
		pluginInfoLayout.AddWidget(pluginDownloads, 0, 0)
		pluginInfoLayout.AddWidget(pluginAuthor, 0, 0)
		pluginInfo.SetLayout(pluginInfoLayout)
		pluginInfo.SetFixedWidth(pluginStars.Width() + pluginDownloads.Width() + pluginAuthor.Width() + 3*10)

		// * plugin install button
		pluginInstallLabel := widgets.NewQLabel(nil, 0)
		pluginInstallLabel.SetFixedWidth(65)
		pluginInstallLabel.SetContentsMargins(5, 0, 5, 0)
		pluginInstallLabel.SetAlignment(core.Qt__AlignCenter)
		pluginInstall := widgets.NewQWidget(nil, 0)
		pluginInstallLayout := widgets.NewQHBoxLayout()
		pluginInstallLayout.SetContentsMargins(0, 0, 0, 0)
		pluginInstallLayout.AddWidget(pluginInstallLabel, 0, 0)
		pluginInstall.SetLayout(pluginInstallLayout)
		pluginInstall.SetObjectName("installbutton")

		// ret, _ := editor.workspaces[editor.active].nvim.CommandOutput(":call dein#check_install('" + p.GithubRepoName + "')")
		// bg := editor.bgcolor
		// if ret != "0" {
		pluginInstallLabel.SetText("Install")
		// pluginInstall.SetStyleSheet(fmt.Sprintf(" #installbutton QLabel { margin-top: 1px; margin-bottom: 1px; color: rgba(%d, %d, %d, 1); background: %s;} ", fg.R, fg.G, fg.B, labelColor))
		pluginInstall.SetStyleSheet(fmt.Sprintf(" #installbutton QLabel { margin-top: 1px; margin-bottom: 1px; color: #ffffff; background: %s;} ", labelColor))
		// } else {
		//   pluginInstallLabel.SetText("Installed")
		//   pluginInstall.SetStyleSheet(fmt.Sprintf(" #installbutton QLabel { color: rgba(%d, %d, %d, 1); background: rgba(%d, %d, %d, 1);} ", fg.R, fg.G, fg.B, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))
		// }

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
		pluginHeadLayout.AddWidget(pluginInstall, 0, 0)

		pluginInfo.SetMaximumHeight(height)
		pluginInfo.SetMinimumHeight(height)

		// make widget
		pluginLayout.AddWidget(pluginHead, 0, 0)
		if p.ShortDesc != "" {
			pluginLayout.AddWidget(pluginDesc, 0, 0)
		}
		pluginLayout.AddWidget(pluginInfo, 0, 0)

		// add to parent in side widget
		layout.AddWidget(pluginWidget, 0, 0)

		plugin := &Plugin{
			widget:        pluginWidget,
			head:          pluginHead,
			desc:          pluginDesc,
			info:          pluginInfo,
			installButton: pluginInstall,
			repo:          p.GithubOwner + "/" + p.GithubRepoName,
		}
		plugin.widget.ConnectEnterEvent(plugin.enterWidget)
		plugin.widget.ConnectLeaveEvent(plugin.leaveWidget)

		plugin.installButton.ConnectEnterEvent(plugin.enterButton)
		plugin.installButton.ConnectLeaveEvent(plugin.leaveButton)
		plugin.installButton.ConnectMousePressEvent(plugin.pressButton)

		resultplugins = append(resultplugins, plugin)
	}
	widget.AdjustSize()

	editor.deinSide.plugincontent.RemoveWidget(editor.deinSide.installedplugins.widget)
	editor.deinSide.plugincontent.RemoveWidget(editor.deinSide.searchresult.widget)

	searchresult := &Searchresult{
		widget:  widget,
		plugins: resultplugins,
	}
	editor.deinSide.searchresult = searchresult

	editor.deinSide.plugincontent.AddWidget(editor.deinSide.searchresult.widget)
	editor.deinSide.plugincontent.SetCurrentWidget(editor.deinSide.searchresult.widget)

}

func (p *Plugin) enterWidget(event *core.QEvent) {
	bg := editor.bgcolor
	p.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: rgba(%d, %d, %d, 1);} ", gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))
}

func (p *Plugin) leaveWidget(event *core.QEvent) {
	bg := editor.bgcolor
	p.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: rgba(%d, %d, %d, 1);} ", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B))
}

func (p *Plugin) enterButton(event *core.QEvent) {
	fg := editor.fgcolor
	p.installButton.SetStyleSheet(fmt.Sprintf(" #installbutton QLabel { color: rgba(%d, %d, %d, 1); background: %s;} ", fg.R, fg.G, fg.B, editor.config.accentColor))
}

func (p *Plugin) leaveButton(event *core.QEvent) {
	fg := editor.fgcolor
	labelColor := darkenHex(editor.config.accentColor)
	p.installButton.SetStyleSheet(fmt.Sprintf(" #installbutton QLabel { color: rgba(%d, %d, %d, 1); background: %s;} ", fg.R, fg.G, fg.B, labelColor))
}

func (p *Plugin) pressButton(event *gui.QMouseEvent) {
	editor.workspaces[editor.active].nvim.Command(":silent! call dein#direct_install('" + p.repo + "')")
}

func (d *DeinSide) enterConfigIcon(event *core.QEvent) {
	w := editor.workspaces[editor.active]
	fg := editor.fgcolor
	svgConfigContent := w.getSvg("configfile", newRGBA(warpColor(fg, -20).R, warpColor(fg, -20).G, warpColor(fg, -20).B, 1))
	d.config.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
}

func (d *DeinSide) leaveConfigIcon(event *core.QEvent) {
	w := editor.workspaces[editor.active]
	bg := editor.bgcolor
	svgConfigContent := w.getSvg("configfile", newRGBA(gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, 1))
	d.config.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
}

func pressConfigIcon(event *gui.QMouseEvent) {
	w := editor.workspaces[editor.active]
	var userPath, basePath string
	userPath, _ = w.nvim.CommandOutput("echo g:dein#cache_directory")
	basePath, _ = w.nvim.CommandOutput("echo g:dein#_base_path")
	var deinDirectInstallPath string
	if userPath == "" {
		deinDirectInstallPath = basePath
	} else {
		deinDirectInstallPath = userPath
	}
	editor.workspaces[editor.active].nvim.Command(":tabnew " + deinDirectInstallPath + "/direct_install.vim")

}

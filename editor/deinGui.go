package editor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

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
	searchbox    *widgets.QLineEdit
	searchresult *widgets.QWidget
	config       *svg.QSvgWidget
}

func newDeinSide() *DeinSide {
	w := editor.workspaces[editor.active]

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
	svgConfigContent := w.getSvg("configfile", nil)
	configIcon.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
	headerLayout.AddWidget(header, 0, 0)
	headerLayout.AddWidget(configIcon, 0, 0)
	headerWidget.SetLayout(headerLayout)

	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)

	searchWidget := widgets.NewQWidget(nil, 0)
	searchLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, searchWidget)
	searchLayout.SetContentsMargins(0, 5, 0, 100)

	searchBoxLayout := widgets.NewQHBoxLayout()
	searchBoxLayout.SetContentsMargins(20, 5, 20, 5)
	searchBoxLayout.SetSpacing(0)
	searchbox := widgets.NewQLineEdit(nil)
	searchbox.SetFocusPolicy(core.Qt__ClickFocus)
	searchbox.SetFixedWidth(editor.config.sideWidth - (20 + 20))
	searchBoxLayout.AddWidget(searchbox, 0, 0)
	searchBoxWidget := widgets.NewQWidget(nil, 0)
	searchBoxWidget.SetLayout(searchBoxLayout)

	searchbox.ConnectReturnPressed(doPluginSearch)

	searchresult := widgets.NewQWidget(nil, 0)

	searchLayout.AddWidget(searchBoxWidget, 0, 0)
	searchLayout.AddWidget(searchresult, 0, 0)

	side := &DeinSide{
		widget:       widget,
		layout:       layout,
		title:        header,
		searchlayout: searchLayout,
		searchbox:    searchbox,
		searchresult: searchresult,
		config:       configIcon,
	}
	configIcon.ConnectEnterEvent(side.enterConfigIcon)
	configIcon.ConnectLeaveEvent(side.leaveConfigIcon)
	configIcon.ConnectMousePressEvent(pressConfigIcon)

	layout.AddWidget(headerWidget)
	layout.AddWidget(searchWidget)

	side.title.Show()
	side.searchbox.Show()

	// Set Color for DeinSide
	fg := editor.fgcolor
	bg := editor.bgcolor
	deinSideStyle := fmt.Sprintf("QWidget {	color: rgba(%d, %d, %d, 1);		border-right: 0px solid;	}", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B)
	side.widget.SetStyleSheet(fmt.Sprintf(".QWidget {padding-top: 5px;	background-color: rgba(%d, %d, %d, 1);	}	", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B) + deinSideStyle)
	side.searchbox.SetStyleSheet(fmt.Sprintf(".QLineEdit { border: 1px solid	%s; border-radius: 1px; background: rgba(%d, %d, %d, 1); selection-background-color: rgba(%d, %d, %d, 1); }	", editor.config.accentColor, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B) + deinSideStyle)

	return side
}

type PluginResults struct {
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
	installButton *widgets.QWidget
	repo          string
}

func doPluginSearch() {
	w := editor.workspaces[editor.active]
	fg := editor.fgcolor

	response, _ := http.Get("http://vimawesome.com/api/plugins?query=" + editor.deinSide.searchbox.Text())
	defer response.Body.Close()

	data, _ := ioutil.ReadAll(response.Body)
	jsonBytes := ([]byte)(data)

	var results PluginResults
	if err := json.Unmarshal(jsonBytes, &results); err != nil {
		fmt.Println("JSON Unmarshal error:", err)
		return
	}

	labelColor := darkenHex(editor.config.accentColor)

	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, widget)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(1)

	for _, p := range results.Plugins {
		pluginWidget := widgets.NewQWidget(nil, 0)
		pluginLayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, pluginWidget)
		pluginLayout.SetContentsMargins(20, 5, 20, 5)
		pluginLayout.SetSpacing(1)
		pluginWidget.SetFixedWidth(editor.config.sideWidth)

		// * plugin name
		pluginName := widgets.NewQLabel(nil, 0)
		pluginName.SetText(p.Name)
		pluginName.SetStyleSheet(fmt.Sprintf(" .QLabel {font: bold; color: rgba(%d, %d, %d, 1);} ", fg.R, fg.G, fg.B))

		// * plugin description
		var pluginDesc *widgets.QLabel
		if p.ShortDesc != "" {
			pluginDesc = widgets.NewQLabel(nil, 0)
			pluginDesc.SetText(p.ShortDesc)
			pluginDesc.SetWordWrap(true)
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
		pluginInstallLabel.SetText("Install")
		pluginInstallLabel.SetFixedWidth(65)
		pluginInstallLabel.SetAlignment(core.Qt__AlignCenter)
		pluginInstall := widgets.NewQWidget(nil, 0)
		pluginInstallLayout := widgets.NewQHBoxLayout()
		pluginInstallLayout.SetContentsMargins(0, 0, 0, 0)
		pluginInstallLayout.AddWidget(pluginInstallLabel, 0, 0)
		pluginInstall.SetLayout(pluginInstallLayout)
		pluginInstall.SetObjectName("installbutton")
		pluginInstall.SetStyleSheet(fmt.Sprintf(" #installbutton QLabel { color: rgba(%d, %d, %d, 1); background: %s;} ", fg.R, fg.G, fg.B, labelColor))

		// * plugin name & button
		pluginHead := widgets.NewQWidget(nil, 0)
		pluginHeadLayout := widgets.NewQHBoxLayout()
		pluginHeadLayout.SetContentsMargins(0, 0, 0, 0)
		pluginHeadLayout.SetSpacing(5)
		pluginHead.SetFixedWidth(editor.config.sideWidth - 2)
		pluginHead.SetLayout(pluginHeadLayout)
		pluginHeadLayout.AddWidget(pluginName, 0, 0)
		pluginHeadLayout.AddWidget(pluginInstall, 0, 0)

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
			installButton: pluginInstall,
			repo:          p.GithubOwner + "/" + p.GithubRepoName,
		}
		plugin.widget.ConnectEnterEvent(plugin.enterWidget)
		plugin.widget.ConnectLeaveEvent(plugin.leaveWidget)

		plugin.installButton.ConnectEnterEvent(plugin.enterButton)
		plugin.installButton.ConnectLeaveEvent(plugin.leaveButton)
		plugin.installButton.ConnectMousePressEvent(plugin.pressButton)
	}
	widget.AdjustSize()

	editor.deinSide.searchlayout.RemoveWidget(editor.deinSide.searchresult)
	editor.deinSide.searchresult = widget
	editor.deinSide.searchlayout.AddWidget(editor.deinSide.searchresult, 0, 0)
	widget.Show()

}

func (p *Plugin) enterWidget(event *core.QEvent) {
	bg := editor.bgcolor
	p.widget.SetStyleSheet(fmt.Sprintf(" .QWidget { background: rgba(%d, %d, %d, 1);} ", shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B))
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
	editor.workspaces[editor.active].nvim.Command(":call dein#direct_install('" + p.repo + "')")
}

func (d *DeinSide) enterConfigIcon(event *core.QEvent) {
	fmt.Println("hoge")
	w := editor.workspaces[editor.active]
	fg := editor.fgcolor
	svgConfigContent := w.getSvg("configfile", newRGBA(shiftColor(fg, -20).R, shiftColor(fg, -20).G, shiftColor(fg, -20).B, 1))
	d.config.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
}

func (d *DeinSide) leaveConfigIcon(event *core.QEvent) {
	fmt.Println("fuga")
	w := editor.workspaces[editor.active]
	svgConfigContent := w.getSvg("configfile", nil)
	d.config.Load2(core.NewQByteArray2(svgConfigContent, len(svgConfigContent)))
}

func pressConfigIcon(event *gui.QMouseEvent) {
	w := editor.workspaces[editor.active]
	var userPath, defaultPath string
	userPath, _ = w.nvim.CommandOutput("echo g:dein#cache_directory")
	defaultPath, _ = w.nvim.CommandOutput("echo g:dein#_base_path")
	var deinDirectInstallPath string
	if userPath == "" {
		deinDirectInstallPath = defaultPath
	} else {
		deinDirectInstallPath = userPath
	}
	editor.workspaces[editor.active].nvim.Command(":tabnew " + deinDirectInstallPath + "/direct_install.vim")

}

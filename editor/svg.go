package editor

import (
	"fmt"
)

// SvgXML is
type SvgXML struct {
	xml       string
	width     int
	height    int
	thickness float64
	color     *RGBA
}

// // Svg is
// type Svg struct {
// 	width  int
// 	height int
// 	color  *RGBA
// 	bg     *RGBA
// 	name   string
// }

// Giitub Programming Language color
// ref: https://github.com/ozh/github-colors/blob/master/colors.json
const (
	GithubLang1CEnterprise           = "#814CCC"
	GithubLangABAP                   = "#E8274B"
	GithubLangActionScript           = "#882B0F"
	GithubLangAda                    = "#02f88c"
	GithubLangAgda                   = "#315665"
	GithubLangAGSScript              = "#B9D9FF"
	GithubLangAlloy                  = "#64C800"
	GithubLangAMPL                   = "#E6EFBB"
	GithubLangAngelScript            = "#C7D7DC"
	GithubLangANTLR                  = "#9DC3FF"
	GithubLangAPIBlueprint           = "#2ACCA8"
	GithubLangAPL                    = "#5A8164"
	GithubLangAppleScript            = "#101F1F"
	GithubLangArc                    = "#aa2afe"
	GithubLangASP                    = "#6a40fd"
	GithubLangAspectJ                = "#a957b0"
	GithubLangAssembly               = "#6E4C13"
	GithubLangAsymptote              = "#4a0c0c"
	GithubLangATS                    = "#1ac620"
	GithubLangAutoHotkey             = "#6594b9"
	GithubLangAutoIt                 = "#1C3552"
	GithubLangBallerina              = "#FF5000"
	GithubLangBatchfile              = "#C1F12E"
	GithubLangBlitzMax               = "#cd6400"
	GithubLangBoo                    = "#d4bec1"
	GithubLangBrainfuck              = "#2F2530"
	GithubLangC                      = "#555555"
	GithubLangCSharp                 = "#178600"
	GithubLangCpp                    = "#f34b7d"
	GithubLangCeylon                 = "#dfa535"
	GithubLangChapel                 = "#8dc63f"
	GithubLangCirru                  = "#ccccff"
	GithubLangClarion                = "#db901e"
	GithubLangClean                  = "#3F85AF"
	GithubLangClick                  = "#E4E6F3"
	GithubLangClojure                = "#db5855"
	GithubLangCoffeeScript           = "#244776"
	GithubLangColdFusion             = "#ed2cd6"
	GithubLangCommonLisp             = "#3fb68b"
	GithubLangCommonWorkflowLanguage = "#B5314C"
	GithubLangComponentPascal        = "#B0CE4E"
	GithubLangCrystal                = "#000100"
	GithubLangCSS                    = "#563d7c"
	GithubLangCuda                   = "#3A4E3A"
	GithubLangD                      = "#ba595e"
	GithubLangDafny                  = "#FFEC25"
	GithubLangDart                   = "#00B4AB"
	GithubLangDataWeave              = "#003a52"
	GithubLangDhall                  = "#dfafff"
	GithubLangDM                     = "#447265"
	GithubLangDockerfile             = "#384d54"
	GithubLangDogescript             = "#cca760"
	GithubLangDylan                  = "#6c616e"
	GithubLangE                      = "#ccce35"
	GithubLangeC                     = "#913960"
	GithubLangECL                    = "#8a1267"
	GithubLangEiffel                 = "#946d57"
	GithubLangElixir                 = "#6e4a7e"
	GithubLangElm                    = "#60B5CC"
	GithubLangEmacsLisp              = "#c065db"
	GithubLangEmberScript            = "#FFF4F3"
	GithubLangEQ                     = "#a78649"
	GithubLangErlang                 = "#B83998"
	GithubLangFSharp                 = "#b845fc"
	GithubLangFStar                  = "#572e30"
	GithubLangFactor                 = "#636746"
	GithubLangFancy                  = "#7b9db4"
	GithubLangFantom                 = "#14253c"
	GithubLangFaust                  = "#c37240"
	GithubLangFLUX                   = "#88ccff"
	GithubLangForth                  = "#341708"
	GithubLangFortran                = "#4d41b1"
	GithubLangFreeMarker             = "#0050b2"
	GithubLangFrege                  = "#00cafe"
	GithubLangFuthark                = "#5f021f"
	GithubLangGcode                  = "#D08CF2"
	GithubLangGameMakerLanguage      = "#71b417"
	GithubLangGAML                   = "#FFC766"
	GithubLangGDScript               = "#355570"
	GithubLangGenie                  = "#fb855d"
	GithubLangGherkin                = "#5B2063"
	GithubLangGlyph                  = "#c1ac7f"
	GithubLangGnuplot                = "#f0a9f0"
	GithubLangGo                     = "#00ADD8"
	GithubLangGolo                   = "#88562A"
	GithubLangGosu                   = "#82937f"
	GithubLangGrammaticalFramework   = "#79aa7a"
	GithubLangGroovy                 = "#e69f56"
	GithubLangHack                   = "#878787"
	GithubLangHarbour                = "#0e60e3"
	GithubLangHaskell                = "#5e5086"
	GithubLangHaxe                   = "#df7900"
	GithubLangHiveQL                 = "#dce200"
	GithubLangHolyC                  = "#ffefaf"
	GithubLangHTML                   = "#e34c26"
	GithubLangHy                     = "#7790B2"
	GithubLangIDL                    = "#a3522f"
	GithubLangIdris                  = "#b30000"
	GithubLangIGORPro                = "#0000cc"
	GithubLangIo                     = "#a9188d"
	GithubLangIoke                   = "#078193"
	GithubLangIsabelle               = "#FEFE00"
	GithubLangJ                      = "#9EEDFF"
	GithubLangJava                   = "#b07219"
	GithubLangJavaScript             = "#f1e05a"
	GithubLangJolie                  = "#843179"
	GithubLangJSONiq                 = "#40d47e"
	GithubLangJsonnet                = "#0064bd"
	GithubLangJulia                  = "#a270ba"
	GithubLangJupyterNotebook        = "#DA5B0B"
	GithubLangKaitaiStruct           = "#773b37"
	GithubLangKotlin                 = "#F18E33"
	GithubLangKRL                    = "#28430A"
	GithubLangLasso                  = "#999999"
	GithubLangLex                    = "#DBCA00"
	GithubLangLFE                    = "#4C3023"
	GithubLangLiveScript             = "#499886"
	GithubLangLLVM                   = "#185619"
	GithubLangLOLCODE                = "#cc9900"
	GithubLangLookML                 = "#652B81"
	GithubLangLSL                    = "#3d9970"
	GithubLangLua                    = "#000080"
	GithubLangMacaulay2              = "#d8ffff"
	GithubLangMakefile               = "#427819"
	GithubLangMask                   = "#f97732"
	GithubLangMATLAB                 = "#e16737"
	GithubLangMax                    = "#c4a79c"
	GithubLangMAXScript              = "#00a6a6"
	GithubLangmcfunction             = "#E22837"
	GithubLangMercury                = "#ff2b2b"
	GithubLangMeson                  = "#007800"
	GithubLangMetal                  = "#8f14e9"
	GithubLangMirah                  = "#c7a938"
	GithubLangmIRCScript             = "#926059"
	GithubLangMLIR                   = "#5EC8DB"
	GithubLangModula3                = "#223388"
	GithubLangMQL4                   = "#62A8D6"
	GithubLangMQL5                   = "#4A76B8"
	GithubLangMTML                   = "#b7e1f4"
	GithubLangNCL                    = "#28431f"
	GithubLangNearley                = "#990000"
	GithubLangNemerle                = "#3d3c6e"
	GithubLangnesC                   = "#94B0C7"
	GithubLangNetLinx                = "#0aa0ff"
	GithubLangNetLinxERB             = "#747faa"
	GithubLangNetLogo                = "#ff6375"
	GithubLangNewLisp                = "#87AED7"
	GithubLangNextflow               = "#3ac486"
	GithubLangNim                    = "#ffc200"
	GithubLangNit                    = "#009917"
	GithubLangNix                    = "#7e7eff"
	GithubLangNu                     = "#c9df40"
	GithubLangObjectiveC             = "#438eff"
	GithubLangObjectiveCpp           = "#6866fb"
	GithubLangObjectiveJ             = "#ff0c5a"
	GithubLangObjectScript           = "#424893"
	GithubLangOCaml                  = "#3be133"
	GithubLangOdin                   = "#60AFFE"
	GithubLangOmgrofl                = "#cabbff"
	GithubLangooc                    = "#b0b77e"
	GithubLangOpal                   = "#f7ede0"
	GithubLangOpenQASM               = "#AA70FF"
	GithubLangOxygene                = "#cdd0e3"
	GithubLangOz                     = "#fab738"
	GithubLangP4                     = "#7055b5"
	GithubLangPan                    = "#cc0000"
	GithubLangPapyrus                = "#6600cc"
	GithubLangParrot                 = "#f3ca0a"
	GithubLangPascal                 = "#E3F171"
	GithubLangPawn                   = "#dbb284"
	GithubLangPep8                   = "#C76F5B"
	GithubLangPerl                   = "#0298c3"
	GithubLangPHP                    = "#4F5D95"
	GithubLangPigLatin               = "#fcd7de"
	GithubLangPike                   = "#005390"
	GithubLangPLSQL                  = "#dad8d8"
	GithubLangPogoScript             = "#d80074"
	GithubLangPostScript             = "#da291c"
	GithubLangPowerBuilder           = "#8f0f8d"
	GithubLangPowerShell             = "#012456"
	GithubLangProcessing             = "#0096D8"
	GithubLangProlog                 = "#74283c"
	GithubLangPropellerSpin          = "#7fa2a7"
	GithubLangPuppet                 = "#302B6D"
	GithubLangPureBasic              = "#5a6986"
	GithubLangPureScript             = "#1D222D"
	GithubLangPython                 = "#3572A5"
	GithubLangq                      = "#0040cd"
	GithubLangQSharp                 = "#fed659"
	GithubLangQML                    = "#44a51c"
	GithubLangQtScript               = "#00b841"
	GithubLangQuake                  = "#882233"
	GithubLangR                      = "#198CE7"
	GithubLangRacket                 = "#3c5caa"
	GithubLangRagel                  = "#9d5200"
	GithubLangRaku                   = "#0000fb"
	GithubLangRAML                   = "#77d9fb"
	GithubLangRascal                 = "#fffaa0"
	GithubLangReason                 = "#ff5847"
	GithubLangRebol                  = "#358a5b"
	GithubLangRed                    = "#f50000"
	GithubLangRenPy                  = "#ff7f7f"
	GithubLangRing                   = "#2D54CB"
	GithubLangRiot                   = "#A71E49"
	GithubLangRoff                   = "#ecdebe"
	GithubLangRouge                  = "#cc0088"
	GithubLangRuby                   = "#701516"
	GithubLangRUNOFF                 = "#665a4e"
	GithubLangRust                   = "#dea584"
	GithubLangSaltStack              = "#646464"
	GithubLangSAS                    = "#B34936"
	GithubLangScala                  = "#c22d40"
	GithubLangScheme                 = "#1e4aec"
	GithubLangsed                    = "#64b970"
	GithubLangSelf                   = "#0579aa"
	GithubLangShell                  = "#89e051"
	GithubLangShen                   = "#120F14"
	GithubLangSlash                  = "#007eff"
	GithubLangSlice                  = "#003fa2"
	GithubLangSmalltalk              = "#596706"
	GithubLangSmPL                   = "#c94949"
	GithubLangSolidity               = "#AA6746"
	GithubLangSourcePawn             = "#5c7611"
	GithubLangSQF                    = "#3F3F3F"
	GithubLangSquirrel               = "#800000"
	GithubLangSRecodeTemplate        = "#348a34"
	GithubLangStan                   = "#b2011d"
	GithubLangStandardML             = "#dc566d"
	GithubLangStarlark               = "#76d275"
	GithubLangSuperCollider          = "#46390b"
	GithubLangSwift                  = "#ffac45"
	GithubLangSystemVerilog          = "#DAE1C2"
	GithubLangTcl                    = "#e4cc98"
	GithubLangTerra                  = "#00004c"
	GithubLangTeX                    = "#3D6117"
	GithubLangTIProgram              = "#A0AA87"
	GithubLangTuring                 = "#cf142b"
	GithubLangTypeScript             = "#2b7489"
	GithubLangUnrealScript           = "#a54c4d"
	GithubLangV                      = "#5d87bd"
	GithubLangVala                   = "#fbe5cd"
	GithubLangVBA                    = "#867db1"
	GithubLangVBScript               = "#15dcdc"
	GithubLangVCL                    = "#148AA8"
	GithubLangVerilog                = "#b2b7f8"
	GithubLangVHDL                   = "#adb2cb"
	GithubLangVimscript              = "#199f4b"
	GithubLangVisualBasicNET         = "#945db7"
	GithubLangVolt                   = "#1F1F1F"
	GithubLangVue                    = "#2c3e50"
	GithubLangwdl                    = "#42f1f4"
	GithubLangWebAssembly            = "#04133b"
	GithubLangwisp                   = "#7582D1"
	GithubLangWollok                 = "#a23738"
	GithubLangX10                    = "#4B6BEF"
	GithubLangxBase                  = "#403a40"
	GithubLangXC                     = "#99DA07"
	GithubLangXQuery                 = "#5232e7"
	GithubLangXSLT                   = "#EB8CEB"
	GithubLangYacc                   = "#4B6C4B"
	GithubLangYARA                   = "#220000"
	GithubLangYASnippet              = "#32AB90"
	GithubLangZAP                    = "#0d665e"
	GithubLangZenScript              = "#00BCD1"
	GithubLangZephir                 = "#118f9e"
	GithubLangZig                    = "#ec915c"
	GithubLangZIL                    = "#dc75e5"
)

func (e *Editor) getSvg(name string, color *RGBA) string {
	// e.svgsOnce.Do(func() {
	// 	e.initSVGS()
	// })
	svg := e.svgs[name]
	var fg *RGBA
	if e.colors == nil {
		fg = newRGBA(255, 255, 255, 1)
	} else {
		fg = e.colors.fg
	}
	if svg == nil {
		svg = e.svgs["default"]
	}
	if color == nil {
		if svg.color == nil {
			color = fg
		} else {
			color = svg.color
		}
	}
	if color == nil {
		color = newRGBA(255, 255, 255, 1)
	}
	if len(e.workspaces) > 0 {
		if color.diff(e.workspaces[0].background) < 3.5 {
			color = invertColor(color)
		}
	}

	return fmt.Sprintf(svg.xml, color.Hex())
}

func (e *Editor) initSVGS() {
	e.iconSize = e.extFontSize * 11 / 9
	e.svgs = map[string]*SvgXML{}

	e.svgs["default"] = &SvgXML{
		width:     200,
		height:    200,
		thickness: 0.5,
		xml:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200"><g transform="scale(0.9)"><path fill="%s" d="M22.34999879828441,17.7129974066314 h147.91225647948028 v20.748584330809138 H22.34999879828441 V17.7129974066314 zM22.34999879828441,65.18324337560382 h126.22055467908892 v20.748584330809138 H22.34999879828441 V65.18324337560382 zM22.34999879828441,113.91097930401922 h155.3000099912078 v20.748584330809138 H22.34999879828441 V113.91097930401922 zM22.34999879828441,161.538411517922 h91.01083581468554 v20.748584330809138 H22.34999879828441 V161.538411517922 z"/></g></svg>`,
	}

	// icons for goneovim
	e.svgs["gonvim_fuzzy_buffers"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M15.5,2C13,2 11,4 11,6.5C11,9 13,11 15.5,11C16.4,11 17.2,10.7 17.9,10.3L21,13.4L22.4,12L19.3,8.9C19.7,8.2 20,7.4 20,6.5C20,4 18,2 15.5,2M4,4A2,2 0 0,0 2,6V20A2,2 0 0,0 4,22H18A2,2 0 0,0 20,20V15L18,13V20H4V6H9.03C9.09,5.3 9.26,4.65 9.5,4H4M15.5,4C16.9,4 18,5.1 18,6.5C18,7.9 16.9,9 15.5,9C14.1,9 13,7.9 13,6.5C13,5.1 14.1,4 15.5,4Z" /></svg>`,
	}

	e.svgs["gonvim_fuzzy_files"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M14,2H6A2,2 0 0,0 4,4V20A2,2 0 0,0 6,22H13C12.59,21.75 12.2,21.44 11.86,21.1C11.53,20.77 11.25,20.4 11,20H6V4H13V9H18V10.18C18.71,10.34 19.39,10.61 20,11V8L14,2M20.31,18.9C21.64,16.79 21,14 18.91,12.68C16.8,11.35 14,12 12.69,14.08C11.35,16.19 12,18.97 14.09,20.3C15.55,21.23 17.41,21.23 18.88,20.32L22,23.39L23.39,22L20.31,18.9M16.5,19A2.5,2.5 0 0,1 14,16.5A2.5,2.5 0 0,1 16.5,14A2.5,2.5 0 0,1 19,16.5A2.5,2.5 0 0,1 16.5,19Z" /></svg>`,
	}

	e.svgs["gonvim_fuzzy_keyword"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M9.5,3A6.5,6.5 0 0,1 16,9.5C16,11.11 15.41,12.59 14.44,13.73L14.71,14H15.5L20.5,19L19,20.5L14,15.5V14.71L13.73,14.44C12.59,15.41 11.11,16 9.5,16A6.5,6.5 0 0,1 3,9.5A6.5,6.5 0 0,1 9.5,3M9.5,5C7,5 5,7 5,9.5C5,12 7,14 9.5,14C12,14 14,12 14,9.5C14,7 12,5 9.5,5Z" /></svg>`,
	}

	e.svgs["gonvim_fuzzy_bufferlines"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M9,2A7,7 0 0,1 16,9C16,10.57 15.5,12 14.61,13.19L15.41,14H16L22,20L20,22L14,16V15.41L13.19,14.61C12,15.5 10.57,16 9,16A7,7 0 0,1 2,9A7,7 0 0,1 9,2M5,8V10H13V8H5Z" /></svg>`,
	}

	// icons for vim
	// TODO: Add vim completion kind
	e.svgs["vim_keyword"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M15.5,12C18,12 20,14 20,16.5C20,17.38 19.75,18.21 19.31,18.9L22.39,22L21,23.39L17.88,20.32C17.19,20.75 16.37,21 15.5,21C13,21 11,19 11,16.5C11,14 13,12 15.5,12M15.5,14A2.5,2.5 0 0,0 13,16.5A2.5,2.5 0 0,0 15.5,19A2.5,2.5 0 0,0 18,16.5A2.5,2.5 0 0,0 15.5,14M5,3H19C20.11,3 21,3.89 21,5V13.03C20.5,12.23 19.81,11.54 19,11V5H5V19H9.5C9.81,19.75 10.26,20.42 10.81,21H5C3.89,21 3,20.11 3,19V5C3,3.89 3.89,3 5,3M7,7H17V9H7V7M7,11H12.03C11.23,11.5 10.54,12.19 10,13H7V11M7,15H9.17C9.06,15.5 9,16 9,16.5V17H7V15Z" /></svg>`,
		// xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M3 7A2 2 0 0 0 1 9V17H3V13H5V17H7V9A2 2 0 0 0 5 7H3M3 9H5V11H3M15 10.5V9A2 2 0 0 0 13 7H9V17H13A2 2 0 0 0 15 15V13.5A1.54 1.54 0 0 0 13.5 12A1.54 1.54 0 0 0 15 10.5M13 15H11V13H13V15M13 11H11V9H13M19 7A2 2 0 0 0 17 9V15A2 2 0 0 0 19 17H21A2 2 0 0 0 23 15V14H21V15H19V9H21V10H23V9A2 2 0 0 0 21 7Z" /></svg>`,
	}

	e.svgs["vim_ctrl_x"] = &SvgXML{
		width:  24,
		height: 24,
	}
	e.svgs["vim_whole_line"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M15.5,12C18,12 20,14 20,16.5C20,17.38 19.75,18.21 19.31,18.9L22.39,22L21,23.39L17.88,20.32C17.19,20.75 16.37,21 15.5,21C13,21 11,19 11,16.5C11,14 13,12 15.5,12M15.5,14A2.5,2.5 0 0,0 13,16.5A2.5,2.5 0 0,0 15.5,19A2.5,2.5 0 0,0 18,16.5A2.5,2.5 0 0,0 15.5,14M5,3H19C20.11,3 21,3.89 21,5V13.03C20.5,12.23 19.81,11.54 19,11V5H5V19H9.5C9.81,19.75 10.26,20.42 10.81,21H5C3.89,21 3,20.11 3,19V5C3,3.89 3.89,3 5,3M7,7H17V9H7V7M7,11H12.03C11.23,11.5 10.54,12.19 10,13H7V11M7,15H9.17C9.06,15.5 9,16 9,16.5V17H7V15Z" /></svg>`,
	}
	e.svgs["vim_files"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M14,2H6A2,2 0 0,0 4,4V20A2,2 0 0,0 6,22H18A2,2 0 0,0 20,20V8L14,2M18,20H6V4H13V9H18V20Z" /></svg>`,
	}
	e.svgs["vim_tags"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<svg style="width:24px;height:24px" viewBox="0 0 24 24">
    <path fill="%s" d="M5.5,9A1.5,1.5 0 0,0 7,7.5A1.5,1.5 0 0,0 5.5,6A1.5,1.5 0 0,0 4,7.5A1.5,1.5 0 0,0 5.5,9M17.41,11.58C17.77,11.94 18,12.44 18,13C18,13.55 17.78,14.05 17.41,14.41L12.41,19.41C12.05,19.77 11.55,20 11,20C10.45,20 9.95,19.78 9.58,19.41L2.59,12.42C2.22,12.05 2,11.55 2,11V6C2,4.89 2.89,4 4,4H9C9.55,4 10.05,4.22 10.41,4.58L17.41,11.58M13.54,5.71L14.54,4.71L21.41,11.58C21.78,11.94 22,12.45 22,13C22,13.55 21.78,14.05 21.42,14.41L16.04,19.79L15.04,18.79L20.75,13L13.54,5.71Z" /></svg>`,
	}
	e.svgs["vim_path_defines"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M18,15A3,3 0 0,1 21,18A3,3 0 0,1 18,21C16.69,21 15.58,20.17 15.17,19H14V17H15.17C15.58,15.83 16.69,15 18,15M18,17A1,1 0 0,0 17,18A1,1 0 0,0 18,19A1,1 0 0,0 19,18A1,1 0 0,0 18,17M18,8A1.43,1.43 0 0,0 19.43,6.57C19.43,5.78 18.79,5.14 18,5.14C17.21,5.14 16.57,5.78 16.57,6.57A1.43,1.43 0 0,0 18,8M18,2.57A4,4 0 0,1 22,6.57C22,9.56 18,14 18,14C18,14 14,9.56 14,6.57A4,4 0 0,1 18,2.57M8.83,17H10V19H8.83C8.42,20.17 7.31,21 6,21A3,3 0 0,1 3,18C3,16.69 3.83,15.58 5,15.17V14H7V15.17C7.85,15.47 8.53,16.15 8.83,17M6,17A1,1 0 0,0 5,18A1,1 0 0,0 6,19A1,1 0 0,0 7,18A1,1 0 0,0 6,17M6,3A3,3 0 0,1 9,6C9,7.31 8.17,8.42 7,8.83V10H5V8.83C3.83,8.42 3,7.31 3,6A3,3 0 0,1 6,3M6,5A1,1 0 0,0 5,6A1,1 0 0,0 6,7A1,1 0 0,0 7,6A1,1 0 0,0 6,5M11,19V17H13V19H11M7,13H5V11H7V13Z" /></svg>`,
	}
	e.svgs["vim_path_paterns"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M18,15A3,3 0 0,1 21,18A3,3 0 0,1 18,21C16.69,21 15.58,20.17 15.17,19H14V17H15.17C15.58,15.83 16.69,15 18,15M18,17A1,1 0 0,0 17,18A1,1 0 0,0 18,19A1,1 0 0,0 19,18A1,1 0 0,0 18,17M18,8A1.43,1.43 0 0,0 19.43,6.57C19.43,5.78 18.79,5.14 18,5.14C17.21,5.14 16.57,5.78 16.57,6.57A1.43,1.43 0 0,0 18,8M18,2.57A4,4 0 0,1 22,6.57C22,9.56 18,14 18,14C18,14 14,9.56 14,6.57A4,4 0 0,1 18,2.57M8.83,17H10V19H8.83C8.42,20.17 7.31,21 6,21A3,3 0 0,1 3,18C3,16.69 3.83,15.58 5,15.17V14H7V15.17C7.85,15.47 8.53,16.15 8.83,17M6,17A1,1 0 0,0 5,18A1,1 0 0,0 6,19A1,1 0 0,0 7,18A1,1 0 0,0 6,17M6,3A3,3 0 0,1 9,6C9,7.31 8.17,8.42 7,8.83V10H5V8.83C3.83,8.42 3,7.31 3,6A3,3 0 0,1 6,3M6,5A1,1 0 0,0 5,6A1,1 0 0,0 6,7A1,1 0 0,0 7,6A1,1 0 0,0 6,5M11,19V17H13V19H11M7,13H5V11H7V13Z" /></svg>`,
	}

	e.svgs["vim_dictionary"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M5.81,2C4.83,2.09 4,3 4,4V20C4,21.05 4.95,22 6,22H18C19.05,22 20,21.05 20,20V4C20,2.89 19.1,2 18,2H12V9L9.5,7.5L7,9V2H6C5.94,2 5.87,2 5.81,2M12,13H13A1,1 0 0,1 14,14V18H13V16H12V18H11V14A1,1 0 0,1 12,13M12,14V15H13V14H12M15,15H18V16L16,19H18V20H15V19L17,16H15V15Z" /></svg>`,
	}
	e.svgs["vim_thesaurus"] = &SvgXML{
		width:  24,
		height: 24,
	}
	e.svgs["vim_cmdline"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg viewBox="0 0 128 128"><g transform="translate(2,3) scale(0.9)"><path fill="%s" d="M72.6 80.5c.2.2.6.5.9.5h5.3c.3 0 .7-.3.9-.5l1.4-1.5c.2-.2.3-.4.3-.6l1.5-5.1c.1-.5 0-1-.3-1.3l-1.1-.9c-.2-.2-.6-.1-.9-.1h-4.8l-.2-.2-.1-.1c-.2 0-.4-.1-.6.1l-1.9 1.2c-.2 0-.3.5-.4.7l-1.6 4.9c-.2.5-.1 1.1.3 1.5l1.3 1.4zM73.4 106.9l-.4.1h-1.2l7.2-21.1c.2-.7-.1-1.5-.8-1.7l-.4-.1h-12.1c-.5.1-.9.5-1 1l-.7 2.5c-.2.7.3 1.3 1 1.5l.3-.1h1.8l-7.3 20.9c-.2.7.1 1.6.8 1.9l.4.3h11.2c.6 0 1.1-.5 1.3-1.1l.7-2.4c.3-.7-.1-1.5-.8-1.7zM126.5 87.2l-1.9-2.5v-.1c-.3-.3-.6-.6-1-.6h-7.2c-.4 0-.7.4-1 .6l-2 2.4h-3.1l-2.1-2.4v-.1c-.2-.3-.6-.5-1-.5h-4l20.2-20.2-22.6-22.4 20.2-20.8v-9l-2.8-3.6h-40.9l-3.3 3.5v2.9l-11.3-11.4-7.7 7.5-2.4-2.5h-40.4l-3.2 3.7v9.4l3 2.9h3v26.1l-14 14 14 14v32l5.2 2.9h11.6l9.1-9.5 21.6 21.6 14.5-14.5c.1.4.4.5.9.7l.4-.2h9.4c.6 0 1.1-.1 1.2-.6l.7-2c.2-.7-.1-1.3-.8-1.5l-.4.1h-.4l3.4-10.7 2.3-2.3h5l-5 15.9c-.2.7.2 1.1.9 1.4l.4-.2h9.1c.5 0 1-.1 1.2-.6l.8-1.8c.3-.7-.1-1.3-.7-1.6-.1-.1-.3 0-.5 0h-.4l4.2-13h6.1l-5.1 15.9c-.2.7.2 1.1.9 1.3l.4-.3h10c.5 0 1-.1 1.2-.6l.8-2c.3-.7-.1-1.3-.8-1.5-.1-.1-.3.1-.5.1h-.7l5.6-18.5c.2-.5.1-1.1-.1-1.4zm-63.8-82.3l11.3 11.3v4.7l3.4 4.1h1.6l-29 28v-28h3.3l2.7-4.2v-8.9l-.2-.3 6.9-6.7zm-59.8 59.2l12.1-12.1v24.2l-12.1-12.1zm38.9 38.3l58.4-60 21.4 21.5-20.2 20.2h-.1c-.3.1-.5.3-.7.5l-2.1 2.4h-2.9l-2.2-2.4c-.2-.3-.6-.6-1-.6h-8.8c-.6 0-1.1.4-1.3 1l-.8 2.5c-.2.7.1 1.3.8 1.6h1.5l-6.4 18.9-15.1 15.2-20.5-20.8z"></path></g></svg>`,
	}
	e.svgs["vim_function"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M20,12A8,8 0 0,0 12,4A8,8 0 0,0 4,12A8,8 0 0,0 12,20A8,8 0 0,0 20,12M22,12A10,10 0 0,1 12,22A10,10 0 0,1 2,12A10,10 0 0,1 12,2A10,10 0 0,1 22,12M10,9.5C10,10.3 9.3,11 8.5,11C7.7,11 7,10.3 7,9.5C7,8.7 7.7,8 8.5,8C9.3,8 10,8.7 10,9.5M17,9.5C17,10.3 16.3,11 15.5,11C14.7,11 14,10.3 14,9.5C14,8.7 14.7,8 15.5,8C16.3,8 17,8.7 17,9.5M12,17.23C10.25,17.23 8.71,16.5 7.81,15.42L9.23,14C9.68,14.72 10.75,15.23 12,15.23C13.25,15.23 14.32,14.72 14.77,14L16.19,15.42C15.29,16.5 13.75,17.23 12,17.23Z" /></svg>`,
	}
	e.svgs["vim_omni"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<svg style="width:24px;height:24px" viewBox="0 0 24 24">
<path fill="%s" d="M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M12,20C7.59,20 4,16.41 4,12C4,7.59 7.59,4 12,4C16.41,4 20,7.59 20,12C20,16.41 16.41,20 12,20M15,12A3,3 0 0,1 12,15A3,3 0 0,1 9,12A3,3 0 0,1 12,9A3,3 0 0,1 15,12Z" /></svg>`,
	}
	e.svgs["vim_spell"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<svg style="width:24px;height:24px" viewBox="0 0 24 24">
    <path fill="%s" d="M21.59,11.59L13.5,19.68L9.83,16L8.42,17.41L13.5,22.5L23,13M6.43,11L8.5,5.5L10.57,11M12.45,16H14.54L9.43,3H7.57L2.46,16H4.55L5.67,13H11.31L12.45,16Z" /></svg>`,
	}
	e.svgs["vim_eval"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M18.5 2H5.5C3.6 2 2 3.6 2 5.5V18.5C2 20.4 3.6 22 5.5 22H16L22 16V5.5C22 3.6 20.4 2 18.5 2M20 15H18.5C16.6 15 15 16.6 15 18.5V20H5.8C4.8 20 4 19.2 4 18.2V5.8C4 4.8 4.8 4 5.8 4H18.3C19.3 4 20.1 4.8 20.1 5.8V15M15.2 8.2L16.7 9.7L10.7 15.7L7.2 12.2L8.7 10.7L10.7 12.7L15.2 8.2Z" />
</svg>`,
	}
	e.svgs["vim_unknown"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<svg style="width:24px;height:24px" viewBox="0 0 24 24">
<path fill="%s" d="M14,2H6A2,2 0 0,0 4,4V20A2,2 0 0,0 6,22H18A2,2 0 0,0 20,20V8L14,2M6,4H13L18,9V17.58L16.16,15.74C17.44,13.8 17.23,11.17 15.5,9.46C14.55,8.5 13.28,8 12,8C10.72,8 9.45,8.5 8.47,9.46C6.5,11.41 6.5,14.57 8.47,16.5C9.44,17.5 10.72,17.97 12,17.97C12.96,17.97 13.92,17.69 14.75,17.14L17.6,20H6V4M14.11,15.1C13.55,15.66 12.8,16 12,16C11.2,16 10.45,15.67 9.89,15.1C9.33,14.54 9,13.79 9,13C9,12.19 9.32,11.44 9.89,10.88C10.45,10.31 11.2,10 12,10C12.8,10 13.55,10.31 14.11,10.88C14.67,11.44 15,12.19 15,13C15,13.79 14.68,14.54 14.11,15.1Z" /></svg>`,
	}

	// icons for LSP
	// TODO: Add LSP completion kind
	e.svgs["lsp_text"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5M12,4.15L6.04,7.5L12,10.85L17.96,7.5L12,4.15M5,15.91L11,19.29V12.58L5,9.21V15.91M19,15.91V9.21L13,12.58V19.29L19,15.91Z" /></svg>`,
	}

	e.svgs["lsp_method"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5M12,4.15L6.04,7.5L12,10.85L17.96,7.5L12,4.15M5,15.91L11,19.29V12.58L5,9.21V15.91M19,15.91V9.21L13,12.58V19.29L19,15.91Z" /></svg>`,
	}

	e.svgs["lsp_function"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5M12,4.15L6.04,7.5L12,10.85L17.96,7.5L12,4.15M5,15.91L11,19.29V12.58L5,9.21V15.91M19,15.91V9.21L13,12.58V19.29L19,15.91Z" /></svg>`,
	}

	e.svgs["lsp_constructor"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5M12,4.15L6.04,7.5L12,10.85L17.96,7.5L12,4.15M5,15.91L11,19.29V12.58L5,9.21V15.91M19,15.91V9.21L13,12.58V19.29L19,15.91Z" /></svg>`,
	}

	e.svgs["lsp_variable"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5M12,4.15L6.04,7.5L12,10.85L17.96,7.5L12,4.15Z" /></svg>`,
	}

	e.svgs["lsp_field"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5M12,4.15L6.04,7.5L12,10.85L17.96,7.5L12,4.15Z" /></svg>`,
	}

	e.svgs["lsp_class"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M3,3H9V7H3V3M15,10H21V14H15V10M15,17H21V21H15V17M13,13H7V18H13V20H7L5,20V9H7V11H13V13Z" /></svg>`,
	}

	e.svgs["lsp_interface"] = &SvgXML{ // Need to change to a better icon
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M4,9C5.31,9 6.42,9.83 6.83,11H17.17C17.58,9.83 18.69,9 20,9A3,3 0 0,1 23,12A3,3 0 0,1 20,15C18.69,15 17.58,14.17 17.17,13H6.83C6.42,14.17 5.31,15 4,15A3,3 0 0,1 1,12A3,3 0 0,1 4,9Z" /></svg>`,
	}

	e.svgs["lsp_module"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `
<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M8,3A2,2 0 0,0 6,5V9A2,2 0 0,1 4,11H3V13H4A2,2 0 0,1 6,15V19A2,2 0 0,0 8,21H10V19H8V14A2,2 0 0,0 6,12A2,2 0 0,0 8,10V5H10V3M16,3A2,2 0 0,1 18,5V9A2,2 0 0,0 20,11H21V13H20A2,2 0 0,0 18,15V19A2,2 0 0,1 16,21H14V19H16V14A2,2 0 0,1 18,12A2,2 0 0,1 16,10V5H14V3H16Z" /></svg>`,
	}

	e.svgs["lsp_property"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `
<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M22.7,19L13.6,9.9C14.5,7.6 14,4.9 12.1,3C10.1,1 7.1,0.6 4.7,1.7L9,6L6,9L1.6,4.7C0.4,7.1 0.9,10.1 2.9,12.1C4.8,14 7.5,14.5 9.8,13.6L18.9,22.7C19.3,23.1 19.9,23.1 20.3,22.7L22.6,20.4C23.1,20 23.1,19.3 22.7,19Z" /></svg>`,
	}

	e.svgs["lsp_unit"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M22,14A2,2 0 0,1 20,16H4A2,2 0 0,1 2,14V10A2,2 0 0,1 4,8H20A2,2 0 0,1 22,10V14M4,14H8V10H4V14M10,14H14V10H10V14M16,14H20V10H16V14Z" /></svg>`,
	}

	e.svgs["lsp_value"] = &SvgXML{ // Need to change to a better icon
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M18,18H12V12.21C11.34,12.82 10.47,13.2 9.5,13.2C7.46,13.2 5.8,11.54 5.8,9.5A3.7,3.7 0 0,1 9.5,5.8C11.54,5.8 13.2,7.46 13.2,9.5C13.2,10.47 12.82,11.34 12.21,12H18M19,3H5C3.89,3 3,3.89 3,5V19A2,2 0 0,0 5,21H19A2,2 0 0,0 21,19V5C21,3.89 20.1,3 19,3Z" /></svg>`,
	}

	e.svgs["lsp_enum"] = &SvgXML{ // Need to change to a better icon
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M18,18H12V12.21C11.34,12.82 10.47,13.2 9.5,13.2C7.46,13.2 5.8,11.54 5.8,9.5A3.7,3.7 0 0,1 9.5,5.8C11.54,5.8 13.2,7.46 13.2,9.5C13.2,10.47 12.82,11.34 12.21,12H18M19,3H5C3.89,3 3,3.89 3,5V19A2,2 0 0,0 5,21H19A2,2 0 0,0 21,19V5C21,3.89 20.1,3 19,3Z" /></svg>`,
	}

	e.svgs["lsp_keyword"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M3,7H9V13H3V7M3,3H21V5H3V3M21,7V9H11V7H21M21,11V13H11V11H21M3,15H17V17H3V15M3,19H21V21H3V19Z" /></svg>`,
	}

	e.svgs["lsp_snippet"] = &SvgXML{ // Need to change to a better icon
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M2,3H8V5H4V19H8V21H2V3M7,17V15H9V17H7M11,17V15H13V17H11M15,17V15H17V17H15M22,3V21H16V19H20V5H16V3H22Z" /></svg>`,
	}

	e.svgs["lsp_color"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M17.5,12A1.5,1.5 0 0,1 16,10.5A1.5,1.5 0 0,1 17.5,9A1.5,1.5 0 0,1 19,10.5A1.5,1.5 0 0,1 17.5,12M14.5,8A1.5,1.5 0 0,1 13,6.5A1.5,1.5 0 0,1 14.5,5A1.5,1.5 0 0,1 16,6.5A1.5,1.5 0 0,1 14.5,8M9.5,8A1.5,1.5 0 0,1 8,6.5A1.5,1.5 0 0,1 9.5,5A1.5,1.5 0 0,1 11,6.5A1.5,1.5 0 0,1 9.5,8M6.5,12A1.5,1.5 0 0,1 5,10.5A1.5,1.5 0 0,1 6.5,9A1.5,1.5 0 0,1 8,10.5A1.5,1.5 0 0,1 6.5,12M12,3A9,9 0 0,0 3,12A9,9 0 0,0 12,21A1.5,1.5 0 0,0 13.5,19.5C13.5,19.11 13.35,18.76 13.11,18.5C12.88,18.23 12.73,17.88 12.73,17.5A1.5,1.5 0 0,1 14.23,16H16A5,5 0 0,0 21,11C21,6.58 16.97,3 12,3Z" /></svg>`,
	}

	e.svgs["lsp_file"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M13,9V3.5L18.5,9M6,2C4.89,2 4,2.89 4,4V20A2,2 0 0,0 6,22H18A2,2 0 0,0 20,20V8L14,2H6Z" /></svg>`,
	}

	e.svgs["lsp_reference"] = &SvgXML{ // Need to change to a better icon
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M3,6V22H21V24H3A2,2 0 0,1 1,22V6H3M16,9H21.5L16,3.5V9M7,2H17L23,8V18A2,2 0 0,1 21,20H7C5.89,20 5,19.1 5,18V4A2,2 0 0,1 7,2M7,4V18H21V11H14V4H7Z" /></svg>`,
	}

	e.svgs["lsp_folder"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
	}

	e.svgs["lsp_enumMember"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
	}

	e.svgs["lsp_constant"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
	}

	e.svgs["lsp_struct"] = e.svgs["lsp_class"]

	e.svgs["lsp_event"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
	}

	e.svgs["lsp_operator"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
	}

	e.svgs["lsp_typeParameter"] = &SvgXML{ // TODO
		width:  24,
		height: 24,
	}

	// icons for ext_message in goneovim
	e.svgs["quickfix"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><g transform="translate(0,2) scale(0.9)"><path fill="%s" d="M7.5,5.6L5,7L6.4,4.5L5,2L7.5,3.4L10,2L8.6,4.5L10,7L7.5,5.6M19.5,15.4L22,14L20.6,16.5L22,19L19.5,17.6L17,19L18.4,16.5L17,14L19.5,15.4M22,2L20.6,4.5L22,7L19.5,5.6L17,7L18.4,4.5L17,2L19.5,3.4L22,2M13.34,12.78L15.78,10.34L13.66,8.22L11.22,10.66L13.34,12.78M14.37,7.29L16.71,9.63C17.1,10 17.1,10.65 16.71,11.04L5.04,22.71C4.65,23.1 4,23.1 3.63,22.71L1.29,20.37C0.9,20 0.9,19.35 1.29,18.96L12.96,7.29C13.35,6.9 14,6.9 14.37,7.29Z" /></g></svg>`,
	}

	e.svgs["return_prompt"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><g transform="translate(0,1) scale(0.9)"><path fill="%s" d="M19,7V11H5.83L9.41,7.41L8,6L2,12L8,18L9.41,16.58L5.83,13H21V7H19Z" /></g></svg>`,
	}

	e.svgs["echoerr"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><g transform="translate(0,3) scale(0.88)"><path fill="%s" d="M13,10H11V6H13M13,14H11V12H13M20,2H4A2,2 0 0,0 2,4V22L6,18H20A2,2 0 0,0 22,16V4C22,2.89 21.1,2 20,2Z" /></g></svg>`,
	}

	e.svgs["echomsg"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><g transform="translate(0,3) scale(0.88)"><path fill="%s" d="M13,10H11V6H13M13,14H11V12H13M20,2H4A2,2 0 0,0 2,4V22L6,18H20A2,2 0 0,0 22,16V4C22,2.89 21.1,2 20,2Z" /></g></svg>`,
	}

	e.svgs["echo"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><g transform="translate(0,3) scale(0.88)"><path fill="%s" d="M13,10H11V6H13M13,14H11V12H13M20,2H4A2,2 0 0,0 2,4V22L6,18H20A2,2 0 0,0 22,16V4C22,2.89 21.1,2 20,2Z" /></g></svg>`,
	}

	e.svgs["emsg"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><g transform="translate(0,3) scale(0.88)"><path fill="%s" d="M13,10H11V6H13M13,14H11V12H13M20,2H4A2,2 0 0,0 2,4V22L6,18H20A2,2 0 0,0 22,16V4C22,2.89 21.1,2 20,2Z" /></g></svg>`,
	}

	// icons for filer in goneovim
	e.svgs["chevron-down"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M7.41,8.58L12,13.17L16.59,8.58L18,10L12,16L6,10L7.41,8.58Z" /></svg>`,
	}

	e.svgs["chevron-right"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M8.59,16.58L13.17,12L8.59,7.41L10,6L16,12L10,18L8.59,16.58Z" /></svg>`,
	}

	// icons for lint in goneovim
	e.svgs["warn"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M13,14H11V10H13M13,18H11V16H13M1,21H23L12,2L1,21Z" /></svg>`,
	}

	e.svgs["info"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M13,9H11V7H13M13,17H11V11H13M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2Z" /></svg>`,
	}

	e.svgs["linterr"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 2,
		xml:       `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M13 13H11V7H13M13 17H11V15H13M12 2A10 10 0 0 0 2 12A10 10 0 0 0 12 22A10 10 0 0 0 22 12A10 10 0 0 0 12 2Z" /></svg>`,
	}

	e.svgs["lintwrn"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 2,
		xml:       `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M13 14H11V10H13M13 18H11V16H13M1 21H23L12 2L1 21Z" /></svg>`,
	}

	// icons for goneovim external statusline
	e.svgs["lock"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M12,17A2,2 0 0,0 14,15C14,13.89 13.1,13 12,13A2,2 0 0,0 10,15A2,2 0 0,0 12,17M18,8A2,2 0 0,1 20,10V20A2,2 0 0,1 18,22H6A2,2 0 0,1 4,20V10C4,8.89 4.9,8 6,8H7V6A5,5 0 0,1 12,1A5,5 0 0,1 17,6V8H18M12,3A3,3 0 0,0 9,6V8H15V6A3,3 0 0,0 12,3Z" /></svg>`,
	}

	e.svgs["thought"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M3.5,19A1.5,1.5 0 0,1 5,20.5A1.5,1.5 0 0,1 3.5,22A1.5,1.5 0 0,1 2,20.5A1.5,1.5 0 0,1 3.5,19M8.5,16A2.5,2.5 0 0,1 11,18.5A2.5,2.5 0 0,1 8.5,21A2.5,2.5 0 0,1 6,18.5A2.5,2.5 0 0,1 8.5,16M14.5,15C13.31,15 12.23,14.5 11.5,13.65C10.77,14.5 9.69,15 8.5,15C6.54,15 4.91,13.59 4.57,11.74C3.07,11.16 2,9.7 2,8A4,4 0 0,1 6,4C6.26,4 6.5,4.03 6.77,4.07C7.5,3.41 8.45,3 9.5,3C10.69,3 11.77,3.5 12.5,4.35C13.23,3.5 14.31,3 15.5,3C17.46,3 19.09,4.41 19.43,6.26C20.93,6.84 22,8.3 22,10A4,4 0 0,1 18,14L17.23,13.93C16.5,14.59 15.55,15 14.5,15Z" /></svg>`,
	}

	e.svgs["command"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><g transform="scale(0.93)"><path fill="%s" d="M6 2A4 4 0 0 1 10 6V8H14V6A4 4 0 0 1 18 2A4 4 0 0 1 22 6A4 4 0 0 1 18 10H16V14H18A4 4 0 0 1 22 18A4 4 0 0 1 18 22A4 4 0 0 1 14 18V16H10V18A4 4 0 0 1 6 22A4 4 0 0 1 2 18A4 4 0 0 1 6 14H8V10H6A4 4 0 0 1 2 6A4 4 0 0 1 6 2M16 18A2 2 0 0 0 18 20A2 2 0 0 0 20 18A2 2 0 0 0 18 16H16V18M14 10H10V14H14V10M6 16A2 2 0 0 0 4 18A2 2 0 0 0 6 20A2 2 0 0 0 8 18V16H6M8 6A2 2 0 0 0 6 4A2 2 0 0 0 4 6A2 2 0 0 0 6 8H8V6M18 8A2 2 0 0 0 20 6A2 2 0 0 0 18 4A2 2 0 0 0 16 6V8H18Z" /></g></svg>`,
	}

	e.svgs["replace"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M12 5C16.97 5 21 7.69 21 11C21 12.68 19.96 14.2 18.29 15.29C19.36 14.42 20 13.32 20 12.13C20 9.29 16.42 7 12 7V10L8 6L12 2V5M12 19C7.03 19 3 16.31 3 13C3 11.32 4.04 9.8 5.71 8.71C4.64 9.58 4 10.68 4 11.88C4 14.71 7.58 17 12 17V14L16 18L12 22V19Z" /></svg>`,
	}

	// terminal
	e.svgs["terminal"] = &SvgXML{
		width:  32,
		height: 32,
		xml:    `<svg width="32" height="32" viewBox="0 0 32 32"><g transform="scale(1.16) translate(-3,-3)"><path fill="%s" d="M25.716 6.696h-19.296c-0.888 0-1.608 0.72-1.608 1.608v16.080c0 0.888 0.72 1.608 1.608 1.608h19.296c0.888 0 1.608-0.72 1.608-1.608v-16.080c0-0.888-0.72-1.608-1.608-1.608zM8.028 17.952l3.216-3.216-3.216-3.216 1.608-1.608 4.824 4.824-4.824 4.824-1.608-1.608zM20.892 19.56h-6.432v-1.608h6.432v1.608z"></path></g></svg>`,
	}

	// for Insert Mode
	e.svgs["edit"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<?xml version="1.0" encoding="utf-8"?>
   <svg width="24" height="24" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><g transform="translate(0,0)"><path fill="%s" d="M20.71,7.04C21.1,6.65 21.1,6 20.71,5.63L18.37,3.29C18,2.9 17.35,2.9 16.96,3.29L15.12,5.12L18.87,8.87M3,17.25V21H6.75L17.81,9.93L14.06,6.18L3,17.25Z" /></g></svg>`,
	}

	// for Visual Mode
	e.svgs["select"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M14,17H17V14H19V17H22V19H19V22H17V19H14V17M12,17V19H9V17H12M7,17V19H3V15H5V17H7M3,13V10H5V13H3M3,8V4H7V6H5V8H3M9,4H12V6H9V4M15,4H19V8H17V6H15V4M19,10V12H17V10H19Z" /></svg>`,
	}

	e.svgs["bad"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M11 15H13V17H11V15M11 7H13V13H11V7M12 2C6.47 2 2 6.5 2 12A10 10 0 0 0 12 22A10 10 0 0 0 22 12A10 10 0 0 0 12 2M12 20A8 8 0 0 1 4 12A8 8 0 0 1 12 4A8 8 0 0 1 20 12A8 8 0 0 1 12 20Z" /></svg>`,
		//xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M13.46 12L19 17.54V19H17.54L12 13.46L6.46 19H5V17.54L10.54 12L5 6.46V5H6.46L12 10.54L17.54 5H19V6.46L13.46 12Z" /></svg>`,
	}

	e.svgs["exclamation"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 0,
		xml:       `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M12 2L1 21H23M12 6L19.53 19H4.47M11 10V14H13V10M11 16V18H13V16" /></svg>`,
	}

	e.svgs["fire"] = &SvgXML{
		width:  1792,
		height: 1792,
		xml: `<?xml version="1.0" encoding="utf-8"?>
<svg width="1792" height="1792" viewBox="0 0 1792 1792" xmlns="http://www.w3.org/2000/svg"><path fill="%s" d="M1600 1696v64q0 13-9.5 22.5t-22.5 9.5h-1344q-13 0-22.5-9.5t-9.5-22.5v-64q0-13 9.5-22.5t22.5-9.5h1344q13 0 22.5 9.5t9.5 22.5zm-256-1056q0 78-24.5 144t-64 112.5-87.5 88-96 77.5-87.5 72-64 81.5-24.5 96.5q0 96 67 224l-4-1 1 1q-90-41-160-83t-138.5-100-113.5-122.5-72.5-150.5-27.5-184q0-78 24.5-144t64-112.5 87.5-88 96-77.5 87.5-72 64-81.5 24.5-96.5q0-94-66-224l3 1-1-1q90 41 160 83t138.5 100 113.5 122.5 72.5 150.5 27.5 184z"/></svg>`,
	}

	e.svgs["comment"] = &SvgXML{
		width:  1792,
		height: 1792,
		xml: `<?xml version="1.0" encoding="utf-8"?>
<svg width="1792" height="1792" viewBox="0 0 1792 1792" xmlns="http://www.w3.org/2000/svg"><path fill="%s" d="M1792 896q0 174-120 321.5t-326 233-450 85.5q-70 0-145-8-198 175-460 242-49 14-114 22-17 2-30.5-9t-17.5-29v-1q-3-4-.5-12t2-10 4.5-9.5l6-9 7-8.5 8-9q7-8 31-34.5t34.5-38 31-39.5 32.5-51 27-59 26-76q-157-89-247.5-220t-90.5-281q0-130 71-248.5t191-204.5 286-136.5 348-50.5q244 0 450 85.5t326 233 120 321.5z"/></svg>`,
	}

	// icons for tabline in goneovim
	e.svgs["hoverclose"] = &SvgXML{
		width:  1792,
		height: 1792,
		xml:    `<svg width="1792" height="1792" viewBox="0 0 1792 1792" xmlns="http://www.w3.org/2000/svg"><path fill="%s" d="M1277 1122q0-26-19-45l-181-181 181-181q19-19 19-45 0-27-19-46l-90-90q-19-19-46-19-26 0-45 19l-181 181-181-181q-19-19-45-19-27 0-46 19l-90 90q-19 19-19 46 0 26 19 45l181 181-181 181q-19 19-19 45 0 27 19 46l90 90q19 19 46 19 26 0 45-19l181-181 181 181q19 19 45 19 27 0 46-19l90-90q19-19 19-46zm387-226q0 209-103 385.5t-279.5 279.5-385.5 103-385.5-103-279.5-279.5-103-385.5 103-385.5 279.5-279.5 385.5-103 385.5 103 279.5 279.5 103 385.5z"/></svg>`,
	}

	e.svgs["cross"] = &SvgXML{
		width:     100,
		height:    100,
		thickness: 2,
		xml:       `<?xml version="1.0" encoding="utf-8"?><svg width="1792" height="1792" viewBox="0 0 1792 1792" xmlns="http://www.w3.org/2000/svg"><path fill="%s" d="M1490 1322q0 40-28 68l-136 136q-28 28-68 28t-68-28l-294-294-294 294q-28 28-68 28t-68-28l-136-136q-28-28-28-68t28-68l294-294-294-294q-28-28-28-68t28-68l136-136q28-28 68-28t68 28l294 294 294-294q28-28 68-28t68 28l136 136q28 28 28 68t-28 68l-294 294 294 294q28 28 28 68z"/></svg>`,
	}

	e.svgs["directory"] = &SvgXML{
		width:  24,
		height: 24,
		xml: `<?xml version="1.0" encoding="utf-8"?>
<svg width="24" height="24" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="%s" d="M10 4H4C2.89 4 2 4.89 2 6V18A2 2 0 0 0 4 20H20A2 2 0 0 0 22 18V8C22 6.89 21.1 6 20 6H12L10 4Z" /></svg>`,
	}

	e.svgs["empty"] = &SvgXML{
		width:  200,
		height: 200,
		xml:    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200"><g><path fill="%s" d=""/></g></svg>`,
	}

	e.svgs["folder"] = &SvgXML{
		width:     1792,
		height:    1792,
		thickness: 0.5,
		xml:       `<svg width="1792" height="1792" viewBox="0 0 1792 1792" xmlns="http://www.w3.org/2000/svg"><g><path fill="%s" d="M1600 1312v-704q0-40-28-68t-68-28h-704q-40 0-68-28t-28-68v-64q0-40-28-68t-68-28h-320q-40 0-68 28t-28 68v960q0 40 28 68t68 28h1216q40 0 68-28t28-68zm128-704v704q0 92-66 158t-158 66h-1216q-92 0-158-66t-66-158v-960q0-92 66-158t158-66h320q92 0 158 66t66 158v32h672q92 0 158 66t66 158z"/></g></svg>`,
	}
	e.svgs["git"] = &SvgXML{
		width:  24,
		height: 24,
		xml:    `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M2.6,10.59L8.38,4.8L10.07,6.5C9.83,7.35 10.22,8.28 11,8.73V14.27C10.4,14.61 10,15.26 10,16A2,2 0 0,0 12,18A2,2 0 0,0 14,16C14,15.26 13.6,14.61 13,14.27V9.41L15.07,11.5C15,11.65 15,11.82 15,12A2,2 0 0,0 17,14A2,2 0 0,0 19,12A2,2 0 0,0 17,10C16.82,10 16.65,10 16.5,10.07L13.93,7.5C14.19,6.57 13.71,5.55 12.78,5.16C12.35,5 11.9,4.96 11.5,5.07L9.8,3.38L10.59,2.6C11.37,1.81 12.63,1.81 13.41,2.6L21.4,10.59C22.19,11.37 22.19,12.63 21.4,13.41L13.41,21.4C12.63,22.19 11.37,22.19 10.59,21.4L2.6,13.41C1.81,12.63 1.81,11.37 2.6,10.59Z" /></svg>`,
	}

	e.svgs["json"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 0,
		color:     hexToRGBA(GithubLangJSONiq),
		xml:       `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M5 3H7V5H5V10A2 2 0 0 1 3 12A2 2 0 0 1 5 14V19H7V21H5C3.93 20.73 3 20.1 3 19V15A2 2 0 0 0 1 13H0V11H1A2 2 0 0 0 3 9V5A2 2 0 0 1 5 3M19 3A2 2 0 0 1 21 5V9A2 2 0 0 0 23 11H24V13H23A2 2 0 0 0 21 15V19A2 2 0 0 1 19 21H17V19H19V14A2 2 0 0 1 21 12A2 2 0 0 1 19 10V5H17V3H19M12 15A1 1 0 0 1 13 16A1 1 0 0 1 12 17A1 1 0 0 1 11 16A1 1 0 0 1 12 15M8 15A1 1 0 0 1 9 16A1 1 0 0 1 8 17A1 1 0 0 1 7 16A1 1 0 0 1 8 15M16 15A1 1 0 0 1 17 16A1 1 0 0 1 16 17A1 1 0 0 1 15 16A1 1 0 0 1 16 15Z" /></svg>`,
	}

	e.svgs["ts"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 0,
		color:     hexToRGBA(GithubLangTypeScript),
		xml:       `<svg width="24" height="24" viewBox="0 0 24 24"><path fill="%s" d="M3 3H21V21H3V3M13.71 17.86C14.21 18.84 15.22 19.59 16.8 19.59C18.4 19.59 19.6 18.76 19.6 17.23C19.6 15.82 18.79 15.19 17.35 14.57L16.93 14.39C16.2 14.08 15.89 13.87 15.89 13.37C15.89 12.96 16.2 12.64 16.7 12.64C17.18 12.64 17.5 12.85 17.79 13.37L19.1 12.5C18.55 11.54 17.77 11.17 16.7 11.17C15.19 11.17 14.22 12.13 14.22 13.4C14.22 14.78 15.03 15.43 16.25 15.95L16.67 16.13C17.45 16.47 17.91 16.68 17.91 17.26C17.91 17.74 17.46 18.09 16.76 18.09C15.93 18.09 15.45 17.66 15.09 17.06L13.71 17.86M13 11.25H8V12.75H9.5V20H11.25V12.75H13V11.25Z" /></svg>`,
	}

	e.svgs["js"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 0,
		color:     hexToRGBA(GithubLangJavaScript),
		xml:       `<svg style="width:24px;height:24px" viewBox="0 0 24 24"><path fill="%s" d="M3 3H21V21H3V3M7.73 18.04C8.13 18.89 8.92 19.59 10.27 19.59C11.77 19.59 12.8 18.79 12.8 17.04V11.26H11.1V17C11.1 17.86 10.75 18.08 10.2 18.08C9.62 18.08 9.38 17.68 9.11 17.21L7.73 18.04M13.71 17.86C14.21 18.84 15.22 19.59 16.8 19.59C18.4 19.59 19.6 18.76 19.6 17.23C19.6 15.82 18.79 15.19 17.35 14.57L16.93 14.39C16.2 14.08 15.89 13.87 15.89 13.37C15.89 12.96 16.2 12.64 16.7 12.64C17.18 12.64 17.5 12.85 17.79 13.37L19.1 12.5C18.55 11.54 17.77 11.17 16.7 11.17C15.19 11.17 14.22 12.13 14.22 13.4C14.22 14.78 15.03 15.43 16.25 15.95L16.67 16.13C17.45 16.47 17.91 16.68 17.91 17.26C17.91 17.74 17.46 18.09 16.76 18.09C15.93 18.09 15.45 17.66 15.09 17.06L13.71 17.86Z" /></svg>`,
	}

	e.svgs["markdown"] = &SvgXML{
		width:     1024,
		height:    1024,
		thickness: 0,
		xml:       `<svg height="1024" width="1024" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><path fill="%s" d="M950.154 192H73.846C33.127 192 0 225.12699999999995 0 265.846v492.308C0 798.875 33.127 832 73.846 832h876.308c40.721 0 73.846-33.125 73.846-73.846V265.846C1024 225.12699999999995 990.875 192 950.154 192zM576 703.875L448 704V512l-96 123.077L256 512v192H128V320h128l96 128 96-128 128-0.125V703.875zM767.091 735.875L608 512h96V320h128v192h96L767.091 735.875z" /></svg>`,
	}

	e.svgs["sh"] = &SvgXML{
		width:     18,
		height:    18,
		thickness: 0,
		xml:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 18 18"><g><path fill="%s" d="M1.9426860809326185,2.020794440799331 V15.979206420212122 H16.057314060551434 V2.020794440799331 zm2.1962214082875993,1.8041484023191938 l1.488652194536292,1.1762194099782244 l-1.488652194536292,1.2543270180082322 zm0,3.606765445891675 H13.234695018669578 v1.0200022335531784 H4.138907489220218 zm0,2.9007278261308826 h5.723346475939733 v1.0200022335531784 H4.138907489220218 zm0,2.824150479043169 h7.60560350567875 v1.018470796391866 H4.138907489220218 z" /></g></svg>`,
	}
	e.svgs["py"] = &SvgXML{
		width:     128,
		height:    128,
		thickness: 0,
		color:     hexToRGBA(GithubLangPython),
		xml:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128"><g><path fill="%s" d="M49.33 62h29.159c8.117 0 14.511-6.868 14.511-15.019v-27.798c0-7.912-6.632-13.856-14.555-15.176-5.014-.835-10.195-1.215-15.187-1.191-4.99.023-9.612.448-13.805 1.191-12.355 2.181-14.453 6.751-14.453 15.176v10.817h29v4h-40.224000000000004c-8.484 0-15.914 5.108-18.237 14.811-2.681 11.12-2.8 17.919 0 29.53 2.075 8.642 7.03 14.659 15.515 14.659h9.946v-13.048c0-9.637 8.428-17.952 18.33-17.952zm-1.838-39.11c-3.026 0-5.478-2.479-5.478-5.545 0-3.079 2.451-5.581 5.478-5.581 3.015 0 5.479 2.502 5.479 5.581-.001 3.066-2.465 5.545-5.479 5.545zM122.281 48.811c-2.098-8.448-6.103-14.811-14.599-14.811h-10.682v12.981c0 10.05-8.794 18.019-18.511 18.019h-29.159c-7.988 0-14.33 7.326-14.33 15.326v27.8c0 7.91 6.745 12.564 14.462 14.834 9.242 2.717 17.994 3.208 29.051 0 7.349-2.129 14.487-6.411 14.487-14.834v-11.126h-29v-4h43.682c8.484 0 11.647-5.776 14.599-14.66 3.047-9.145 2.916-17.799 0-29.529zm-41.955 55.606c3.027 0 5.479 2.479 5.479 5.547 0 3.076-2.451 5.579-5.479 5.579-3.015 0-5.478-2.502-5.478-5.579 0-3.068 2.463-5.547 5.478-5.547z"/></g></svg>`,
	}
	e.svgs["pyc"] = e.svgs["py"]
	e.svgs["c"] = &SvgXML{
		width:     128,
		height:    128,
		thickness: 1,
		color:     hexToRGBA(GithubLangC),
		xml:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128"><g><path fill="%s" d="M97.5221405461895,45.03725476832212 L108.76640973908529,40.28547464292772 C108.56169322951193,38.08885210339795 94.3709708336648,16.355134752322773 69.08566426354695,16.60775090230729 C43.80035769342909,16.86037464885613 18.857250847109363,37.373658362115926 19.237894612168745,64.2531288574854 C19.61853837722812,91.1325841597262 44.09328059332486,111.39442159390717 68.83481490446007,111.39442919047148 C93.57635717502413,111.39443678703581 108.34661354359856,92.47813321193519 107.91159096075516,92.0739200243284 C107.47656837791175,91.66970683672162 100.93955315316323,85.45629455970422 100.26450603444523,85.0460952794112 C99.58945891572723,84.63589599911819 90.07591976589798,98.4070389619417 68.86888921929224,98.15440761882854 C47.66185867268652,97.90177627571538 33.5390540830676,80.57280520208417 33.375925589144344,64.50575260403423 C33.212797095221084,48.43870000598427 47.89705183525975,31.110936786080387 69.10406646300783,30.858313039531545 C90.31109700961355,30.605681696418387 93.33611773457105,42.36367434123211 97.5221405461895,45.03725476832212 z"></path></g></svg>`,
	}
	e.svgs["cpp"] = &SvgXML{
		width:     24,
		height:    24,
		thickness: 0.5,
		color:     hexToRGBA(GithubLangCpp),
		xml: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
    <path fill="%s" d="M10.5 15.97L10.91 18.41C10.65 18.55 10.23 18.68 9.67 18.8C9.1 18.93 8.43 19 7.66 19C5.45 18.96 3.79 18.3 2.68 17.04C1.56 15.77 1 14.16 1 12.21C1.05 9.9 1.72 8.13 3 6.89C4.32 5.64 5.96 5 7.94 5C8.69 5 9.34 5.07 9.88 5.19C10.42 5.31 10.82 5.44 11.08 5.59L10.5 8.08L9.44 7.74C9.04 7.64 8.58 7.59 8.05 7.59C6.89 7.58 5.93 7.95 5.18 8.69C4.42 9.42 4.03 10.54 4 12.03C4 13.39 4.37 14.45 5.08 15.23C5.79 16 6.79 16.4 8.07 16.41L9.4 16.29C9.83 16.21 10.19 16.1 10.5 15.97M11 11H13V9H15V11H17V13H15V15H13V13H11V11M18 11H20V9H22V11H24V13H22V15H20V13H18V11Z" />
</svg>`,
	}
	e.svgs["go"] = &SvgXML{
		width:     128,
		height:    128,
		thickness: 0,
		color:     hexToRGBA(GithubLangGo),
		xml:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128"><g><path fill="%s" d="M108.2 64.8c-.1-.1-.2-.2-.4-.2l-.1-.1c-.1-.1-.2-.1-.2-.2l-.1-.1c-.1 0-.2-.1-.2-.1l-.2-.1c-.1 0-.2-.1-.2-.1l-.2-.1c-.1 0-.2-.1-.2-.1-.1 0-.1 0-.2-.1l-.3-.1c-.1 0-.1 0-.2-.1l-.3-.1h-.1l-.4-.1h-.2c-.1 0-.2 0-.3-.1h-2.3c-.6-13.3.6-26.8-2.8-39.6 12.9-4.6 2.8-22.3-8.4-14.4-7.4-6.4-17.6-7.8-28.3-7.8-10.5.7-20.4 2.9-27.4 8.4-2.8-1.4-5.5-1.8-7.9-1.1v.1c-.1 0-.3.1-.4.2-.1 0-.3.1-.4.2h-.1c-.1 0-.2.1-.4.2h-.1l-.3.2h-.1l-.3.2h-.1l-.3.2s-.1 0-.1.1l-.3.2s-.1 0-.1.1l-.3.2s-.1 0-.1.1l-.3.2-.1.1c-.1.1-.2.1-.2.2l-.1.1-.2.2-.1.1c-.1.1-.1.2-.2.2l-.1.1c-.1.1-.1.2-.2.2l-.1.1c-.1.1-.1.2-.2.2l-.1.1c-.1.1-.1.2-.2.2l-.1.1c-.1.1-.1.2-.2.2l-.1.1-.1.3s0 .1-.1.1l-.1.3s0 .1-.1.1l-.1.3s0 .1-.1.1l-.1.3s0 .1-.1.1c.4.3.4.4.4.4v.1l-.1.3v.1c0 .1 0 .2-.1.3v3.1c0 .1 0 .2.1.3v.1l.1.3v.1l.1.3s0 .1.1.1l.1.3s0 .1.1.1l.1.3s0 .1.1.1l.2.3s0 .1.1.1l.2.3s0 .1.1.1l.2.3.1.1.3.3.3.3h.1c1 .9 2 1.6 4 2.2v-.2c-4.2 12.6-.7 25.3-.5 38.3-.6 0-.7.4-1.7.5h-.5c-.1 0-.3 0-.5.1-.1 0-.3 0-.4.1l-.4.1h-.1l-.4.1h-.1l-.3.1h-.1l-.3.1s-.1 0-.1.1l-.3.1-.2.1c-.1 0-.2.1-.2.1l-.2.1-.2.1c-.1 0-.2.1-.2.1l-.2.1-.4.3c-.1.1-.2.2-.3.2l-.4.4-.1.1c-.1.2-.3.4-.4.5l-.2.3-.3.6-.1.3v.3c0 .5.2.9.9 1.2.2 3.7 3.9 2 5.6.8l.1-.1c.2-.2.5-.3.6-.3h.1l.2-.1c.1 0 .1 0 .2-.1.2-.1.4-.1.5-.2.1 0 .1-.1.1-.2l.1-.1c.1-.2.2-.6.2-1.2l.1-1.3v1.8c-.5 13.1-4 30.7 3.3 42.5 1.3 2.1 2.9 3.9 4.7 5.4h-.5c-.2.2-.5.4-.8.6l-.9.6-.3.2-.6.4-.9.7-1.1 1c-.2.2-.3.4-.4.5l-.4.6-.2.3c-.1.2-.2.4-.2.6l-.1.3c-.2.8 0 1.7.6 2.7l.4.4h.2c.1 0 .2 0 .4.1.2.4 1.2 2.5 3.9.9 2.8-1.5 4.7-4.6 8.1-5.1l-.5-.6c5.9 2.8 12.8 4 19 4.2 8.7.3 18.6-.9 26.5-5.2 2.2.7 3.9 3.9 5.8 5.4l.1.1.1.1.1.1.1.1s.1 0 .1.1c0 0 .1 0 .1.1 0 0 .1 0 .1.1h2.1000000000000005s.1 0 .1-.1h.1s.1 0 .1-.1h.1s.1 0 .1-.1c0 0 .1 0 .1-.1l.1-.1s.1 0 .1-.1l.1-.1h.1l.2-.2.2-.1h.1l.1-.1h.1l.1-.1.1-.1.1-.1.1-.1.1-.1.1-.1.1-.1v-.1s0-.1.1-.1v-.1s0-.1.1-.1v-.1s0-.1.1-.1v-1.4000000000000001s-.3 0-.3-.1l-.3-.1v-.1l.3-.1s.2 0 .2-.1l.1-.1v-2.1000000000000005s0-.1-.1-.1v-.1s0-.1-.1-.1v-.1s0-.1-.1-.1c0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1 0 0 0-.1-.1-.1l-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1v-.1l-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1-.1c2-1.9 3.8-4.2 5.1-6.9 5.9-11.8 4.9-26.2 4.1-39.2h.1c.1 0 .2.1.2.1h.30000000000000004s.1 0 .1.1h.1s.1 0 .1.1l.2.1c1.7 1.2 5.4 2.9 5.6-.8 1.6.6-.3-1.8-1.3-2.5zm-72.2-41.8c-3.2-16 22.4-19 23.3-3.4.8 13-20 16.3-23.3 3.4zm36.1 15c-1.3 1.4-2.7 1.2-4.1.7 0 1.9.4 3.9.1 5.9-.5.9-1.5 1-2.3 1.4-1.2-.2-2.1-.9-2.6-2l-.2-.1c-3.9 5.2-6.3-1.1-5.2-5-1.2.1-2.2-.2-3-1.5-1.4-2.6.7-5.8 3.4-6.3.7 3 8.7 2.6 10.1-.2 3.1 1.5 6.5 4.3 3.8 7.1zm-7-17.5c-.9-13.8 20.3-17.5 23.4-4 3.5 15-20.8 18.9-23.4 4zM41.7 17c-1.9 0-3.5 1.7-3.5 3.8 0 2.1 1.6 3.8 3.5 3.8s3.5-1.7 3.5-3.8c0-2.1-1.5-3.8-3.5-3.8zm1.6 5.7c-.5 0-.8-.4-.8-1 0-.5.4-1 .8-1 .5 0 .8.4.8 1 0 .5-.3 1-.8 1zM71.1 16.1c-1.9 0-3.4 1.7-3.4 3.8 0 2.1 1.5 3.8 3.4 3.8s3.4-1.7 3.4-3.8c0-2.1-1.5-3.8-3.4-3.8zm1.6 5.6c-.4 0-.8-.4-.8-1 0-.5.4-1 .8-1s.8.4.8 1-.4 1-.8 1z"/></g></svg>`,
	}

	e.svgs["lua"] = &SvgXML{
		width:     128,
		height:    128,
		thickness: 0,
		color:     hexToRGBA(GithubLangLua),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><g transform="scale(1.2)">	<path fill="%[1]s" d="M14.7,26C9.9,26,6,22.1,6,17.3c0-4.8,3.9-8.8,8.9-8.7c4.7,0.1,8.5,3.9,8.5,8.7C23.4,22.1,19.5,26,14.7,26z M18.3,11.1		c-1.4,0-2.6,1.2-2.6,2.6c0,1.4,1.1,2.6,2.6,2.6c1.4,0,2.6-1.1,2.6-2.6C20.9,12.3,19.8,11.1,18.3,11.1z"/>	<path fill="%[1]s" d="M23.4,11.1c-1.4,0-2.6-1.1-2.5-2.6C20.9,7.1,22,6,23.5,6C24.9,6,26,7.2,26,8.6C26,10,24.8,11.1,23.4,11.1z"/></g></svg>`,
	}

	e.svgs["rust"] = &SvgXML{
		width:     128,
		height:    128,
		thickness: 0,
		color:     hexToRGBA(GithubLangRust),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><g transform="scale(1.25)"><path fill="%s" d="M21.7,8.4c0,0.2,0,0.3,0,0.5c0,0,0,0.1,0,0.1c0,0,0.1,0.1,0.1,0.1c0,0,0.1,0,0.1,0c0.3-0.1,0.6-0.1,0.9-0.2	C23,8.8,23.2,9,23.1,9.2c-0.1,0.3-0.1,0.6-0.2,0.9c0,0,0,0.1,0,0.1c0,0,0,0,0.1,0.1c0,0.1,0.1,0.1,0.2,0.1c0.3,0,0.6,0,0.9,0	c0.2,0,0.3,0.1,0.3,0.3c0,0.1,0,0.1,0,0.2c-0.1,0.3-0.3,0.6-0.4,0.8c0,0,0,0,0,0.1c0,0,0.1,0.1,0.1,0.2c0,0,0,0,0.1,0	c0.3,0.1,0.6,0.1,0.9,0.2c0.2,0,0.3,0.3,0.2,0.5c-0.2,0.3-0.4,0.5-0.5,0.7c0,0,0,0.1,0,0.1c0,0,0,0,0,0.1c0,0.1,0.1,0.1,0.2,0.2	c0.3,0.1,0.5,0.2,0.8,0.3c0.2,0.1,0.3,0.3,0.1,0.5c-0.2,0.2-0.4,0.4-0.7,0.6c0,0,0,0,0,0.1c0,0.1,0,0.1,0,0.2c0,0,0.1,0.1,0.2,0.1	c0.2,0.1,0.4,0.3,0.7,0.4c0.2,0.1,0.2,0.4,0,0.5c-0.3,0.2-0.5,0.3-0.8,0.5c0,0,0,0,0,0.1c0,0.2,0,0.2,0.1,0.3	c0.2,0.2,0.4,0.4,0.6,0.5c0.2,0.2,0.1,0.4-0.1,0.5c-0.3,0.1-0.6,0.2-0.8,0.3c0,0-0.1,0-0.1,0.1c0,0,0,0,0,0c-0.1,0.1,0,0.2,0,0.3	c0.2,0.2,0.3,0.4,0.5,0.7c0.1,0.1,0.1,0.3-0.1,0.4c-0.1,0-0.1,0-0.2,0.1c-0.3,0-0.5,0.1-0.8,0.1c0,0,0,0-0.1,0	c0,0.1-0.1,0.1-0.1,0.2c0,0.1,0,0.1,0.1,0.2c0.1,0.2,0.2,0.5,0.3,0.7c0.1,0.1,0,0.3-0.1,0.4c-0.1,0-0.1,0-0.2,0c-0.3,0-0.6,0-0.9,0	c0,0-0.1,0-0.1,0c-0.1,0.1-0.1,0.2-0.1,0.3c0.1,0.3,0.1,0.5,0.2,0.8c0.1,0.2-0.1,0.4-0.4,0.4c-0.3-0.1-0.6-0.1-0.9-0.2	c0,0-0.1,0-0.1,0c0,0-0.1,0.1-0.1,0.1c0,0-0.1,0.1,0,0.1c0,0.3,0,0.6,0,0.9c0,0.2-0.1,0.3-0.3,0.3c-0.1,0-0.1,0-0.2,0	c-0.3-0.1-0.5-0.2-0.8-0.4c0,0-0.1,0-0.1,0c0,0,0,0,0,0c-0.1,0-0.2,0.1-0.2,0.2c0,0.3-0.1,0.5-0.1,0.8c0,0.2-0.3,0.3-0.5,0.2	c-0.2-0.2-0.5-0.4-0.7-0.5c0,0-0.1,0-0.1,0c0,0,0,0,0,0c-0.1,0-0.2,0.1-0.2,0.2c-0.1,0.3-0.2,0.5-0.3,0.8c-0.1,0.2-0.2,0.2-0.3,0.2	c-0.1,0-0.1-0.1-0.1-0.1c-0.2-0.2-0.4-0.4-0.6-0.7c0,0,0,0-0.1,0c0,0,0,0-0.1,0c-0.1,0-0.2,0.1-0.2,0.2c-0.1,0.2-0.3,0.5-0.4,0.7	c-0.1,0.2-0.4,0.2-0.5,0c-0.2-0.3-0.3-0.5-0.5-0.8c0,0,0,0-0.1,0c0,0-0.1,0-0.1,0c-0.1,0-0.1,0-0.2,0.1c-0.2,0.2-0.4,0.4-0.6,0.6	c-0.1,0.1-0.2,0.1-0.4,0.1c-0.1,0-0.1-0.1-0.1-0.2c-0.1-0.3-0.2-0.6-0.3-0.9c0,0,0-0.1-0.1-0.1c0,0-0.1,0-0.1,0c-0.1,0-0.2,0-0.2,0	c-0.2,0.2-0.4,0.3-0.7,0.5C12.3,25.3,12,25.2,12,25c-0.1-0.3-0.1-0.6-0.1-0.9c0,0,0-0.1-0.1-0.1c-0.1,0-0.1-0.1-0.2-0.1	c-0.1,0-0.1,0-0.2,0.1c-0.2,0.1-0.5,0.2-0.7,0.3c-0.2,0.1-0.4,0-0.4-0.2c0,0,0-0.1,0-0.1c0-0.3,0-0.6,0-0.9c0,0,0-0.1,0-0.1	c0,0-0.1-0.1-0.1-0.1c0,0-0.1,0-0.1,0c-0.3,0.1-0.6,0.1-0.9,0.2C9,23.2,8.8,23,8.9,22.8c0.1-0.3,0.1-0.6,0.2-0.9c0,0,0-0.1,0-0.1	c0,0-0.1-0.1-0.1-0.1c0,0,0,0-0.1,0c-0.3,0-0.6,0-0.9,0c-0.2,0-0.3-0.1-0.3-0.3c0-0.1,0-0.1,0-0.2c0.1-0.3,0.3-0.6,0.4-0.8	c0,0,0-0.1,0-0.1c0,0,0-0.1-0.1-0.1c0-0.1-0.1-0.1-0.1-0.1c-0.3,0-0.6-0.1-0.9-0.1c-0.2,0-0.3-0.3-0.2-0.5C7,19.3,7.2,19,7.3,18.8	c0,0,0-0.1,0-0.1c0-0.1,0-0.1-0.1-0.2c0,0-0.1-0.1-0.2-0.1c-0.2-0.1-0.5-0.2-0.7-0.3c-0.2-0.1-0.3-0.3-0.1-0.5	C6.5,17.5,6.7,17.2,7,17c0,0,0,0,0-0.1c0,0,0,0,0-0.1c0-0.1,0-0.2-0.1-0.2c-0.2-0.1-0.5-0.3-0.7-0.4c-0.2-0.1-0.2-0.4,0-0.5	c0.3-0.2,0.5-0.3,0.8-0.5c0,0,0,0,0-0.1c0,0,0-0.1,0-0.1C7,15,7,15,6.9,14.9c-0.2-0.2-0.4-0.4-0.6-0.6c-0.1-0.1-0.1-0.3,0-0.4	c0,0,0.1,0,0.1-0.1c0.3-0.1,0.6-0.2,0.9-0.3c0,0,0,0,0,0c0,0,0,0,0-0.1c0.1-0.1,0-0.2,0-0.3c-0.2-0.2-0.3-0.4-0.5-0.6	C6.7,12.3,6.8,12,7,12c0.3-0.1,0.6-0.1,0.9-0.2c0,0,0,0,0.1,0c0-0.1,0.1-0.1,0.1-0.2c0-0.1,0-0.1-0.1-0.2c-0.1-0.2-0.2-0.5-0.3-0.7	c-0.1-0.2,0-0.4,0.2-0.4c0,0,0.1,0,0.1,0c0.3,0,0.6,0,0.9,0c0,0,0.1,0,0.1,0c0,0,0-0.1,0.1-0.1c0,0,0-0.1,0-0.1	C9,9.8,8.9,9.5,8.9,9.2C8.8,9,9,8.8,9.2,8.9C9.5,8.9,9.8,9,10.1,9.1c0,0,0.1,0,0.1,0c0,0,0.1-0.1,0.1-0.1c0,0,0.1-0.1,0-0.1	c0-0.3,0-0.6,0-0.9c0-0.2,0.1-0.3,0.3-0.3c0.1,0,0.1,0,0.2,0c0.3,0.1,0.6,0.3,0.8,0.4c0,0,0.1,0,0.1,0c0,0,0.1,0,0.1-0.1	c0.1,0,0.1-0.1,0.1-0.1c0-0.3,0.1-0.6,0.1-0.9c0-0.2,0.3-0.3,0.5-0.2C12.7,7,13,7.2,13.2,7.3c0,0,0.1,0,0.1,0c0.1,0,0.1,0,0.2-0.1	c0,0,0-0.1,0.1-0.2c0.1-0.2,0.2-0.5,0.3-0.7c0.1-0.2,0.2-0.2,0.4-0.2c0,0,0.1,0.1,0.1,0.1C14.5,6.5,14.8,6.7,15,7c0,0,0,0,0.1,0	c0,0,0,0,0,0c0.1,0,0.2-0.1,0.2-0.1c0.1-0.2,0.3-0.5,0.4-0.7C15.9,6,16.1,6,16.2,6.1c0,0,0.1,0.1,0.1,0.1c0.2,0.3,0.3,0.5,0.5,0.8	c0,0,0,0,0.1,0c0.1,0,0.1,0,0.2,0c0.1,0,0.1-0.1,0.1-0.1c0.2-0.2,0.4-0.4,0.5-0.6c0.1-0.1,0.3-0.1,0.4,0c0,0,0,0.1,0,0.1	c0.1,0.3,0.2,0.6,0.3,0.8c0,0,0,0,0.1,0.1c0,0,0.1,0,0.1,0c0,0,0.1,0,0.1,0C19,7.2,19.3,7,19.5,6.8C19.7,6.7,20,6.8,20,7	c0,0.3,0.1,0.6,0.1,0.9c0,0,0,0.1,0.1,0.1c0,0,0,0,0.1,0c0.1,0.1,0.2,0.1,0.2,0c0.2-0.1,0.5-0.2,0.8-0.3c0.2-0.1,0.4,0,0.4,0.3	C21.7,8.1,21.7,8.3,21.7,8.4C21.7,8.4,21.7,8.4,21.7,8.4z M10.6,11.1C10.6,11.1,10.6,11.1,10.6,11.1c2.6,0,5.1,0,7.6,0	c0.3,0,0.6,0,0.9,0.1c0.6,0.2,1.1,0.5,1.4,0.9c0.3,0.3,0.5,0.7,0.5,1.2c0,0.4-0.1,0.8-0.3,1.2c-0.2,0.3-0.5,0.6-0.8,0.8	c-0.1,0.1-0.2,0.2-0.3,0.2c0.1,0.1,0.2,0.1,0.3,0.2c0.2,0.2,0.5,0.4,0.6,0.7c0.2,0.3,0.3,0.7,0.3,1c0,0.1,0.1,0.2,0.2,0.3	c0.2,0.2,0.5,0.2,0.8,0.2c0.2,0,0.4-0.1,0.5-0.2c0.2-0.2,0.2-0.4,0.3-0.6c0-0.2,0-0.3,0-0.5c0-0.1,0-0.1,0.1-0.1c0.2,0,0.4,0,0.6,0	c0,0,0,0,0.1,0c0,0,0,0,0,0c0,0,0-0.1,0-0.1c0-0.4,0-0.7,0-1.1c0,0,0-0.1,0-0.1c-0.3-0.1-0.6-0.3-0.9-0.4c-0.1-0.1-0.3-0.1-0.4-0.2	c-0.3-0.1-0.4-0.4-0.3-0.8c0.2-0.5,0.4-1,0.7-1.5c0,0,0-0.1,0-0.1c-0.4-0.6-0.8-1.2-1.4-1.7c-1-0.9-2.2-1.5-3.6-1.8c0,0,0,0-0.1,0	c-0.3,0.3-0.6,0.6-1,0.9c-0.2,0.2-0.6,0.2-0.8,0c-0.3-0.3-0.6-0.6-0.9-0.9c0,0,0,0-0.1,0c-0.4,0.1-0.7,0.2-1,0.3	c-1.1,0.4-2,1-2.8,1.8C10.7,10.9,10.6,11,10.6,11.1z M21.9,20.3c-0.1,0-0.1,0-0.1,0c-1,0-1.9,0-2.9,0c-0.2,0-0.3,0-0.4-0.1	c-0.4-0.2-0.6-0.6-0.7-1c-0.1-0.4-0.2-0.7-0.2-1.1c0-0.2-0.1-0.4-0.2-0.6c-0.2-0.5-0.6-0.8-1.1-0.8c-0.6,0-1.2,0-1.8,0c0,0,0,0,0,0	c0,0.4,0,0.9,0,1.3c0,0,0,0,0.1,0c0.6,0,1.1,0,1.7,0c0.1,0,0.1,0,0.1,0.1c0,0.7,0,1.3,0,2c0,0.1,0,0.1-0.1,0.1c-2,0-3.9,0-5.9,0	c0,0,0,0-0.1,0c0,0,0,0,0,0c0.2,0.3,0.4,0.5,0.6,0.7c0,0,0,0,0.1,0c0.4-0.1,0.8-0.2,1.2-0.2c0.3-0.1,0.6,0.1,0.7,0.4	c0.1,0.4,0.2,0.9,0.3,1.3c0,0,0,0,0,0.1c0.8,0.3,1.6,0.6,2.4,0.6c0.7,0.1,1.4,0,2.1-0.1c0.5-0.1,1-0.3,1.5-0.5c0,0,0,0,0-0.1	c0.1-0.4,0.2-0.9,0.3-1.3c0.1-0.3,0.3-0.5,0.7-0.4c0.4,0.1,0.8,0.2,1.2,0.3c0,0,0,0,0.1,0C21.5,20.8,21.7,20.5,21.9,20.3z M10,13.3	C10,13.3,10,13.3,10,13.3c0.1,0.2,0.2,0.4,0.3,0.6c0,0.1,0.1,0.2,0,0.3c0,0.2-0.2,0.3-0.3,0.4c-0.4,0.2-0.8,0.4-1.2,0.5	c0,0-0.1,0-0.1,0.1c0,0.2,0,0.4,0,0.5c0,0.7,0.1,1.4,0.3,2.2c0,0,0,0.1,0.1,0.1c0.7,0,1.3,0,2,0c0,0,0,0,0.1,0c0-1.6,0-3.1,0-4.7	C10.7,13.3,10.4,13.3,10,13.3z M14.3,14.7C14.4,14.7,14.4,14.7,14.3,14.7c0.8,0,1.6,0,2.3,0c0.2,0,0.4,0,0.6-0.1	c0.1-0.1,0.2-0.1,0.3-0.3c0.1-0.2,0.1-0.5-0.1-0.7c-0.2-0.2-0.5-0.3-0.7-0.3c-0.8,0-1.6,0-2.4,0c0,0,0,0-0.1,0	C14.3,13.8,14.3,14.2,14.3,14.7z M8.3,13.7c0,0.3,0.3,0.6,0.6,0.6c0.3,0,0.6-0.3,0.6-0.6c0-0.3-0.3-0.6-0.6-0.6	C8.6,13.1,8.3,13.4,8.3,13.7z M21,22.1c0-0.3-0.3-0.6-0.6-0.6c-0.3,0-0.6,0.3-0.6,0.6c0,0.3,0.3,0.6,0.6,0.6	C20.7,22.7,21,22.5,21,22.1z M11.6,21.5c-0.3,0-0.6,0.3-0.6,0.6c0,0.3,0.3,0.6,0.6,0.6c0.3,0,0.6-0.3,0.6-0.6	C12.2,21.8,11.9,21.5,11.6,21.5z M16.6,8.4c0-0.3-0.2-0.6-0.6-0.6c-0.3,0-0.6,0.2-0.6,0.6c0,0.3,0.2,0.6,0.6,0.6	C16.3,9,16.5,8.7,16.6,8.4z M23.1,14.4c0.3,0,0.6-0.3,0.6-0.6c0-0.3-0.3-0.6-0.6-0.6c-0.3,0-0.6,0.3-0.6,0.6	C22.5,14.1,22.7,14.4,23.1,14.4z"/></g></svg>`,
	}

	e.svgs["yaml"] = &SvgXML{
		width:  128,
		height: 128,
		color:  hexToRGBA("#945db7"),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><g transform="scale(1.2)">	<path fill-rule="evenodd" clip-rule="evenodd" fill="%[1]s" d="M17.1,19.3c-0.5,0-0.8,0.1-1.1,0.1c-0.5,0-1.1,0-1.8,0		c0.4-4.3,0.7-8.5,1.1-12.8c1.2,0,2.2-0.1,3.2,0c0.3,0,0.7,0.5,0.6,0.8c-0.3,2.3-0.8,4.7-1.2,7C17.7,16,17.4,17.6,17.1,19.3z"/>	<path fill-rule="evenodd" clip-rule="evenodd" fill="%[1]s" d="M17.5,23.3c0.1,1.1-0.8,2.1-2.1,2.2c-1.3,0.1-2.5-0.7-2.6-1.8		c-0.1-1.2,0.8-2.1,2.2-2.2C16.4,21.4,17.4,22.1,17.5,23.3z"/></g></svg>`,
	}

	e.svgs["tex"] = &SvgXML{
		width:  200,
		height: 200,
		color:  hexToRGBA(GithubLangTeX),
		xml: `<svg version="1.1"	 id="Layer_1" inkscape:version="0.91 r13725" sodipodi:docname="tex.svg" xmlns:cc="http://creativecommons.org/ns#" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd" xmlns:svg="http://www.w3.org/2000/svg"	 xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" viewBox="0 0 32 32"	 style="enable-background:new 0 0 32 32;" xml:space="preserve"><style type="text/css">	.st0{fill:none;}</style><sodipodi:namedview  bordercolor="#666666" borderopacity="1" gridtolerance="10" guidetolerance="10" id="namedview7" inkscape:current-layer="Layer_1" inkscape:cx="15.455277" inkscape:cy="12.422063" inkscape:pageopacity="0" inkscape:pageshadow="2" inkscape:window-height="746" inkscape:window-maximized="1" inkscape:window-width="1303" inkscape:window-x="55" inkscape:window-y="-8" inkscape:zoom="16.766602" objecttolerance="10" pagecolor="#ffffff" showgrid="false">	</sodipodi:namedview><g transform="scale(1.2)">	<path fill="%s" class="st0" d="M8.7,18.8c-0.6,0-1.2,0-1.7,0l-0.5,0H11l-0.5,0C9.9,18.8,9.3,18.8,8.7,18.8z"/>	<polygon class="st0" points="24.2,17.5 22.8,15.3 21.3,17.5 21.7,17.6 21.7,18.7 23.9,18.7 23.9,17.5 	"/>	<path fill="%s" d="M7.9,17.5c-0.1,0-0.3,0-0.5,0H6.5v1.3l0.5,0c0.6,0,1.2,0,1.7,0s1.2,0,1.7,0l0.5,0v-1.3h-0.8c-0.3,0-0.4,0-0.5,0		c0,0,0,0,0,0V11H10c1.2,0,1.4,0.1,1.6,1h1.2l-0.2-2.3H4.8l-0.3,3.6h1.2l0-0.5C5.9,11.1,6,11,7.5,11l0.4,0V17.5		C7.9,17.5,7.9,17.5,7.9,17.5z"/>	<path fill="%s" d="M15.8,21.1h-1.7v-2.8h0.6c0.7,0,0.7,0,0.7,0.9v0.5h1.2v-4.1h-1.2v0.5c0,0.9,0,0.9-0.7,0.9h-0.6v-2.4h1.6		c1.5,0,1.6,0.4,1.7,1.8h1.2l-0.3-3.2h-7v1.3h0.5c0.2,0,0.4,0,0.5,0c0,0,0,0,0,0v6.4c0,0,0,0,0,0c-0.1,0-0.3,0-0.5,0h-0.5v1.3h7.1		l0.4-2.6h-1.3C17.5,20.7,17.1,21.1,15.8,21.1z"/>	<path fill="%s" d="M21.7,17.6l-0.5,0l1.5-2.2l1.5,2.2l-0.3,0v1.1h3.6v-1.2h-0.5c-0.6,0-0.6,0-0.7-0.2l-2.4-3.5l1.6-2.3		c0.2-0.2,0.4-0.6,1.1-0.6l0.5,0V9.6l-0.5,0c-0.1,0-0.2,0-0.4,0c0,0,0,0-0.1,0c-0.1,0-0.3,0-0.4,0c0,0,0,0,0,0c0,0-0.1,0-0.1,0		c-0.4,0-0.9,0-1.1,0l-0.5,0l0,1.2l0.5,0c0,0,0,0-0.1,0.1l-1.2,1.7L21.9,11l0.3,0V9.6l-0.5,0c-0.1,0-0.3,0-0.5,0c0,0-0.1,0-0.1,0		c-0.2,0-0.3,0-0.5,0c0,0,0,0,0,0c-0.1,0-0.2,0-0.3,0c-0.3,0-0.9,0-1.2,0l-0.5,0V11h0.5c0.6,0,0.6,0,0.7,0.2l2.1,3.1l-1.8,2.7		L20.1,17c-0.1,0.1-0.2,0.2-0.3,0.3c-0.2,0.2-0.5,0.3-0.8,0.3h-0.5v0.2v1h3.2L21.7,17.6z"/></g></svg>`,
	}

	e.svgs["ruby"] = &SvgXML{
		width:  200,
		height: 200,
		color:  hexToRGBA(GithubLangRuby),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><path fill="%s" d="M17.8,7.5c1.4,0,2.7,0,4.1,0c0.1,0,0.2,0.1,0.3,0.1c1.2,0.3,2,1,2.2,2.2c0,0.2,0.1,0.4,0.1,0.6	c0,0.3,0,0.5,0,0.8c0,0.2-0.1,0.4-0.1,0.6c-0.1,1.4-0.2,2.9-0.3,4.3c-0.2,2.4-0.4,4.7-0.5,7.1c0,0.4-0.1,0.5-0.5,0.5	c-1.1,0.1-2.1,0.1-3.2,0.2c-2.2,0.1-4.4,0.3-6.6,0.4c-0.6,0-1.2,0.1-1.8,0.2c-0.3,0-0.7,0-1,0c-0.1,0-0.2-0.1-0.3-0.1	c-1.2-0.2-2.1-0.9-2.4-2.1c-0.1-0.3-0.1-0.6-0.2-0.9c0-0.2,0-0.3,0-0.5c0-0.1,0.1-0.2,0.1-0.4c0-0.8,0.1-1.7,0.1-2.5	c0-0.5,0-0.9,0.1-1.3c0.7-1.8,1.5-3.5,2.9-4.9c1.7-1.8,3.6-3.2,6-3.8C17.1,7.8,17.4,7.7,17.8,7.5z"/></svg>`,
	}

	e.svgs["haskell"] = &SvgXML{
		width:  200,
		height: 200,
		color:  hexToRGBA(GithubLangHaskell),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><g transform="scale(1.2)">	<path fill="%[1]s" d="M14.2,8.9c1.2,1.7,2.3,3.5,3.5,5.2c2,3,4,5.9,5.9,8.9c0,0,0,0,0,0c-1.2,0-2.4,0-3.5,0c0,0,0,0-0.1-0.1		c-0.9-1.4-1.9-2.8-2.8-4.2c-0.1-0.1,0-0.1-0.1,0c-0.9,1.4-1.9,2.8-2.8,4.2c0,0,0,0-0.1,0.1c-1.2,0-2.4,0-3.5,0c0,0,0,0,0,0		c0,0,0,0,0-0.1c1.5-2.3,3.1-4.6,4.6-6.9c0-0.1,0-0.1,0-0.1c-1.5-2.3-3.1-4.6-4.6-6.9c0,0,0,0,0-0.1C11.9,8.9,13.1,8.9,14.2,8.9z"/>	<path fill="%[1]s" d="M9.5,23.1c-1.2,0-2.4,0-3.5,0C6,23,6,23,6,23c0.4-0.6,0.8-1.3,1.3-1.9c1.1-1.7,2.2-3.3,3.3-5c0-0.1,0-0.1,0-0.1		C9.1,13.6,7.6,11.3,6,9C6,9,6,9,6,9c0,0,0,0,0,0c0.7,0,1.5,0,2.2,0c0.4,0,0.9,0,1.3,0c0,0,0,0,0.1,0.1c0.8,1.1,1.5,2.3,2.3,3.4		c0.8,1.2,1.6,2.3,2.3,3.5c0,0.1,0,0.1,0,0.1C12.7,18.4,11.1,20.7,9.5,23.1C9.6,23,9.6,23,9.5,23.1z"/>	<path fill="%[1]s" d="M26,15.4C26,15.4,26,15.4,26,15.4c-2.1,0-4.2,0-6.2,0c0,0-0.1,0-0.1,0c-0.5-0.8-1-1.5-1.5-2.3c0,0,0,0,0-0.1		c2.6,0,5.2,0,7.8,0C26,13.8,26,14.6,26,15.4z"/>	<path fill="%[1]s" d="M26,19C26,19,26,18.9,26,19c-1.3,0-2.6,0-3.9,0c0,0-0.1,0-0.1,0c-0.5-0.8-1-1.5-1.5-2.3c0,0,0,0,0-0.1c0,0,0,0,0.1,0		c1.8,0,3.6,0,5.3,0c0,0,0,0,0.1,0C26,17.4,26,18.2,26,19z"/></g></svg>`,
	}

	e.svgs["perl"] = &SvgXML{
		width:  200,
		height: 200,
		color:  hexToRGBA(GithubLangPerl),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><g transform="scale(1.2)"><path fill="%s" d="M13.4,26c-0.5,0-1,0-1.5,0c0.1-0.3,0.1-0.7,0.2-0.8c1-0.4,0.7-1.2,0.7-2c0-1.4,0-2.7,0-4.1c0-0.8,0-1.6,0-2.1	c-1-0.2-2-0.1-2.7-0.5c-2.2-1.4-2.8-3.7-1.8-6.1c0.3-0.7,0.2-1.2-0.7-1.3C7.4,9.1,7.1,9,6.8,8.9c0-0.1,0-0.2,0-0.2	c0.6-0.3,1.1-0.7,1.7-1c0.2-0.1,0.6,0,0.8-0.2c1.6-0.8,1.9-0.7,2.8,0.9c0.5,1,0.4,1.9-0.1,2.9c-0.3,0.5-0.7,1.2,0.1,1.5	c2-2.8,3.7-5.7,7.2-6.8c0.2,0,0.5,0,0.7,0c0.5,0.7,1.2,1.2,1.6,2c1.1,1.8,1.9,3.7,3,5.4c0.8,1.2,0.7,3.6-0.3,4.4c0-1.1,0-2.3,0-3.5	c-0.1,0-0.3,0-0.4,0c-0.9,1.3-0.4,2.8-0.4,4.3c0,2.5-0.1,5-0.1,7.4c-0.2,0-0.5,0-0.7,0c0-0.3-0.1-0.5,0-0.8c0.4-3.1,0-6.2-1.1-9.2	c-0.9,0.8-1.3,1.8-1.4,3c-0.1,0.9-0.4,1.8-0.7,2.7c-0.3,0.8-1.1,1.5-0.1,2.3c0.1,0.1-0.1,0.6-0.1,1.1c-0.4-0.5-0.6-0.8-0.8-1	c0,0.1,0,0.2,0,0.3c-0.7,0.2-1.5,0.3-2.2,0.5c0-0.1-0.1-0.3-0.1-0.4c0.2-0.1,0.4-0.2,0.6-0.4c1-0.6,1.1-1.2,0.3-1.9	c-1.3-1-1.7-2.2-1.1-3.7c0.2-0.4,0.1-0.9,0.2-1.7c-0.7,0.4-1.3,0.6-1.4,0.9c-0.4,1.5-0.6,3.1-0.8,4.6C13.7,23.7,13.6,24.8,13.4,26z	 M18,22.2c1.4-1.8,1.6-3.8,0.6-5.2c-0.6,1.2-1.2,2.4-1.8,3.7C17.2,21.2,17.6,21.7,18,22.2z"/></g></svg>`,
	}

	e.svgs["julia"] = &SvgXML{
		width:  200,
		height: 200,
		color:  hexToRGBA(GithubLangJulia),
		xml: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"	 viewBox="0 0 32 32" enable-background="new 0 0 32 32" xml:space="preserve"><g transform="scale(1.15)">	<circle fill="%[1]s" cx="11.3" cy="20.3" r="3.9"/>	<circle fill="%[1]s" cx="20.7" cy="20.3" r="3.9"/>	<circle fill="%[1]s" cx="16" cy="11.7" r="3.9"/></g></svg>`,
	}

	e.svgs["default"] = &SvgXML{
		width:  200,
		height: 200,
		xml:    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200"><g transform="translate(1,25) scale(0.84)"><path fill="%s" d="M22.34999879828441,17.7129974066314 h147.91225647948028 v20.748584330809138 H22.34999879828441 V17.7129974066314 zM22.34999879828441,65.18324337560382 h126.22055467908892 v20.748584330809138 H22.34999879828441 V65.18324337560382 zM22.34999879828441,113.91097930401922 h155.3000099912078 v20.748584330809138 H22.34999879828441 V113.91097930401922 zM22.34999879828441,161.538411517922 h91.01083581468554 v20.748584330809138 H22.34999879828441 V161.538411517922 z"/></g></svg>`,
	}

}

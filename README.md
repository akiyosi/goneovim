<img src="https://raw.githubusercontent.com/wiki/akiyosi/goneovim/images/GoNeovim.png" width="250" align="top" >

---

![test](https://github.com/akiyosi/goneovim/workflows/test/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/akiyosi/goneovim)](https://goreportcard.com/report/github.com/akiyosi/goneovim)
![GitHub Releases](https://img.shields.io/github/downloads/akiyosi/goneovim/v0.4.6/total)
[![Join the chat at https://gitter.im/goneovim/community](https://badges.gitter.im/goneovim/community.svg)](https://gitter.im/goneovim/community)

*GoNeovim* is a Neovim GUI written in Go, using a [Qt binding for Go](https://github.com/therecipe/qt).
This repository forked from the original [Gonvim](https://github.com/dzhou121/gonvim) for the purpose of maintenance and enhancement.

## Features

All of the features are optional. You can use it like a plain nvim or as a nvim with a rich UI.

- Fast (faster than neovim-qt, many other neovim GUI's)
- Cross-platform
- Modern Text Editor Features
  - [x] Markdown Preview
  - [x] Minimap
  - [x] Smooth scroll
  - [x] Fuzzy Finder
  - [x] Workspace feature which manages multiple nvim
  - [x] External File Explorer
  - [x] Scrollbar
  - [x] Support Ligature
  - [x] Built-in Indent guide
  - [x] Support High DPI scaling
- Features as neovim GUI
  - [x] Externalizable tabline, popupmenu, wildmenu, cmdline, messsages
  - [x] Support gui option: `guifont`, `guifontwide`, `guifont=*`, `linespace`, `guicursor`. You don't need `neovim-gui-shim`
  - [x] Support mouse selections
  - [x] Supports border drawing and shadow drawing of float window
  - [x] Independent font settings per window (currently experimental)
  - [x] Attach feature to a remote nvim
- Basic feature as a Text Editor
  - [x] Support multi byte character
  - [x] Drag and Drop files
  - [x] Support InputMethod Editor (for east asian people)
- Miscellaneous
  - [x] Supports application window transparency
  - [x] Desktop Notification of the messages emitted nvim


## Screenshots

* Workspaces, external file explorer
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/workspaces-fileexplorer.png)
* Fuzzy Finder
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/fuzzyfinder.png)
* Markdown preview
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/markdown-preview.png)
* Minimap
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/minimap.png)
* Indentguide, display ligatures(Fira Code)
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/indentguide.png)
* Transparent app window, Transparent message window
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/transparent.png)
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/transparent-message.png)
* Independent font settings per window
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/font-setting-per-window.png)

## Requirements
* Neovim (v0.4.3 or later)

See [Installing Neovim](https://github.com/neovim/neovim/wiki/Installing-Neovim)

## Getting Started
Pre-built packages for Windows, MacOS, and Linux are found at the [Releases](https://github.com/akiyosi/goneovim/releases) page.



## Usage

See [wiki](https://github.com/akiyosi/goneovim/wiki/Usage)


## ToDo

* Add test

* Improve startup time

* Add tree view for external file explorer

* Add Git integration for external file explorer

* Improve Imput Method Editor(IME) feature

In the current implementation, it is not possible to highlight the keyword being converted in the input method input. At the moment I have no idea about how to improve this

* Support GPU rendering

* Support neovim ui `ext_statusline`



## Development

* [Development](https://github.com/akiyosi/goneovim/wiki/Development)


## Similar projects

* [Gnvim](https://github.com/vhakulinen/gnvim)
* [Veonim](https://github.com/veonim/veonim)



## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))



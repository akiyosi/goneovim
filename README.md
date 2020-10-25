<img src="https://raw.githubusercontent.com/wiki/akiyosi/goneovim/images/GoNeovim.png" width="250" align="top" >

---

![test](https://github.com/akiyosi/goneovim/workflows/test/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/akiyosi/goneovim)](https://goreportcard.com/report/github.com/akiyosi/goneovim)
![GitHub Releases](https://img.shields.io/github/downloads/akiyosi/goneovim/v0.4.8.1/total)
[![Join the chat at https://gitter.im/goneovim/community](https://badges.gitter.im/goneovim/community.svg)](https://gitter.im/goneovim/community)

*GoNeovim* is a Neovim GUI written in Go, using a [Qt binding for Go](https://github.com/therecipe/qt).
This repository forked from the original [Gonvim](https://github.com/dzhou121/gonvim) for the purpose of maintenance and enhancement.

![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/screenshot-202009.png)

## Features

All of the features are optional. You can use it like a plain nvim or as a nvim with a rich UI.

- Fast (faster than neovim-qt)
- Cross-platform
- Modern Text Editor Features
  - Markdown Preview (it is provide as a plugin for goneovim)
  - Minimap
  - Smooth scroll (with MacOS touch pad)
  - Workspace feature which manages multiple nvim
  - External File Explorer
  - Scrollbar
  - Support Ligature
  - Built-in Indent guide
  - Support High DPI scaling
- Features as neovim GUI
  - Externalizable tabline, popupmenu, wildmenu, cmdline, messsages
  - Support gui option: `guifont`, `guifontwide`, `guifont=*`, `linespace`, `guicursor`. You don't need `neovim-gui-shim`
  - Support mouse
  - Support for drawing borders and shadows in a float window
  - Independent font settings per window (currently experimental)
  - Attach feature to a remote nvim
- Basic feature as a Text Editor
  - Support multi byte character
  - Drag and Drop files
  - Support InputMethod Editor (for east asian users)
- Miscellaneous
  - Supports application window transparency
  - Desktop Notification of the messages emitted nvim


## Requirements
* Neovim (v0.4.4 or later)

See [Installing Neovim](https://github.com/neovim/neovim/wiki/Installing-Neovim)


## Getting Started
Pre-built packages for Windows, MacOS, and Linux are found at the [Releases](https://github.com/akiyosi/goneovim/releases) page.

Or you can get the latest binary from Github Actions CI. See [Actions](https://github.com/akiyosi/goneovim/actions) page.


## Usage

See [wiki](https://github.com/akiyosi/goneovim/wiki/Usage)


## Development

See [Development](https://github.com/akiyosi/goneovim/blob/master/Development.md)


## ToDo

* Add test

* Improve Imput Method Editor(IME) feature

* Improve startup time

* Support GPU rendering

* Support neovim ui `ext_statusline`


## Similar projects

* [Neovide](https://github.com/Kethku/neovide)
* [Gnvim](https://github.com/vhakulinen/gnvim)
* [FVim](https://github.com/yatli/fvim)
* [Uivonim](https://github.com/smolck/uivonim)


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

## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))



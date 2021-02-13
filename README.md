Goneovim
---

![test](https://github.com/akiyosi/goneovim/workflows/test/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/akiyosi/goneovim)](https://goreportcard.com/report/github.com/akiyosi/goneovim)
![GitHub Releases](https://img.shields.io/github/downloads/akiyosi/goneovim/v0.4.10/total)
[![Join the chat at https://gitter.im/goneovim/community](https://badges.gitter.im/goneovim/community.svg)](https://gitter.im/goneovim/community)

Goneovim is a Neovim GUI written in Go, using a [Qt binding for Go](https://github.com/therecipe/qt).
This repository forked from the original [Gonvim](https://github.com/dzhou121/gonvim) for the purpose of maintenance and enhancement.

![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/v0.4.10-top.png)

## Features

All of the features are optional. You can use it like a plain nvim or as a nvim with a rich UI.

- Fast (faster than neovim-qt)
- Cross-platform
- Modern Text Editor Features
  - Markdown Preview
  - Minimap
  - Smooth scroll (Support for both touchpad and Vim scroll command reactions.)
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

## Screenshots

### Workspaces
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/v0.4.10-workspaces.gif)

### Smooth Scroll with touchpad
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/v0.4.10-smoothscroll-1.gif)

### Smooth Scroll for neovim scroll commands
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/v0.4.10-smoothscroll-2.gif)

### Minimap, External Float Window
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/v0.4.10-top.png)

### Markdown Preview
![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/v0.4.10-markdown.png)


## Development

See [Development](https://github.com/akiyosi/goneovim/blob/master/Development.md)



## Similar projects

* [Neovide](https://github.com/Kethku/neovide)
* [Gnvim](https://github.com/vhakulinen/gnvim)
* [FVim](https://github.com/yatli/fvim)
* [Uivonim](https://github.com/smolck/uivonim)

## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))



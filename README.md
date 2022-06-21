Goneovim
---

![test](https://github.com/akiyosi/goneovim/workflows/test/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/akiyosi/goneovim)](https://goreportcard.com/report/github.com/akiyosi/goneovim)
![GitHub Releases](https://img.shields.io/github/downloads/akiyosi/goneovim/v0.6.2/total)
[![Join the chat at https://gitter.im/goneovim/community](https://badges.gitter.im/goneovim/community.svg)](https://gitter.im/goneovim/community)

Goneovim is a Neovim GUI written in Go, using a [Qt binding for Go](https://github.com/therecipe/qt).
This repository forked from the original [Gonvim](https://github.com/dzhou121/gonvim) for the purpose of maintenance and enhancement.

![](https://raw.githubusercontent.com/wiki/akiyosi/goneovim/screenshots/goneovim.png)


## Why Neovim GUI

Traditionally, Neovim (and even Vim) has been optimized for working with Terminal, and some Terminal-based workflows cannot be achieved with a GUI. 
Therefore, for some people, a GUI would be an unnecessary additional stuff. 
On the other hand, in my opinion, there are some attractive features of GUI as follows.

* **More meta keys can be used**
  * Since Neovim performs all of its operations with the keyboard, having more meta keys available is a simple advantage.
* **Terminal is also available in Neovim GUI**
  * Neovim has an embedded terminal emulator that can be run in `:terminal`, so you can run basic terminal workflows using `:terminal` with `bash` or `zsh` in Neovim GUI. It is also possible to use remote control tools such as [nvr](https://github.com/mhinz/neovim-remote) to avoid nvim in nvim in Neovim GUI.
* **Experience the rich drawing expressions of GUI**
  * For example, it is possible to scroll based on pixels, to set different font families and point sizes for each window.

If you are interested in these GUI attractions, try goneovim.


## Features

All of the features are optional. You can use it like a plain nvim or as a nvim with a rich UI.

- Fast (faster than neovim-qt)
- Cross-platform
- Modern Text Editor Features
  - Minimap
  - Smooth pixel scroll (Support for both touchpad and Vim scroll command reactions.)
  - Animated Cursor
  - Workspace feature which manages multiple nvim
  - Ligatures
  - Built-in Indent guide
  - Scrollbar
  - High DPI scaling
- Features as neovim GUI
  - Externalizable tabline, popupmenu, wildmenu, cmdline, messsages
  - Support gui option: `guifont`, `guifontwide`, `guifont=*`, `linespace`, `guicursor`. You don't need `neovim-gui-shim`
  - Support mouse
  - Support for drawing borders and shadows in a float window
  - Independent font settings per window (currently experimental)
  - Attach/Connect feature to a remote nvim
- Basic feature as a Text Editor
  - Multi byte character
  - Drag and Drop files
  - InputMethod Editor (for east asian users)
- Miscellaneous
  - Supports application window transparency
  - Desktop Notification of the messages emitted nvim


## Requirements
* Neovim [v0.6.1](https://github.com/neovim/neovim/releases/tag/v0.6.1) or [Nightly](https://github.com/neovim/neovim/releases/tag/nightly)

See [Installing Neovim](https://github.com/neovim/neovim/wiki/Installing-Neovim)


## Getting Started
Pre-built packages for Windows, MacOS, and Linux are found at the [Releases](https://github.com/akiyosi/goneovim/releases) page.

Or you can get the latest binary from Github Actions CI. See [Actions](https://github.com/akiyosi/goneovim/actions) page.

Windows users can install using scoop:

```
scoop bucket add extras
scoop install goneovim
```

or

```
scoop bucket add versions
scoop install goneovim-nightly
```

Mac users can install using homebrew:

`brew install --cask goneovim`

It is recommended that archlinux users use `Goneovim-v{version}-linux_archlinux.tar.bz2`.

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


## Development

See [Development](https://github.com/akiyosi/goneovim/blob/master/Development.md)



## Similar projects

* [Neovide](https://github.com/Kethku/neovide)
* [Gnvim](https://github.com/vhakulinen/gnvim)
* [FVim](https://github.com/yatli/fvim)
* [Uivonim](https://github.com/smolck/uivonim)

## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))



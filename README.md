<img src="https://raw.githubusercontent.com/wiki/akiyosi/goneovim/images/GoNeovim.png" width="250" align="top" >

---

[![Join the chat at https://gitter.im/goneovim/community](https://badges.gitter.im/goneovim/community.svg)](https://gitter.im/goneovim/community)
[![Go Report Card](https://goreportcard.com/badge/github.com/akiyosi/goneovim)](https://goreportcard.com/report/github.com/akiyosi/goneovim)

*GoNeovim* is a Neovim GUI written in Go, using a [Qt binding for Go](https://github.com/therecipe/qt).
This repository forked from the original [Gonvim](https://github.com/dzhou121/gonvim) for the purpose of maintenance and enhancement.

## Features

* Workspace feature which manages multiple nvim
* Fuzzy Finder
* Markdown Preview
* External File Explorer
* Minimap
* Transparent window
* Indent guide
* Independent font settings per window (alpha stage)
* Support display ligatures
* Support IME
* Desktop Notification of the messages
* Remote attachment

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
* Neovim (v0.4.2 or later)

See [Installing Neovim](https://github.com/neovim/neovim/wiki/Installing-Neovim)

## Getting Started
Pre-built packages for Windows, MacOS, and Linux are found at the [Releases](https://github.com/akiyosi/goneovim/releases) page.



## Usage

See [wiki](https://github.com/akiyosi/goneovim/wiki/Usage)


## ToDo
* Add test

* Real-time synchronization between editing buffer and minimap

* Improved startup speed (especially on the Windows platform)

* Improve IME feature

In the current implementation, it is not possible to highlight the keyword being converted in the input method input. At the moment I have no idea about how to improve this

* Keyword filtering feature in external file explorer

* Support GPU rendering

* Support neovim ui `ext_statusline`

* Smooth scrolling



## Development

* [Development](https://github.com/akiyosi/goneovim/wiki/Development)


## Similar projects

* [Gnvim](https://github.com/vhakulinen/gnvim)
* [Veonim](https://github.com/veonim/veonim)



## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))



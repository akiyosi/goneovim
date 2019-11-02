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
* Desktop Notification of the messages
* Remote attachment

## Screenshots

* Workspaces, external file explorer
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/workspaces-fileexplorer.png)
* Fuzzy Finder
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/fuzzyfinder.png)
* Markdown preview
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/markdown-preview.png)
* Minimap
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/minimap.png)
* Indentguid, display lifatures(Fira Code)
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/indentguide.png)
* Transparent window
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/transparent.png)
* Independent font settings per window
![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/screenshots/font-setting-per-window.png)

## Requirements
* Neovim (v0.4.2 or later)

See [Installing Neovim](https://github.com/neovim/neovim/wiki/Installing-Neovim)

## Getting Started
Pre-built packages for Windows, MacOS, and Linux are found at the [Releases](https://github.com/akiyosi/goneovim/releases) page.



## Configurations

* neovim configuration example for goneovim

The sample configuration is [goneovim-init.vim](https://github.com/akiyosi/goneovim-init.vim)

* goneovim configuration

For details of `~/.goneovim/setting.toml`, See [wiki](https://github.com/akiyosi/goneovim/wiki/Configurations)


## ToDo
* Improved startup speed (especially on the Windows platform)
* Improve IME feature
In the current implementation, it is not possible to highlight the keyword being converted in the input method input. At the moment I have no idea about how to improve this

* Support GUI rendering

* Support neovim ui `ext_statusline`

* Keyword filtering feature in external file explorer


## Development

* [Development](https://github.com/akiyosi/goneovim/wiki/Development)


## Similar projects

* [Gnvim](https://github.com/vhakulinen/gnvim)
* [Veonim](https://github.com/veonim/veonim)



## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))



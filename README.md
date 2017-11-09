# Gonvim

[![Join the chat at https://gitter.im/gonvim/gonvim](https://badges.gitter.im/gonvim/gonvim.svg)](https://gitter.im/gonvim/gonvim?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

Neovim GUI written in Golang, using a [Golang qt backend](https://github.com/therecipe/qt)

`Downloads:` Pre-built packages for Windows, macOS, and Linux are found at the [Releases](https://github.com/dzhou121/gonvim/releases/) page.

`Requirements:` [Neovim](https://github.com/neovim/neovim) (v0.2)


## Features

### Tabline, Statusline, Lint Message, Command line and Message

![](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/main.png)

Disable Statusline by using
```vim
let g:gonvim_draw_statusline = 0
```

Disable Tabline by using
```vim
let g:gonvim_draw_tabline = 0
```

Disable the lint message popup using
```vim
let g:gonvim_draw_lint = 0
```

Start in fullscreen mode
```vim
let g:gonvim_start_fullscreen = 1
```

### Fuzzy finder in GUI

![](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/fuzzy.png)

To set up fuzzy finder :

* Install [dzhou121/gonvim-fuzzy](https://github.com/dzhou121/gonvim-fuzzy).

Add any these in your `init.vim` file based on your plugin manager.
```vim
Plug 'dzhou121/gonvim-fuzzy'       " Vim plug
Neobundle 'dzhou121/gonvim-fuzzy'  " Neobundle
Plugin 'dzhou121/gonvim-fuzzy'     " Vundle
```

Now you have the following commands available for fuzzy gui search:
```
GonvimFuzzyFiles   - For Files
GonvimFuzzyBLines  - For Lines in the Current File
GonvimFuzzyAg      - For runing FZF_AG ( searches current directory )
GonvimFuzzyBuffers - For searching opened Buffers
```


## Autocomplete Menu and Function signature

![Readme](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/popup.png)
![Readme](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/signature.png)



## Miscellaneous Configuration

#### Disable drawing of split by gonvim
```vim
let g:gonvim_draw_split = 0
```

#### Font and Line spacing

To change font and line spacing

* Install [equalsraf/neovim-gui-shim](https://github.com/equalsraf/neovim-gui-shim)

Add any these in your `init.vim` file based on your plugin manager.
```vim
Plug 'equalsraf/neovim-gui-shim'       " Vim plug
Neobundle 'equalsraf/neovim-gui-shim'  " Neobundle
Plugin 'equalsraf/neovim-gui-shim'     " Vundle
```
* Now use the following in your `ginit.vim`
```vim
GuiFont Monaco:h13
GuiLinespace 8
```

## Manual Installation

Install Qt (https://www1.qt.io/download-open-source/)

Install Qt go bindings
```
$> go get -u -v github.com/therecipe/qt/cmd/...
$> qtsetup
```

Clone this repository and doing some code generation
```
$> go get -u github.com/dzhou121/gonvim/editor/
$> cd GOPATH/github.com/dzhou121/gonvim/editor/
$> qtmoc
```

Now you can build and run
```
$> go build && ./gonvim
```

# Gonvim

Neovim GUI written in Golang, using a [Golang qt backend](https://github.com/therecipe/qt)

`Downloads:` Pre-built packages for Windows, macOS, and Linux are found at the [Releases](https://github.com/dzhou121/gonvim/releases/) page.

`Requirements:` [Neovim](https://github.com/neovim/neovim) (v0.2)


## Features

### Statusline and Tabline

![](https://i.imgur.com/BfQi6MV.png)

> By default both Statusline and Tabline are enabled

Disable Statusline by using
```vim
let g:gonvim_draw_statusline = 0
```

### Fuzzy finder in GUI

![](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/fuzzyfinder.gif)

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


## Function signature and Lint Message

![Readme](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/signature.gif)
![Readme](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/lint.gif)


Disable the lint message popup using
```vim
let g:gonvim_draw_lint = 0
```


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

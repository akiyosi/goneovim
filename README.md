Neovim GUI written in Golang, using a Golang qt backend (https://github.com/therecipe/qt)

# Requirements
- Neovim (0.2)

# Downloads
Pre-built packages for Windows, macOS, and Linux are found at the [Releases](https://github.com/dzhou121/gonvim/releases/) page.

# Configuration
Disable the drawing of split by gonvim
```let g:gonvim_draw_split = 0```

Disable the drawing of statusline by gonvim
```let g:gonvim_draw_statusline = 0```

Disable the lint popup message
```let g:gonvim_draw_lint = 0```

# Features

- Tabline

![](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/tab.gif)

- A fuzzy finder in the GUI

![](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/fuzzyfinder.gif)

- Function signature popup

![Readme](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/signature.gif)

- Lint message popup

![Readme](https://raw.githubusercontent.com/wiki/dzhou121/gonvim/screenshots/lint.gif)

# Font
To change the font, install https://github.com/equalsraf/neovim-gui-shim 

change the font and size call
```:GuiFont Monaco:h13```
change the line space
```:GuiLinespace 8```

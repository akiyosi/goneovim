Neovim GUI written in Golang, using a forked ui libray from https://github.com/andlabs/ui

# Requirements
- Neovim (0.2)

# Install
Working on macOS and Linux
```
go get -u github.com/dzhou121/gonvim/cmd/gonvim
```

After succesfully installing to GOBIN, you can run it
```
gonvim
```

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

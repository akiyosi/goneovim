
# Gonvim

[![Join the chat at https://gitter.im/gonvim/gonvim](https://badges.gitter.im/gonvim/gonvim.svg)](https://gitter.im/gonvim/gonvim?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Go Report Card](https://goreportcard.com/badge/github.com/akiyosi/gonvim)](https://goreportcard.com/report/github.com/akiyosi/gonvim)

Gonvim is a Neovim GUI written in Golang, using a [Golang qt backend](https://github.com/therecipe/qt).
 
This repository forked from the original [Gonvim](https://github.com/dzhou121/gonvim) for the purpose of maintenance and enhancement.

![](https://raw.githubusercontent.com/wiki/akiyosi/gonvim/images/0.3.0.png)

## Table of contents

- [Getting started](#getting-started)
- [Configurations](#configurations)
- [Features](#features)
- [Development](#development)
- [Contributing](#contributing)
- [Similar Projects](#similar-projects)
- [License](#license)
- [Credits](#credits)

<br>

## Getting Started
Pre-built packages for Windows, MacOS, and Linux are found at the [Releases](https://github.com/akiyosi/gonvim/releases) page.


### Prerequisites
MacOS or Linux users need to install neovim. See [Installing Neovim](https://github.com/neovim/neovim/wiki/Installing-Neovim)

### MacOS
gonvim.app looks for the nvim process from the following.

```
/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/local/bin:/opt/local/sbin
```

Deployment example:

```
cd /path/to
curl -LO https://github.com/neovim/neovim/releases/download/nightly/nvim-macos.tar.gz
tar xf nvim-macos.tar.gz
ln -s /path/to/bin/nvim /usr/local/bin/nvim
```

### Linux
Deploy Neovim under the `$PATH`.

### Windows
Add Gonvim `bin` path to `%PATH%` environment variable.



## Configurations

| application | config file |
|:------------|:------------|
| Gonvim (GUI front end of neovim) | `~/.gonvim/setting.toml` |
| Neovim      | `$XDG_CONFIG_HOME/nvim/init.vim` |

where the default value of `$XDG_CONFIG_HOME` is the below.

| OS      |  `$XDG_CONFIG_HOME`  |
|:--------|:---------------------|
| Unix    |  `~/.config`         |
| Windows |  `~/AppData/Local`   |

### neovim configuration for gonvim

The sample configuration is below.

[gonvim-init.vim](https://github.com/akiyosi/gonvim-init.vim)

### gonvim configuration

For details of `~/.gonvim/setting.toml`, See [wiki](https://github.com/akiyosi/gonvim/wiki/Configurations)



## Features

* [Fuzzy Finder](https://github.com/akiyosi/gonvim/wiki/Features#fuzzy-finder-in-gui)
* [Markdown Preview](https://github.com/akiyosi/gonvim/wiki/Features#markdown-preview)
* [MiniMap](https://github.com/akiyosi/gonvim/wiki/Features#minimap)
* [Dein.vim GUI](https://github.com/akiyosi/gonvim/wiki/Features#deinvim-gui)



## Development

* [Development](https://github.com/akiyosi/gonvim/wiki/Development)



## Contributing
Gonvim welcomes contribution. If you want to add your changes to the Gonvim, Please following steps

1. **Fork this repository**
1. **Create a branch** for your contribution
1. **Add contributions** to the branch
1. **Create a pull request** against the `master` branch of the Gonvim repository.


## Similar projects

* [Oni](https://github.com/onivim/oni)
* [Veonim](https://github.com/veonim/veonim)



## License
Gonvim is released under MIT license


## Credits

* Gonvim was created by dzhou121 ([https://github.com/dzhou121/gonvim](https://github.com/dzhou121/gonvim))
* Gonvim application icon was made using [gopher-vector](https://github.com/golang-samples/gopher-vector) made by Takuya Ueda ([https://twitter.com/tenntenn](https://twitter.com/tenntenn)). The original gopher was designed by Renee French [http://reneefrench.blogspot.com/](http://reneefrench.blogspot.com/).



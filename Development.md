Development of goneovim
=======================
Note that the information on this page is not updated frequently. If you want reliable build instructions, the CI scripts in Github Actions could be helpful.
See https://github.com/akiyosi/goneovim/blob/master/.github/workflows/ci.yaml

## For Linux, MacOS, Windows(MSYS2)
  * Install Qt
    - FreeBSD
      - Install the Qt5 dev packages and export **QT_PKG_CONFIG=true**

      ```
      pkg install devel/qt5
      ```
    
    - Linux
      - Install the Qt5 dev packages through your systems package manager and export **QT_PKG_CONFIG=true**. (You will need to install the `html/doc` packages containing the `*.index` files as well.)
        - Debian/Ubuntu (apt-get): `sudo apt-get --no-install-recommends install libqt*5-dev qt*5-dev qml-module-qtquick-* qt*5-doc-html`
        - Fedora/RHEL/CentOS (yum): `sudo yum install qt5-* qt5-*-doc`
        - openSUSE (zypper): `sudo zypper install --no-recommends libqt5-qt*-devel`
        - Arch Linux (pacman): `sudo pacman -S --needed qt5`
    
    - MacOS
      - Install the Qt5 packages through HomeBrew and export **QT_HOMEBREW=true**

        ```
        brew install qt5
        ```

    - Windows(MSYS2)
      - Install MSYS2
      - Install Qt5 on MSYS2 and export the following environment variables

        ```
        pacman --noconfirm -S sed git unzip zip mingw-w64-x86_64-qt-creator mingw-w64-x86_64-qt5-static
        ```

        | environment variable name | value |
        | ------------------ | --------------- |
        | QT_MSYS2           | true            |
        | QT_MSYS2_DIR       | {Path to MSYS2} |
        | QT_MSYS2_STATIC    | true            |
        | QT_MSYS2_ARCH      | amd64           |

        Note that the path specified in QT_MSYS2_DIR must be Windows style, not MSYS2 style.
  

  * Export Environment variables **QT_API=5.13.0**

  * Install Go

  * Checkout this repogitory and cd
    
    ```
    git clone https://github.com/akiyosi/goneovim.git
    cd goneovim
    ```


  * Setup Qt binding
    NOTE: If you are using FreeBSD, you need to use gmake instead of make.

    ```
    make qt_bindings
    ```


  * Get Dependent Libraries
    
    ```
    make deps
    ```

  * Test

    ```
    make test
    ```

  * Build

    ```
    make app
    ```



## For Windows (MSVC)

  In this section, we will assume you are working on your home directory; `%USERPROFILE%`

  * Install MSVC 2017 Visual C++ Buildtools

  * Install Qt 5.14.2
  
    - Qt installation on Windows
      - [https://download.qt.io/official_releases/online_installers/qt-unified-windows-x86-online.exe](https://download.qt.io/official_releases/online_installers/qt-unified-windows-x86-online.exe)


  * Export Environment variables
  
    We should export the following environment variables:

    | environment variable name | value |
    | -----------------| ----- |
    | QT_API           | The version of the Qt API to generate. This project now uses `5.12.6` |
    | QT_VERSION       | The version of Qt you installed |
    | QT_DIR           | The directory path where qt is installed |
    | GOVSVARSPATH     | \Path\To\BuildTools\VC\Auxiliary\Build\vcvars64.bat |
    | QT_MSVC          | true |


  * Install Go and Qt binding
    Currently, We need to use Go's Experimental feature to enable the markdown preview feature in Goneovim.
    Therefore, we are generating a Windows build using a patched version of Go to Go 1.11.9.


    * Install Go 1.11.9

      ```
      curl -sL --retry 10 --retry-delay 60 -O https://dl.google.com/go/go1.11.9.windows-amd64.zip
      expand-archive -path go1.11.9.windows-amd64.zip -destinationpath .
      Move-Item -Path go -Destination go-root
      ```

    * Get Go binding for Qt
  
      ```
      GO111MODULE=off %USERPROFILE%\go-root\bin\go.exe get -v -tags=no_env github.com/therecipe/qt/cmd/...
      ```

    * Set `PATH`
      ```
      $env:PATH = "$env:USERPROFILE\go-root\bin;$env:PATH"
      $env:PATH = "$env:USERPROFILE\BuildTools\VC\Tools\MSVC\14.16.27023\bin\Hostx64\x64;$env:PATH"
      ```

    * Patch for Go 1.11.9

      ```
      git clone https://github.com/golang/go.git go-msvc
      cd go-msvc
      git fetch "https://go.googlesource.com/go" refs/changes/46/133946/5
      Git checkout FETCH_HEAD
      echo "devel +6741b7009d" > VERSION
      curl -sL --retry 10 --retry-delay 60 https://github.com/golang/go/commit/e4535772ca3f11084ee5fa4d4bd3a542e143b80f.patch | patch -p1 -R
      curl -sL --retry 10 --retry-delay 60 https://github.com/golang/go/commit/f10815898c0732e2e6cdb697d6f95f33f8650b4e.patch | patch -p1 -R
      cd ..
      Move-Item -Path go-root -Destination go-boot
      Move-Item -Path go-msvc -Destination go-root
      cd ${{ github.workspace }}\go-root\src
      .\make.bat
      ```

  * Setup Go binding

    ```
    %GOPATH%\bin\qtsetup.exe -test=false
    ```

  * Clone this repository

    ```
    GO111MODULE=off go get -d github.com/akiyosi/goneovim/...
    ```

  * Generate moc files

    ```
    cd %GOPATH%/src/github.com/akiyosi/goneovim
    %GOPATH%/bin/qtmoc.exe
    ```

  * Test

    ```
    cd %GOPATH%/src/github.com/akiyosi/goneovim
    %GOPATH%/bin/qtdeploy.exe build desktop
    ```

  * Build

    ```
    cd %GOPATH%/src/github.com/akiyosi/goneovim
    %GOPATH%/bin/qtdeploy.exe build desktop
    ```



# Update go.mod, go.sum

```
rm go.mod go.sum
rm -fr vendor/*
go mod init github.com/akiyosi/goneovim
go mod tidy
```

Next, if necessary, explicitly update the module version.

```
go get -u github.com/neovim/go-client@HEAD
```

Next, run the following

```
go get github.com/therecipe/qt/internal/cmd@v0.0.0-20200904063919-c0c124a5770d ; go get github.com/therecipe/qt/internal/binding/files/docs/5.12.0 ; go get github.com/therecipe/qt/internal/binding/files/docs/5.13.0
```

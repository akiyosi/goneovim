Development of goneovim
=======================

## For Linux or MacOS
  * Install Qt
  
    - Qt installation on Linux
      - [https://download.qt.io/official_releases/online_installers/qt-unified-linux-x64-online.run](https://download.qt.io/official_releases/online_installers/qt-unified-linux-x64-online.run)
  
    - Qt installation on MacOS
      - [https://download.qt.io/official_releases/online_installers/qt-unified-mac-x64-online.dmg](https://download.qt.io/official_releases/online_installers/qt-unified-mac-x64-online.dmg)
  

  * Export Environment variables

    We should export the following environment variables:


    | environment variable name | value |
    | ------------- | ----- |
    | QT_API        | The version of the Qt API to generate. This project now uses `5.13.0` |
    | QT_VERSION    | The version of Qt you installed |
    | QT_DIR        | The directory path where qt is installed |

    e.g.

    ```
    export QT_DIR=/path/to/Qt
    export QT_VERSION=5.14.2
    export QT_API=5.13.0
    ```

  * Install Go

  * Get Go binding for Qt

    ```
    export GO111MODULE=off; go get -v github.com/therecipe/qt/cmd/...
    ```

  * Setup Go binding

    ```
    $(go env GOPATH)/bin/qtsetup -test=false
    ```

  * Clone this repository

    ```
    go get -d github.com/akiyosi/goneovim/...
    ```

  * Generate moc files

    ```
    cd $GOPATH/src/github.com/akiyosi/goneovim
    $(go env GOPATH)/bin/qtmoc
    ```

  * Test
    
    ```
    go test github.com/akiyosi/goneovim/editor

    ```

  * Build

    ```
    cd $GOPATH/src/github.com/akiyosi/goneovim/cmd/goneovim
    $(go env GOPATH)/bin/qtdeploy build desktop
    ```


## For Windows (MSYS2)

See also: https://github.com/therecipe/qt/wiki/Installation-on-Windows#msys2-version-without-android-support

  * Install MSYS2

  * Install Qt5 on MSYS2

    ```
    pacman --noconfirm -S sed git mingw-w64-x86_64-toolchain mingw-w64-x86_64-qt5
    ```

  * Export Environment variables

    We should export the following environment variables:


    | environment variable name | value |
    | ------------- | ----- |
    | QT_MSYS2           | true            |
    | QT_MSYS2_DIR       | {Path to MSYS2} |
    | QT_MSYS2_ARCH      | amd64           |
    | QT_DEBUG           | false           |
    | GO111MODULE        | off             |
    | CGO_CFLAGS_ALLOW   | ".*"            |
    | CGO_CXXFLAGS_ALLOW | ".*"            |
    | CGO_LDFLAGS_ALLOW  | ".*"            |


  * Install Go

  * Get Go binding for Qt in cmd.exe

    ```
    go.exe get -v github.com/therecipe/qt/cmd/...
    ```

  * Setup Go binding in MSYS2 shell

    ```
    $(go env GOPATH)/bin/qtsetup -test=false
    ```

  * Clone this repository in cmd.exe

    ```
    go.exe get -d github.com/akiyosi/goneovim/...
    ```

  * Generate moc files in MSYS2 shell

    ```
    $(go env GOPATH)/bin/qtmoc
    ```

  * Test in cmd.exe

    ```
    go.exe test github.com/akiyosi/goneovim/editor
    ```

  * Build in MSYS2 shell

    ```
    cd $GOPATH/src/github.com/akiyosi/goneovim/cmd/goneovim
    $(go env GOPATH)/bin/qtdeploy build desktop
    ```


## For Windows (MSVC)

  In this section, we will assume you are working on your home directory; `%USERPROFILE%`

  * Install MSVC 2017 Visual C++ Buildtools

  * Install Qt; Note that we recommend to install Qt 5.12.X (where X is 0-6)
  
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
    | CGO_ENABLED      | 1 |
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

  * Build

    ```
    cd %GOPATH%/src/github.com/akiyosi/goneovim
    %GOPATH%/bin/qtdeploy.exe build desktop
    ```

    If you have Qt5.13 installed, you can run the following command

    ```
    %GOPATH%/bin/qtdeploy build desktop
    ```


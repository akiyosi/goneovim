#!/usr/bin/env bash

DIR="$HOME/go/src/github.com/akiyosi/gonvim/cmd/gonvim"

# build darwin
cd $DIR
echo "qtdeploy darwin"
qtdeploy build desktop

rm -f $DIR/deploy/darwin/gonvim.app/Contents/Info.plist
rm -f $DIR/deploy/darwin/gonvim.app/Info.plist
rm -f $DIR/deploy/darwin/gonvim.app/gonvim.icns
cp $DIR/darwin/Info.plist $DIR/deploy/darwin/gonvim.app/Contents/
cp $DIR/darwin/gonvim.icns $DIR/deploy/darwin/gonvim.app/Contents/Resources/
cd $DIR/deploy/darwin/
rm -f ~/Downloads/gonvim-macos.zip
zip -r ~/Downloads/gonvim-macos.zip gonvim.app


# build windows
cd $DIR
echo "qtdeploy windows"
qtdeploy -docker build windows
rm -rf /tmp/gonvim
cp -r ~/Downloads/nvim-win64 /tmp/gonvim
rsync -r $DIR/deploy/windows/ /tmp/gonvim/bin/
rm -f /tmp/gonvim/bin/nvim-qt.exe
cd /tmp
rm -f ~/Downloads/gonvim-win64.zip
zip -r ~/Downloads/gonvim-win64.zip gonvim


# build linux
cd $DIR
echo "qtdeploy linux"
qtdeploy -docker build linux
cd $DIR/deploy/
rm -rf gonvim
cp -r linux gonvim
rm -f ~/Downloads/gonvim-linux.zip
zip -r ~/Downloads/gonvim-linux.zip gonvim

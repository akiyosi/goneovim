package goneovim

//go:generate sh -c "printf %s $(git describe --tags) > editor/version.txt"
//go:generate go run generate_objcbridge.go

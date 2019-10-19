package filer

import (
	"fmt"

	"github.com/akiyosi/gonvim/util"
	"github.com/neovim/go-client/nvim"
)

type Filer struct {
	nvim               *nvim.Nvim
}

// RegisterPlugin registers this remote plugin
func RegisterPlugin(nvim *nvim.Nvim, isRemoteAttachment bool) {
	nvim.Subscribe("GonvimFiler")
	shim := &Filer{
		nvim:               nvim,
		max:                20,
		isRemoteAttachment: isRemoteAttachment,
	}
	nvim.RegisterHandler("GonvimFuzzy", func(args ...interface{}) {
		go shim.handle(args...)
	})
}

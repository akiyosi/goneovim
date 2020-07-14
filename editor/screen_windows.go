package editor

func createExternalWin() *ExternalWin {
	extwin := NewExternalWin(nil, 0)

	extwin.SetWindowFlag(core.Qt__WindowMaximizeButtonHint,    false)
	extwin.SetWindowFlag(core.Qt__WindowMinimizeButtonHint,    false)
	extwin.SetWindowFlag(core.Qt__WindowCloseButtonHint,       false)
	extwin.SetWindowFlag(core.Qt__WindowContextHelpButtonHint, false)

	return extwin
}

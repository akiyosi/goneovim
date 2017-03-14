package gonvim

func isNormalWidth(char string) bool {
	if char[0] > 127 {
		return false
	}
	return true
}

func reflectToInt(iface interface{}) int {
	o, ok := iface.(int64)
	if ok {
		return int(o)
	}
	return int(iface.(uint64))
}

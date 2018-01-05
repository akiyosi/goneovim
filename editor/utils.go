package editor

func reflectToInt(iface interface{}) int {
	o, ok := iface.(int64)
	if ok {
		return int(o)
	}
	return int(iface.(uint64))
}

func isZero(d interface{}) bool {
	if d == nil {
		return false
	}
	switch a := d.(type) {
	case int64:
		if a == 0 {
			return true
		}
	case uint64:
		if a == 0 {
			return true
		}
	}
	return false
}

func isTrue(d interface{}) bool {
	if d == nil {
		return false
	}
	switch a := d.(type) {
	case int64:
		if a == 1 {
			return true
		}
	case uint64:
		if a == 1 {
			return true
		}
	}
	return false
}

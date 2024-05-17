package utils

import "strconv"

func Bool2String(b bool) string {
	if b {
		return "1"
	} else {
		return "0"
	}
}

func Int2String(n int) string {
	s := strconv.Itoa(n)
	return s
}

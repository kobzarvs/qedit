//go:build !darwin

package keyboard

func CurrentLayout() string {
	return ""
}

func CurrentLayoutRaw() string {
	return ""
}

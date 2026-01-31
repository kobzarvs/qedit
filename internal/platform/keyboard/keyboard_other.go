//go:build !darwin || !cgo

package keyboard

func CurrentLayout() string {
	return ""
}

func CurrentLayoutRaw() string {
	return ""
}

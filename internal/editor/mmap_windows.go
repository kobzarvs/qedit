//go:build windows

package editor

import (
	"os"
	"syscall"
	"unsafe"
)

// mmapFile maps the entire file into memory as a read-only byte slice.
func mmapFile(path string, size int64) ([]byte, error) {
	if size <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hi := uint32(size >> 32)
	lo := uint32(size & 0xFFFFFFFF)

	h, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY, hi, lo, nil)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)

	addr, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		return nil, err
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), int(size))
	return data, nil
}

// mmapAdviseRandom is a no-op on Windows (no madvise equivalent).
func mmapAdviseRandom(data []byte) {}

// munmapFile releases the mmap'd memory region.
func munmapFile(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return syscall.UnmapViewOfFile(uintptr(unsafe.Pointer(&data[0])))
}

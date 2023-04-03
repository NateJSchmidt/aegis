//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	for {
		userDll := syscall.NewLazyDLL("user32.dll")
		getWindowRectProc := userDll.NewProc("GetCursorPos")
		type POINT struct {
			X, Y int32
		}
		var pt POINT
		_, _, eno := syscall.SyscallN(getWindowRectProc.Addr(), uintptr(unsafe.Pointer(&pt)))
		if eno != 0 {
			fmt.Println(eno)
		}
		fmt.Printf("[cursor.Pos] X:%d Y:%d\n", pt.X, pt.Y)
		time.Sleep(5 * time.Second)
	}
}

package main

import "syscall"

// consoleUTF8 makes the classic Windows console show and read accented French text.
func consoleUTF8() {
	k := syscall.NewLazyDLL("kernel32.dll")
	k.NewProc("SetConsoleOutputCP").Call(65001)
	k.NewProc("SetConsoleCP").Call(65001)
}

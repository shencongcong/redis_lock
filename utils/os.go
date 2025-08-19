package utils

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// 获取当前进程的ID
func GetCurrentProcessID() string {
	return strconv.Itoa(os.Getpid())
}

// 获取当前协程ID
func GetCurrentGoroutineID() string {
	buf := make([]byte, 128)
	buf = buf[:runtime.Stack(buf, false)]
	stackInfo := string(buf)
	return strings.TrimSpace(strings.Split(strings.Split(stackInfo, "[running]")[0], "goroutine")[1])
}

func GetProcessAndGoroutineIDStr() string {
	return GetCurrentProcessID() + "_" + GetCurrentGoroutineID()
}

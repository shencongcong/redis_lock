package utils

import (
	"testing"
)

func TestGetCurrentGoroutineID(t *testing.T) {
	// 多次获取，确保每次都能拿到非空且为数字
	for i := 0; i < 5; i++ {
		gid := GetCurrentGoroutineID()
		if gid == "" {
			t.Errorf("第%d次: GetCurrentGoroutineID 返回了空字符串", i+1)
		}
		for _, c := range gid {
			if c < '0' || c > '9' {
				t.Errorf("第%d次: GetCurrentGoroutineID 返回了非数字字符: %s", i+1, gid)
				break
			}
		}
		t.Logf("第%d次: GetCurrentGoroutineID: %s", i+1, gid)
	}

	// 并发测试，确保每个 goroutine 拿到的 id 不同
	const goroutineNum = 10
	ids := make(chan string, goroutineNum)
	done := make(chan struct{})
	for i := 0; i < goroutineNum; i++ {
		go func() {
			ids <- GetCurrentGoroutineID()
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutineNum; i++ {
		<-done
	}
	close(done)
	close(ids)

	idSet := make(map[string]struct{})
	for id := range ids {
		if _, exists := idSet[id]; exists {
			t.Errorf("并发测试: 检测到重复的 goroutine id: %s", id)
		}
		idSet[id] = struct{}{}
	}
	if len(idSet) != goroutineNum {
		t.Errorf("并发测试: goroutine id 数量不匹配，期望 %d，实际 %d", goroutineNum, len(idSet))
	}
}

package redis_lock

import (
	"context"
	"testing"
	"sync"
	"time"
)

func Test_blockingLock(t *testing.T) {
	addr :=  "127.0.0.1:6379"
	password := ""
	
	client := NewClient("tcp",addr, password)

	lock1 := NewRedisLock("test_key", client, WithBlockMode(),WithExpireSeconds(1))
	lock2 := NewRedisLock("test_key", client, WithBlockMode(),WithBlockWaitingSeconds(2))

	ctx := context.Background()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		if err :=  lock1.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()
		if err :=  lock2.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Wait()

	t.Log("success")
}


func Test_nonBlockingLock(t *testing.T) {
	addr := "127.0.0.1:6379"
	password := ""

	client := NewClient("tcp", addr, password)

	lock1 := NewRedisLock("test_nonblock_key", client, WithExpireSeconds(2))
	lock2 := NewRedisLock("test_nonblock_key", client, WithExpireSeconds(2))

	ctx := context.Background()

	// lock1 先加锁
	if err := lock1.Lock(ctx); err != nil {
		t.Fatalf("lock1 加锁失败: %v", err)
	}

	// lock2 尝试加锁（非阻塞），应该失败
	err := lock2.Lock(ctx)
	if err == nil {
		t.Error("lock2 非阻塞加锁应该失败，但实际加锁成功")
	} else {
		t.Logf("lock2 非阻塞加锁失败，符合预期: %v", err)
	}

	// 解锁 lock1
	if lock1.stopDog != nil {
		lock1.stopDog()
	}
	_ = client.Del(ctx, "REDIS_LOCK_PREFIX_test_nonblock_key")
}

func Test_blockingLock2(t *testing.T) {
	addr := "127.0.0.1:6379"
	password := ""

	client := NewClient("tcp", addr, password)

	lockKey := "test_blocking_key"
	lock1 := NewRedisLock(lockKey, client, WithBlockMode(), WithBlockWaitingSeconds(3), WithExpireSeconds(2))
	lock2 := NewRedisLock(lockKey, client, WithBlockMode(), WithBlockWaitingSeconds(3), WithExpireSeconds(2))

	ctx := context.Background()

	// lock1 先加锁
	if err := lock1.Lock(ctx); err != nil {
		t.Fatalf("lock1 加锁失败: %v", err)
	}

	locked2 := make(chan error, 1)

	// lock2 尝试加锁（阻塞），应该在lock1释放后加锁成功
	go func() {
		err := lock2.Lock(ctx)
		locked2 <- err
	}()

	// 等待1秒，确保lock2进入阻塞
	time.Sleep(1 * time.Second)

	// lock2 应该还没加锁成功
	select {
	case err := <-locked2:
		if err == nil {
			t.Error("lock2 阻塞加锁时不应该立即成功")
		} else {
			t.Logf("lock2 阻塞加锁时未立即成功，err: %v", err)
		}
	default:
		t.Log("lock2 正在阻塞等待加锁")
	}

	// 释放lock1
	if lock1.stopDog != nil {
		lock1.stopDog()
	}
	_ = client.Del(ctx, RedisLockKeyPrefix+lockKey)

	// lock2 应该能加锁成功
	select {
	case err := <-locked2:
		if err != nil {
			t.Errorf("lock2 阻塞加锁在lock1释放后失败: %v", err)
		} else {
			t.Log("lock2 阻塞加锁在lock1释放后成功")
		}
	case <-time.After(3 * time.Second):
		t.Error("lock2 阻塞加锁超时未成功")
	}

	// 清理
	if lock2.stopDog != nil {
		lock2.stopDog()
	}
	_ = client.Del(ctx, RedisLockKeyPrefix+lockKey)
}

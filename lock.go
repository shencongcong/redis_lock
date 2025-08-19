package redis_lock

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/shencongcong/redis_lock/utils"
)

const RedisLockKeyPrefix = "REDIS_LOCK_PREFIX_"

var ErrLockAcquireByOthers = errors.New("lock is acquired by others")

func IsRetryableErr(err error) bool {
	return errors.Is(err, ErrLockAcquireByOthers)
}

type RedisLock struct {
	LockOptions

	key    string
	token  string
	client LockClient

	// 看门狗运作标识
	runningDog int32
	stopDog    context.CancelFunc
}

func NewRedisLock(key string, client LockClient, opts ...LockOption) *RedisLock {
	r := RedisLock{
		key:    key,
		token:  utils.GetProcessAndGoroutineIDStr(),
		client: client,
	}

	for _, opt := range opts {
		opt(&r.LockOptions)
	}
	repairLock(&r.LockOptions)
	return &r
}

// Lock 加锁
func (r *RedisLock) Lock(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			return
		}
		// 加锁成功的情况下会启用看门狗
		// 关于该锁本身是不可重入的，所以不会出现同一把锁下看门狗重复启动的情况
		r.watchDog(ctx)
	}()

	// 不管是不是阻塞模式都要取一次锁
	err = r.tryLock(ctx)
	if err == nil {
		return nil
	}
	// 非阻塞模式下加锁失败直接返回错误
	if !r.isBlock {
		return err
	}

	// 判断错误是否允许重试，不允许重试直接返回错误
	if !IsRetryableErr(err) {
		return err
	}
	// 基于阻塞模式持续轮询取锁
	err = r.blockingLock(ctx)

	return
}

// 启动看门狗
func (r *RedisLock) watchDog(ctx context.Context) {

	// 1. 非看门狗模式不处理
	if !r.LockOptions.watchDogMode {
		return
	}

	// 2. 确保之前启动的看门狗已经正常回收
	for !atomic.CompareAndSwapInt32(&r.runningDog, 0, 1) {
	}

	// 3. 启动看门狗
	ctx, r.stopDog = context.WithCancel(ctx)
	go func() {
		defer func() {
			defer atomic.StoreInt32(&r.runningDog, 0)
		}()
		r.runWatchDog(ctx)
	}()

}

func (r *RedisLock) runWatchDog(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(WatchDogWorkIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 看门狗负责在用户未显式解锁时，持续为分布式锁进行续期
		// 通过 lua 脚本，延期之前会确保保证锁仍然属于自己
		// 为避免因为网络延迟而导致锁被提前释放的问题，watch dog 续约时需要把锁的过期时长额外增加 5 s
		r.DelayExpire(ctx, r.expireSeconds+5)
	}
}

// 更新锁过期时间, 基于lua 脚本实现操作原子性
func (r *RedisLock) DelayExpire(ctx context.Context, expireSeconds int64) error {
	keysAndArgs := []interface{}{
		r.getLockKey(),
		r.token,
		expireSeconds,
	}

	reply, err := r.client.Eval(ctx, LuaCheckAndExpireDistributionLock, 1, keysAndArgs)
	if err != nil {
		return err
	}
	if reply == nil {
		return errors.New("can not expire lock without ownership of lock")
	}

	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not expire lock without ownership of lock")
	}

	return nil
}

func (r *RedisLock) tryLock(ctx context.Context) error {
	reply, err := r.client.SetNXEX(ctx, r.getLockKey(), r.token, r.expireSeconds)
	if err != nil {
		return err
	}
	if reply != 1 {
		return fmt.Errorf("reply: %d, err: %w", reply, ErrLockAcquireByOthers)
	}

	return nil
}

func (r *RedisLock) getLockKey() string {
	return RedisLockKeyPrefix + r.key
}

func (r *RedisLock) blockingLock(ctx context.Context) error {
	// 阻塞模式等待锁时间上限
	timeoutCh := time.After(time.Duration(r.blockWaitingSeconds) * time.Second)
	// 每隔50ms 获取一次锁
	ticker := time.NewTicker(time.Duration(50) * time.Millisecond)

	defer ticker.Stop()

	for range ticker.C {
		select {
		// ctx 终止了
		case <-ctx.Done():
			return fmt.Errorf("lock failed, ctx timeout, err: %w", ctx.Err())
		// 阻塞等锁上限时间
		case <-timeoutCh:
			return fmt.Errorf("block waiting time out, err: %w", ErrLockAcquireByOthers)
		// 放行
		default:
		}
		err := r.tryLock(ctx)
		if err == nil {
			return nil
		}
		// 不可重试类型的错误，直接返回
		if !IsRetryableErr(err) {
			return err
		}
	}
	return nil
}

// Unlock 解锁基于lua 脚本实现原子性操作
func (r *RedisLock) Unlock(ctx context.Context) error {
	defer func() {
		// 停止看门狗
		if r.stopDog != nil {
			r.stopDog()
		}
	}()
	keysAndArgs := []interface{}{
		r.getLockKey(),
		r.token,
	}

	reply, err := r.client.Eval(ctx, LuaCheckAndDeleteDistuributionLock, 1, keysAndArgs)
	if err != nil {
		return err
	}
	if reply == nil {
		return errors.New("can not unlock lock without ownership of lock")
	}
	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not unlock lock without ownership of lock")
	}

	return nil
}

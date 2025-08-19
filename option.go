package redis_lock

import "time"

const (
	// 默认链接池超过 10s 释放链接
	DefaultIdleTimeoutSeconds = 10
	// 默认最大活跃连接数
	DefaultMaxActive = 100
	// 默认最大空闲连接数
	DefaultMaxIdle = 10

	// 默认分布式锁过期时间
	DefaultLockTTLSeconds = 30
	// 看门狗工作时间间隙
	WatchDogWorkIntervalSeconds = 10
)

type ClientOptions struct {

	// 必填参数
	network  string
	address  string
	password string

	// 可选参数
	maxIdle            int
	maxActive          int
	idleTimeoutSeconds int
	wait               bool
}

type ClientOption func(c *ClientOptions)

func WithIdleTimeoutSeconds(idleTimeoutSeconds int) ClientOption {
	return func(c *ClientOptions) {
		c.idleTimeoutSeconds = idleTimeoutSeconds
	}
}

func WithWaitMode(wait bool) ClientOption {
	return func(c *ClientOptions) {
		c.wait = wait
	}
}

func WithMaxIdle(maxIdle int) ClientOption {
	return func(c *ClientOptions) {
		c.maxIdle = maxIdle
	}
}

func WithMaxActive(maxActive int) ClientOption {
	return func(c *ClientOptions) {
		c.maxActive = maxActive
	}
}

func repairClient(c *ClientOptions) {
	if c.maxIdle <= 0 {
		c.maxIdle = DefaultMaxIdle
	}
	if c.maxActive <= 0 {
		c.maxActive = DefaultMaxActive
	}
	if c.idleTimeoutSeconds <= 0 {
		c.idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}
}

// 锁

type LockOptions struct {
	isBlock             bool
	blockWaitingSeconds int64
	expireSeconds       int64
	watchDogMode        bool
}

type LockOption func(c *LockOptions)

func WithBlockMode() LockOption {
	return func(c *LockOptions) {
		c.isBlock = true
	}
}

func WithBlockWaitingSeconds(blockWaitingSeconds int64) LockOption {
	return func(c *LockOptions) {
		c.blockWaitingSeconds = blockWaitingSeconds
	}
}

func WithExpireSeconds(expireSeconds int64) LockOption {
	return func(c *LockOptions) {
		c.expireSeconds = expireSeconds
	}
}

func repairLock(c *LockOptions) {
	if c.isBlock && c.blockWaitingSeconds <= 0 {
		c.blockWaitingSeconds = 5
	}
	// 倘若未设置分布式锁的过期时间，则会启动watchdog
	if c.expireSeconds > 0 {
		return
	}

	c.expireSeconds = DefaultLockTTLSeconds
	c.watchDogMode = true
}

type SingleNodeConfig struct {
	Network  string
	Address  string
	Password string

	Opts []ClientOption
}

type RedLockOption func(c *RedLockOptions)

type RedLockOptions struct {
	singleNodeTimeout time.Duration
	expireDuration    time.Duration
}

func WithSingleNodeTimeout(singleNodeTimeout time.Duration) RedLockOption {
	return func(c *RedLockOptions) {
		c.singleNodeTimeout = singleNodeTimeout
	}
}

func WithExpireDuration(expireDuration time.Duration) RedLockOption {
	return func(c *RedLockOptions) {
		c.expireDuration = expireDuration
	}
}

func repairRedLock(c *RedLockOptions) {
	if c.singleNodeTimeout <= 0 {
		c.singleNodeTimeout = DefaultRedLockNodeTimeout
	}
}

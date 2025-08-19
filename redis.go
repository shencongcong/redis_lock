package redis_lock

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

type LockClient interface {
	SetNXEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error)
	Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error)
}

type Client struct {
	ClientOptions
	pool *redis.Pool
}

func NewClient(network, address, password string, opts ...ClientOption) *Client {

	c := Client{
		ClientOptions: ClientOptions{
			network:  network,
			address:  address,
			password: password,
		},
	}

	for _, opt := range opts {
		opt(&c.ClientOptions)
	}

	repairClient(&c.ClientOptions)

	pool := c.getRedisPool()

	return &Client{
		pool: pool,
	}
}

func (c *Client) getRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     c.maxIdle,
		MaxActive:   c.maxActive,
		IdleTimeout: time.Duration(c.idleTimeoutSeconds) * time.Second,
		Wait:        c.wait,
		Dial: func() (redis.Conn, error) {
			c, err := c.getRedisConn()
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// getRedisConn 获取redis链接
func (c *Client) getRedisConn() (redis.Conn, error) {
	if c.address == "" {
		panic("address is required")
	}
	var dialOpts []redis.DialOption
	if len(c.password) > 0 {
		dialOpts = append(dialOpts, redis.DialPassword(c.password))
	}

	conn, err := redis.DialContext(context.Background(), c.network, c.address, dialOpts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("key is required")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return redis.String(conn.Do("GET", key))
}

func (c *Client) Set(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("key or value not is empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	resp, err := conn.Do("SET", key, value)
	if err != nil {
		return -1, err
	}

	if respStr, ok := resp.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	return redis.Int64(resp, err)
}

func (c *Client) SetNXEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SETNXEX key or value can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	reply, err := conn.Do("SET", key, value, "EX", expireSeconds, "NX")
	if err != nil {
		return -1, err
	}
	if reply == nil {
		// reply为nil时，说明SET NX未成功（key已存在），直接返回0
		return 0, nil
	}

	if respStr, ok := reply.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	return redis.Int64(reply, err)
}

func (c *Client) Del(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("redis Del key can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)

	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Do("DEL", key)

	return err
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return -1, errors.New("redis Incr key can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	reply, err := conn.Do("INCR", key)

	if err != nil {
		return -1, err
	}

	if respStr, ok := reply.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	return redis.Int64(reply, err)
}

func (c *Client) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	args := make([]interface{}, 2+len(keysAndArgs))
	args[0] = src
	args[1] = keyCount
	copy(args[2:], keysAndArgs)

	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	return conn.Do("EVAL", args...)
}

package redis_lock

// 判断是否拥有分布式锁的归属权，是则删除
const LuaCheckAndDeleteDistuributionLock = `
	local lock_key = KEYS[1]
	local target_token = ARGV[1]
	local current_token = redis.call("GET", lock_key)
	if current_token == target_token then
		redis.call("DEL", lock_key)
		return 1
	end
	return 0
`

// 判断是否拥有分布式锁的归属权,是则设置过期时间
const LuaCheckAndExpireDistributionLock = `
	local lock_key = KEYS[1]
	local target_token = ARGV[1]
	local expire_time = ARGV[2]
	local current_token = redis.call("GET", lock_key)
	if current_token == target_token then
		redis.call("EXPIRE", lock_key, expire_time)
		return 1
	end
	return 0
`

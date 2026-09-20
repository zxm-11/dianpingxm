local voucherId = ARGV[1]
local userId = ARGV[2]
local orderId = ARGV[3]  --没用到

local stockKey = "seckill:stock:" .. voucherId
local orderKey = "seckill:order:" .. voucherId
local stock = redis.call('get', stockKey)

if (not stock or tonumber(stock) <= 0) then
    return 1  --库存不足
end
--s is member 判断userid是否在集合orderkey里
if (redis.call('sismember', orderKey, userId) == 1) then
    return 2  --不符合一人一单
end

redis.call('incrby', stockKey, -1)
redis.call('sadd', orderKey, userId)  --s add 把userId加入集合orderKey中
return 0      --判断并扣减成功

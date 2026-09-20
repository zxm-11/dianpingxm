package constants

import "time"

const (
	LoginCodeKey  = "login:code:"
	LoginUserKey  = "login:token:"
	CacheShopKey  = "cache:shop:"
	CacheTypeKey  = "cache:shop:type:"
	LockShopKey   = "lock:shop:"
	SeckillStock  = "seckill:stock:"
	BlogLikedKey  = "blog:liked:"
	FollowKey     = "follows:"
	FeedKey       = "feed:"
	UserNickPrefx = "user_"
	ShopGeoKey    = "shop:geo:" // 店铺地理坐标(按类型分桶)
	SignKey       = "sign:"     // 用户签到位图
)

const (
	DefaultPageSize = 5
	MaxPageSize     = 10
	CountBits       = 32
	// ShopGeoRadius 附近商户默认搜索半径，单位：米
	ShopGeoRadius = 5000.0
)

const BeginTimestamp int64 = 1767225600

const (
	LoginCodeTTL  = 2 * time.Minute
	LoginUserTTL  = 36000 * time.Second
	CacheNullTTL  = 2 * time.Minute
	CacheShopTTL  = 30 * time.Minute
	LockShopTTL   = 10 * time.Second
	ShopTypeCache = 24 * time.Hour
)

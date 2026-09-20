package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"hm-dianping/internal/constants"
)

type RedisIDWorker struct {
	rdb *redis.Client
}

func NewRedisIDWorker(rdb *redis.Client) *RedisIDWorker {
	return &RedisIDWorker{rdb: rdb}
}

// NextID :redis 实现全局id生成器
func (w *RedisIDWorker) NextID(ctx context.Context, prefix string) (uint64, error) {
	now := time.Now().UTC()
	seconds := now.Unix() - constants.BeginTimestamp
	key := fmt.Sprintf("icr:%s:%s", prefix, now.Format("2006:01:02")) //key -> icr:order:2026:09:16
	seq, err := w.rdb.Incr(ctx, key).Result()                         //当天第几单
	if err != nil {
		return 0, err
	}
	return uint64(seconds)<<constants.CountBits | uint64(seq), nil //随机id
}

package service

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
)

type ShopTypeService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewShopTypeService(db *gorm.DB, rdb *redis.Client) *ShopTypeService {
	return &ShopTypeService{db: db, rdb: rdb}
}

func (s *ShopTypeService) List(ctx context.Context) ([]model.ShopType, error) {
	key := constants.CacheTypeKey + "list"
	if raw, err := s.rdb.Get(ctx, key).Result(); err == nil && raw != "" {
		var list []model.ShopType
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			return list, nil
		}
	}
	var list []model.ShopType
	if err := s.db.WithContext(ctx).Order("sort ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	bytes, _ := json.Marshal(list)
	_ = s.rdb.Set(ctx, key, bytes, constants.ShopTypeCache).Err()
	return list, nil
}

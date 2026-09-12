package service

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
)

type FollowService struct {
	db  *gorm.DB
	rdb *redis.Client
	us  *UserService
}

func NewFollowService(db *gorm.DB, rdb *redis.Client, us *UserService) *FollowService {
	return &FollowService{db: db, rdb: rdb, us: us}
}

func (s *FollowService) Follow(ctx context.Context, userID, followUserID uint64, isFollow bool) error {
	key := constants.FollowKey + strconv.FormatUint(userID, 10)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isFollow {
			follow := model.Follow{UserID: userID, FollowUserID: followUserID}
			if err := tx.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).FirstOrCreate(&follow).Error; err != nil {
				return err
			}
			return s.rdb.SAdd(ctx, key, followUserID).Err()
		}
		if err := tx.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).Delete(&model.Follow{}).Error; err != nil {
			return err
		}
		return s.rdb.SRem(ctx, key, followUserID).Err()
	})
}

func (s *FollowService) IsFollow(ctx context.Context, userID, followUserID uint64) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Follow{}).Where("user_id = ? AND follow_user_id = ?", userID, followUserID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *FollowService) Commons(ctx context.Context, userID, otherID uint64) ([]model.UserView, error) {
	ids, err := s.rdb.SInter(ctx, constants.FollowKey+strconv.FormatUint(userID, 10), constants.FollowKey+strconv.FormatUint(otherID, 10)).Result()
	if err != nil || len(ids) == 0 {
		return []model.UserView{}, err
	}
	uintIDs := make([]uint64, 0, len(ids))
	for _, id := range ids {
		parsed, err := strconv.ParseUint(id, 10, 64)
		if err == nil {
			uintIDs = append(uintIDs, parsed)
		}
	}
	users, err := s.us.UsersByIDs(ctx, uintIDs)
	if err != nil {
		return nil, err
	}
	result := make([]model.UserView, 0, len(users))
	for _, id := range uintIDs {
		if u, ok := users[id]; ok {
			result = append(result, u)
		}
	}
	return result, nil
}

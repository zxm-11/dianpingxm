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

// Follow 关注/取关某人
func (s *FollowService) Follow(ctx context.Context, userID, followUserID uint64, isFollow bool) error {
	key := constants.FollowKey + strconv.FormatUint(userID, 10)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isFollow {
			follow := model.Follow{UserID: userID, FollowUserID: followUserID}
			if err := tx.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).
				FirstOrCreate(&follow).Error; err != nil { //已关注:First 未关注:Create
				return err
			}
			return s.rdb.SAdd(ctx, key, followUserID).Err() //存入集合
		}
		if err := tx.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).Delete(&model.Follow{}).Error; err != nil {
			return err
		}
		return s.rdb.SRem(ctx, key, followUserID).Err()
	})
}

// IsFollow 查看是否关注
func (s *FollowService) IsFollow(ctx context.Context, userID, followUserID uint64) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&model.Follow{}).
		Where("user_id = ? AND follow_user_id = ?", userID, followUserID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Commons 查看和某人的共同关注
func (s *FollowService) Commons(ctx context.Context, userID, otherID uint64) ([]model.UserView, error) {
	//SInter:取两个集合的交集
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
	users, err := s.us.UsersByIDs(ctx, uintIDs) //批量获取用户视图
	if err != nil {
		return nil, err
	}
	result := make([]model.UserView, 0, len(users))
	for _, id := range uintIDs {
		if u, ok := users[id]; ok { //ok是避免(redis存了id 而 mysql没存id)错误
			result = append(result, u)
		}
	}
	return result, nil
}

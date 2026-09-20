package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
)

type BlogService struct {
	db  *gorm.DB
	rdb *redis.Client
	us  *UserService
}

func NewBlogService(db *gorm.DB, rdb *redis.Client, us *UserService) *BlogService {
	return &BlogService{db: db, rdb: rdb, us: us}
}

// 写探店博客
func (s *BlogService) Save(ctx context.Context, blog *model.Blog, userID uint64) error {
	blog.UserID = userID
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(blog).Error; err != nil {
			return err
		}
		//feed流 推送给粉丝(推模式(写扩散))
		var follows []model.Follow //反查出粉丝
		if err := tx.Where("follow_user_id = ?", userID).Find(&follows).Error; err != nil {
			return err
		}
		score := float64(time.Now().UnixMilli())
		for _, follow := range follows {
			key := constants.FeedKey + strconv.FormatUint(follow.UserID, 10)
			_ = s.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: blog.ID}).Err()
		}
		return nil
	})
}

// 查询对应页数的热点博客 (按照点赞量从高到底排序)
func (s *BlogService) QueryHot(ctx context.Context, current int, viewerID uint64) ([]model.Blog, error) {
	if current < 1 {
		current = 1
	}
	var blogs []model.Blog
	// select * from `tb_blogs` order by liked desc limit 10 offset 10*(current-1) --> (OFFSET = (页码 - 1) × 每页条数)
	if err := s.db.WithContext(ctx).
		Order("liked DESC").
		Offset((current - 1) * constants.MaxPageSize).
		Limit(constants.MaxPageSize).
		Find(&blogs).Error; err != nil {
		return nil, err
	}
	return s.enrich(ctx, blogs, viewerID)
}

// 查看探店博客(通过blog id查看)
func (s *BlogService) QueryByID(ctx context.Context, id uint64, viewerID uint64) (*model.Blog, error) {
	var blog model.Blog
	if err := s.db.WithContext(ctx).First(&blog, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	blogs, err := s.enrich(ctx, []model.Blog{blog}, viewerID)
	if err != nil {
		return nil, err
	}
	return &blogs[0], nil //对应博客id 只对应一条博客信息
}

// 探店博客点赞
func (s *BlogService) Like(ctx context.Context, blogID uint64, userID uint64) error {
	key := constants.BlogLikedKey + strconv.FormatUint(blogID, 10)
	member := strconv.FormatUint(userID, 10)
	_, err := s.rdb.ZScore(ctx, key, member).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	isLiked := err == nil
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !isLiked {
			res := tx.Model(&model.Blog{}).Where("id = ?", blogID).UpdateColumn("liked", gorm.Expr("liked + 1"))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return errors.New("博客不存在")
			}
			return s.rdb.ZAdd(ctx, key, redis.Z{Score: float64(time.Now().UnixMilli()), Member: member}).Err()
		}
		//isliked == true  取消点赞
		res := tx.Model(&model.Blog{}).Where("id = ?", blogID).UpdateColumn("liked", gorm.Expr("liked - 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("博客不存在")
		}
		return s.rdb.ZRem(ctx, key, member).Err()
	})
}

// 查看探店博客点赞列表
func (s *BlogService) QueryLikes(ctx context.Context, blogID uint64) ([]model.UserView, error) {
	key := constants.BlogLikedKey + strconv.FormatUint(blogID, 10)
	members, err := s.rdb.ZRange(ctx, key, 0, 4).Result() //查看点赞最早的前四个客户
	if err != nil || len(members) == 0 {
		return []model.UserView{}, err
	}
	ids := make([]uint64, 0, len(members))
	for _, member := range members {
		id, err := strconv.ParseUint(member, 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	//批量获取对应用户的视图
	users, err := s.us.UsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	ordered := make([]model.UserView, 0, len(ids))
	//保证按时间顺序排序(ids里面是存好的)
	for _, id := range ids {
		if user, ok := users[id]; ok {
			ordered = append(ordered, user)
		}
	}
	return ordered, nil
}

// 查询对应用户的博客
func (s *BlogService) QueryByUser(ctx context.Context, userID uint64, current int, viewerID uint64) ([]model.Blog, error) {
	if current < 1 {
		current = 1
	}
	var blogs []model.Blog
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("create_time DESC").
		Offset((current - 1) * constants.MaxPageSize).
		Limit(constants.MaxPageSize).Find(&blogs).Error; err != nil {
		return nil, err
	}
	return s.enrich(ctx, blogs, viewerID)
}

// 关注流
func (s *BlogService) QueryFeed(ctx context.Context, userID uint64, max int64, offset int64) (model.ScrollResult, error) {
	key := constants.FeedKey + strconv.FormatUint(userID, 10)
	//Zset Rev:降序 ByScore:按区间取 WithScores:返回结果带上Score
	items, err := s.rdb.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{Min: "0", Max: strconv.FormatInt(max, 10), Offset: offset, Count: constants.MaxPageSize}).Result()
	if err != nil || len(items) == 0 {
		return model.ScrollResult{List: []model.Blog{}, MinTime: 0, Offset: 0}, err
	}
	ids := make([]uint64, 0, len(items))
	minTime := int64(items[0].Score)
	newOffset := int64(1)
	for i, item := range items {
		id, err := strconv.ParseUint(fmt.Sprint(item.Member), 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
		score := int64(item.Score)
		//offset偏移量:下一个时间戳区间跳过offset条,(跳过‘重复时间已经读取过的’ )
		if i == 0 || score < minTime {
			minTime = score
			newOffset = 1
		} else if score == minTime {
			newOffset++
		}
	}
	blogs, err := s.blogsByIDs(ctx, ids, userID)
	if err != nil {
		return model.ScrollResult{}, err
	}
	return model.ScrollResult{List: blogs, MinTime: minTime, Offset: newOffset}, nil
}

// (逐条)补充作者信息(Name,Icon) && 查看当前用户是否点过赞
func (s *BlogService) enrich(ctx context.Context, blogs []model.Blog, viewerID uint64) ([]model.Blog, error) {
	for i := range blogs {
		view, err := s.us.GetUserView(ctx, blogs[i].UserID)
		if err == nil {
			blogs[i].Name = view.NickName
			blogs[i].Icon = view.Icon
		}
		if viewerID != 0 {
			key := constants.BlogLikedKey + strconv.FormatUint(blogs[i].ID, 10)
			_, err := s.rdb.ZScore(ctx, key, strconv.FormatUint(viewerID, 10)).Result()
			blogs[i].IsLike = err == nil
		}
	}
	return blogs, nil
}

// 获取对应id的博客
func (s *BlogService) blogsByIDs(ctx context.Context, ids []uint64, viewerID uint64) ([]model.Blog, error) {
	if len(ids) == 0 {
		return []model.Blog{}, nil
	}
	var blogs []model.Blog
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&blogs).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.Blog, len(blogs))
	for _, blog := range blogs {
		byID[blog.ID] = blog
	}
	ordered := make([]model.Blog, 0, len(ids))
	for _, id := range ids {
		if blog, ok := byID[id]; ok {
			ordered = append(ordered, blog)
		}
	}
	return s.enrich(ctx, ordered, viewerID)
}

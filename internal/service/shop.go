package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
	"hm-dianping/internal/script"
)

type ShopService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewShopService(db *gorm.DB, rdb *redis.Client) *ShopService {
	return &ShopService{db: db, rdb: rdb}
}

// 项目精华之一:分布式锁/缓存击穿/缓存穿透
func (s *ShopService) GetByID(ctx context.Context, id uint64) (*model.Shop, error) { //此方法只在noroute里用到了
	key := constants.CacheShopKey + strconv.FormatUint(id, 10)

	cached, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		if cached == "" { //解决缓存穿透:查到缓存空值,直接返回nil,不再查数据库
			return nil, nil
		}
		var shop model.Shop
		if err := json.Unmarshal([]byte(cached), &shop); err == nil { //缓存有数据
			return &shop, nil
		}
	}
	if err != nil && err != redis.Nil {
		return nil, err
	}
	//缓存无数据->数据库->互斥锁1.trylock(防止热点key失效导致的缓存击穿)
	lockKey := constants.LockShopKey + strconv.FormatUint(id, 10)
	owner := uuid.NewString() //随机uuid字符串

	accquired := false //确保是真拿到了锁,而不是超过5次自动退出循环
	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {

		//  先看缓存好了没——别的线程可能已经建完了
		if cached, err := s.rdb.Get(ctx, key).Result(); err == nil {
			if cached == "" {
				return nil, nil
			}
			var shop model.Shop
			if json.Unmarshal([]byte(cached), &shop) == nil {
				return &shop, nil // 命中:直接返回,不用抢锁
			}
		}
		//NX:没有才建
		locked, err := s.rdb.SetNX(ctx, lockKey, owner, constants.LockShopTTL).Result() //同时抢锁+设ttl(set cache:lock:id<uuid> nx ex 10sec)
		if err != nil {
			return nil, err
		}
		if locked {
			accquired = true
			break
		}
		backoff := time.Duration(50+(i*50)) * time.Millisecond //(ms)
		time.Sleep(backoff)
	}
	if !accquired {
		log.Printf("[LOCK-FAIL] 抢锁失败 key=%s", lockKey)
		return nil, errors.New("系统繁忙,请稍后再试")
	}

	defer s.rdb.Eval(context.Background(), script.UnlockLua, []string{lockKey}, owner)

	//2.doublecheck:抢到锁重查缓存,避免多次重新查库
	switch cached, err := s.rdb.Get(ctx, key).Result(); {
	case err == nil:
		if cached == "" {
			return nil, nil //确实不存在,返回缓存空值
		}

		var shop model.Shop
		if jsonErr := json.Unmarshal([]byte(cached), &shop); jsonErr == nil {
			return &shop, nil //命中:直接返回,不查库
		}
		//jsonErr!=nil->反序列化失败,当miss处理,继续查库
	case errors.Is(err, redis.Nil):
	default:
		return nil, err //redis故障
	}
	//doublecheck确认缓存真的没有才查库
	var shop model.Shop
	if err := s.db.WithContext(ctx).First(&shop, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.rdb.Set(ctx, key, "", constants.CacheNullTTL).Err() //数据库无数据->防止缓存穿透-redis缓存空对象
			return nil, nil
		}
		return nil, err
	}
	bytes, _ := json.Marshal(shop)
	_ = s.rdb.Set(ctx, key, bytes, constants.CacheShopTTL).Err()
	return &shop, nil
}

// 创建商铺
func (s *ShopService) Create(ctx context.Context, shop *model.Shop) error {
	return s.db.WithContext(ctx).Create(shop).Error
}

// 更新商铺信息
func (s *ShopService) Update(ctx context.Context, shop *model.Shop) error {
	if shop.ID == 0 {
		return errors.New("店铺id不能为空")
	}
	//事务操作:一旦缓存删不掉就把数据库改动一起撤销
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { //transaction事务操作(用tx而不是db)/任何err都会回滚事务,返回nil提交事务
		if err := tx.Model(&model.Shop{}).Where("id = ?", shop.ID).Updates(shop).Error; err != nil { //参考gorm-update:批量更新
			return err
		}
		return s.rdb.Del(ctx, constants.CacheShopKey+strconv.FormatUint(shop.ID, 10)).Err()
	})
}

// 按类型查询商铺
func (s *ShopService) PageByType(ctx context.Context, typeID uint64, current int) ([]model.Shop, error) {
	if current < 1 {
		current = 1
	}
	var shops []model.Shop
	err := s.db.WithContext(ctx).Where("type_id = ?", typeID).
		Offset((current - 1) * constants.DefaultPageSize). //Offset 请指定在开始返回记录之前要跳过的数量
		Limit(constants.DefaultPageSize).Find(&shops).Error
	return shops, err
}

func (s *ShopService) PageByName(ctx context.Context, name string, current int) ([]model.Shop, error) {
	if current < 1 {
		current = 1
	}
	var shops []model.Shop
	q := s.db.WithContext(ctx)
	if name != "" {
		q = q.Where("name LIKE ?", fmt.Sprintf("%%%s%%", name)) //支持模糊查询
	}
	err := q.Offset((current - 1) * constants.MaxPageSize).Limit(constants.MaxPageSize).Find(&shops).Error
	return shops, err
}

// LoadShopGeoData 把 MySQL 中所有店铺的经纬度导入 Redis GEO(按店铺类型分桶)。
// 对应教程里的 loadShopData()，只需在初始化/修复时跑一次，多次执行是幂等(幂等:对于同一个操作,执行一次和执行多次效果是相同的)的。
func (s *ShopService) LoadShopGeoData(ctx context.Context) error {
	var shops []model.Shop
	if err := s.db.WithContext(ctx).Find(&shops).Error; err != nil {
		return err
	}
	// 按 typeId 分组，同一类型的店铺写进同一个 key:shop:geo:{typeId}
	grouped := make(map[uint64][]*redis.GeoLocation)
	for _, shop := range shops {
		grouped[shop.TypeID] = append(grouped[shop.TypeID], &redis.GeoLocation{
			Name:      strconv.FormatUint(shop.ID, 10), // member 用店铺 id 字符串
			Longitude: shop.X,                          // x = 经度
			Latitude:  shop.Y,                          // y = 纬度
		})
	}
	for typeID, locations := range grouped {
		key := constants.ShopGeoKey + strconv.FormatUint(typeID, 10)
		if err := s.rdb.GeoAdd(ctx, key, locations...).Err(); err != nil {
			return err
		}
	}
	return nil
}

// PageByTypeWithGeo 基于 Redis GEO 按「距当前位置」由近到远查询某类店铺。
// GEO 没有 offset，所以用 Count 取到 current 页末尾，再在内存里切页。
func (s *ShopService) PageByTypeWithGeo(ctx context.Context, typeID uint64, current int, x, y float64) ([]model.Shop, error) {
	if current < 1 {
		current = 1
	}
	key := constants.ShopGeoKey + strconv.FormatUint(typeID, 10)
	pageSize := constants.DefaultPageSize

	//location 在原来基础上附带请求coord/dist/hash
	query := &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  x,
			Latitude:   y,
			Radius:     constants.ShopGeoRadius, //圆形区域:半径 5000 米
			RadiusUnit: "m",                     //单位
			Sort:       "ASC",                   //由近到远
			Count:      current * pageSize,
		},
		WithDist: true, //命令里带上 WITHDIST，结果才有 Dist
	}
	locations, err := s.rdb.GeoSearchLocation(ctx, key, query).Result()
	if err != nil {
		return nil, err
	}
	//GEO 索引为空(还没导入)时降级为普通分页，避免直接返回空列表
	if len(locations) == 0 {
		return s.PageByType(ctx, typeID, current)
	}

	from := (current - 1) * pageSize
	if from >= len(locations) {
		return []model.Shop{}, nil
	}
	to := from + pageSize
	if to > len(locations) {
		to = len(locations)
	}
	page := locations[from:to] //GEO 不支持 offset，只能先取全量再切页((current-1)*pageSize->(current-1)*pageSize+pageSize)

	ids := make([]uint64, 0, len(page))
	distance := make(map[uint64]float64, len(page))
	for _, loc := range page {
		id, err := strconv.ParseUint(loc.Name, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		distance[id] = loc.Dist
	}
	if len(ids) == 0 {
		return []model.Shop{}, nil
	}

	var shops []model.Shop
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&shops).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.Shop, len(shops))
	for _, shop := range shops {
		byID[shop.ID] = shop
	}
	//IN 查询不保证顺序，必须按 GEO 返回的 ids 顺序回填，distance 一并写入非表字段
	ordered := make([]model.Shop, 0, len(ids))
	for _, id := range ids {
		shop, ok := byID[id]
		if !ok {
			continue
		}
		shop.Distance = distance[id]
		ordered = append(ordered, shop)
	}
	return ordered, nil
}

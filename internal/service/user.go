package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
)

var phonePattern = regexp.MustCompile("^1[3-9]\\d{9}$")

type UserService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewUserService(db *gorm.DB, rdb *redis.Client) *UserService {
	return &UserService{db: db, rdb: rdb}
}

// SendCode 为指定手机号生成 6 位登录验证码并缓存到 Redis（含过期时间）。
func (s *UserService) SendCode(ctx context.Context, phone string) (string, error) {
	if !phonePattern.MatchString(phone) {
		return "", errors.New("手机号格式错误")
	}
	code := fmt.Sprintf("%06d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1000000))
	if err := s.rdb.Set(ctx, constants.LoginCodeKey+phone, code, constants.LoginCodeTTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

// 用户登录
func (s *UserService) Login(ctx context.Context, phone, code string) (string, error) {
	if !phonePattern.MatchString(phone) {
		return "", errors.New("手机号格式错误")
	}
	cacheCode, err := s.rdb.Get(ctx, constants.LoginCodeKey+phone).Result()
	if err == redis.Nil || cacheCode == "" {
		return "", errors.New("验证码不存在或已过期")
	}
	if err != nil {
		return "", err
	}
	if cacheCode != code {
		return "", errors.New("验证码错误")
	}

	var user model.User
	if err := s.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) { //first,last,take查不到会返回ErrRecordNotFound错误而 find会返回空值
			return "", err
		}
		user = model.User{Phone: phone, NickName: constants.UserNickPrefx + uuid.NewString()[:12]}
		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return "", err
		}
	}

	token := uuid.NewString()
	key := constants.LoginUserKey + token
	values := map[string]any{
		"id":       strconv.FormatUint(user.ID, 10),
		"nickName": user.NickName,
		"icon":     user.Icon,
	}
	if err := s.rdb.HSet(ctx, key, values).Err(); err != nil {
		return "", err
	}
	if err := s.rdb.Expire(ctx, key, constants.LoginUserTTL).Err(); err != nil {
		return "", err
	} //设置ttl
	_ = s.rdb.Del(ctx, constants.LoginCodeKey+phone).Err() //错误不做处理了,起码有ttl兜底
	return token, nil
}

// 用户登出
func (s *UserService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.rdb.Del(ctx, constants.LoginUserKey+token).Err() //删除Redis里的对话key->token失效->再次请求401
}

// 获取指定用户全部信息
func (s *UserService) GetInfo(ctx context.Context, id uint64) (*model.UserInfo, error) {
	var info model.UserInfo
	if err := s.db.WithContext(ctx).First(&info, "user_id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &info, nil
}

// 获取用户视图(1)
func (s *UserService) GetUserView(ctx context.Context, id uint64) (model.UserView, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return model.UserView{}, err
	}
	return toUserView(user), nil
}

// 获取用户视图(2)
func toUserView(user model.User) model.UserView {
	return model.UserView{ID: user.ID, NickName: user.NickName, Icon: user.Icon}
}

// UsersByIDs 批量查询用户视图
func (s *UserService) UsersByIDs(ctx context.Context, ids []uint64) (map[uint64]model.UserView, error) {
	if len(ids) == 0 {
		return map[uint64]model.UserView{}, nil
	}
	var users []model.User
	//select * from `tb_user` where id in (ids)
	if err := s.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64]model.UserView, len(users))
	for _, user := range users {
		result[user.ID] = toUserView(user)
	}
	return result, nil
}

// signKey 生成某用户<某个月>的签到位图 key：sign:{userId}:{yyyyMM}
// 一个月一个 key，位偏移 0 表示 1 号，偏移 day-1 表示当天。
func signKey(userID uint64, t time.Time) string {
	return constants.SignKey + strconv.FormatUint(userID, 10) + ":" + t.Format("200601")
}

// Sign 用户签到：把「今天」对应的位从 0 置 1。
// 返回 true 表示本次签到成功，false 表示今天已经签过了。
func (s *UserService) Sign(ctx context.Context, userID uint64) (bool, error) {
	now := time.Now()
	key := signKey(userID, now)
	offset := int64(now.Day() - 1) //1 号 -> bit0
	//SETBIT 的返回值是这一位「原来的值」，天然可以判断今天是否已签到，无需先 GETBIT
	old, err := s.rdb.SetBit(ctx, key, offset, 1).Result() //result返回旧值(old==0代表没签过-->返回true签到成功)
	if err != nil {
		return false, err
	}
	return old == 0, nil
}

// SignCount 统计本月「连续」签到天数(从今天往前数，遇到第一个 0 就停)。
func (s *UserService) SignCount(ctx context.Context, userID uint64) (int, error) {
	now := time.Now()
	dayOfMonth := now.Day()
	//BITFIELD GET u{dayOfMonth} 0：一次取出「1 号到今天」的所有位，bit0 = 1 号、bit(day-1) = 今天
	values, err := s.rdb.BitField(ctx, signKey(userID, now), "GET", "u"+strconv.Itoa(dayOfMonth), 0).Result()
	//子操作(读) 读取的位数(20:0-19)[u:编码格式usigned u20(假设20号)] 开始位
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	num := values[0]
	count := 0
	for i := dayOfMonth - 1; i >= 0; i-- { //高位(今天)往低位(1 号)倒着数
		if (num>>uint(dayOfMonth-1-i))&1 == 0 { //num>>uint(i)把第i位移到最低位(bit0), &1(二者都是1才为1)  -->遇到0直接结束循环
			break
		}
		count++
	}
	return count, nil
}

// SignStatus 返回本月从 1 号到今天每一天是否签到(1 已签 / 0 未签)，用于前端签到日历。
func (s *UserService) SignStatus(ctx context.Context, userID uint64) ([]int, error) {
	now := time.Now()
	dayOfMonth := now.Day()
	values, err := s.rdb.BitField(ctx, signKey(userID, now), "GET", "u"+strconv.Itoa(dayOfMonth), 0).Result()
	if err != nil {
		return nil, err
	}
	var num int64
	if len(values) > 0 {
		num = values[0]
	}
	days := make([]int, dayOfMonth) //自动填充默认值0
	for i := 0; i < dayOfMonth; i++ {
		if (num>>uint(dayOfMonth-1-i))&1 == 1 {
			days[i] = 1
		}
	}
	return days, nil
}

/*
key 里的位:   offset 0   offset 1  ...  offset 19
↓          ↓             ↓
num 里的位:    bit 19     bit 18   ...   bit 0
(最高位)                    (最低位)
*/

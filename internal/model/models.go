package model

import "time"

// UserView 用户公开信息视图
type UserView struct {
	ID       uint64 `json:"id,string"`
	NickName string `json:"nickName"` //昵称
	Icon     string `json:"icon"`     //头像的url
}

// ScrollResult 滚动分页结果
type ScrollResult struct {
	List    any   `json:"list"`
	MinTime int64 `json:"minTime"`
	Offset  int64 `json:"offset"`
}

type Blog struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id,string"`
	ShopID     uint64    `gorm:"column:shop_id" json:"shopId,string"`
	UserID     uint64    `gorm:"column:user_id" json:"userId,string"`
	Title      string    `gorm:"column:title" json:"title"`
	Images     string    `gorm:"column:images" json:"images"`
	Content    string    `gorm:"column:content" json:"content"`
	Liked      int       `gorm:"column:liked" json:"liked"`
	Comments   int       `gorm:"column:comments" json:"comments"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	Name       string    `gorm:"-" json:"name,omitempty"`
	Icon       string    `gorm:"-" json:"icon,omitempty"`
	IsLike     bool      `gorm:"-" json:"isLike"`
}

func (Blog) TableName() string { return "tb_blog" }

type Follow struct {
	ID           uint64    `gorm:"column:id;primaryKey" json:"id,string"`
	UserID       uint64    `gorm:"column:user_id" json:"userId,string"`
	FollowUserID uint64    `gorm:"column:follow_user_id" json:"followUserId,string"`
	CreateTime   time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
}

func (Follow) TableName() string { return "tb_follow" }

type SeckillVoucher struct {
	VoucherID  uint64    `gorm:"column:voucher_id;primaryKey" json:"voucherId,string"`
	Stock      int       `gorm:"column:stock" json:"stock"`
	BeginTime  time.Time `gorm:"column:begin_time;default:CURRENT_TIMESTAMP" json:"beginTime"`
	EndTime    time.Time `gorm:"column:end_time;default:CURRENT_TIMESTAMP" json:"endTime"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

func (SeckillVoucher) TableName() string { return "tb_seckill_voucher" }

type Shop struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id,string"`
	Name       string    `gorm:"column:name" json:"name"`
	TypeID     uint64    `gorm:"column:type_id" json:"typeId,string"`
	Images     string    `gorm:"column:images" json:"images"`
	Area       string    `gorm:"column:area" json:"area"`
	Address    string    `gorm:"column:address" json:"address"`
	X          float64   `gorm:"column:x" json:"x"`
	Y          float64   `gorm:"column:y" json:"y"`
	AvgPrice   int64     `gorm:"column:avg_price" json:"avgPrice"`
	Sold       int       `gorm:"column:sold" json:"sold"`
	Comments   int       `gorm:"column:comments" json:"comments"`
	Score      int       `gorm:"column:score" json:"score"`
	OpenHours  string    `gorm:"column:open_hours" json:"openHours"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	Distance   float64   `gorm:"-" json:"distance,omitempty"`
}

func (Shop) TableName() string { return "tb_shop" }

type ShopType struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id,string"`
	Name       string    `gorm:"column:name" json:"name"`
	Icon       string    `gorm:"column:icon" json:"icon"`
	Sort       int       `gorm:"column:sort" json:"sort"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

func (ShopType) TableName() string { return "tb_shop_type" }

type User struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id,string"`
	Phone      string    `gorm:"column:phone" json:"phone"`
	Password   string    `gorm:"column:password" json:"password,omitempty"`
	NickName   string    `gorm:"column:nick_name" json:"nickName"`
	Icon       string    `gorm:"column:icon" json:"icon"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

func (User) TableName() string { return "tb_user" }

type UserInfo struct {
	UserID     uint64     `gorm:"column:user_id;primaryKey" json:"userId,string"`
	City       string     `gorm:"column:city" json:"city"`
	Introduce  string     `gorm:"column:introduce" json:"introduce"`
	Fans       int        `gorm:"column:fans" json:"fans"`
	Followee   int        `gorm:"column:followee" json:"followee"`
	Gender     bool       `gorm:"column:gender" json:"gender"`
	Birthday   *time.Time `gorm:"column:birthday" json:"birthday,omitempty"`
	Credits    int        `gorm:"column:credits" json:"credits"`
	Level      int        `gorm:"column:level" json:"level"`
	CreateTime time.Time  `gorm:"column:create_time;autoCreateTime" json:"-"`
	UpdateTime time.Time  `gorm:"column:update_time;autoUpdateTime" json:"-"`
}

func (UserInfo) TableName() string { return "tb_user_info" }

type Voucher struct {
	ID          uint64     `gorm:"column:id;primaryKey" json:"id,string"`
	ShopID      uint64     `gorm:"column:shop_id" json:"shopId,string"`
	Title       string     `gorm:"column:title" json:"title"`
	SubTitle    string     `gorm:"column:sub_title" json:"subTitle"`
	Rules       string     `gorm:"column:rules" json:"rules"`
	PayValue    int64      `gorm:"column:pay_value" json:"payValue"`
	ActualValue int64      `gorm:"column:actual_value" json:"actualValue"`
	Type        int        `gorm:"column:type" json:"type"`
	Status      int        `gorm:"column:status" json:"status"`
	CreateTime  time.Time  `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime  time.Time  `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	Stock       *int       `gorm:"column:stock;->" json:"stock,omitempty"`          //库存 *int -> 区分nil(普通券)
	BeginTime   *time.Time `gorm:"column:begin_time;->" json:"beginTime,omitempty"` //*区分nil(普通券)
	EndTime     *time.Time `gorm:"column:end_time;->" json:"endTime,omitempty"`     //*区分nil(普通券)
}

func (Voucher) TableName() string { return "tb_voucher" }

type VoucherOrder struct {
	ID         uint64     `gorm:"column:id;primaryKey" json:"id,string"`
	UserID     uint64     `gorm:"column:user_id" json:"userId,string"`
	VoucherID  uint64     `gorm:"column:voucher_id" json:"voucherId,string"`
	PayType    int        `gorm:"column:pay_type" json:"payType"`
	Status     int        `gorm:"column:status" json:"status"` //1:上架 2:下架 3:过期
	CreateTime time.Time  `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	PayTime    *time.Time `gorm:"column:pay_time" json:"payTime,omitempty"`
	UseTime    *time.Time `gorm:"column:use_time" json:"useTime,omitempty"`
	RefundTime *time.Time `gorm:"column:refund_time" json:"refundTime,omitempty"`
	UpdateTime time.Time  `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

func (VoucherOrder) TableName() string { return "tb_voucher_order" }

// Sign 用户签到记录表。
// 说明：本项目签到主链路走的是 Redis BitMap（见 service/user.go），
// 这张表保留用于「补签 / 明细备份 / 对账」，与教程的 tb_sign 一致。
type Sign struct {
	ID       uint64     `gorm:"column:id;primaryKey" json:"id,string"`
	UserID   uint64     `gorm:"column:user_id" json:"userId,string"`
	Year     int        `gorm:"column:year" json:"year"`
	Month    int        `gorm:"column:month" json:"month"`
	Date     *time.Time `gorm:"column:date" json:"date,omitempty"`
	IsBackup int        `gorm:"column:is_backup" json:"isBackup"`
}

func (Sign) TableName() string { return "tb_sign" }

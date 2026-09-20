package service

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
)

type VoucherService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewVoucherService(db *gorm.DB, rdb *redis.Client) *VoucherService {
	return &VoucherService{db: db, rdb: rdb}
}

// QueryOfShop 查询商铺所有的券
func (s *VoucherService) QueryOfShop(ctx context.Context, shopID uint64) ([]model.Voucher, error) {
	var vouchers []model.Voucher
	err := s.db.WithContext(ctx).Table("tb_voucher AS v").
		Select("v.*, sv.stock, sv.begin_time, sv.end_time").
		Joins("LEFT JOIN tb_seckill_voucher AS sv ON v.id = sv.voucher_id").
		Where("v.shop_id = ? AND v.status = 1", shopID).
		Scan(&vouchers).Error //这条查询的每一块都是手写的,没有绑定model,所以用Scan写进model而不是Find
	//左外查询:查询左表所有数据,以及两表交集部分数据
	//SELECT v.*,sv.stock,sv.begin_time,sv.end_time FROM tb_voucher AS v LEFT JOIN tb_seckill_voucher AS sv ON v.id=sv.voucher_id WHERE v.shop_id= ?(shopID) AND v.status=1(上架)
	return vouchers, err
}

// Add 添加普通券
func (s *VoucherService) Add(ctx context.Context, voucher *model.Voucher) error {
	return s.db.WithContext(ctx).Create(voucher).Error
}

// AddSeckill 添加秒杀券
func (s *VoucherService) AddSeckill(ctx context.Context, voucher *model.Voucher) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(voucher).Error; err != nil {
			return err
		}
		stock := 0
		if voucher.Stock != nil {
			stock = *voucher.Stock
		}
		seckill := model.SeckillVoucher{VoucherID: voucher.ID, Stock: stock}
		if voucher.BeginTime != nil {
			seckill.BeginTime = *voucher.BeginTime
		}
		if voucher.EndTime != nil {
			seckill.EndTime = *voucher.EndTime
		}
		if err := tx.Create(&seckill).Error; err != nil {
			return err
		}
		return s.rdb.Set(ctx, constants.SeckillStock+strconv.FormatUint(voucher.ID, 10), stock, 0).Err() //ttl=0->永久(当下架时才删key)
	})
}

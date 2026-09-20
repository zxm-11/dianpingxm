package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"hm-dianping/internal/constants"
	"hm-dianping/internal/model"
	"hm-dianping/internal/script"
)

var (
	ErrSeckillStockNotEnough = errors.New("库存不足")
	ErrSeckillDuplicateOrder = errors.New("你已经抢过了")
)

type VoucherOrderService struct {
	db       *gorm.DB
	rdb      *redis.Client
	writer   *kafka.Writer
	idWorker *RedisIDWorker
}

func NewVoucherOrderService(db *gorm.DB, rdb *redis.Client, writer *kafka.Writer) *VoucherOrderService {
	return &VoucherOrderService{db: db, rdb: rdb, writer: writer, idWorker: NewRedisIDWorker(rdb)}
}

// OrderSeckill 实现秒杀下单
func (s *VoucherOrderService) OrderSeckill(ctx context.Context, voucherID, userID uint64) (uint64, error) {
	orderID, err := s.idWorker.NextID(ctx, "order") //获取全局订单id
	if err != nil {
		return 0, err
	}
	//lua脚本 把判断和扣减压进一次原子操作
	result, err := s.rdb.Eval(ctx, script.SeckillLua, []string{}, voucherID, userID, orderID).Int()
	if err != nil {
		return 0, err
	}
	if result == 1 {
		return 0, ErrSeckillStockNotEnough
	}
	if result == 2 {
		return 0, ErrSeckillDuplicateOrder
	}

	order := model.VoucherOrder{ID: orderID, VoucherID: voucherID, UserID: userID, Status: 1}
	if s.writer == nil {
		// 同步落库
		if err := s.CreateVoucherOrder(ctx, &order); err != nil {
			s.rollbackRedis(ctx, voucherID, userID)
			return 0, err
		}
		return orderID, nil
	}
	// 异步落库：发 Kafka 消息
	body, err := json.Marshal(order)
	if err != nil {
		s.rollbackRedis(ctx, voucherID, userID)
		return 0, err
	}
	if err := s.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(strconv.FormatUint(userID, 10)),
		Value: body,
		Time:  time.Now(),
	}); err != nil {
		s.rollbackRedis(ctx, voucherID, userID)
		return 0, err
	}
	return orderID, nil
}

// StartConsumer 启动 Kafka 消费者（指数退避重试）
func (s *VoucherOrderService) StartConsumer(ctx context.Context, reader *kafka.Reader) {
	go func() {
		const (
			initialBackoff = 100 * time.Millisecond //初始化出错，等 100ms 再试
			maxBackoff     = 5 * time.Second        //等待的上限,最多等5s
		)
		backoff := initialBackoff //等待时间

		for {
			msg, err := reader.FetchMessage(ctx) //取消息
			if err != nil {
				if ctx.Err() != nil {
					return
				} //ctx出错->服务关闭
				log.Printf("fetch voucher order message: %v (retry in %v)", err, backoff)
				time.Sleep(backoff)
				backoff *= 2 //设置下次等待时间
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			backoff = initialBackoff // 重置退避

			var order model.VoucherOrder
			if err := json.Unmarshal(msg.Value, &order); err != nil {
				log.Printf("decode voucher order message: %v", err)
				_ = reader.CommitMessages(ctx, msg)
				continue
			}
			if err := s.CreateVoucherOrder(ctx, &order); err != nil {
				log.Printf("create voucher order %d: %v", order.ID, err)
				if errors.Is(err, ErrSeckillDuplicateOrder) || errors.Is(err, ErrSeckillStockNotEnough) {
					_ = reader.CommitMessages(ctx, msg)
				}
				continue
			}
			if err := reader.CommitMessages(ctx, msg); err != nil { //手动提交offset  Kafka通过Offset的方式确认消息/存档->服务器重启后从存档点继续,所以要当事务处理完之后才offset
				log.Printf("commit voucher order message: %v", err)
			}
		}
	}()
}

// CreateVoucherOrder 查看此用户是否下单/扣减秒杀券库存/写入订单记录
func (s *VoucherOrderService) CreateVoucherOrder(ctx context.Context, order *model.VoucherOrder) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		//查看此用户是否下单
		if err := tx.Model(&model.VoucherOrder{}).Where("user_id = ? AND voucher_id = ?", order.UserID, order.VoucherID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrSeckillDuplicateOrder
		}
		//扣减秒杀券库存
		res := tx.Model(&model.SeckillVoucher{}).
			Where("voucher_id = ? AND stock > 0", order.VoucherID).
			UpdateColumn("stock", gorm.Expr("stock - 1")) //stock>0+Update行级锁 兜底保证不会出现超卖问题(库存扣减到负数)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrSeckillStockNotEnough
		}
		//写入订单记录
		return tx.Create(order).Error
	})
}

// Mysql落库失败,回滚Redis
func (s *VoucherOrderService) rollbackRedis(ctx context.Context, voucherID, userID uint64) {
	rollbackCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stockKey := constants.SeckillStock + strconv.FormatUint(voucherID, 10)
	orderKey := "seckill:order:" + strconv.FormatUint(voucherID, 10)

	//回退券数量
	if err := s.rdb.Incr(rollbackCtx, stockKey).Err(); err != nil {
		log.Printf("rollback seckill stock %d: %v", voucherID, err)
	}
	//删除lua脚本里写的集合
	if err := s.rdb.SRem(rollbackCtx, orderKey, userID).Err(); err != nil {
		log.Printf("rollback seckill order set voucher=%d user=%d: %v", voucherID, userID, err)
	}
}

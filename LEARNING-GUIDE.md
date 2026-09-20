# hm-dianping 项目从 0 学习指南

> 一个类似「大众点评」的本地生活服务后端，用 **Go + Gin + GORM + Redis + Kafka** 实现。
> 本文档按「先建立全局认知 → 再逐层拆解 → 最后动手实验」的顺序组织，适合零基础学习本仓库。

---

## 0. 学习前必读：项目位置与 Go 模块

```
d:\learnproject\                 <- 外层（工作区根目录，只有 IDE 配置）
└── Lifestyle-Platform\          <- 真正的项目
    ├── go.mod                   <- 唯一的模块文件：module hm-dianping，go 1.25.10
    ├── cmd\server\main.go
    ├── config\config.yaml
    ├── internal\...
    ├── migrations\...
    └── seed\seed.sql
```

**注意**：整个工作区只有 `Lifestyle-Platform\go.mod` 这一个模块文件。所有命令都要先 `cd D:\learnproject\Lifestyle-Platform`。

---

## 1. 这个项目是干什么的（功能全景）

| 功能模块 | 具体能力 | 涉及中间件 |
|---|---|---|
| 用户 | 手机号 + 验证码登录/登出、查看个人信息 | Redis |
| 商铺 | 按 id 查、按类型/名称分页、新增、更新 | Redis 缓存 |
| 商铺类型 | 分类列表 | Redis 缓存 |
| 博客 | 发布、点赞/取消、热门、按用户查、点赞榜 | Redis ZSet |
| 关注 | 关注/取关、是否关注、共同关注 | Redis Set |
| Feed | 关注流推送 + 滚动分页 | Redis ZSet |
| 优惠券 | 普通券、秒杀券、店铺券列表 | MySQL |
| 秒杀下单 | Lua 原子扣库存 + 一人一单 + Kafka 异步落库 | Redis Lua + Kafka |
| 上传 | 探店图片上传/删除 | 本地文件系统 |

### 技术栈一句话版

- **Gin** —— HTTP 路由 + 中间件
- **GORM** —— MySQL 操作
- **go-redis/v9** —— Redis 客户端
- **kafka-go** —— 秒杀订单异步队列
- **Viper** —— YAML 配置 + 环境变量覆盖
- **golang-migrate** —— 数据库版本迁移
- **go:embed** —— 把 Lua 脚本编译进二进制
- **google/uuid** —— 生成 token / 文件名 / 锁 owner

---

## 2. 全局认知：目录结构与一次请求的完整链路

### 2.1 目录职责

```
cmd/server/          启动入口（只做三件事：加载配置 → 组装应用 → 启动等待信号）
internal/            私有业务代码（Go 规定：外部模块不能 import）
  app/               依赖组装（DB/Redis/Kafka/Service/Router 全部在这里 new 出来）
  config/            Viper 配置加载
  constants/         Redis Key 前缀、TTL、分页等常量（消灭魔数）
  ctx/               把「当前登录用户」存进/取出 Gin Context
  handler/           HTTP 处理器（闭包函数风格）
  middleware/        Auth / Recovery 中间件
  model/             GORM 数据模型（单源真相：一个结构体多种 tag）
  response/          统一 JSON 响应格式
  router/            路由注册（含 NoRoute 兼容旧路径）
  script/            go:embed 嵌入的 Lua 脚本
  service/           核心业务逻辑（事务、缓存、MQ 都在这层）
migrations/          golang-migrate 的 up/down SQL
config/              本地配置文件
seed/                种子数据（幂等 INSERT IGNORE）
docs/                教学文档
```

### 2.2 一次请求的完整链路（务必先记住这张图）

```
客户端
  → Gin Router
  → 全局中间件：gin.Logger → Recovery → CORS → Auth
  → Handler 闭包（解析参数 → 调 Service → writeOK/writeFail）
  → Service（事务 / 缓存 / Redis Lua / Kafka）
  → GORM(MySQL) / Redis / Kafka
  → 统一 JSON：{ success, data } 或 { success:false, errorMsg }
```

### 2.3 分层原则（本项目最重要的设计思想）

| 层 | 只负责 | 不负责 |
|---|---|---|
| Handler | 解析 HTTP 参数、调用 Service、写响应 | 业务规则、SQL |
| Service | 业务规则、事务、缓存、MQ | 感知 HTTP（不 import gin） |
| Model | 描述表结构（tag 映射） | 逻辑 |

> 验证点：`internal/service` 里没有任何一个文件 import `gin`，这就是分层的证据。

---

## 3. 分阶段学习路线（建议按顺序）

| 阶段 | 目标 | 重点文件 | 预计投入 |
|---|---|---|---|
| **Stage 0** | 把项目跑起来 | `config/config.yaml`、`main.go` | 0.5 天 |
| **Stage 1** | 看懂启动链路 | `cmd/server/main.go`、`internal/app/app.go`、`internal/config/config.go` | 0.5 天 |
| **Stage 2** | 看懂请求链路 | `internal/router`、`internal/middleware`、`internal/handler`、`internal/ctx` | 1 天 |
| **Stage 3** | 看懂数据层 | `internal/model`、`migrations/*.sql`、`internal/service/user.go` | 1 天 |
| **Stage 4** | 缓存与并发（核心） | `internal/service/shop.go`、`shop_type.go`、`internal/script/unlock.lua` | 1.5 天 |
| **Stage 5** | 秒杀异步链路（最核心） | `internal/service/voucher*.go`、`internal/script/seckill.lua`、`internal/service/redis_id.go` | 1.5 天 |
| **Stage 6** | 社交功能 | `internal/service/blog.go`、`follow.go` | 1 天 |
| **Stage 7** | 收尾与工程化 | `internal/service/upload.go`、`interfaces.go`、`go:embed` | 0.5 天 |

---

## 4. 逐模块精讲

### Stage 1：启动链路（入口只有三件事）

```go
// cmd/server/main.go
func main() {
	cfg, err := config.Load("config/config.yaml")   // 1. 加载配置
	if err != nil { log.Fatalf("load config: %v", err) }

	application, err := app.New(cfg)                // 2. 组装应用
	if err != nil { log.Fatalf("build app: %v", err) }
	defer application.Close()

	server := &http.Server{ Addr: ":" + strconv.Itoa(cfg.Server.Port), Handler: application.Router, ... }
	go func() { server.ListenAndServe() }()          // 3. 启动，并用 channel 等待 Ctrl+C
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	// 优雅关闭：10s 超时内关掉正在处理的请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
```

**要点**：
1. `main` 不写业务，只做「加载 → 组装 → 启动」。
2. `signal.Notify + server.Shutdown` 是 Go 服务优雅退出的标准写法。

**配置加载**（`internal/config/config.go`）：Viper 读取 YAML，并支持 `HMDP_` 前缀环境变量覆盖（`HMDP_MYSQL_DSN` → `mysql.dsn`）。
- 小坑：代码默认 `server.port = 8080`，但 `config/config.yaml` 写的是 `8081`，最终以配置文件为准。

**依赖组装**（`internal/app/app.go`）：这是全项目唯一「认识所有构件」的地方。

```go
// internal/app/app.go
func New(cfg *config.Config) (*App, error) {
	gormDB, _ := gorm.Open(mysql.Open(cfg.MySQL.DSN), ...)   // GORM + 连接池
	rdb := redis.NewClient(&redis.Options{...})              // Redis
	if err := rdb.Ping(context.Background()).Err(); err != nil { return nil, err }

	if cfg.Kafka.Enabled {                                   // Kafka 可选
		writer = &kafka.Writer{...}
		reader = kafka.NewReader(...)
	}
	// Service 各自独立构造，没有 Container
	userSvc := service.NewUserService(gormDB, rdb)
	shopSvc := service.NewShopService(gormDB, rdb)
	...
	blogSvc := service.NewBlogService(gormDB, rdb, userSvc)  // 显式把 UserService 传给 BlogService
	if reader != nil { voucherOrderSvc.StartConsumer(consumerCtx, reader) } // 启动 Kafka 消费者
	engine := router.New(cfg, rdb, userSvc, shopSvc, ...)    // 直接把 Service 传进路由
	return &App{Router: engine, ...}, nil
}
```

**要点**：Go 不需要「上帝容器」，每个 Service 只拿自己需要的依赖；`app.Close()` 统一释放 Kafka/Redis/DB。
**注意**：kafka-go 是**懒连接**——Kafka 没起来服务照样能启动，只有秒杀异步链路不可用。

---

### Stage 2：请求链路（中间件 → Handler → 统一响应）

#### 2.1 路由与中间件顺序

```go
// internal/router/router.go
r := gin.New()
r.Use(gin.Logger(), middleware.Recovery())     // 日志 + panic 恢复
r.Use(cors.New(cors.Config{...}))              // 跨域
r.Use(middleware.Auth(cfg, rdb))               // 鉴权（最后执行，能拿到完整路径）
```

**NoRoute 兜底**：Gin 路由树不允许 `/shop/:id` 和 `/shop/of/type` 同时注册，所以旧格式路径放到 `NoRoute` 手动分发（`GET /shop/1`、`GET /blog/1`、`PUT /follow/1/true`）。

#### 2.2 鉴权中间件（Auth）

```go
// internal/middleware/auth.go
func Auth(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" { path = c.Request.URL.Path }
		if isPublic(c.Request.Method, path) {     // 白名单：直接放行（但仍尝试解析可选用户）
			loadOptionalUser(c, rdb)
			c.Next(); return
		}
		token := strings.TrimSpace(c.GetHeader("authorization"))
		if token == "" { /* 未登录 → 返回失败，或按配置放行 */ }
		if !loadUserByToken(c, rdb, token)       // 从 Redis Hash 读用户
			{ /* 401 登录状态已失效 */ }
		c.Next()
	}
}
```

**要点**：
- 用户会话存在 **Redis Hash** `login:token:{token}`，三个字段 `id / nickName / icon`。
- 请求头沿用旧前端的 `authorization`（不是标准的 `Authorization`，两者都做了兼容）。
- 白名单函数 `isPublic` 支持 `/shop*`、`/shop-type*`、GET `/voucher*`、`/upload*`、部分 `/blog*`。
- 每次访问都会 `Expire` 续期（滑动过期）。

#### 2.3 当前用户上下文

```go
// internal/ctx/user.go
func SaveUser(c *gin.Context, user model.UserView) { c.Set(userKey, user) }

func CurrentUser(c *gin.Context) (model.UserView, error) {
	value, ok := c.Get(userKey)
	if !ok { return model.UserView{}, errors.New("用户未登录") }
	user, ok := value.(model.UserView)
	if !ok || user.ID == 0 { return model.UserView{}, errors.New("用户未登录") }
	return user, nil
}
```

这是 Java 版 `UserHolder(ThreadLocal)` 的 Go 等价物——但 Go 不用 ThreadLocal，而是把用户挂在 `gin.Context` 上，随请求传递。

#### 2.4 Handler 是「闭包函数」（本项目最重要的风格）

```go
// internal/handler/user.go
func HandleUserSendCode(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sendCodeRequest
		if err := c.ShouldBind(&req); err != nil {
			req.Phone = c.PostForm("phone")     // 兼容 JSON 与 form-data
			if req.Phone == "" { req.Phone = c.Query("phone") }
		}
		code, err := svc.SendCode(c.Request.Context(), req.Phone)
		if err != nil { writeFail(c, err); return }
		writeOK(c, code)
	}
}
```

**为什么闭包好**：不需要 Handler 结构体、不需要 Handler Container；路由注册时一眼能看出「这个接口依赖哪个 Service」；测试时传入 mock 即可。

**参数解析小工具**（`internal/handler/parse.go`）：`pathUint` / `queryUint` / `queryInt` / `viewerID`（未登录返回 0）。

#### 2.5 统一响应

```go
// internal/response/result.go
type Result struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"errorMsg,omitempty"`
	Data     any    `json:"data,omitempty"`
	Total    *int64 `json:"total,omitempty"`
}
func OK(data ...any) Result { ... }
func OKList(data any, total int64) Result { ... }
func Fail(message string) Result { ... }
```

Handler 侧用 `writeOK/writeFail` 包装（`internal/handler/helpers.go`），全项目响应格式统一。

---

### Stage 3：数据层（模型 + 迁移 + 登录业务）

#### 3.1 「单源真相」——一个结构体多种 tag

```go
// internal/model/models.go
type User struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id,string"`
	Phone      string    `gorm:"column:phone" json:"phone"`
	Password   string    `gorm:"column:password" json:"password,omitempty"`
	NickName   string    `gorm:"column:nick_name" json:"nickName"`
	Icon       string    `gorm:"column:icon" json:"icon"`
	CreateTime time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time" json:"updateTime"`
}
func (User) TableName() string { return "tb_user" }
```

**要点**：
- `gorm:"..."` 服务数据库映射，`json:"..."` 服务 API 序列化，一个结构体同时满足两端。
- `json:"id,string"`：把 `uint64` 的 id 序列化成**字符串**，避免 JS 大数精度丢失（前端常见坑）。
- `json:"password,omitempty"` 而不是 `json:"-"`：写入时带密码，读取时零值被忽略。
- `gorm:"-"` 表示「不是数据库字段」，如 `Blog.Name/Icon/IsLike`、`Shop.Distance`，只用于返回给前端的补充信息。
- `gorm:"column:stock;->"` 表示**只读字段**（`Voucher` 从 join 查询里读，不写库）。

**公开视图** `UserView`（同包内轻量结构体），避免为了对外展示再建一个 DTO 包。

#### 3.2 数据库迁移

表结构由 `migrations/000001_init_schema.up.sql` 管理（10 张表：用户、用户信息、商铺、类型、博客、评论、关注、优惠券、秒杀券、订单）。
**为什么不用 AutoMigrate**：迁移可版本化、可回滚、能删列，适合生产。

#### 3.3 登录业务（含一个被修复的 bug）

```go
// internal/service/user.go
func (s *UserService) Login(ctx context.Context, phone, code string) (string, error) {
	if !phonePattern.MatchString(phone) { return "", errors.New("手机号格式错误") }
	cacheCode, err := s.rdb.Get(ctx, constants.LoginCodeKey+phone).Result()
	if err == redis.Nil || cacheCode == "" { return "", errors.New("验证码不存在或已过期") }
	if cacheCode != code { return "", errors.New("验证码错误") }   // ← Java 版漏了这一步比较
	// 查用户，不存在则创建
	var user model.User
	if err := s.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) { return "", err }
		user = model.User{Phone: phone, NickName: constants.UserNickPrefx + uuid.NewString()[:12]}
		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil { return "", err }
	}
	token := uuid.NewString()
	key := constants.LoginUserKey + token
	s.rdb.HSet(ctx, key, map[string]any{"id": ..., "nickName": ..., "icon": ...})
	s.rdb.Expire(ctx, key, constants.LoginUserTTL)
	s.rdb.Del(ctx, constants.LoginCodeKey+phone)   // 用完即删验证码
	return token, nil
}
```

**学习点**：`errors.Is(err, gorm.ErrRecordNotFound)` 判断「记录不存在」，而不是判断字符串——这是 Go 错误处理的标准姿势。

---

### Stage 4：缓存与并发（缓存穿透 + 互斥锁 + Lua 解锁）

这是本项目**最能体现工程功底**的一段。目标：查商铺时先走 Redis，未命中再查库，并防止「缓存穿透」和「缓存击穿」。

```go
// internal/service/shop.go
func (s *ShopService) GetByID(ctx context.Context, id uint64) (*model.Shop, error) {
	key := constants.CacheShopKey + strconv.FormatUint(id, 10)

	cached, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		if cached == "" { return nil, nil }        // 空值命中 → 直接返回，防穿透
		var shop model.Shop
		if err := json.Unmarshal([]byte(cached), &shop); err == nil { return &shop, nil }
	}
	if err != nil && err != redis.Nil { return nil, err }

	lockKey := constants.LockShopKey + strconv.FormatUint(id, 10)
	owner := uuid.NewString()
	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {              // 循环退避（不是递归！）
		locked, err := s.rdb.SetNX(ctx, lockKey, owner, constants.LockShopTTL).Result()
		if err != nil { return nil, err }
		if locked { break }
		time.Sleep(time.Duration(50+(i*25)) * time.Millisecond)
	}

	defer s.rdb.Eval(context.Background(), script.UnlockLua, []string{lockKey}, owner)  // 用 Background 释放锁

	var shop model.Shop
	if err := s.db.WithContext(ctx).First(&shop, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.rdb.Set(ctx, key, "", constants.CacheNullTTL).Err()   // 击穿空值缓存
			return nil, nil
		}
		return nil, err
	}
	bytes, _ := json.Marshal(shop)
	_ = s.rdb.Set(ctx, key, bytes, constants.CacheShopTTL).Err()
	return &shop, nil
}
```

**四个关键设计（对比 Java 版，都是刻意改进）**：
1. **空值缓存**：查不到的 id 往 Redis 写空串（TTL 2 分钟）→ 防缓存穿透。
2. **互斥锁**：`SET NX EX` 抢锁，只有一个 goroutine 去查库 → 防缓存击穿。
3. **循环退避代替递归**：Java 版用递归重试，有栈溢出风险；这里用 `for + sleep` 递增退避。
4. **`context.Background()` 释放锁**：如果用请求的 ctx，用户一断开连接 ctx 就被取消，锁就删不掉了（锁泄漏）。改用独立 ctx 清锁。

**安全解锁 Lua**（只删自己的锁）：

```lua
-- internal/script/unlock.lua
if (redis.call('GET', KEYS[1]) == ARGV[1]) then
    return redis.call('DEL', KEYS[1])
end
return 0
```

> 深入思考（留给你）：如果 5 次都没抢到锁，代码会直接往下走去查库——这算不算「降级」？有什么风险？可以自己想清楚。

**更新时删缓存**（Cache-Aside 模式）：

```go
// internal/service/shop.go
func (s *ShopService) Update(ctx context.Context, shop *model.Shop) error {
	if shop.ID == 0 { return errors.New("店铺id不能为空") }
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Shop{}).Where("id = ?", shop.ID).Updates(shop).Error; err != nil { return err }
		return s.rdb.Del(ctx, constants.CacheShopKey+strconv.FormatUint(shop.ID, 10)).Err()
	})
}
```

**商铺类型缓存**（`shop_type.go`）：`cache:shop:type:list`，加 24h TTL 防止永久脏数据。

---

### Stage 5：秒杀异步链路（全项目最核心）

秒杀要同时解决三件事：**超卖**、**一人一单**、**高性能**。本项目的方案是
「**Redis Lua 原子预扣 → Kafka 异步落库**」。

#### 5.1 全局唯一 ID 生成器（时间戳 + 自增序列）

```go
// internal/service/redis_id.go
func (w *RedisIDWorker) NextID(ctx context.Context, prefix string) (uint64, error) {
	now := time.Now().UTC()
	seconds := now.Unix() - constants.BeginTimestamp          // 相对起始时间
	key := fmt.Sprintf("icr:%s:%s", prefix, now.Format("2006:01:02"))
	seq, err := w.rdb.Incr(ctx, key).Result()                 // Redis 自增序列
	if err != nil { return 0, err }
	return uint64(seconds)<<constants.CountBits | uint64(seq), nil  // 高位时间 + 低位序列
}
```

**原理**：高 32 位放「秒数」，低 32 位放「当天的 Redis 自增计数」→ 全局唯一、趋势递增、可按天统计。这是分布式 ID 的经典做法。

#### 5.2 seckill.lua（原子判断库存 + 一人一单）

```lua
-- internal/script/seckill.lua
local voucherId = ARGV[1]
local userId = ARGV[2]
local orderId = ARGV[3]

local stockKey = "seckill:stock:" .. voucherId
local orderKey = "seckill:order:" .. voucherId
local stock = redis.call('get', stockKey)

if (not stock or tonumber(stock) <= 0) then return 1 end          -- 1 = 库存不足
if (redis.call('sismember', orderKey, userId) == 1) then return 2 end -- 2 = 重复下单
redis.call('incrby', stockKey, -1)                                -- 扣库存
redis.call('sadd', orderKey, userId)                              -- 记录该用户已抢
return 0                                                          -- 0 = 成功
```

**为什么必须用 Lua**：Redis 单线程执行 Lua，整个「查库存 → 查是否下过单 → 扣减 → 记录」是一段**原子**操作，天然避免并发超卖。
**数据结构**：库存用 String，用户集合用 Set（`sismember` O(1) 判重）。
> 彩蛋：`orderId` 作为 ARGV[3] 传进来了，但脚本里其实没用上——可以思考「为什么不需要」，或要不要删掉。

#### 5.3 下单入口（先 Redis，再决定同步 or 异步）

```go
// internal/service/voucher_order.go
func (s *VoucherOrderService) OrderSeckill(ctx context.Context, voucherID, userID uint64) (uint64, error) {
	orderID, err := s.idWorker.NextID(ctx, "order")            // 先造订单号
	if err != nil { return 0, err }

	result, err := s.rdb.Eval(ctx, script.SeckillLua, []string{}, voucherID, userID, orderID).Int()
	if err != nil { return 0, err }
	if result == 1 { return 0, ErrSeckillStockNotEnough }
	if result == 2 { return 0, ErrSeckillDuplicateOrder }

	order := model.VoucherOrder{ID: orderID, VoucherID: voucherID, UserID: userID, Status: 1}
	if s.writer == nil {                                       // Kafka 未启用 → 同步落库
		if err := s.CreateVoucherOrder(ctx, &order); err != nil {
			s.rollbackRedis(ctx, voucherID, userID); return 0, err
		}
		return orderID, nil
	}
	body, _ := json.Marshal(order)
	if err := s.writer.WriteMessages(ctx, kafka.Message{...}); err != nil {   // 异步落库
		s.rollbackRedis(ctx, voucherID, userID); return 0, err
	}
	return orderID, nil                                        // 立即返回订单号
}
```

#### 5.4 Kafka 消费者（指数退避）

```go
// internal/service/voucher_order.go
func (s *VoucherOrderService) StartConsumer(ctx context.Context, reader *kafka.Reader) {
	go func() {
		const (initialBackoff = 100*time.Millisecond; maxBackoff = 5*time.Second)
		backoff := initialBackoff
		for {
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil { return }             // 主动取消 → 退出
				time.Sleep(backoff); backoff *= 2          // 指数退避，避免烧 CPU
				if backoff > maxBackoff { backoff = maxBackoff }
				continue
			}
			backoff = initialBackoff                       // 成功则重置
			var order model.VoucherOrder
			if err := json.Unmarshal(msg.Value, &order); err != nil { reader.CommitMessages(ctx, msg); continue }
			if err := s.CreateVoucherOrder(ctx, &order); err != nil {
				log.Printf(...)
				if errors.Is(err, ErrSeckillDuplicateOrder) || errors.Is(err, ErrSeckillStockNotEnough) {
					_ = reader.CommitMessages(ctx, msg)    // 永久业务错误 → 提交，不再重试
				}
				continue
			}
			reader.CommitMessages(ctx, msg)
		}
	}()
}
```

**学习点**：
- **指数退避**：Kafka 不可用时 `100ms → 200ms → ... → 5s`，不会全速空转。
- **区分「可重试」和「永久错误」**：一人一单/库存不足是业务终态，必须提交消息，否则会无限重投。

#### 5.5 落库（DB 二次校验 + 乐观扣减）

```go
// internal/service/voucher_order.go
func (s *VoucherOrderService) CreateVoucherOrder(ctx context.Context, order *model.VoucherOrder) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&model.VoucherOrder{}).Where("user_id = ? AND voucher_id = ?", order.UserID, order.VoucherID).Count(&count)
		if count > 0 { return ErrSeckillDuplicateOrder }           // DB 层再查一人一单

		res := tx.Model(&model.SeckillVoucher{}).
			Where("voucher_id = ? AND stock > 0", order.VoucherID).
			UpdateColumn("stock", gorm.Expr("stock - 1"))          // 条件更新，天然乐观锁
		if res.RowsAffected == 0 { return ErrSeckillStockNotEnough }
		return tx.Create(order).Error
	})
}
```

**为什么要双重校验**：Redis 保证高并发下的正确性，但 Redis 可能丢数据/不一致；DB 事务是最终可靠兜底。
**`Where("stock > 0").UpdateColumn("stock - 1")`**：把判断和扣减合成一条 SQL，避免「先查后改」的竞态。

#### 5.6 失败回滚（保持 Redis 与 DB 最终一致）

```go
// internal/service/voucher_order.go
func (s *VoucherOrderService) rollbackRedis(ctx context.Context, voucherID, userID uint64) {
	rollbackCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stockKey := constants.SeckillStock + strconv.FormatUint(voucherID, 10)
	orderKey := "seckill:order:" + strconv.FormatUint(voucherID, 10)
	s.rdb.Incr(rollbackCtx, stockKey)         // 库存加回来
	s.rdb.SRem(rollbackCtx, orderKey, userID) // 移除「已抢」标记
}
```

**要点**：回滚用**独立的 context + 3s 超时**，保证即使原请求已取消也能完成补偿。

#### 5.7 秒杀券的创建（事务中同时写 Redis 库存）

`VoucherService.AddSeckill`：在一个事务里同时写 `tb_voucher` + `tb_seckill_voucher`，并 `SET seckill:stock:{id}`。注意这里 Redis 写在事务内——如果事务最后失败，Redis 库存会残留，是**可优化点**（可以用 after-commit 回调）。

---

### Stage 6：社交功能（博客 / 点赞 / 关注 / Feed）

#### 6.1 点赞：MySQL 计数 + Redis ZSet 存「谁点了」

```go
// internal/service/blog.go
func (s *BlogService) Like(ctx context.Context, blogID uint64, userID uint64) error {
	key := constants.BlogLikedKey + strconv.FormatUint(blogID, 10)   // blog:liked:{id}
	member := strconv.FormatUint(userID, 10)
	_, err := s.rdb.ZScore(ctx, key, member).Result()
	isLiked := err == nil                                            // 用 ZScore 判断是否已点赞
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !isLiked {
			tx.Model(&model.Blog{}).Where("id = ?", blogID).UpdateColumn("liked", gorm.Expr("liked + 1"))
			return s.rdb.ZAdd(ctx, key, redis.Z{Score: float64(time.Now().UnixMilli()), Member: member}).Err()
		}
		tx.Model(&model.Blog{}).Where("id = ?", blogID).UpdateColumn("liked", gorm.Expr("liked - 1"))
		return s.rdb.ZRem(ctx, key, member).Err()
	})
}
```

**设计**：`liked` 字段做**计数**（列表页直接读，快），ZSet 存**明细**（谁点的、什么时候，用于「点赞榜」和 `isLike` 判断）。ZSet 的 score 是毫秒时间戳 → 天然按时间排序。
`QueryLikes` 用 `ZRange 0 4` 取前 5 个点赞用户，再批量查用户信息（避免 N+1）。

#### 6.2 Feed 推送：写扩散（推模式）

```go
// internal/service/blog.go
func (s *BlogService) Save(ctx context.Context, blog *model.Blog, userID uint64) error {
	blog.UserID = userID
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Create(blog)
		var follows []model.Follow
		tx.Where("follow_user_id = ?", userID).Find(&follows)      // 找出作者的粉丝
		score := float64(time.Now().UnixMilli())
		for _, follow := range follows {
			key := constants.FeedKey + strconv.FormatUint(follow.UserID, 10)  // feed:{粉丝id}
			s.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: blog.ID})      // 推进粉丝收件箱
		}
		return nil
	})
}
```

**读扩散 vs 写扩散**：本项目用**写扩散（推模式）**——发布时就把 blogId 塞进每个粉丝的收件箱 ZSet，粉丝读取时直接 `ZRevRangeByScore`，读性能极高。
**代价**：大 V 发一条要写很多粉丝，且只对「已有粉丝」生效（新粉丝看不到历史）。

#### 6.3 滚动分页（Feed 专用，比 offset 分页更适合时间流）

`QueryFeed` 用 `ZRevRangeByScore(min, max, offset, count)`，返回 `{list, minTime, offset}`：
- `minTime`：本页最小（最旧）的时间戳，下一页把它作为 `lastId` 传入。
- `offset`：处理**同一毫秒有多条**的边界，告诉下一页要跳过几条。
> 这是典型的「不用 offset 也能分页」——避免列表插入/删除导致的分页错乱。

#### 6.4 关注 + 共同关注

```go
// internal/service/follow.go
func (s *FollowService) Follow(ctx context.Context, userID, followUserID uint64, isFollow bool) error {
	key := constants.FollowKey + strconv.FormatUint(userID, 10)   // follows:{userId}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isFollow {
			follow := model.Follow{UserID: userID, FollowUserID: followUserID}
			tx.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).FirstOrCreate(&follow) // 幂等
			return s.rdb.SAdd(ctx, key, followUserID).Err()
		}
		tx.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).Delete(&model.Follow{})
		return s.rdb.SRem(ctx, key, followUserID).Err()
	})
}
```

**共同关注**用 `SINTER`（Redis 集合求交集）直接算出两人共同关注的人，再批量查用户信息：

```go
ids, err := s.rdb.SInter(ctx, constants.FollowKey+..., constants.FollowKey+...).Result()
```

**要点**：`FirstOrCreate` 保证重复关注幂等；MySQL 存关系、Redis Set 存「关注列表」用于快速取交集。

---

### Stage 7：上传与工程化技巧

#### 7.1 小接口 + 可替换实现

```go
// internal/service/interfaces.go
type FileStore interface {
	Save(file *multipart.FileHeader, target string) error
	Remove(path string) error
}
```

`UploadService` 默认持有 `&localFS{}`，并暴露 `SetFileStore()` 供测试注入 mock（`upload.go`）。这就是 Go「小接口、大组合」的体现——将来接 OSS 只需换实现，Handler 不变。

#### 7.2 路径穿越防护

```go
// internal/service/upload.go
func (s *UploadService) DeleteBlogImage(name string) error {
	relative := strings.TrimPrefix(name, s.cfg.Upload.PublicPrefix)
	relative = strings.TrimLeft(relative, "\\/")
	clean := filepath.Clean(relative)
	if clean == "." || filepath.IsAbs(clean) || strings.Contains(clean, "..") {
		return errors.New("非法文件路径")     // 阻止 ../../etc/passwd 这类攻击
	}
	return s.store.Remove(filepath.Join(s.cfg.Upload.ImageDir, clean))
}
```

#### 7.3 go:embed 编译期嵌入 Lua

```go
// internal/script/scripts.go
import _ "embed"
//go:embed seckill.lua
var SeckillLua string
//go:embed unlock.lua
var UnlockLua string
```

好处：Lua 脚本随二进制一起分发，无需额外文件，单二进制部署。

#### 7.4 panic 恢复中间件

```go
// internal/middleware/recovery.go
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("panic: %v", recovered)
		c.JSON(http.StatusOK, response.Fail("服务器异常"))
	})
}
```

捕获 panic，避免单个请求让整个服务崩掉，同时保持统一响应格式。

---

## 5. Redis Key 速查表（背下来就掌握了半壁江山）

| Key 模式 | 结构 | 用途 | TTL |
|---|---|---|---|
| `login:code:{phone}` | String | 登录验证码 | 2 分钟 |
| `login:token:{token}` | Hash | 登录会话（id/nickName/icon） | 10 小时（滑动续期） |
| `cache:shop:{id}` | String(JSON) | 商铺缓存 | 30 分钟 |
| `cache:shop:{id}` = `""` | String | 空值缓存（防穿透） | 2 分钟 |
| `cache:shop:type:list` | String(JSON) | 商铺类型列表 | 24 小时 |
| `lock:shop:{id}` | String | 商铺互斥锁 | 10 秒 |
| `seckill:stock:{voucherId}` | String | 秒杀库存 | 永久 |
| `seckill:order:{voucherId}` | Set | 已抢该券的用户 | 永久 |
| `blog:liked:{blogId}` | ZSet | 点赞用户（score=时间） | 无 |
| `follows:{userId}` | Set | 我关注的人 | 无 |
| `feed:{userId}` | ZSet | 收件箱（blogId, score=时间） | 无 |
| `icr:{prefix}:{yyyy:MM:dd}` | String | ID 自增计数 | 无 |

---

## 6. 动手实验清单（强烈建议逐个做）

### 实验一：把项目跑起来

```powershell
cd D:\learnproject\Lifestyle-Platform

# 1. 确认依赖（MySQL 3306、Redis 6379）
netstat -an | Select-String "3306|6379"

# 2. 建库 + 迁移
mysql -uroot -p126070mxZ -e "CREATE DATABASE IF NOT EXISTS dianping DEFAULT CHARACTER SET utf8mb4;"
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" up

# 3. 灌种子数据
mysql --default-character-set=utf8mb4 dianping < seed/seed.sql

# 4. 预热秒杀库存（注意 -n 1！应用连的是 db 1）
redis-cli -n 1 SET seckill:stock:10 100

# 5. 运行
go run ./cmd/server
```

### 实验二：逐个接口打通

```powershell
curl "http://localhost:8081/shop-type/list"      # 商铺类型
curl "http://localhost:8081/shop/1"              # 商铺详情（触发缓存）
curl -X POST "http://localhost:8081/user/code" -d "phone=13800138000"
# ↑ 响应 data 里就是验证码，复制它
curl -X POST "http://localhost:8081/user/login" -d "phone=13800138000&code=<验证码>"
# ↑ 响应 data 是 token，后续带上 header: authorization: <token>
```

### 实验三：用 redis-cli 观察缓存行为

```powershell
redis-cli -n 1
> KEYS cache:shop:*
> GET cache:shop:1          # 第一次 curl 后应看到 JSON
> TTL cache:shop:1
> GET cache:shop:9999       # 不存在的 id → 返回空串（空值缓存）
> KEYS login:token:*
> HGETALL login:token:<你的token>
```

**思考题**：连续两次 `curl /shop/1`，第二次为什么不会查数据库？把 `GET cache:shop:1` 的内容和 GORM 日志对照一下。

### 实验四：观察秒杀链路

```powershell
# 先登录拿 token，然后秒杀 voucherId=10
curl -X POST "http://localhost:8081/voucher-order/seckill/10" -H "authorization: <token>"
# 再点一次 → 应该返回「你已经抢过了」
redis-cli -n 1 GET seckill:stock:10
redis-cli -n 1 SMEMBERS seckill:order:10
```

### 实验五：改代码（巩固理解）

1. 把 `constants.CacheShopTTL` 改成 `10*time.Second`，观察缓存过期后重新查库。
2. 把 `shop.go` 的 `owner := uuid.NewString()` 改成固定值，体会 unlock.lua 为什么需要校验 owner。
3. 把 `Login` 里 `if cacheCode != code` 那行注释掉，体会「Java 版 bug」的后果。
4. 把 `config.yaml` 的 `kafka.enabled` 改成 `false`，观察秒杀走同步落库路径。

---

## 7. 自查清单（能答上来就说明学懂了）

- [ ] 为什么 Go 项目用 `internal/` 目录？（外部模块不能 import）
- [ ] `main.go` 为什么只做三件事？`app.New()` 的作用是什么？
- [ ] 为什么 Handler 用闭包而不是 struct + method？
- [ ] `json:"id,string"` 解决什么问题？（JS 大整数精度）
- [ ] 缓存穿透 / 缓存击穿 / 缓存雪崩分别是什么？本项目怎么处理的？
- [ ] 为什么释放 Redis 锁要用 `context.Background()` 而不是请求的 ctx？
- [ ] 安全解锁为什么必须用 Lua 而不用 GET + DEL 两步？
- [ ] seckill.lua 返回 1 / 2 / 0 分别代表什么？
- [ ] 为什么秒杀要在 Redis 和 MySQL 都做「一人一单」校验？
- [ ] 消费者为什么要区分「可重试错误」和「永久业务错误」？
- [ ] ID 生成器为什么用「高 32 位时间 + 低 32 位序列」？
- [ ] 点赞为什么既存 MySQL 计数又存 Redis ZSet？
- [ ] Feed 用写扩散（推模式）有什么优缺点？
- [ ] 滚动分页里 `minTime` 和 `offset` 各解决什么问题？
- [ ] `FileStore` 接口存在的意义是什么？（可替换存储 + 可测试）

---

## 8. 进阶思考（学完之后挑战）

1. **一致性**：`AddSeckill` 在事务里写 Redis，事务失败 Redis 会残留库存 → 如何用「事务提交后回调」修复？
2. **秒杀可靠性**：接口返回订单号后，异步落库可能失败（Kafka 丢失/消费者崩溃）→ 如何用「本地消息表 + 定时补偿」保证最终一致？
3. **热点 key**：`seckill:stock:{id}` 是热点，单分片 Redis 可能扛不住 → 库存分片（把 100 拆成 10×10）怎么做？
4. **Feed 大 V 问题**：写扩散对千万粉丝的大 V 不可行 → 「推拉结合」怎么设计？
5. **可观测性**：目前只有标准库 log → 引入 `slog` 或 OpenTelemetry。
6. **文档漂移**：`docs/GO_ARCHITECTURE.md` 里提到的 `internal/dto` 实际并不存在（项目选择了「单源真相」），可以顺手修正文档。

---

## 9. 一句话总结

> 这个项目用 Go 的方式重写了一个 Spring Boot 的点评后端：**没有 Container、没有 DTO 泛滥、没有注解魔法**，
> 而是用「闭包 Handler + 构造函数注入 + 小接口 + 单源模型」把依赖和逻辑写得显式而清晰；
> 同时在**缓存穿透防护、互斥锁、Redis Lua 原子秒杀、Kafka 异步解耦**这些高并发场景上，给出了教科书级的实现。

# hm-dianping Go 后端项目——从零搭建教程

本教程带你从零开始搭建 `hm-dianping` 项目，一个类似大众点评的本地生活服务后端。
整个教程 **从 Go 的工程哲学出发**，不只是教你怎么写，更教你为什么这么写。

---

## 目录

1. [项目概览](#1-项目概览)
2. [环境准备](#2-环境准备)
3. [项目初始化](#3-项目初始化)
4. [配置模块](#4-配置模块)
5. [数据模型：单源真相](#5-数据模型单源真相)
6. [数据库迁移](#6-数据库迁移)
7. [统一响应格式](#7-统一响应格式)
8. [常量定义](#8-常量定义)
9. [中间件](#9-中间件)
10. [服务层](#10-服务层)
11. [处理器层：闭包 Handler](#11-处理器层闭包-handler)
12. [路由注册](#12-路由注册)
13. [依赖组装](#13-依赖组装)
14. [Lua 脚本](#14-lua-脚本)
15. [启动入口](#15-启动入口)
16. [运行项目](#16-运行项目)
17. [架构总结](#17-架构总结)

---

## 1. 项目概览

### 功能
- 用户注册登录（手机号 + 验证码）
- 商户展示与缓存（防缓存穿透）
- 探店博客发布、点赞、Feed 推送
- 用户关注与共同关注
- 优惠券秒杀（Redis Lua 原子扣减 + Kafka 异步落库）
- 图片上传

### 技术栈

`
| 技术 | 用途 |
|------|------|
| Go 1.25+ | 开发语言 |
| Gin | HTTP 路由、中间件 |
| GORM | MySQL ORM |
| go-redis | Redis 客户端 |
| kafka-go | Kafka 生产者/消费者 |
| Viper | YAML 配置 + 环境变量覆盖 |
| golang-migrate | 数据库版本迁移 |
| go:embed | 编译期嵌入 Lua 脚本 |
| google/uuid | 生成 token、文件名 |
`

### 架构风格

```
cmd/server/          # 启动入口（只做三件事：加载配置 → 组装应用 → 启动）
internal/            # 私有业务代码（外部不可 import）
  app/               # 依赖组装
  config/            # 配置加载
  constants/         # 全局常量
  ctx/               # 用户上下文
  handler/           # HTTP 处理器（闭包函数）
  middleware/        # 中间件
  model/             # 数据模型（单源真相）
  response/          # 统一响应格式
  router/            # 路由注册
  script/            # 嵌入的 Lua 脚本
  service/           # 核心业务逻辑（带接口）
migrations/          # 数据库迁移 SQL
config/              # 本地配置文件
```

### 请求流程

```
客户端
  → Gin Router
  → Middleware（Auth / Recovery / CORS）
  → Handler 闭包（参数解析 → 调用 Service → 统一响应）
  → Service（事务、缓存、消息队列）
  → GORM / Redis / Kafka
  → JSON 响应
```

---

## 2. 环境准备

### Go

```powershell
# 下载安装后验证
go version
# 确保 PATH 包含 Go bin
$env:Path += ";$env:USERPROFILE\go\bin"
```

### MySQL

本项目实际用的是 **Windows 原生 MySQL 8.0**（装在 `C:\Program Files\MySQL\MySQL Server 8.0`，已注册为 Windows 服务）。

库名必须是 `dianping`，要和 `config/config.yaml` 的 DSN 对齐，先建库：

```sql
CREATE DATABASE IF NOT EXISTS dianping DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

想用 Docker 也可以，但库名和密码要保持一致，否则应用连不上：

```powershell
docker run -d --name mysql-dianping -p 3306:3306 `
  -e MYSQL_ROOT_PASSWORD=126070mxZ `
  -e MYSQL_DATABASE=dianping `
  mysql:8.0 --character-set-server=utf8mb4 --collation-server=utf8mb4_general_ci
```

### Redis

本项目实际用的是 **Windows 原生 Redis**（`redis-server.exe`，已注册为 Windows 服务，监听 `127.0.0.1:6379`）。

⚠️ 应用连的是 **db 1**（见 `config/config.yaml` 的 `redis.db`），不是默认的 db 0。手动敲 `redis-cli` 时别漏掉 `-n 1`，否则会看到"库存没设进去"的假象。

```powershell
docker run -d --name redis-dianping -p 6379:6379 redis:7-alpine
```

### Kafka（可选，秒杀异步下单需要）

`config/config.yaml` 配的是 **3 个 broker**：

```yaml
kafka:
  brokers:
    - localhost:9094
    - localhost:9095
    - localhost:9096
  enabled: true
```

Kafka 是**可选**的。`app.New()` 只构造 kafka-go 的 Writer/Reader，不建立连接（kafka-go 是懒连接），所以 **Kafka 没起来服务照样能正常启动**，只有秒杀下单的异步链路不可用——消费者会在后台指数退避重试，日志里会出现连接报错。

只跑单节点 broker 的话，把端口对齐成 9094，并且把 `brokers` 删到只剩一条，否则 kafka-go 会一直去重试连不上的 9095 / 9096。完全不用 Kafka 就把 `enabled` 改成 `false`，此时秒杀会走**同步落库**路径（见 `internal/service/voucher_order.go` 的 `writer == nil` 分支）。

---

## 3. 项目初始化

### 3.1 创建目录

```powershell
mkdir cmd/server, config, docs, migrations
mkdir internal/app, internal/config, internal/constants
mkdir internal/ctx, internal/handler, internal/middleware
mkdir internal/migration, internal/model, internal/response
mkdir internal/router, internal/script, internal/service
```

### 3.2 初始化 Module 并安装依赖

```powershell
go mod init hm-dianping

go get github.com/gin-gonic/gin
go get github.com/gin-contrib/cors
go get gorm.io/gorm
go get gorm.io/driver/mysql
go get github.com/redis/go-redis/v9
go get github.com/segmentio/kafka-go
go get github.com/spf13/viper
go get github.com/golang-migrate/migrate/v4
go get github.com/google/uuid
```

### 3.3 .gitignore

```gitignore
bin/
dist/
build/
*.exe
vendor/
go.work
go.work.sum
config/*.local.yaml
.env
.idea/
.vscode/
```

---

## 4. 配置模块

配置是项目的基石。我们用 **Viper** 加载 YAML，并支持环境变量覆盖。

### config.yaml

```yaml
server:
  port: 8081
  mode: debug
  read_timeout: 10s
  write_timeout: 10s

mysql:
  dsn: root:126070mxZ@tcp(127.0.0.1:3306)/dianping?charset=utf8mb4&parseTime=True&loc=Local
  max_idle_conns: 10
  max_open_conns: 50

redis:
  addr: 127.0.0.1:6379
  password: ""
  db: 1

kafka:
  brokers:
    - localhost:9094
    - localhost:9095
    - localhost:9096
  topic: kafka-orders
  group_id: my-kafka-group
  enabled: true

upload:
  image_dir: 'D:\HeiMaDianPing\nginx-1.18.0\html\hmdp\imgs\'
  public_prefix: /imgs

auth:
  compatible_missing_token: false
```

### config.go

```go
package config

import (
    "strings"
    "time"
    "github.com/spf13/viper"
)

type Config struct {
    Server ServerConfig `mapstructure:"server"`
    MySQL  MySQLConfig  `mapstructure:"mysql"`
    Redis  RedisConfig  `mapstructure:"redis"`
    Kafka  KafkaConfig  `mapstructure:"kafka"`
    Upload UploadConfig `mapstructure:"upload"`
    Auth   AuthConfig   `mapstructure:"auth"`
}

func Load(path string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)
    v.SetEnvPrefix("HMDP")
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    v.AutomaticEnv()
    // 设默认值
    v.SetDefault("server.port", 8081)
    v.SetDefault("server.mode", "debug")
    // ...
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

**设计要点：**
- `mapstructure` 标签将 YAML 的 `snake_case` 映射到 Go 的 `PascalCase`
- `SetEnvPrefix("HMDP")` + `AutomaticEnv()` 支持 `HMDP_MYSQL_DSN` 环境变量覆盖
- 所有配置集中在 struct 中，编译期类型安全

---

## 5. 数据模型：单源真相

**Go 的哲学：一个模型，多种 tag。**

不同于 Java 的 Entity/VO/DTO 三层分离，Go 的结构体可以通过 tag 系统同时服务 ORM、JSON 序列化、Redis 序列化等需求。

```go
package model

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

当你需要一个**公开视图**（比如只返回 id、昵称、头像），Go 的做法是加一个轻量的 **View 类型**，放在同一个 `model` 包里：

```go
// UserView 用户公开信息视图
type UserView struct {
    ID       uint64 `json:"id,string"`
    NickName string `json:"nickName"`
    Icon     string `json:"icon"`
}
```

没有 `UserDTO`，没有 `toUserDTO()` 映射函数，没有单独一层 DTO 包。一个结构体不够用就加一个——别为它建一个新包。

**为什么不在 model 里用 json:"-" 隐藏字段？**
因为敏感字段（如 Password）在写入时需要序列化，读取时不需要。`omitempty` 比 `-` 更合适：写入时带值，读取时零值被忽略。

---

## 6. 数据库迁移

使用 `golang-migrate` 管理表结构版本，比 GORM 的 AutoMigrate 更可控。

```sql
-- migrations/000001_init_schema.up.sql
CREATE TABLE IF NOT EXISTS `tb_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `phone` varchar(11) NOT NULL,
  `password` varchar(128) DEFAULT '',
  `nick_name` varchar(32) DEFAULT '',
  `icon` varchar(255) DEFAULT '',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniqe_key_phone` (`phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
```

执行迁移：

```powershell
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" up
```

**为什么不用 GORM AutoMigrate？**
因为迁移应该是版本控制的、可回滚的、与代码分开管理的。AutoMigrate 只能加列不能减列，不适合生产环境。

---

## 7. 统一响应格式

```go
package response

type Result struct {
    Success  bool   `json:"success"`
    ErrorMsg string `json:"errorMsg,omitempty"`
    Data     any    `json:"data,omitempty"`
    Total    *int64 `json:"total,omitempty"`
}

func OK(data ...any) Result {
    if len(data) == 0 { return Result{Success: true} }
    return Result{Success: true, Data: data[0]}
}

func Fail(message string) Result {
    return Result{Success: false, ErrorMsg: message}
}
```

**为什么不叫 `dto`？**
`response` 包名描述的是"用途"（这是 HTTP 响应），而 `dto` 描述的是"模式"（数据传输对象）。Go 偏爱用途导向的命名。

---

## 8. 常量定义

Redis Key 前缀、TTL、分页参数等魔数集中管理：

```go
package constants

const (
    LoginCodeKey = "login:code:"      // 验证码
    LoginUserKey = "login:token:"     // 登录会话
    CacheShopKey = "cache:shop:"      // 商户缓存
    LockShopKey  = "lock:shop:"       // 互斥锁
    SeckillStock = "seckill:stock:"   // 秒杀库存
    BlogLikedKey = "blog:liked:"      // 博客点赞
    FollowKey    = "follows:"         // 关注列表
    FeedKey      = "feed:"            // 收件箱
)

const (
    LoginCodeTTL = 2 * time.Minute
    LoginUserTTL = 36000 * time.Second
    CacheShopTTL = 30 * time.Minute
    CacheNullTTL = 2 * time.Minute    // 空值缓存 TTL（防穿透）
    LockShopTTL  = 10 * time.Second
)
```

---

## 9. 中间件

### 鉴权（Auth）

用户登录状态存储在 Redis Hash `login:token:{token}` 中，三个字段：`id`、`nickName`、`icon`。

```go
func Auth(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        if isPublic(c.Request.Method, c.FullPath()) {
            loadOptionalUser(c, rdb)
            c.Next()
            return
        }
        token := strings.TrimSpace(c.GetHeader("authorization"))
        if token == "" { /* 返回 401 */ }

        var user model.UserView
        values, _ := rdb.HGetAll(c.Request.Context(), constants.LoginUserKey+token).Result()
        // 解析 id, nickName, icon ...
        userctx.SaveUser(c, user)
        c.Next()
    }
}
```

### HTTP 上下文（ctx）

```go
package ctx

func SaveUser(c *gin.Context, user model.UserView) {
    c.Set("currentUser", user)
}

func CurrentUser(c *gin.Context) (model.UserView, error) {
    value, ok := c.Get("currentUser")
    if !ok { return model.UserView{}, errors.New("用户未登录") }
    user := value.(model.UserView)
    if user.ID == 0 { return model.UserView{}, errors.New("用户未登录") }
    return user, nil
}
```

---

## 10. 服务层

**Go 的服务层设计原则：**
- 每个 Service 独立构造，没有中央 Container
- 依赖通过构造函数参数注入（显式、编译期可检查）
- 关键的横切关注点抽成接口（FileStore、IDGenerator）
- 避免递归、避免 mutable var hack

### 10.1 接口先行

```go
package service

// FileStore 文件存储接口——替代 var saveMultipartFile hack
type FileStore interface {
    Save(file *multipart.FileHeader, target string) error
    Remove(path string) error
}

// IDGenerator 分布式 ID 生成器接口
type IDGenerator interface {
    NextID(prefix string) (uint64, error)
}

// localFS 默认实现
type localFS struct{}
func (l *localFS) Save(file *multipart.FileHeader, target string) error {
    os.MkdirAll(filepath.Dir(target), 0755)
    // ... 打开源文件，创建目标文件，Copy
}
```

**接口要小** — Go 的哲学是"小接口，大组合"。每个接口只做一件事。`FileStore` 只做文件存储，`IDGenerator` 只做 ID 生成。

### 10.2 UserService

```go
type UserService struct {
    db  *gorm.DB
    rdb *redis.Client
}

func NewUserService(db *gorm.DB, rdb *redis.Client) *UserService {
    return &UserService{db: db, rdb: rdb}
}

func (s *UserService) Login(ctx context.Context, phone, code string) (string, error) {
    // 1. 校验手机号
    // 2. 从 Redis 读验证码并比较（这里修复了 Java 版只判空不比较的 bug）
    // 3. 查询或创建用户
    // 4. 生成 token，写入 Redis Hash {id, nickName, icon}
    // 5. 返回 token
}
```

注意 `Login` 方法的参数是 `phone, code string`，不是 `LoginForm` 结构体。Handler 层负责解析 HTTP 参数，Service 层只关心领域类型。这是一个重要的分层原则。

### 10.3 ShopService：缓存穿透 + 互斥锁

```go
func (s *ShopService) GetByID(ctx context.Context, id uint64) (*model.Shop, error) {
    // 1. 先读 Redis
    // 2. 命中空字符串 → 返回 nil（缓存穿透防护）
    // 3. 未命中 → 尝试互斥锁（循环退避，不递归！）
    const maxRetries = 5
    for i := 0; i < maxRetries; i++ {
        locked, _ := s.rdb.SetNX(ctx, lockKey, owner, LockShopTTL).Result()
        if locked { break }
        time.Sleep(time.Duration(50+(i*25)) * time.Millisecond)
    }
    // 4. 用 context.Background() 释放锁（请求取消不泄漏锁）
    defer s.rdb.Eval(context.Background(), script.UnlockLua, []string{lockKey}, owner)
    // 5. 查库 → 写入 Redis（空值或真实数据）
}
```

**关键改进 (vs 原版)：**
- 循环退避代替递归 —— 不会有栈溢出风险
- `context.Background()` 释放锁 —— 请求取消后锁不会泄漏

### 10.4 VoucherOrderService：秒杀 + Kafka

```go
func (s *VoucherOrderService) StartConsumer(ctx context.Context, reader *kafka.Reader) {
    go func() {
        const (
            initialBackoff = 100 * time.Millisecond
            maxBackoff     = 5 * time.Second
        )
        backoff := initialBackoff

        for {
            msg, err := reader.FetchMessage(ctx)
            if err != nil {
                time.Sleep(backoff)
                backoff *= 2
                if backoff > maxBackoff { backoff = maxBackoff }
                continue
            }
            backoff = initialBackoff // 成功时重置
            // ... 处理订单
        }
    }()
}
```

**指数退避** — 当 Kafka 不可用时，避免全速重试烧 CPU。

### 10.5 其他 Service

| Service | 职责 | 亮点 |
|---------|------|------|
| `VoucherService` | 优惠券 CRUD + 秒杀券创建 | 事务中也写入 Redis 库存 |
| `ShopTypeService` | 商户类型列表 | 24h TTL 缓存 |
| `BlogService` | 博客、点赞、Feed | ZSet 点赞排行 + 推模式 Feed |
| `FollowService` | 关注/取关/共同关注 | Redis Set + MySQL 事务，SINTER 求交集 |
| `UploadService` | 图片上传 | FileStore 接口可注入 mock |

---

## 11. 处理器层：闭包 Handler

**核心变化：Handler 不是 struct，是闭包函数。**

```go
// 旧（Java 风格）
type UserHandler struct { svc *UserService }
func (h *UserHandler) Login(c *gin.Context) { ... }

// 新（Go 风格）
func HandleUserLogin(svc *service.UserService) gin.HandlerFunc {
    return func(c *gin.Context) {
        phone := c.PostForm("phone")
        code := c.PostForm("code")
        token, err := svc.Login(c.Request.Context(), phone, code)
        if err != nil { writeFail(c, err); return }
        writeOK(c, token)
    }
}
```

**为什么这样更好？**
1. **不需要 Handler 结构体** — 函数捕获 `svc` 闭包，没有多余的抽象
2. **不需要 Handler Container** — 每个 Handler 只依赖它需要的 Service
3. **显式依赖** — 路由注册时能看到每个 Handler 依赖什么 Service
4. **测试简单** — 直接传入 mock service 就行

响应辅助函数：

```go
func writeOK(c *gin.Context, data any) {
    c.JSON(http.StatusOK, response.OK(data))
}

func writeFail(c *gin.Context, err error) {
    if err == nil {
        c.JSON(http.StatusOK, response.Fail("服务器异常"))
        return
    }
    c.JSON(http.StatusOK, response.Fail(err.Error()))
}
```

---

## 12. 路由注册

路由函数接受**独立的 Service 指针**，而不是 Handler Container：

```go
func New(
    cfg *config.Config,
    rdb *redis.Client,
    userSvc *service.UserService,
    shopSvc *service.ShopService,
    shopTypeSvc *service.ShopTypeService,
    blogSvc *service.BlogService,
    followSvc *service.FollowService,
    voucherSvc *service.VoucherService,
    voucherOrderSvc *service.VoucherOrderService,
    uploadSvc *service.UploadService,
) *gin.Engine {
```

路由注册直接用 Handler 闭包：

```go
user.POST("/code", handler.HandleUserSendCode(userSvc))
user.POST("/login", handler.HandleUserLogin(userSvc))
// ...
shop.GET("/of/type", handler.HandleShopOfType(shopSvc))
```

**路径兼容** — 旧版支持 `GET /shop/{id}` 格式，通过 `NoRoute` 兜底实现：

```go
r.NoRoute(func(c *gin.Context) {
    path := strings.Trim(c.Request.URL.Path, "/")
    parts := strings.Split(path, "/")
    switch {
    case c.Request.Method == "GET" && len(parts) == 2 && parts[0] == "shop":
        handler.HandleShopGet(shopSvc)(c)
    // ... 更多兼容
    }
})
```

---

## 13. 依赖组装

依赖组装在 `internal/app/app.go` 中集中完成。没有 `handler.Container`，没有 `service.Container`。

```go
func New(cfg *config.Config) (*App, error) {
    // 1. GORM
    gormDB, _ := gorm.Open(mysql.Open(cfg.MySQL.DSN), &gorm.Config{...})
    db, _ := gormDB.DB()
    db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)

    // 2. Redis
    rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, ...})

    // 3. Kafka（可选）
    if cfg.Kafka.Enabled {
        writer = &kafka.Writer{...}
        reader = kafka.NewReader(...)
    }

    // 4. Service（独立构造！）
    userSvc := service.NewUserService(gormDB, rdb)
    shopSvc := service.NewShopService(gormDB, rdb)
    blogSvc := service.NewBlogService(gormDB, rdb, userSvc) // 显式传 UserService
    // ...

    // 5. 启动 Kafka 消费者
    if reader != nil {
        voucherOrderSvc.StartConsumer(consumerCtx, reader)
    }

    // 6. Router（传 Service 指针，不传 Container）
    engine := router.New(cfg, rdb, userSvc, shopSvc, ...)
    return &App{Router: engine, ...}, nil
}
```

**为什么没有 Container？**
Go 不需要一个"上帝对象"来管理所有依赖。每个 Service 只持有它需要的依赖。想测试 `BlogService`？只给它传 `gorm.DB`、`redis.Client` 和 `UserService` 就行，不需要构造整个项目。

---

## 14. Lua 脚本

通过 `//go:embed` 编译期嵌入：

```go
package script

import _ "embed"

//go:embed seckill.lua
var SeckillLua string

//go:embed unlock.lua
var UnlockLua string
```

**seckill.lua** — 原子判断库存 + 一人一单：

```lua
local stockKey = "seckill:stock:" .. ARGV[1]
local orderKey = "seckill:order:" .. ARGV[1]

local stock = redis.call('get', stockKey)
if (not stock or tonumber(stock) <= 0) then return 1 end
if (redis.call('sismember', orderKey, ARGV[2]) == 1) then return 2 end

redis.call('incrby', stockKey, -1)
redis.call('sadd', orderKey, ARGV[2])
return 0
```

**unlock.lua** — 安全释放锁：

```lua
if (redis.call('GET', KEYS[1]) == ARGV[1]) then
    return redis.call('DEL', KEYS[1])
end
return 0
```

**安全释放** — 只有持有锁的 owner 能删，避免误删其他 goroutine 的锁。

---

## 15. 启动入口

```go
func main() {
    cfg, err := config.Load("config/config.yaml")
    if err != nil { log.Fatalf("load config: %v", err) }

    application, err := app.New(cfg)
    if err != nil { log.Fatalf("build app: %v", err) }
    defer application.Close()

    server := &http.Server{
        Addr:    ":" + strconv.Itoa(cfg.Server.Port),
        Handler: application.Router,
    }

    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("listen: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    server.Shutdown(ctx)
}
```

**三件事原则**：main 只做三件事：加载配置、组装应用、启动并等待信号。

---

## 16. 运行项目

```powershell
# 0. 进项目目录（go.mod 在 Lifestyle-Platform 子目录，命令都要在这一层执行）
cd D:\learnproject\Lifestyle-Platform

# 1. 确认依赖在跑
#    MySQL → 127.0.0.1:3306（Windows 服务 MySQL80）
#    Redis → 127.0.0.1:6379（Windows 服务，应用用 db 1）
#    Kafka → localhost:9094~9096（可选，没起也能启动，见第 2 节）
netstat -an | Select-String "3306|6379"

# 2. 执行数据库迁移（migrate 的 DSN 格式和 GORM 的不同，别混用）
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" up

# 3. 灌种子数据（幂等，INSERT IGNORE，可以重复执行）
mysql --default-character-set=utf8mb4 dianping < seed/seed.sql

# 4. 预热秒杀券 Redis 库存（注意 -n 1，应用连的是 db 1 不是 db 0）
redis-cli -n 1 SET seckill:stock:10 100
redis-cli -n 1 SET seckill:stock:11 50

# 5. 设置环境变量（可选，覆盖 config.yaml，注意前缀是 HMDP_）
$env:HMDP_MYSQL_DSN="root:126070mxZ@tcp(127.0.0.1:3306)/dianping?charset=utf8mb4&parseTime=True&loc=Local"

# 6. 运行
go run ./cmd/server

# 7. 测试
curl "http://localhost:8081/shop-type/list"
curl "http://localhost:8081/shop/1"
curl -X POST "http://localhost:8081/user/code" -d "phone=13800138000"
# 验证码直接返回在响应 data 里，拿它去登录
curl -X POST "http://localhost:8081/user/login" -d "phone=13800138000&code=<上一步 data 里的验证码>"
```

---

## 17. 架构总结

### 整体架构图

```mermaid
graph TD
    Client --> Gin

    subgraph Middleware
        Recovery --> CORS --> Auth
    end
    Gin --> Recovery

    subgraph Handler["Handler（闭包函数）"]
        HUser[UserHandler]
        HShop[ShopHandler]
        HBlog[BlogHandler]
        HFollow[FollowHandler]
        HVoucher[VoucherHandler]
        HUpload[UploadHandler]
    end
    Auth --> HUser & HShop & HBlog & HFollow & HVoucher & HUpload

    subgraph Service["Service（独立构造）"]
        SUser[UserService]
        SShop[ShopService]
        SBlog[BlogService]
        SFollow[FollowService]
        SVoucher[VoucherService]
        SUpload[UploadService<br/>+FileStore interface]
    end
    HUser --> SUser
    HShop --> SShop
    HBlog --> SBlog
    HFollow --> SFollow
    HVoucher --> SVoucher
    HUpload --> SUpload

    subgraph Data
        MySQL[(MySQL)]
        Redis[(Redis)]
        Kafka[Kafka]
    end
    SUser --> MySQL & Redis
    SShop --> MySQL & Redis
    SBlog --> MySQL & Redis
    SFollow --> MySQL & Redis
    SVoucher --> MySQL & Redis
    SUpload --> LocalFS[本地文件/mock]

    subgraph Lua["Redis Lua"]
        SeckillLua[seckill.lua<br/>原子扣库存+一人一单]
        UnlockLua[unlock.lua<br/>安全释放锁]
    end
    SShop --> UnlockLua
    SVoucher --> SeckillLua
    SeckillLua --> Redis
    UnlockLua --> Redis
```

### 和 Java/Spring 风格的对比

`
| 维度 | Java/Spring 风格 | Go 风格（本项目的做法） |
|------|-----------------|----------------------|
| 项目结构 | `controller/service/mapper/entity/vo/dto` | `handler/service/model/response` |
| 依赖管理 | `@Autowired` + ApplicationContext | 构造函数参数注入 + 直接传 |
| 中间件 | `@Transactional` AOP | GORM `Transaction(func(tx) { ... })` |
| 序列化 | Jackson 注解 | 结构体 tag（一个结构体多种 tag） |
| 数据映射 | Entity ↔ VO/DTO 手动映射 | 一个模型多种 tag，View 类型放同一包 |
| Handler | `@RequestMapping` 注解在 struct 方法上 | `func NewHandler(svc) gin.HandlerFunc { ... }` 闭包 |
| 错误处理 | `@ControllerAdvice` 全局异常拦截 | 函数返回值 `if err != nil { writeFail }` |
| 测试策略 | Mockito + Spring Test | 接口注入 + 闭包构造 |
| 锁重试 | `while(true) { sleep; recursive call }` | `for i := 0; i < max; i++ { backoff; break }` |
`

### 学到了什么

1. **Go 项目标准布局** — `cmd/server + internal/*`，`internal` 阻止外部 import
2. **单源真相** — 一个模型结构体 + 多种 tag，避免 DTO 泛滥
3. **闭包 Handler** — 比 struct + method 更轻量，依赖更明确
4. **小接口** — `FileStore`、`IDGenerator`，只做一件事
5. **循环退避** — 代替递归做重试，不乱堆栈
6. **指数退避** — 消费者优雅处理暂时不可用
7. **`context.Background()` 释放锁** — 避免请求取消导致锁泄漏
8. **`go:embed` 编译期嵌入** — 单二进制部署，无外部依赖
9. **显式依赖注入** — 无反射，无 Service Locator，编译期检查

---

*教程基于实际项目 `hm-dianping`，当前源码在 `D:\learnproject\Lifestyle-Platform`。*
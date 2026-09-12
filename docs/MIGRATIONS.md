# golang-migrate 使用教程

本项目使用 `golang-migrate` 管理 MySQL 表结构，迁移文件放在 `migrations/`。

## 当前迁移文件

```text
migrations/000001_init_schema.up.sql
migrations/000001_init_schema.down.sql
```

`up.sql` 用于创建表，`down.sql` 用于回滚删除表。

## 安装 CLI

PowerShell：

```powershell
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

安装后确认：

```powershell
migrate -version
```

如果提示找不到命令，把 Go bin 目录加入 PATH：

```powershell
$env:Path += ";$env:USERPROFILE\go\bin"
```

## 创建数据库

先在 MySQL 中创建数据库。

⚠️ 库名是 **`dianping`**，不是 `hmdp`。要和 `config/config.yaml` 里的 GORM DSN 保持一致：

```sql
CREATE DATABASE IF NOT EXISTS dianping DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

## 当前 dianping 库的状态

本机 `dianping` 库里的 11 张表**已经存在**，但 `schema_migrations` 表**不存在**，说明这些表当初不是用 `migrate` 建的（应该是直接执行的 SQL）。

这不影响使用：`000001_init_schema.up.sql` 里所有建表语句都用了 `CREATE TABLE IF NOT EXISTS`，所以现在直接跑 `migrate up` 是安全的——它只会补上 `schema_migrations` 并记录为版本 1，不会动已有数据。

自检当前状态：

```sql
USE dianping;
SHOW TABLES;                                        -- 应该有 11 张 tb_ 表
SELECT * FROM schema_migrations;                    -- 迁移前会报 1146 表不存在，迁移后应有一行 version=1
```

## 执行迁移

```powershell
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" up
```

说明：

```text
-path migrations                      指定迁移文件目录
-database "mysql://..."               指定数据库连接
up                                    执行所有未执行的 up 迁移
multiStatements=true                  允许一个迁移文件内包含多个 SQL 语句
```

## 回滚迁移

回滚 1 个版本：

```powershell
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" down 1
```

回滚到指定版本：

```powershell
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" goto 1
```

## 查看版本

```powershell
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" version
```

## 新增迁移

例如新增签到功能表结构：

```powershell
migrate create -ext sql -dir migrations -seq add_sign_feature
```

会生成：

```text
migrations/000002_add_sign_feature.up.sql
migrations/000002_add_sign_feature.down.sql
```

规则：

```text
up.sql 写正向变更，例如 CREATE TABLE、ALTER TABLE ADD COLUMN。
down.sql 写反向变更，例如 DROP TABLE、ALTER TABLE DROP COLUMN。
```

## dirty 状态处理

如果迁移执行中断，`migrate version` 可能显示 dirty。确认数据库状态后，可以强制修复版本：

```powershell
migrate -path migrations -database "mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true" force 1
```

不要随意 `force`，必须先确认当前表结构和版本号一致。

## 与应用配置的关系

应用运行使用 GORM DSN：

```yaml
mysql:
  dsn: root:126070mxZ@tcp(127.0.0.1:3306)/dianping?charset=utf8mb4&parseTime=True&loc=Local
```

迁移 CLI 使用 migrate DSN：

```text
mysql://root:126070mxZ@tcp(127.0.0.1:3306)/dianping?multiStatements=true
```

两者格式不同，不要混用。

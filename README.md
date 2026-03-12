# Duty Log System

基于 Go + Gin + GORM + PostgreSQL 的值班管理系统。

![docker-image](https://github.com/bigcaole/duty-log/actions/workflows/docker-image.yml/badge.svg)

## 技术栈

- Go `1.23`
- Gin `v1.9.1`
- GORM + PostgreSQL
- bcrypt 密码哈希
- AES-256-GCM 敏感配置加密（`system_configs`）
- TOTP 2FA
- html/template + Tailwind CSS
- 首页交班总览（昨日操作 + 昨日记录内容）
- 周期提醒（到期前 N 天全员提醒）
- 多周期报告（周报/月报/半年报/年报）
- 附件上传（工单/故障记录）存储在数据库

## 目录结构

```text
cmd/server/main.go
internal/models/models.go
internal/handlers/*.go
pkg/utils/*.go
templates/**/*.html
```

## 快速启动（Docker）

1. 复制环境变量文件

```bash
cp .env.example .env
```

2. 按需修改 `.env`（至少修改以下字段）

- `SECRET_KEY`
- `DB_PASSWORD`
- 邮件相关配置（如需备份邮件）

3. 启动服务

```bash
docker compose up -d --build
```

4. 健康检查

```bash
curl http://127.0.0.1:5001/livez
curl http://127.0.0.1:5001/readyz
```

期望返回：

```json
{"ok":true}
```

## 本地开发

1. 准备 PostgreSQL 数据库并创建库（默认 `duty_log`）
2. 配置 `.env`（`DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD`）
3. 运行：

```bash
go mod tidy
go run ./cmd/server
```

## 默认管理员

系统首次启动会自动创建默认管理员：

- 用户名：`admin`
- 密码：`admin123`

首次登录后请立即修改密码。

## 关键环境变量（可选）

- `GIN_MODE`: `debug|release|test`
- `TRUSTED_PROXIES`: 逗号分隔，示例 `127.0.0.1,::1`
- `HTTP_READ_TIMEOUT_SEC`: HTTP 读超时
- `HTTP_WRITE_TIMEOUT_SEC`: HTTP 写超时
- `HTTP_IDLE_TIMEOUT_SEC`: HTTP 空闲超时
- `HTTP_SHUTDOWN_TIMEOUT_SEC`: 优雅停机超时
- `LOGIN_MAX_ATTEMPTS`: 登录窗口最大失败次数
- `LOGIN_WINDOW_SECONDS`: 登录失败统计窗口（秒）
- `LOGIN_BLOCK_SECONDS`: 登录失败封禁时长（秒）
- `FEISHU_WEBHOOK_URL`: 飞书机器人 Webhook（也可在后台配置）
- `REPORT_FEISHU_ENABLED`: 报表推送开关（后台可设置）
- `REMINDER_FEISHU_ENABLED`: 提醒推送开关（后台可设置）
- `NEXTCLOUD_URL`: Nextcloud 地址（备份上传用，后台可设置）
- `NEXTCLOUD_USERNAME`: Nextcloud 用户名（后台可设置）
- `NEXTCLOUD_PASSWORD`: Nextcloud 应用密码（后台可设置）
- `NEXTCLOUD_PATH`: 备份上传目录（后台可设置）

## 新手最小化配置（推荐）

启动服务真正“必须”的环境变量只有数据库连接相关：

- `DB_HOST`
- `DB_PASSWORD`

其余启动参数大多有安全默认值或可选值。

业务参数（如登录限流、2FA 发行方、备份策略）支持在 Web 首次初始化向导中配置：

- 首次登录后会自动跳转到 `/admin/setup/config`
- 需要先完成“必要配置”后才能进入其它页面
- 其它可选项可在 `/admin/config` 中按需设置

这意味着容器平台里不需要一次性填很多环境变量。

## 常用命令

```bash
make run
make build
make test
make compose-up
make compose-logs
```

## CI/CD 自动构建 Docker 镜像

- 工作流文件：`.github/workflows/docker-image.yml`
- 触发条件：
  - push 到 `main`
  - push `v*` tag（如 `v1.0.0`）
  - pull request 到 `main`（仅构建，不推送镜像）
- 镜像仓库：`ghcr.io/bigcaole/duty-log`
- 默认标签示例：
  - `main`
  - `latest`（默认分支）
  - `sha-<commit>`
  - `v*`（发布标签）

拉取示例：

```bash
docker pull ghcr.io/bigcaole/duty-log:latest
```

## 备份说明

- 后台可手动触发备份：`/admin/backup-notifications`
- 支持自动备份（`BACKUP_ENABLED=true` + 调度配置）
- 备份解密密码在数据库中以 AES-256-GCM 加密存储（兼容历史明文）
- 后台支持“一键规范化密码存储”，可批量迁移历史明文为密文
- 支持保留策略：`BACKUP_RETENTION_DAYS`
- 支持备份上传到 Nextcloud（WebDAV），并通过飞书推送备份结果
- 系统配置页提供飞书测试按钮，配置后可先测试再启用自动任务

## 附件存储说明

- 工单/故障记录附件默认存入 PostgreSQL，不需要挂载宿主机目录
- 旧版本已落盘的附件仍可通过 `/static/uploads` 访问
- 附件越多数据库越大，建议结合备份策略与存储监控使用

## 周报自动化与提醒

- 管理员可在 `/reports` 手动生成周报/月报/半年报/年报，并下载 PDF
- 自动推送任务为“周报”模式，支持按 Cron 定时生成并推送邮件/飞书（由系统配置控制）
- 报表页提供“测试自动周报推送”按钮，便于联调邮件/飞书参数
- 新增提醒模块 `/reminders`：
  - 支持记录周期任务（开始日期/结束日期/提前提醒天数）
  - 到期前 N 天起在首页“到期提醒（全员）”区域提示

## 交班场景支持

- 首页新增“昨日交班总览”：
  - 昨日操作轨迹（审计日志）
  - 昨日业务记录内容（值班日志/IDC 值班/IDC 运维工单/网络运维工单/网络故障记录）
- 用于今日值班人员快速接手，减少口头交接依赖

## 审计日志保留

- 配置项：`AUDIT_RETENTION_DAYS`
- 后台可手动清理：`/admin/audit-logs`

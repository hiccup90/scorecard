# scorecard

家庭积分电子打卡系统。后端 **Go + SQLite**，前端 React/Vite，可选 Android WebView 壳。

面向「自建单机 / 家庭 NAS」场景，按正式工程方式组织：分层代码、SQL 迁移、结构化日志、健康检查、CI、非 root 镜像。

## 核心规则

- 积分余额只从 `point_transactions` 流水汇总。
- 支持重复打卡与补签（默认窗口 30 天，`MAKEUP_DAYS`）。
- 连续天数按 `activity_date` 计算。
- 审核通过写正流水；撤回写负流水冲正。
- 兑换扣分；需审批的奖励由家长通过后扣分。
- 孩子端 / 家长端 API 均需 PIN 登录 token（`X-Auth-Token`）。

## 快速开始（开发）

```bash
cp .env.example .env
# 开发可直接：
export ALLOW_DEFAULT_PIN=1

make dev
# 或
npm --prefix web install && npm --prefix web run build
go run ./cmd/scorecard
```

- 孩子端：http://localhost:3003/
- 家长后台：http://localhost:3003/admin

## 生产（Docker + SQLite）

```bash
mkdir -p ./data
export ADMIN_PIN='your-strong-pin'
docker compose up -d --build
```

数据文件：`./data/scorecard.db`（备份只需拷贝该文件及相关 `-wal`/`-shm`）。

容器默认用户 `uid=10001`。若宿主机 `./data` 权限不足：

```bash
sudo chown -R 10001:10001 ./data
```

## 常用命令

```bash
make help
make test      # go test
make check     # test + web build
make build     # web + api 二进制 dist/scorecard
make docker    # 本地镜像 scorecard:local
```

## 环境变量

| 变量 | 说明 | 默认 |
|------|------|------|
| `ADMIN_PIN` | 家长 PIN（生产必填，禁止裸用 1234） | — |
| `CHILD_PIN` | 孩子 PIN | 同 `ADMIN_PIN` |
| `ALLOW_DEFAULT_PIN` / `SCORECARD_DEV` | 开发允许默认 PIN | 关 |
| `PORT` | 端口 | `3003` |
| `DB_PATH` | SQLite 路径 | `data/scorecard.db` |
| `STATIC_DIR` | 前端静态目录 | `web/dist` |
| `TZ` | 时区 | `Asia/Shanghai` |
| `MAKEUP_DAYS` | 补签窗口 | `30` |
| `TOKEN_TTL_HOURS` | 会话小时 | `24` |
| `LOG_LEVEL` | debug/info/warn/error | `info` |
| `APP_VERSION` | 版本号 | `dev` |

## 目录结构

```text
cmd/scorecard/                 进程入口（优雅退出）
internal/config/               配置
internal/database/             SQLite 打开 + 种子数据
internal/migrate/sql/          嵌入式 SQL 迁移
internal/domain/               领域模型与计分规则
internal/server/               HTTP 传输层
internal/platform/middleware/  RequestID / slog / recover
web/src/api|lib|...            前端模块
migrations/                    SQL 可读副本
docs/ARCHITECTURE.md           架构说明
legacy/                        旧 Node 实现（对照）
```

## 运维探针

- `GET /healthz` — 存活
- `GET /readyz` — 数据库可连
- `GET /api/v1/version` — 版本与时区

## 安全摘要

- 生产必须设置非默认 `ADMIN_PIN`
- 静态资源路径防穿越
- 登录限流；会话落 SQLite
- 用户身份以 token 为准
- 容器非 root 运行

详见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## License

MIT

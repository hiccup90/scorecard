# scorecard

一个面向家庭场景的积分电子打卡系统。新版后端改为 Go + SQLite，前端改为 React/Vite，目标是让补签、重复打卡、审核、撤回和积分流水更可靠。

## 核心规则

- 积分余额只从 `point_transactions` 流水汇总得到。
- 打卡可以重复提交，也可以选择过去日期补签。
- 打卡记录有 `activity_date`，连续天数按这个日期计算，不按提交时间计算。
- 审核通过会创建正积分流水；撤回不会删除旧流水，而是创建负积分冲正流水。
- 奖励兑换会创建负积分流水；需要审批的奖励由家长通过后扣分。

## 本地开发

需要 Go 和 Node.js。

```bash
npm --prefix web install
npm --prefix web run build
go run ./cmd/scorecard
```

默认访问：

- 孩子端：`http://localhost:3003/`
- 家长后台：`http://localhost:3003/admin`

默认 PIN：

- `ADMIN_PIN=1234`
- `CHILD_PIN` 未设置时跟随 `ADMIN_PIN`

## 常用命令

```bash
npm run build:web
npm run build:api
npm run build
npm run check
```

## Docker

```bash
mkdir -p ./data
docker compose up -d --build
```

SQLite 数据库默认写入：

```text
./data/scorecard.db
```

## 目录结构

```text
cmd/scorecard/          Go 服务入口
internal/config/        配置读取
internal/database/      SQLite 建表和默认数据
internal/server/        HTTP API 和业务规则
web/                    React/Vite 前端
android/                Android WebView 壳
```

## 旧版说明

旧的 Node/Express 源码仍保留在 `src/` 和 `public/`，方便对照或后续写数据迁移脚本。新版运行入口是 `cmd/scorecard`，Docker 也已经切到 Go 服务。

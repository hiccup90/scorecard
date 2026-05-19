# scorecard

一个面向家庭场景的积分电子打卡系统：

- 孩子端：Web 页面 + Android WebView APK 壳
- 家长端：Web 管理后台
- 后端：Node.js + Express + SQLite

## 本地启动

```bash
npm install
node src/server.js
```

默认访问：

- 孩子端：`http://localhost:3003/`
- 家长后台：`http://localhost:3003/admin`

## Android APK 壳

Android 工程位于：

```text
android/
```

调试构建：

```bash
cd android
./gradlew assembleDebug
```

## Docker

容器内服务默认监听 `3003` 端口，SQLite 数据库文件写入 `/app/data/scorecard.db`。
首次启动前请先准备宿主机数据目录：

```bash
mkdir -p ./data
```

### docker run

仓库已通过 GitHub Actions 自动构建并发布镜像到 GHCR：`ghcr.io/hiccup90/scorecard:latest`

```bash
docker run -d \
  --name scorecard \
  -p 3003:3003 \
  -e PORT=3003 \
  -e ADMIN_PIN=1234 \
  -e CHILD_PIN=1234 \
  -v "$(pwd)/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/hiccup90/scorecard:latest
```

如果你要基于本地源码自行构建：

```bash
docker build -t scorecard .
```

启动后访问：

- 孩子端：`http://localhost:3003/`
- 家长后台：`http://localhost:3003/admin`

### docker compose

项目已包含 `docker-compose.yml`，关键配置如下：

```yaml
services:
  api:
    image: ghcr.io/hiccup90/scorecard:latest
    ports:
      - "3003:3003"
    environment:
      PORT: 3003
      ADMIN_PIN: 1234
      CHILD_PIN: 1234
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

启动命令：

```bash
docker compose up -d
```

说明：

- `-p 3003:3003` / `ports: 3003:3003`：把宿主机 `3003` 端口映射到容器内应用端口。
- `ADMIN_PIN`：家长后台登录 PIN；未设置时默认值是 `1234`。
- `CHILD_PIN`：孩子端校验 PIN；未设置时默认跟随 `ADMIN_PIN`。
- `./data:/app/data`：持久化 SQLite 数据目录，数据库文件位于 `./data/scorecard.db`。
- 如果你要修改宿主机端口，可以同时调整左侧映射端口；容器内端口仍保持 `3003`，或额外通过 `PORT` 环境变量同步修改应用监听端口。

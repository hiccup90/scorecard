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
- 家长后台：`http://localhost:3003/admin.html`

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

```bash
docker compose up -d --build
```

# 私人日记 (Private Diary)

一个现代化的在线私人日记应用，采用前后端分离架构：Go (Backend) + React (Frontend)。

## 架构
- **后端**: Go 1.25 + SQLite (JSON API)
- **前端**: React + Vite + TailwindCSS (SPA)

## 功能
- **安全登录**: Token 认证 (Cookie)
- **日记编辑**: 按日期记录，支持自动保存
- **日历导航**: 轻松切换日期
- **历史回顾**: 无限滚动查看过往日记
- **全文搜索**: 支持按内容或日期搜索
- **数据管理**: 导入/导出 JSON 数据
- **个性化**: 设置用户名、头像，支持暗黑模式

## 运行方式

### 1. 本地开发运行

需要安装 Go 和 Node.js。

**编译前端**:
```bash
cd frontend
npm install
npm run build
cd ..
```

**运行后端**:
```bash
# 设置 Token (可选，默认 "changeme")
$env:DIARY_LOGIN_TOKEN = "你的密钥"

go run main.go
```
访问: http://localhost:8080

### 2. Docker 部署 (推荐)

**构建镜像**:
```bash
docker build -t diary-app .
```

**运行容器**:
需要挂载 `data` 目录以持久化数据库和配置。

```bash
# 1. 创建数据目录
mkdir data
# 复制示例配置 (可选)
cp config.sample.toml data/config.toml

# 2. 启动容器
docker run -d -p 8080:8080 -v ${PWD}/data:/app/data --name diary diary-app
```
访问: http://localhost:8080

## 配置
配置文件 `config.toml` (或环境变量):
- `auth.token`: 登录凭证
- `server.address`: 监听地址 (默认 :8080)
- `database.path`: SQLite 数据库路径

## 目录结构
```
.
├─ main.go                # 后端入口 (API + 静态文件服务)
├─ Dockerfile             # 多阶段构建文件
├─ frontend/              # React 前端项目
│  ├─ src/
│  ├─ dist/               # 编译后的静态文件
│  └─ ...
└─ ...
```

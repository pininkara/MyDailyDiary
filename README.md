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

### 2. Docker 部署 (服务器内构建)

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

### 3. 预编译 Docker 部署（本地编译后上传服务器）

如果服务器也是 x86_64 / amd64，可以在本地先编译好前端和 Go 后端，再把产物上传到服务器构建轻量镜像。这样服务器不需要安装 Go、Node.js，也不需要在 Docker build 阶段下载依赖。

**本地打包**:
```powershell
# 生成 dist-package 目录
.\scripts\build-prebuilt-package.ps1

# 或者同时生成 dist-package.zip，方便上传
.\scripts\build-prebuilt-package.ps1 -Zip
```

脚本会生成：
```text
dist-package/
├─ diary                 # linux/amd64 后端二进制
├─ Dockerfile            # 由 Dockerfile.prebuilt 复制而来
├─ config.sample.toml
├─ README.deploy.md
└─ frontend/
	 └─ dist/              # 前端静态文件
```

**上传到服务器后构建镜像**:
```bash
# 如果上传的是 zip，先解压并进入目录
cd dist-package

docker build -t diary-app:latest .
```

**运行容器**:
```bash
mkdir -p data
cp config.sample.toml data/config.toml
# 编辑 data/config.toml，设置 token、数据库路径、LLM 配置等

docker run -d \
	--name diary \
	-p 8080:8080 \
	-v $PWD/data:/app/data \
	diary-app:latest
```

访问: http://服务器IP:8080

## 配置
配置文件 `config.toml` (或环境变量):
- `auth.token`: 登录凭证
- `server.address`: 监听地址 (默认 :8080)
- `database.path`: SQLite 数据库路径
- `llm.enabled`: 是否启用 LLM 自动标题总结
- `llm.base_url`: OpenAI-compatible API Base URL，应用会调用 `{base_url}/responses`
- `llm.api_key`: LLM API Key
- `llm.model`: LLM 模型名称
- `llm.prompt`: 自动标题总结 Prompt

## 目录结构
```
.
├─ main.go                # 后端入口 (API + 静态文件服务)
├─ Dockerfile             # 多阶段构建文件
├─ Dockerfile.prebuilt    # 使用本地预编译产物的 Dockerfile
├─ scripts/               # 构建/打包脚本
│  └─ build-prebuilt-package.ps1
├─ frontend/              # React 前端项目
│  ├─ src/
│  ├─ dist/               # 编译后的静态文件
│  └─ ...
└─ ...
```

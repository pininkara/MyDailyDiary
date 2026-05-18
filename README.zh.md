# 每日记

English version: [README.md](README.md)

一个现代化的在线日记应用，采用前后端分离架构：Go (Backend) + React (Frontend)。

🤖Vibe coding 声明：本项目超过 90% 的代码由 AI 辅助生成，我做了有限测试来确认可用性，但不能保证完全没有 bug。

## 项目描述
每日记是一个面向个人记录场景的 Web 应用，重点放在“长期保存、快速记录、随时回看”这三个核心体验上。它不是一个社交型内容平台，而是一个更偏向私密、轻量、可控的数据工具：你可以按日期写日记、回顾历史内容、搜索过往记录，并把数据保留在自己的 SQLite 数据库里。

项目采用前后端分离的单页应用结构。前端负责日记编辑、历史浏览、搜索和设置等交互，后端负责认证、数据持久化、导入导出以及可选的 LLM 自动标题生成。整体设计尽量保持简单：界面直观、接口清晰、部署方式也尽量少依赖外部服务，适合本地运行、Docker 部署或放到个人服务器上使用。

如果你想把它当作个人知识库的日记层、情绪记录工具，或者长期可迁移的私有笔记仓库，这个项目都比较合适。后续也会围绕“稳定存储、易迁移、可扩展”的方向继续演进。

## 设计理念 / Why this project
这个项目的初衷很简单：做一个真正属于自己的在线日记系统。数据放在自己的服务器或设备上，登录后可以在任何设备上继续写，不依赖第三方平台，也不把内容交给社交网络或云笔记服务来决定。

我把它设计成一个“够用、稳定、长期可维护”的工具，而不是一个功能堆得很满的内容创作平台。日记是写给自己看的，所以不追求复杂排版，也没有加入 Markdown、图片、视频、位置坐标这些插入能力；同样因为是个人使用场景，也没有做多用户协作、评论、分享或社交关系链。

它更关注记录本身：当天发生了什么、当时的情绪如何、环境是什么样、这件事最终完成得怎么样，以及一段时间之后回头看时能不能快速找到答案。也正因为这样，应用保留了足够轻量的交互和数据结构，让它可以长期自部署、长期迁移、长期自己掌控。

数据库也保持明文而不是在应用层再做一次加密，原因是这个项目的目标不是对抗复杂的共享环境，而是保证自己能随时搜索、导入、导出和迁移数据。如果把内容加密到应用层，搜索性能、统计能力和数据交换都会明显复杂化，最后会偏离这个“简单、可控、可持续使用”的初衷。对于这个场景，把安全边界交给自部署环境和系统层加密就足够了。


## 架构
- **后端**: Go 1.25 + SQLite (JSON API)
- **前端**: React + Vite + TailwindCSS (SPA)

## 功能
- **私有部署**: 数据保留在你自己的环境里，可以在本机、服务器或 Docker 中运行
- **随时记录**: 按日期快速写日记，修改后可手动保存
- **轻量写作**: 不做 Markdown、图片、视频和复杂富文本，只保留最直接的记录体验
- **情绪与状态**: 记录 Mood、Fulfillment、基础天气和氛围天气，帮助回看当天的状态
- **日历式回顾**: 通过日历和历史列表快速回到任意一天
- **全文搜索**: 可以按内容、日期等条件快速找到旧记录
- **统计页面**: 查看一段时间内的记录概览和趋势
- **LLM 总结**: 可选地自动生成标题，让长日记更容易扫读
- **数据导入导出**: 支持 JSON 导入/导出，便于备份和迁移
- **基础个性化**: 设置用户名、头像和主题外观

## 运行方式

### 1. 本地开发运行

需要安装 Go 和 Node.js。

**安装前端依赖**:
```bash
cd frontend
npm install
```

**启动前端开发服务器**:
```bash
cd frontend
npm run dev
```

**启动后端**:
```bash
cd ..

# PowerShell
$env:DIARY_LOGIN_TOKEN="你的密钥"
go run ./cmd/diary
```

如果你要模拟生产环境静态资源，也可以先构建前端再启动后端：

```bash
cd frontend
npm run build
cd ..

go run ./cmd/diary
```

访问: http://localhost:8080

### 2. Docker 部署 

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
- `llm.enabled`: 是否启用 LLM 自动标题总结
- `llm.base_url`: OpenAI-compatible API Base URL，应用会调用 `{base_url}/responses`
- `llm.api_key`: LLM API Key
- `llm.model`: LLM 模型名称
- `llm.prompt`: 自动标题总结 Prompt

## 目录结构
```
.
├─ cmd/
│  └─ diary/
│     └─ main.go          # 程序入口
├─ internal/
│  └─ app/
│     ├─ config.go        # 配置加载与保存
│     ├─ db.go            # 数据库、迁移、查询
│     ├─ handlers.go      # HTTP 处理器
│     ├─ helpers.go       # 通用辅助函数
│     ├─ http.go          # 中间件与 HTTP 辅助
│     ├─ llm.go           # LLM 标题生成逻辑
│     ├─ server.go        # 应用启动与路由注册
│     └─ types.go         # 核心类型定义
├─ Dockerfile             # 多阶段构建文件
├─ frontend/              # React 前端项目
│  ├─ src/
│  ├─ dist/               # 编译后的静态文件
│  └─ ...
├─ data/                  # SQLite 数据与运行时配置
├─ config.sample.toml     # 配置示例
├─ go.mod
└─ go.sum
```
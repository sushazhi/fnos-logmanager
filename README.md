# 飞牛日志管理 (LogManager for fnOS)

飞牛日志管理工具，集中管理飞牛三方应用散落在各个文件夹的日志文件。

## 功能特点

- **多目录支持** - 支持管理多个日志目录
  - 存储空间应用日志 (@appdata/@appshare 等)
  - /var/log/apps/ (系统应用日志)
  - Docker 容器日志
  - 归档日志文件 (.gz, .bz2, .xz, .zip 等)

- **日志查看** - 在线查看日志内容，支持搜索过滤
  - 流式读取大文件，内存占用低
  - 支持倒序查看最新日志
  - 深色终端风格日志显示界面
  - 关键词/正则搜索高亮
  - Web Worker 后台搜索，不阻塞 UI

- **实时追踪** - 类似 tail -f 的实时日志追踪
  - HTTP 轮询方式，兼容 fnOS iframe 反向代理
  - 支持文件日志和 Docker 容器日志实时追踪
  - 自动滚动到最新行

- **日志导出** - 多格式导出
  - TXT 纯文本
  - JSON 结构化
  - CSV 表格
  - 支持 Docker 容器日志导出

- **日志管理**
  - 删除已卸载应用的日志文件
  - 清空大日志文件
  - 批量清理旧归档
  - 清理已卸载应用的空文件夹

- **自动清理** - 定时自动清理策略
  - 支持 cron 表达式和秒级自定义间隔
  - 按文件大小/天数/正则匹配清理
  - 独立清理规则管理

- **书签/收藏** - 快速访问常用日志
  - 收藏常用日志文件/容器
  - 一键打开书签日志
  - Docker 容器书签支持

- **备份**
  - 一键备份所有日志

- **通知推送** - 日志监控与多渠道通知
  - 支持 Bark、钉钉、飞书、企业微信、Telegram、QQ机器人 等 22 种通知渠道
  - 自定义监控规则，关键词匹配（支持正则表达式）
  - 日志级别过滤
  - 冷却时间与静默时段设置

- **系统日志监控** - 监控系统事件日志
  - 实时监控数据库事件
  - 多级别事件过滤
  - 事件统计与历史记录

- **安全特性**
  - Argon2id 密码哈希
  - 登录失败锁定（5次失败锁定30分钟）
  - 敏感信息自动过滤
  - 审计日志记录
  - CSRF 双重保护（Token + SameSite Cookie）
  - 路径遍历防护（三重检查：isAllowedPath + safePath + isSymlinkPath）
  - Docker 容器名白名单验证
  - 命令注入防护（spawn 数组参数，非 shell 拼接）
  - XSS 防护（DOMPurify 净化所有 v-html）
  - SSRF 防护（Webhook URL 私有地址检测）
  - CSRF token 时序安全比较（crypto.timingSafeEqual）
  - SSE/WebSocket 连接数限制（防 DoS）
  - 敏感操作速率限制
  - 统一错误处理，避免信息泄露

- **UI 设计** - 鸿蒙 NEXT 6.0 设计风格
  - 全局 CSS 变量色彩体系
  - 日间/夜间主题
  - 自定义主题色
  - 字体大小调节
  - 深色终端风格日志显示
  - 响应式布局适配

- **性能优化**
  - 流式日志读取，支持大文件
  - Web Worker 后台搜索
  - 虚拟滚动（10万+行流畅滚动）
  - 内存缓存机制
  - 请求去重和重试
  - 代码分割优化加载

## 支持的通知渠道

| 渠道 | 说明 |
|------|------|
| Bark | iOS 推送应用 |
| 钉钉机器人 | 钉钉群机器人 |
| 飞书机器人 | 飞书自定义机器人(Webhook) |
| 飞书企业应用 | 飞书企业自建应用 |
| 企业微信机器人 | 企业微信群机器人 |
| 企业微信应用 | 企业微信应用消息 |
| 企业微信智能机器人 | WebSocket 长连接模式 |
| Telegram | Telegram Bot |
| QQ机器人 | QQ开放平台机器人 |
| Server酱 | 微信推送服务 |
| PushPlus | 多渠道推送 |
| Ntfy | 开源推送服务 |
| Gotify | 自建推送服务 |
| PushDeer | 开源推送服务 |
| 自定义Webhook | 自定义HTTP推送 |
| iGot | 推送服务 |
| Synology Chat | 群晖聊天 |
| QMsg | QQ推送 |
| PushMe | 推送服务 |
| WxPusher | 微信推送 |
| AIBotK | 智能机器人 |
| WePlusBot | 机器人 |

## 安装

1. 下载最新的 `.fpk` 文件 from [Releases](../../releases)
2. 在飞牛 NAS 应用中心安装
3. 首次访问时设置登录密码

> **注意**：本应用仅在 ARM64 架构测试通过，AMD64 架构请自测。

## 使用方法

### 登录

首次访问时设置密码，后续使用设置的密码登录。

### 主要功能

| 功能 | 说明 |
|------|------|
| 查看日志 | 点击日志列表中的"查看"按钮，深色终端风格显示 |
| 实时追踪 | 查看日志时点击"追踪"按钮，HTTP 轮询实时获取新内容 |
| 导出日志 | 点击"导出"按钮，选择 TXT/JSON/CSV 格式 |
| 书签收藏 | 点击"书签"按钮收藏常用日志，书签栏快速访问 |
| 搜索日志 | 支持关键词和正则模式搜索，自动高亮匹配 |
| 删除日志 | 已卸载应用的日志会显示删除按钮 |
| 清空日志 | 查看日志时可点击"清空"按钮 |
| 查看归档 | 点击"归档日志"查看压缩的日志文件 |
| Docker日志 | 点击"Docker日志"查看容器日志 |
| 清理空文件夹 | 点击"清理空文件夹"删除已卸载应用的空目录 |
| 自动清理 | 配置定时清理规则，按大小/天数/模式自动清理 |
| 通知设置 | 点击"通知设置"配置监控规则和通知渠道 |
| 系统日志 | 点击"系统日志"监控系统事件 |

### 通知规则配置

1. 添加通知渠道（如 Bark、钉钉、飞书等）
   - 飞书机器人：使用 Webhook 方式，适合群机器人推送
   - 飞书企业应用：使用企业自建应用，可发送给指定用户
2. 创建监控规则：
   - 选择监控的应用和日志级别
   - 设置关键词（支持正则表达式：`regex:pattern` 或 `/pattern/flags`）
   - 选择通知渠道
   - 设置冷却时间避免频繁通知
3. 测试通知：添加渠道后可测试通知是否正常

### 设置

点击右上角设置图标进入设置：
- 修改密码
- 切换主题
- 更改主题色
- 查看审计日志

## 本地构建

### 前置要求

- Node.js 24+
- PowerShell (Windows) 或 Bash (Linux)

### 构建步骤

```bash
# Windows
.\build.ps1 -Version 0.5.17

# 或使用 GitHub Actions
git tag v0.5.17
git push --tags
```

## 项目结构

```
├── .github/
│   └── workflows/
│       └── build-and-release.yml   # GitHub Actions
├── app/
│   ├── server/                     # 后端服务
│   │   ├── server.ts
│   │   ├── errors/                 # 错误类型定义
│   │   ├── middleware/             # 中间件
│   │   │   ├── auth.ts             # 认证/CSRF/验证
│   │   │   ├── security.ts         # CSP/安全头/输入净化
│   │   │   ├── rateLimit.ts        # 速率限制
│   │   │   └── errorHandler.ts     # 统一错误处理
│   │   ├── routes/                 # 路由
│   │   │   ├── logs.ts             # 日志 API（查看/导出/追踪/书签/自动清理）
│   │   │   ├── docker.ts           # Docker API（查看/导出/追踪）
│   │   │   ├── auth.ts             # 认证 API（登录/设置/CSRF）
│   │   │   ├── notifications.ts    # 通知 API
│   │   │   ├── eventLogger.ts      # 事件日志 API
│   │   │   └── update.ts           # 更新 API
│   │   ├── services/               # 服务
│   │   │   ├── logStream.ts        # 日志 WebSocket 流（含心跳+连接限制）
│   │   │   ├── dockerLogStream.ts  # Docker 日志 WebSocket 流
│   │   │   ├── notifyWebSocket.ts  # 通知 WebSocket
│   │   │   ├── autoClean.ts        # 自动清理服务（cron/秒级间隔）
│   │   │   ├── bookmark.ts         # 书签服务
│   │   │   ├── session.ts          # 会话服务（timingSafeEqual）
│   │   │   └── ...
│   │   ├── utils/                  # 工具
│   │   │   ├── validation.ts       # 输入验证/路径安全/容器名验证
│   │   │   ├── filter.ts           # 敏感信息过滤
│   │   │   ├── streamReader.ts     # 流式读取
│   │   │   ├── cache.ts            # 缓存层
│   │   │   └── ...
│   │   ├── types/                  # 类型定义
│   │   ├── notify/                 # 通知模块（22种渠道）
│   │   │   └── channels/
│   │   │       └── webhook.ts      # Webhook（含 SSRF 防护）
│   │   └── package.json
│   └── ui/                         # 前端界面
│       ├── src/
│       │   ├── components/
│       │   │   ├── LogModal.vue     # 日志查看（深色终端+追踪+导出+搜索）
│       │   │   ├── BookmarkBar.vue  # 书签栏
│       │   │   ├── AutoCleanPanel.vue # 自动清理面板
│       │   │   └── ...
│       │   ├── composables/
│       │   │   ├── useLogSearch.ts  # 搜索逻辑（Web Worker）
│       │   │   ├── useLogStream.ts  # 日志流 composable
│       │   │   └── useStore.ts      # 统一 Store
│       │   ├── workers/
│       │   │   └── logSearch.worker.ts # 搜索 Web Worker
│       │   ├── stores/             # Pinia stores
│       │   ├── services/
│       │   │   └── api.ts          # API 服务（CSRF/认证/重试）
│       │   ├── styles/
│       │   │   └── main.css        # 全局样式（鸿蒙6.0 CSS 变量）
│       │   └── ...
│       ├── images/
│       └── vite.config.ts
├── cmd/                            # 应用脚本
├── config/                         # 配置文件
├── wizard/                         # 安装向导
├── manifest                        # 应用清单
├── ICON.PNG
└── ICON_256.PNG
```

## 技术栈

### 后端
- **运行时**: Node.js 24+
- **框架**: Express 4.18.2
- **语言**: TypeScript 5.9.3
- **密码加密**: @node-rs/argon2 (Argon2id)
- **日志**: Pino 8.21.0
- **数据库**: sql.js 1.10.3 (SQLite WASM)
- **HTTP客户端**: undici 6.24.1
- **WebSocket**: ws 8.20.0

### 前端
- **框架**: Vue 3.5.31 (Composition API)
- **状态管理**: Pinia 2.1.7
- **构建工具**: Vite 7.3.1
- **语言**: TypeScript 5.9.3
- **安全**: DOMPurify 3.3.3

### 架构特点
- **状态管理**: Pinia 统一管理应用状态
- **错误处理**: 统一的错误类型和响应格式
- **性能优化**: 流式读取、缓存机制、请求去重、Web Worker 搜索
- **类型安全**: 完整的 TypeScript 类型定义
- **UI 体系**: 鸿蒙 NEXT 6.0 CSS 变量色彩体系

## 安全说明

- 密码使用 Argon2id 加密存储
- 登录失败 5 次锁定 30 分钟
- 敏感信息（密码、密钥等）自动过滤
- 审计日志记录所有敏感操作
- CSRF 双重保护（Token + SameSite Cookie）
- 路径遍历防护（isAllowedPath + safePath + isSymlinkPath 三重检查）
- Docker 容器名白名单验证，命令参数数组化防注入
- XSS 防护：所有 v-html 经 DOMPurify 净化
- SSRF 防护：Webhook URL 检测私有地址
- CSRF token 使用 crypto.timingSafeEqual 防时序攻击
- SSE/WS 连接数限制防 DoS
- Cookie httpOnly + SameSite=Lax
- 统一错误处理，生产环境隐藏堆栈信息
- 请求限流保护
- 导出格式白名单验证

## 问题反馈

如有问题或建议，请提交 [GitHub Issues](../../issues)

## 许可证

MIT License

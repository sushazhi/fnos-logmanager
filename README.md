# 飞牛日志管理 (LogManager for fnOS)

飞牛日志管理工具，集中管理飞牛三方应用散落在各个文件夹的日志文件。

## 功能特点

- **统一网关接入**
  - 通过 fnOS 统一网关访问，无需独立端口
  - 网关自动校验登录态，免密码登录
  - 原生 WebSocket 实时通信（日志流+通知推送）
  - 支持 fnOS V1.1.31+

- **多目录支持** 
  - 支持管理多个日志目录
  - 存储空间应用日志 (@appdata/@appshare 等)
  - /var/log/apps/ (系统应用日志)
  - Docker 容器日志
  - 归档日志文件 (.gz, .bz2, .xz, .zip 等)

- **多标签页日志查看** 
  - 同时打开多个日志文件
  - 标签栏切换不同日志文件
  - 非阻塞模式，查看日志时可直接点击其他文件
  - "主页"按钮返回文件列表
  - 重复打开同一文件自动激活已有标签

- **日志查看** 
  - 在线查看日志内容，支持搜索过滤
  - 流式读取大文件，内存占用低
  - 支持倒序查看最新日志
  - 深色终端风格日志显示界面
  - 关键词/正则搜索高亮（鸿蒙6.0分段控件切换）
  - Web Worker 后台搜索，不阻塞 UI

- **实时追踪** 
  - 类似 tail -f 的实时日志追踪
  - 原生 WebSocket 实时推送（统一网关模式）
  - 支持文件日志和 Docker 容器日志实时追踪
  - 自动滚动到最新行

- **日志导出** 
  - 多格式导出
  - TXT 纯文本
  - JSON 结构化
  - CSV 表格
  - 支持 Docker 容器日志导出

- **日志管理**
  - 删除已卸载应用的日志文件
  - 清空大日志文件
  - 批量清理旧归档
  - 清理已卸载应用的空文件夹

- **自动清理** 
  - 定时自动清理策略
  - 支持 cron 表达式和秒级自定义间隔
  - 按文件大小/天数/正则匹配清理
  - 独立清理规则管理

- **书签/收藏** 
  - 快速访问常用日志
  - 收藏常用日志文件/容器
  - 一键打开书签日志
  - Docker 容器书签支持

- **备份**
  - 一键备份所有日志

- **通知推送** 
  - 日志监控与多渠道通知
  - 支持 Bark、钉钉、飞书、企业微信、Telegram、QQ机器人 等 22 种通知渠道
  - 自定义监控规则，关键词匹配（支持正则表达式）
  - 日志级别过滤
  - 冷却时间与静默时段设置
  - QQ 机器人 openID 自动获取（WebSocket 监听 + 前端轮询）
  - 通知状态 WebSocket 实时推送

- **系统日志监控** 
  - 监控系统事件日志
  - 实时监控数据库事件
  - 多级别事件过滤
  - 事件统计与历史记录

- **安全特性**
  - 统一网关认证（X-Trim-* Header）+ 应用密码双重认证
  - Argon2id 密码哈希
  - 登录失败锁定（5次失败锁定30分钟）
  - 敏感信息自动过滤
  - 审计日志记录
  - CSRF 验证（网关模式自动跳过）
  - 路径遍历防护（三重检查：isAllowedPath + safePath + isSymlinkPath）
  - Docker 容器名白名单验证
  - 命令注入防护（spawn 数组参数，非 shell 拼接）
  - XSS 防护（DOMPurify 净化所有 v-html，escapeHtml 转义引号）
  - SSRF 防护（所有通知渠道 URL 私有地址检测）
  - CSRF token 时序安全比较（crypto.timingSafeEqual）
  - WebSocket Origin 验证（防跨域劫持）
  - SSE/WebSocket 连接数限制（防 DoS）
  - 敏感操作速率限制
  - 所有 GET 端点速率限制
  - 通知配置字段白名单过滤
  - 统一错误处理，生产环境隐藏堆栈和错误详情
  - CSP 安全策略（connect-src 限制、frame-ancestors 动态计算）
  - Cookie httpOnly + SameSite=Lax
  - localStorage 解析校验 + CSS 注入防护
  - Token 输入框 type=password

- **UI 设计** 
  - 鸿蒙 NEXT 6.0 设计风格
  - 全局 CSS 变量色彩体系
  - 日间/夜间主题
  - 自定义主题色（预览与实际一致）
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
| QQ机器人 | QQ开放平台机器人（自动获取openID） |
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
3. 通过 fnOS 桌面图标访问，网关自动校验登录态

> **系统要求**：fnOS V1.1.31 及以上版本（统一网关支持）

## 使用方法

### 访问

通过 fnOS 桌面点击应用图标即可访问，统一网关自动完成登录认证，无需输入密码。

### 主要功能

| 功能 | 说明 |
|------|------|
| 查看日志 | 点击日志列表中的"查看"按钮，深色终端风格显示 |
| 多标签页 | 同时打开多个日志文件，标签栏切换，非阻塞模式 |
| 实时追踪 | 查看日志时点击"追踪"按钮，WebSocket 实时推送新内容 |
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

### QQ 机器人通知配置

1. 在 QQ 开放平台创建机器人，获取 AppID 和 AppSecret
2. 添加 QQ 机器人渠道，填入 AppID 和 AppSecret
3. 点击"测试"按钮，系统启动 WebSocket 监听
4. 在 QQ 中给机器人发消息（私聊或群聊@机器人）
5. 前端自动轮询检测 openID，获取成功后自动填入
6. 保存配置后再次点击"测试"发送测试消息

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
.\build.ps1 -Version 0.6.2

# 或使用 GitHub Actions
git tag v0.6.2
git push --tags
```

## 项目结构

```
├── .github/
│   └── workflows/
│       └── build-and-release.yml   # GitHub Actions
├── app/
│   ├── server/                     # 后端服务
│   │   ├── server.ts               # 入口（统一网关前缀剥离+Unix Socket/TCP双模式）
│   │   ├── errors/                 # 错误类型定义
│   │   ├── middleware/             # 中间件
│   │   │   ├── auth.ts             # 认证/CSRF（网关模式X-Trim-* Header自动登录）
│   │   │   ├── security.ts         # CSP/安全头/输入净化
│   │   │   ├── rateLimit.ts        # 速率限制
│   │   │   └── errorHandler.ts     # 统一错误处理
│   │   ├── routes/                 # 路由
│   │   │   ├── logs.ts             # 日志 API（查看/导出/追踪/书签/自动清理）
│   │   │   ├── docker.ts           # Docker API（查看/导出/追踪）
│   │   │   ├── auth.ts             # 认证 API（登录/设置）
│   │   │   ├── notifications.ts    # 通知 API（含 QQ openID 捕获）
│   │   │   ├── eventLogger.ts      # 事件日志 API
│   │   │   └── update.ts           # 更新 API
│   │   ├── services/               # 服务
│   │   │   ├── logStream.ts        # 日志 WebSocket 流（Origin验证+连接限制）
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
│   │   │   ├── httpClient.ts       # HTTP客户端 + SSRF防护（isPrivateUrl）
│   │   │   └── channels/
│   │   │       ├── qqbot.ts        # QQ机器人（WebSocket监听+openID捕获）
│   │   │       ├── webhook.ts      # Webhook
│   │   │       └── ...
│   │   └── package.json
│   └── ui/                         # 前端界面
│       ├── src/
│       │   ├── components/
│       │   │   ├── LogModal.vue     # 日志查看（多标签+深色终端+追踪+导出+搜索）
│       │   │   ├── BookmarkBar.vue  # 书签栏
│       │   │   ├── AutoCleanPanel.vue # 自动清理面板
│       │   │   ├── NotificationPanel.vue # 通知面板（QQ轮询openID）
│       │   │   └── ...
│       │   ├── composables/
│       │   │   ├── useLogSearch.ts  # 搜索逻辑（Web Worker）
│       │   │   ├── useLogStream.ts  # 日志流 WebSocket（网关路径适配）
│       │   │   ├── useNotifyWebSocket.ts # 通知 WebSocket（网关路径适配）
│       │   │   └── useStore.ts      # 统一 Store
│       │   ├── workers/
│       │   │   └── logSearch.worker.ts # 搜索 Web Worker
│       │   ├── stores/             # Pinia stores
│       │   │   └── useLogsStore.ts # 日志Store（多标签管理）
│       │   ├── services/
│       │   │   └── api.ts          # API 服务（CSRF/认证/重试/网关前缀适配）
│       │   ├── styles/
│       │   │   └── main.css        # 全局样式（鸿蒙6.0 CSS 变量）
│       │   └── ...
│       ├── config                   # 统一网关配置（gatewaySocket+gatewayPrefix）
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
- **统一网关**: Unix Socket + 前缀剥离，无需独立端口
- **认证体系**: 网关 X-Trim-* Header 自动登录 + 应用密码可选
- **状态管理**: Pinia 统一管理应用状态（含多标签页管理）
- **错误处理**: 统一的错误类型和响应格式（isOperational + statusCode 双重检查）
- **性能优化**: 流式读取、缓存机制、请求去重、Web Worker 搜索
- **类型安全**: 完整的 TypeScript 类型定义
- **UI 体系**: 鸿蒙 NEXT 6.0 CSS 变量色彩体系

## 安全说明

- 统一网关认证（X-Trim-* Header 自动登录）+ 应用密码双重认证
- 网关模式下 CSRF/内网 IP 检查自动跳过（网关已校验）
- 密码使用 Argon2id 加密存储
- 登录失败 5 次锁定 30 分钟
- 敏感信息（密码、密钥等）自动过滤
- 审计日志记录所有敏感操作
- 路径遍历防护（isAllowedPath + safePath + isSymlinkPath 三重检查）
- Docker 容器名白名单验证，命令参数数组化防注入
- XSS 防护：所有 v-html 经 DOMPurify 净化，escapeHtml 转义引号
- SSRF 防护：所有通知渠道 URL 检测私有地址（isPrivateUrl 共享模块）
- CSRF token 使用 crypto.timingSafeEqual 防时序攻击
- WebSocket Origin 验证防跨域劫持
- SSE/WS 连接数限制防 DoS
- Cookie httpOnly + SameSite=Lax
- 统一错误处理，生产环境隐藏堆栈信息和错误详情
- 请求限流保护（含所有 GET 端点）
- 通知配置字段白名单过滤
- 导出格式白名单验证
- CSP 安全策略（connect-src 限制、frame-ancestors 动态计算主域名）
- localStorage 解析类型校验 + CSS 颜色正则白名单
- QQ 回调端点速率限制 + 事件格式验证

## 问题反馈

如有问题或建议，请提交 [GitHub Issues](../../issues)

## 许可证

MIT License

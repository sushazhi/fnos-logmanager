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
  - 支持上下文搜索

- **日志管理**
  - 删除已卸载应用的日志文件
  - 清空大日志文件
  - 批量清理旧归档
  - 清理已卸载应用的空文件夹

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
  - 登录失败锁定
  - 敏感信息过滤
  - 审计日志记录
  - CSRF 双重保护
  - 统一错误处理

- **个性化设置**
  - 日间/夜间主题
  - 自定义主题色
  - 字体大小调节

- **性能优化**
  - 流式日志读取，支持大文件
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
| 查看日志 | 点击日志列表中的"查看"按钮 |
| 删除日志 | 已卸载应用的日志会显示删除按钮 |
| 清空日志 | 查看日志时可点击"清空"按钮 |
| 查看归档 | 点击"归档日志"查看压缩的日志文件 |
| Docker日志 | 点击"Docker日志"查看容器日志 |
| 清理空文件夹 | 点击"清理空文件夹"删除已卸载应用的空目录 |
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
.\build.ps1 -Version 0.3.0

# 或使用 GitHub Actions
git tag v0.3.0
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
│   │   ├── routes/                 # 路由
│   │   ├── services/               # 服务
│   │   ├── utils/                  # 工具
│   │   │   ├── streamReader.ts     # 流式读取
│   │   │   ├── cache.ts            # 缓存层
│   │   │   └── ...
│   │   ├── types/                  # 类型定义
│   │   ├── notify/                 # 通知模块
│   │   └── package.json
│   └── ui/                         # 前端界面
│       ├── src/
│       │   ├── stores/             # Pinia stores
│       │   │   ├── useLogStore.ts
│       │   │   ├── useNotificationStore.ts
│       │   │   └── useAuthStore.ts
│       │   ├── utils/              # 工具函数
│       │   │   └── request.ts      # 请求工具
│       │   └── ...
│       ├── images/
│       └── vite.config.ts
├── cmd/                            # 应用脚本
├── config/                         # 配置文件
├── wizard/                         # 安装向导
├── manifest                        # 应用清单
├── IMPROVEMENTS.md                 # 改进说明文档
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
- **性能优化**: 流式读取、缓存机制、请求去重
- **类型安全**: 完整的 TypeScript 类型定义

## 安全说明

- 密码使用 Argon2id 加密存储
- 登录失败 5 次锁定 30 分钟
- 敏感信息（密码、密钥等）自动过滤
- 审计日志记录所有敏感操作
- CSRF 双重保护（Token + Cookie）
- 路径验证防止目录遍历
- 统一错误处理，避免信息泄露
- 请求限流保护

## 开发指南

### 使用 Pinia Store

```typescript
import { useLogStore } from '@/stores'

// 在组件中
const logStore = useLogStore()

// 加载日志
await logStore.loadLogs('/var/log/apps')

// 访问状态
console.log(logStore.logs)
console.log(logStore.loading)
console.log(logStore.error)
```

### 使用优化后的 API

```typescript
import { api } from '@/services/api'

// 启用重试
await api.get('/api/logs/list', { retry: true })

// 启用去重
await api.get('/api/logs/list', { dedupe: true })

// 支持取消
await api.get('/api/logs/list', { cancelKey: 'logs-list' })
```

### 使用错误类

```typescript
import { ValidationError, NotFoundError } from '../errors'

// 抛出验证错误
if (!path) {
  throw new ValidationError('路径不能为空')
}

// 抛出未找到错误
if (!file) {
  throw new NotFoundError('文件不存在')
}
```

### 使用流式读取

```typescript
import { readLogStream } from '../utils/streamReader'

// 流式读取日志
const result = await readLogStream(filePath, {
  maxLines: 1000,
  maxSize: 10 * 1024 * 1024,
  reverse: true
})

console.log(result.content)
console.log(result.truncated)
```

### 使用缓存

```typescript
import { globalCache, createCacheKey } from '../utils/cache'

// 设置缓存
const key = createCacheKey('logs', dirPath)
globalCache.set(key, logs, 600000) // 10分钟

// 获取缓存
const cached = globalCache.get(key)
if (cached) {
  return cached
}
```

## 问题反馈

如有问题或建议，请提交 [GitHub Issues](../../issues)

## 许可证

MIT License

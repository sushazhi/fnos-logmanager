# 飞牛日志管理 (LogManager for fnOS)

飞牛日志管理工具，集中管理飞牛三方应用散落在各个文件夹的日志文件。

## 功能特点

- **统一网关接入 & fnOS 开放平台集成**
  - 通过 fnOS 统一网关访问，无需独立端口
  - 网关自动校验登录态，免密码登录
  - 原生 WebSocket 实时通信（日志流+通知推送）
  - 接入 fnOS 开放平台 API（文件权限检查、路径转换、文件选择器）
  - 集成 fnOS JS SDK (@trimjs/web-app)，支持主题/语言/页面标题同步
  - 支持 fnOS V1.2.0401 及以上版本

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
  - 关键词/正则搜索高亮
  - Web Worker 后台搜索，不阻塞 UI

- **日志分组视图**
  - 按应用自动归并日志，解决同一应用日志散落在多个路径的问题
  - 组头显示文件名数与占用大小，便于判断哪个应用占用最多
  - 支持单组折叠 / 展开，以及「全部折叠 / 全部展开」
  - 折叠状态自动记忆，刷新后保持（含单独折叠的组）
  - 搜索时自动展开命中分组，避免结果被折叠隐藏

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

- **系统空间清理**（可视化）
  - 分析磁盘与各分区占用，用占用条 + 分类占比条直观展示「谁占了多少」
  - **扫描结果持久化 + 后台定期刷新**：一次全量扫描要逐项 `stat` 各目录树（真机上要几秒到几十秒），因此结果会落盘到 `{DataDir}/config/space_scan_cache.json`（原子写：临时文件 + `rename`），重启或升级应用后重开面板**不再等待全量扫描**
  - 数据超过 **5 分钟**新鲜期时，接口**先返回已有快照**（stale-while-revalidate），同时后台重扫；面板据此显示「数据更新于 x 前」并自动轮询收敛，用户不必对着骨架屏等
  - 后台每 **30 分钟**检查一次快照，过期就静默刷新；启动后延迟 20 秒再预热，避开开机 IO 高峰
  - 同一时刻**只允许一次全量扫描在飞**（单飞）：多个标签页或客户端同时打开面板不会把磁盘 IO 放大成 N 倍；「重新扫描」按钮走 `?refresh=true`，等待一次真正的新扫描
  - 清理后**不丢弃快照**，只把已删除的条目就地剔除并重算汇总（仍会后台重扫校正剩余体积），保证清理完再打开面板依然是秒开
  - 快照带格式版本与最长信任期（7 天）：停机过久、版本不符、时间戳异常或 JSON 损坏时一律丢弃并退化为一次正常扫描；缓存只存**路径与体积**，清理时仍逐条走服务端规则复算，不因缓存而放宽权限
  - 磁盘列表**只列真实挂载点**，并统一用飞牛语义名显示：系统空间（`/`）、存储空间N（`/volN`）。判定以内核挂载表为准（`/proc/self/mountinfo`，回退 `/proc/mounts`），读不到挂载表时退化为比较文件系统设备号
  - `/vol00`、`/vol02` 这类**看起来像存储卷、实为根文件系统上的普通目录**不再作为分区显示：它们不是独立分区，容量数字还会与系统空间完全重复
  - **外接存储不在这一屏出现**：清理规则本来就不扫 `/vol00/<盘名>` 下的外接盘，列出来只会让人以为这里能管到外接盘的空间，因此服务端不下发、前端也兜底过滤。外接盘的空间状况请看飞牛「存储管理」
  - 除存储卷外还显示**系统分区**占用（真机上最紧张的往往是它，而扫描 `/volN` 看不到）。挂载点判定不了时保留候选行——宁可多显示一行，也不能把真实磁盘藏起来
  - 按 15 个分类列出可清理项：应用中心安装包残留、包管理器缓存、运行时下载缓存、缩略图缓存、应用临时目录、Docker 容器日志、应用运行缓存、容器依赖缓存、应用数据库备份、**系统包缓存（apt 索引）、系统日志、系统零散残留、需包管理器处理（仅提示）**、系统临时目录、运行期依赖（仅提示）
  - **系统分区**上的 apt 索引缓存、系统与服务日志、dpkg 升级残留可直接清理。这些路径不在任何存储卷下，扫描 `/volN` 看不到，必须由本应用以 root 身份扫描
  - 系统分区采用更保守的四层防护：绝对路径规则表 + **兜底拒绝名单**（`/boot`、内核目录、`/etc/passwd`、`/var/lib/dpkg`、`/usr/trim` 等即使规则表被误改也永不接受）+ 符号链接零容忍 + 策略分级
  - **系统临时目录**：`/tmp`、`/var/tmp` 不在任何存储卷下，因此单独列出；判定不看文件名（临时目录里任何进程都能起任意名字），只看运行时事实——没有任何进程打开它，且闲置超过 24 小时
  - 分类默认**折叠**：分类头已给出名称、风险、条目数与可释放体积，点击分类头才展开条目明细，避免十几类子项一次性铺开把汇总信息挤出视野
  - 每项显示实际占用、文件数、最后修改时间与清理理由，勾选后清理
  - 三档安全策略：`delete` 直接删除（可再生缓存，立即释放空间）、`recycle` 移入回收站（数据库备份，保留期内可还原）、`truncate` 截断（Docker 与系统日志，持有句柄的服务无需重启即可继续写入）
  - 「只选安全项」一键勾选低风险分类；支持「试运行」预览将释放的空间而不实际删除
  - 服务端对客户端提交的每个路径独立复算规则，规则外路径一律跳过
  - 正在运行的应用对应的临时目录，以及本应用自身临时目录，不会被清理
  - 逐目录核对真实内容后，已移除三条会误删真实文件的规则（内置 Chromium 安装目录、Puppeteer 浏览器本体、pnpm 工具本体），改为提示展示

- **进程管理**
  - 列出系统中运行的进程（名称/PID/用户/状态/CPU/内存/监听端口/命令行）
  - 支持关键字过滤（进程名/命令行/PID/端口）与排序
  - 查看进程打开的文件（日志优先）
  - 查看进程日志
  - 结束进程（SIGTERM 优雅退出，超时自动升级 SIGKILL）
  - 受保护进程（PID 1、本应用自身）禁止结束

- **回收站机制**
  - 清理已卸载应用残留目录时移入系统回收站（而非直接删除）
  - 支持还原到原始位置，跨文件系统自动复制+删除还原
  - 回收站项目 24 小时自动清空

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

- **备份与恢复**
  - 一键备份所有日志
  - 备份列表管理（查看/删除）
  - 恢复前内容预览（文件清单/大小/冲突标记）
  - 恢复到原位置，可选跳过或覆盖已存在文件
  - 恢复路径三重校验（成员类型白名单 + 路径规范化 + 白名单目录），防 tar 路径穿越

- **通知推送** 
  - 日志监控与多渠道通知
  - 支持 Bark、钉钉、飞书、企业微信、Telegram、QQ机器人 等 22 种通知渠道
  - 自定义监控规则，关键词匹配（支持正则表达式）
  - 日志级别过滤
  - 冷却时间与静默时段设置
  - QQ 机器人 openID 自动获取（WebSocket 监听 + 前端轮询）
  - 钉钉扫码创建应用（自动获取 AppKey 与 AppSecret）、微信 ClawBot 扫码登录
  - 通知状态 WebSocket 实时推送

- **系统日志监控** 
  - 监控系统事件日志
  - 实时监控数据库事件
  - 多级别事件过滤
  - 事件统计与历史记录

- **MCP 服务器 (AI Agent 接入)** 
  - 标准 Model Context Protocol (Streamable HTTP) 服务端
  - 支持 QwenPAW、OpenClaw、Hermes 等 AI Agent 远程接入
  - 开放全部能力为 MCP 工具（日志读取/清理、备份、Docker、事件日志、内核管理、审计等）
  - API Key 鉴权（Authorization Bearer / X-API-Key），支持 SSE 流式响应

- **安全特性**
  - 统一网关认证（X-Trim-* Header）
  - 文件权限检查（trim.file.checkUserACL，按用户过滤日志目录和文件）
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
  - 敏感操作速率限制（按端独立计数）
  - 所有 GET 端点速率限制
  - 通知配置字段白名单过滤
  - 统一错误处理，生产环境隐藏堆栈和错误详情
  - CSP 安全策略（connect-src 限制、frame-ancestors 动态计算）
  - Cookie httpOnly + SameSite=Lax
  - localStorage 解析校验 + CSS 注入防护

- **UI 设计** 
  - 鸿蒙 7.0 Liquid Glass（液态玻璃）设计体系，含完整设计令牌系统 (Design Token System)
  - 全局 CSS 变量色彩体系（主题色 / 渐变 / 统计卡片色系 / 背景色）
  - 空间化 3D 景深阴影 (Spatial Depth Shadows)、动态光效 (Dynamic Glow)、脉冲光晕
  - 统一过渡曲线系统 (Motion Curves)：弹性回弹 / 退出 / 弹簧等完整曲线
  - 液态玻璃表面、聚焦环 (Focus Ring)、空间化变换 (Spatial Transforms)
  - 涟漪按钮动效 (hm-ripple)、骨架屏加载态 (hm-shimmer)、动态彩虹渐变标题栏
  - 日间 / 夜间主题与鸿蒙 7.0 夜间主题重制
  - 自定义主题色（预览与实际一致）、字体大小调节
  - 深色终端风格日志显示、全组件移动端适配（弹窗 / 面板 / 卡片 / 列表）
  - 底部弹出式面板 (Bottom Sheet)、触控友好交互（44px 最小触摸目标）

- **性能优化**
  - 流式日志读取，支持大文件
  - Web Worker 后台搜索
  - 虚拟滚动（10万+行流畅滚动）
  - 内存缓存机制
  - 请求去重和重试
  - 代码分割优化加载
  - 日志目录单次遍历 + TTL 缓存 + single-flight，首页与列表共享同一次扫描
  - 跳过三方应用自带的依赖缓存目录（node_modules / pnpm store / npm _cacache 等），
    避免为找出少量日志而遍历数万条目
  - 展示路径批量转换，避免逐个文件跨进程调用 trim API

## 支持的通知渠道

| 渠道 | 说明 |
|------|------|
| Bark | iOS 推送应用 |
| 钉钉机器人 | 钉钉群机器人 |
| 钉钉应用 | 钉钉企业内部应用（新版 API，支持扫码创建） |
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
| Synology Chat | 群晖聊天 |
| QMsg | QQ推送（v3 接口） |
| PushMe | 推送服务 |
| WxPusher | 微信推送 |
| AIBotK | 智能机器人 |
| 微信 ClawBot | 微信个人号机器人（iLink 协议，扫码登录） |

## 安装

1. 下载最新的 `.fpk` 文件 from [Releases](../../releases)
2. 在飞牛 NAS 应用中心安装
3. 通过 fnOS 桌面图标访问，网关自动校验登录态

> **系统要求**：fnOS V1.2.0401 及以上版本（统一网关 + 开放平台 API 支持）

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
| 日志分组 | 文件列表按应用分组，可折叠/展开，状态自动记忆 |
| 删除日志 | 已卸载应用的日志会显示删除按钮 |
| 清空日志 | 查看日志时可点击"清空"按钮 |
| 查看归档 | 点击"归档日志"查看压缩的日志文件 |
| Docker日志 | 点击"Docker日志"查看容器日志 |
| 进程管理 | 查看运行进程、进程日志，支持结束进程 |
| 清理残留 | 清理已卸载应用的残留目录（移入回收站，可还原） |
| 回收站还原 | 查看回收站项目并还原到原始位置 |
| 自动清理 | 配置定时清理规则，按大小/天数/模式自动清理 |
| 通知设置 | 点击"通知设置"配置监控规则和通知渠道 |
| 系统日志 | 点击"系统日志"监控系统事件 |

### 钉钉机器人扫码配置

**方式一：钉钉应用（推荐，支持扫码）**

1. 添加渠道时选择「钉钉应用」
2. 点击「📱 扫码创建应用」，页面显示二维码
3. 使用钉钉 App 扫描二维码，按提示在手机上完成应用创建与授权确认
4. 授权成功后，AppKey 与 AppSecret 自动填入表单
5. 填写接收人的 userId（可填多个，英文逗号分隔）
6. 保存配置后点击「测试」发送测试消息

该方式使用钉钉新版 API（`api.dingtalk.com`），通过 AppKey/AppSecret 换取 accessToken 后主动推送，支持指定用户接收。

**方式二：钉钉群机器人（手工配置）**

1. 在钉钉群中添加自定义机器人，获取 Webhook 的 access_token 与加签密钥
2. 添加渠道时选择「钉钉机器人」，填入 Token 和 Secret
3. 保存后点击「测试」

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
- 切换主题
- 更改主题色
- 查看审计日志

## MCP 服务器接入 (AI Agent)

本应用内置标准 **Model Context Protocol (Streamable HTTP)** 服务器，可将全部日志管理能力开放给
QwenPAW、OpenClaw、Hermes 等 AI Agent。

### 端点

```
http://<NAS-IP>:<独立端口>/mcp
```

> 端口在应用内「设置 → MCP 服务器 → 独立监听端口」中配置（未配置端口时 MCP 不启动）。
> 未设置 `MCP_API_KEY` 时只允许本机回环访问；对外提供访问请同时配置密钥，
> 绑定地址可用 `MCP_BIND_ADDR`（默认 `0.0.0.0`）。

> ⚠️ 不要使用统一网关路径 `http://<NAS-IP>/app/logmanager/mcp`：外部 AI Agent 没有
> 飞牛登录态，会被网关注入的鉴权逻辑拦截；且 fnOS 1.2.0604 起网关会把请求中的
> `Authorization: Bearer <key>` 当作飞牛自身票据校验，直接返回纯文本 `invalid token`
> （HTTP 200），请求根本到不了应用。因此 MCP 必须走独立端口。

### 配置 API Key

通过环境变量启用鉴权（推荐，可防止未授权访问）：

```
MCP_ENABLED=true          # 默认 true
MCP_API_KEY=你的强密钥     # 不设置则仅允许本机回环访问
MCP_APP_NAME=fnos-logmanager  # 可选，展示给 Agent 的名称
```

### 客户端配置示例

在 QwenPAW / OpenClaw / Hermes 的 MCP 客户端配置中添加：

```json
{
  "mcpServers": {
    "logmanager": {
      "type": "http",
      "url": "http://<NAS-IP>:<独立端口>/mcp",
      "headers": { "Authorization": "Bearer 你的强密钥" }
    }
  }
}
```

### 可用工具

| 分类 | 工具 |
|------|------|
| 日志目录/列表 | `list_dirs` `list_logs` `search_logs` `get_app_names` `get_log_stats` |
| 日志读取 | `read_log` `tail_log` `read_archive` |
| 日志管理 | `truncate_log` `delete_log` `clean_logs` `clean_empty_dirs` |
| 残留清理/回收站 | `clean_uninstalled_dirs` `list_recycle_items` `restore_recycle_item` |
| 进程管理 | `list_processes` `kill_process` |
| 备份/归档 | `backup_logs` `list_backups` `delete_backup` `clean_backups` `list_archives` |
| Docker | `list_docker_containers` `get_docker_logs` |
| 事件日志 | `get_event_logs` `get_event_sources` `event_logger_status` |
| 内核管理 | `list_kernels` `remove_kernel` `cleanup_kernels` |
| 系统信息 | `get_system_info` `convert_path` `get_audit_logs` `get_app_version` |

> 所有破坏性操作（清空/删除/清理/卸载内核）均写入安全审计日志。

## 本地构建

### 前置要求

- Go 1.27+
- Node.js 24+

### 构建步骤

跨平台 Go 构建器（`build/`，独立 module、仅用标准库），Windows/Linux/macOS 通用，无需 Python。

> ⚠️ **先编译再运行**：构建器位于独立 module（`build/go.mod`），根目录没有 `go.mod`，
> 因此**不能**在根目录直接 `go run ./build` —— Go 1.27 会报
> `cannot find main module`。请按下面的方式先编译为二进制再执行：

```bash
# 1) 编译构建器（只需一次，后续可复用）
cd build && go build -o ../.local-build/buildtool .

# 2) 指定版本号构建（推荐：版本号只走 -version，不改动版本文件）
cd .. && ./.local-build/buildtool -version 0.8.1.4

# 省略 -version 则使用 manifest 中的版本号
./.local-build/buildtool

# 跳过 Vue 构建（仅重新编译 Go 服务 + 打包）
./.local-build/buildtool -skip-vue

# 强制重新下载所有依赖
./.local-build/buildtool -force

# 强制全量构建（忽略所有缓存）
./.local-build/buildtool -clean
```

Windows 下二进制为 `.local-build\buildtool.exe`，调用时替换路径分隔符即可。

参数：`-version`/`-v`、`-force`/`-f`、`-skip-vue`、`-clean`。

> **关于版本号**：`-version` 只影响产物文件名（`logmanager-<version>.fpk`）和包内 `manifest` 的版本字段，**不会修改**项目根目录的 `manifest`。正式发版请先手动更新根 `manifest` 的版本 —— CI 用它决定发布版本，应用内更新检查的兜底路径也读它；`-version` 只适合临时本地构建。

或使用 GitHub Actions（基于 tag 自动构建发布）：

```bash
git tag v0.8.0
git push --tags
```

## 项目结构

```
├── .github/
│   └── workflows/
│       └── build-and-release.yml   # GitHub Actions
├── app/
│   ├── server/                     # 后端服务 (Go)
│   │   ├── cmd/server/             # 入口
│   │   ├── internal/
│   │   │   ├── config/             # 配置管理
│   │   │   ├── errors/             # 错误类型定义
│   │   │   ├── middleware/         # 中间件（认证/CSRF/CSP/速率限制/错误处理）
│   │   │   ├── mcp/               # MCP 服务器（Streamable HTTP + 全部能力工具）
│   │   │   ├── notify/            # 通知模块（22种渠道 + SSRF防护）
│   │   │   ├── routes/            # 路由（日志/Docker/通知/事件/更新）
│   │   │   ├── services/          # 服务（日志流/WebSocket/自动清理/监控/书签）
│   │   │   ├── types/             # 类型定义
│   │   │   └── utils/             # 工具（路径安全/SSRF/IP/验证/过滤）
│   │   ├── go.mod
│   │   └── go.sum
│   └── ui/                         # 前端界面 (Vue 3 + Vite)
│       ├── src/
│       │   ├── components/
│       │   │   ├── LogModal.vue     # 日志查看（多标签+深色终端+追踪+导出+搜索）
│       │   │   ├── LogListCard.vue  # 日志列表抽屉（按应用分组+折叠展开+状态记忆+搜索）
│       │   │   ├── BookmarkBar.vue  # 书签栏
│       │   │   ├── AutoCleanPanel.vue # 自动清理面板
│       │   │   ├── NotificationPanel.vue # 通知面板（QQ轮询openID）
│       │   │   ├── ConfirmDialog.vue # 鸿蒙7风格确认对话框
│       │   │   ├── AlertDialog.vue  # 鸿蒙7风格提示对话框
│       │   │   └── ...
│       │   ├── composables/
│       │   │   ├── useLogSearch.ts  # 搜索逻辑（Web Worker）
│       │   │   ├── useLogStream.ts  # 日志流 WebSocket（网关路径适配）
│       │   │   ├── useNotifyWebSocket.ts # 通知 WebSocket
│       │   │   └── useStore.ts      # 统一 Store
│       │   ├── workers/
│       │   │   └── logSearch.worker.ts # 搜索 Web Worker
│       │   ├── stores/             # Pinia stores
│       │   │   └── useLogsStore.ts # 日志Store（多标签管理）
│       │   ├── services/
│       │   │   ├── api.ts          # API 服务（CSRF/认证/重试/网关前缀适配）
│       │   │   └── fnos.ts         # fnOS 开放平台 JS SDK 完整封装（25个方法，可复用）
│       │   ├── styles/
│       │   │   └── main.css        # 全局样式（鸿蒙7.0 CSS 变量 + 设计令牌）
│       │   └── ...
│       ├── config                   # 统一网关配置（gatewaySocket+gatewayPrefix）
│       ├── images/
│       └── vite.config.ts
├── cmd/                            # 应用脚本
├── config/                         # 配置文件
├── wizard/                         # 安装向导
├── manifest                        # 应用清单
├── build/                          # Go 跨平台构建器（独立 module）
│   ├── go.mod
│   └── main.go
├── ICON.PNG
└── ICON_256.PNG
```

## 技术栈

### 后端
- **语言**: Go 1.27+
- **框架**: Gin 1.10+
- **数据库**: modernc.org/sqlite (纯 Go SQLite)
- **WebSocket**: gorilla/websocket
- **HTTP客户端**: net/http (含 SSRF 防护)

### 前端
- **框架**: Vue 3.5+ (Composition API)
- **状态管理**: Pinia 4+
- **构建工具**: Vite 8+
- **语言**: TypeScript 6+
- **安全**: DOMPurify 3+
- **UI设计**: 鸿蒙 7.0 Liquid Glass 设计语言（液态玻璃、动态光效、统一缓动曲线）

### 架构特点
- **统一网关**: Unix Socket + 前缀剥离，无需独立端口
- **开放平台集成**: 接入 fnOS 开放 API（文件权限/路径转换/文件选择器），集成 JS SDK
- **认证体系**: 网关 X-Trim-* Header 自动登录 + trim API Token 认证
- **状态管理**: Pinia 统一管理应用状态（含多标签页管理）
- **错误处理**: 统一的错误类型和响应格式
- **性能优化**: 流式读取、缓存机制、请求去重、Web Worker 搜索
- **类型安全**: 完整的 Go 和 TypeScript 类型定义
- **UI 体系**: 鸿蒙 7.0 Liquid Glass CSS 变量色彩体系

## 安全说明

- 统一网关认证（X-Trim-* Header 自动登录）
- 网关模式下 CSRF/内网 IP 检查自动跳过（网关已校验）
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
- 请求限流保护（含按端独立计数的敏感操作速率限制 + GET 端点限制）
- 通知配置字段白名单过滤
- 导出格式白名单验证
- CSP 安全策略（connect-src 限制、frame-ancestors 动态计算主域名）
- localStorage 解析类型校验 + CSS 颜色正则白名单
- QQ 回调端点速率限制 + 事件格式验证

## 问题反馈

如有问题或建议，请提交 [GitHub Issues](../../issues)

## 许可证

本项目以 MIT License 发布，详见 [LICENSE](LICENSE)。

第三方依赖（Go / npm）的许可证与版权声明见 [THIRD-PARTY-LICENSES.md](THIRD-PARTY-LICENSES.md)。

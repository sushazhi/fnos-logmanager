# 飞牛日志管理 (LogManager for fnOS)

飞牛日志管理工具，集中管理飞牛三方应用散落在各个文件夹的日志文件。

## 功能特点

- 📁 **多目录支持** - 支持管理多个日志目录
  - 存储空间应用日志 (@appdata/@appshare 等)
  - /var/log/apps/ (系统应用日志)
  - Docker 容器日志
  - 归档日志文件 (.gz, .bz2, .xz, .zip 等)

- 🔍 **日志查看** - 在线查看日志内容，支持搜索过滤

- 🗑️ **日志管理**
  - 删除已卸载应用的日志文件
  - 清空大日志文件
  - 批量清理旧归档

- 📦 **备份压缩**
  - 日志文件压缩
  - 一键备份所有日志

- 🔐 **安全特性**
  - 密码登录保护
  - 登录失败锁定
  - 敏感信息过滤
  - 审计日志记录

- 🎨 **个性化设置**
  - 日间/夜间主题
  - 自定义主题色
  - 字体大小调节

## 安装

1. 下载最新的 `.fpk` 文件 from [Releases](../../releases)
2. 在飞牛 NAS 应用中心安装
3. 安装向导中设置登录密码

> ⚠️ **注意**：本应用仅在 ARM64 架构测试通过，AMD64 架构请自测。

## 使用方法

### 登录

使用安装时设置的密码登录应用。

### 主要功能

| 功能 | 说明 |
|------|------|
| 查看日志 | 点击日志列表中的"查看"按钮 |
| 删除日志 | 已卸载应用的日志会显示删除按钮 |
| 清空日志 | 查看日志时可点击"清空"按钮 |
| 查看归档 | 点击"归档日志"查看压缩的日志文件 |
| Docker日志 | 点击"Docker日志"查看容器日志 |

### 设置

点击右上角 ⚙️ 图标进入设置：
- 修改密码
- 切换主题
- 更改主题色
- 查看审计日志

## 本地构建

### 前置要求

- Node.js 22+
- PowerShell (Windows) 或 Bash (Linux)

### 构建步骤

```bash
# Windows
.\build.ps1 -Version 0.1.0

# 或使用 GitHub Actions
git tag v0.1.0
git push --tags
```

## 项目结构

```
├── .github/
│   └── workflows/
│       └── build-and-release.yml   # GitHub Actions
├── app/
│   ├── server/                     # 后端服务
│   │   ├── server.js
│   │   └── package.json
│   └── ui/                         # 前端界面
│       ├── src/
│       ├── images/
│       └── vite.config.js
├── cmd/                            # 应用脚本
├── config/                         # 配置文件
├── wizard/                         # 安装向导
├── manifest                        # 应用清单
├── ICON.PNG
└── ICON_256.PNG
```

## 技术栈

- **后端**: Node.js + Express
- **前端**: Vue 3 + Vite
- **打包**: fnpack

## 安全说明

- 密码使用 SHA-256 加密存储
- 登录失败 5 次锁定 30 分钟
- 敏感信息（密码、密钥等）自动过滤
- 审计日志记录所有敏感操作

## 问题反馈

如有问题或建议，请提交 [GitHub Issues](../../issues)

## 许可证

MIT License

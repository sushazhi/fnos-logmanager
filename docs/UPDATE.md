# 应用自更新配置指南

## 概述

本项目已实现应用自更新功能，直接从 GitHub Releases 检查更新，使用代理加速下载。

## 功能特性

- ✅ 自动版本检查（启动时检查）
- ✅ 从 GitHub Releases 获取更新
- ✅ 使用代理加速下载
- ✅ 一键自动更新（下载+安装+重启）
- ✅ 保留用户数据
- ✅ 架构自动适配（x86/arm）
- ✅ 更新进度显示
- ✅ 后台静默安装
- ✅ 支持忽略版本
- ✅ 24小时内不重复提醒

## 配置说明

### 1. GitHub 仓库配置

更新功能已配置为从 GitHub Releases 获取更新：

```javascript
// app/server/routes/update.js
const GITHUB_REPO = 'sushazhi/fnos-logmanager';
const GITHUB_API = 'https://api.github.com';
```

### 2. 代理配置

使用以下代理加速 GitHub 下载：

```javascript
// app/server/routes/update.js
const MAIN_PROXY = 'https://hk.gh-proxy.org/';
const BINARY_PROXY = 'https://ghfast.top/';
```

### 3. 发布更新

#### 3.1 构建更新包

```bash
# Windows
powershell -ExecutionPolicy Bypass -File build.ps1 -Version 0.3.0

# Linux/Mac
./build.sh 0.3.0
```

#### 3.2 创建 GitHub Release

1. 访问 https://github.com/sushazhi/fnos-logmanager/releases/new
2. 填写版本号（如 `v0.3.0`）
3. 填写更新日志
4. 上传构建好的 FPK 文件（文件名包含 `logmanager` 且以 `.fpk` 结尾即可）
5. 发布 Release

**文件命名示例：**
- `logmanager-0.3.0.fpk`
- `logmanager-x86.fpk`
- `logmanager-arm.fpk`

**注意：** 更新检查不区分架构，会自动查找第一个匹配的 FPK 文件。

## 更新流程

```
应用启动时自动检查更新
    ↓
前端调用 /api/update/check
    ↓
后端查询 GitHub Releases API
    ↓
获取最新版本和下载链接
    ↓
发现新版本，显示更新通知
    ↓
用户点击"立即更新"
    ↓
前端调用 /api/update/install
    ↓
后端使用代理下载 FPK 包
    ↓
创建配置文件（保留用户数据）
    ↓
解压 FPK 包
    ↓
执行 appcenter-cli install-local
    ↓
应用自动重启
    ↓
更新完成
```

## 用户操作

### 检查更新

应用启动时会自动检查更新，也可以手动点击版本号触发检查。

### 更新操作

当发现新版本时，会显示更新通知，用户可以：

1. **立即更新**：点击后自动下载、安装并重启应用
2. **忽略此版本**：不再提醒该版本
3. **稍后提醒**：关闭通知，24小时内不再提醒

## 前端集成

### 组件说明

项目提供了两个更新相关组件：

1. **UpdateNotification.vue** - 更新通知组件（右下角弹窗）
   - 显示新版本信息
   - 显示更新日志
   - 提供"立即更新"、"忽略此版本"、"稍后提醒"按钮

2. **UpdateChecker.vue** - 更新检查组件（对话框形式）
   - 手动检查更新
   - 显示更新进度
   - 显示更新日志

### 使用方法

在应用的主界面中添加更新检查组件：

```vue
<template>
  <div>
    <!-- 其他内容 -->
    <UpdateNotification 
      v-if="updateInfo" 
      :updateInfo="updateInfo" 
      :currentVersion="appVersion"
      @close="updateInfo = null"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useUpdate } from './composables/useUpdate'
import UpdateNotification from './components/UpdateNotification.vue'

const { appVersion, checkForUpdates } = useUpdate()
const updateInfo = ref(null)

onMounted(async () => {
  updateInfo.value = await checkForUpdates()
})
</script>
```

## API 接口

### 1. 检查更新

**请求：**
```
GET /api/update/check
```

**响应：**
```json
{
  "success": true,
  "currentVersion": "0.2.0",
  "latestVersion": "0.3.0",
  "hasUpdate": true,
  "changelog": "更新内容说明...",
  "publishedAt": "2026-03-07T00:00:00Z",
  "message": "发现新版本"
}
```

### 2. 安装更新

**请求：**
```
POST /api/update/install
```

**响应：**
```json
{
  "success": true,
  "message": "开始下载更新，请稍候..."
}
```

### 3. 获取更新状态

**请求：**
```
GET /api/update/status
```

**响应：**
```json
{
  "success": true,
  "ready": "true",
  "updating": true,
  "updateProgress": 45,
  "updateMessage": "正在下载更新包..."
}
```

## 版本号规范

使用语义化版本号：`MAJOR.MINOR.PATCH`

- **MAJOR**：重大更新，不兼容的 API 修改
- **MINOR**：新增功能，向下兼容
- **PATCH**：Bug 修复，向下兼容

示例：
- `0.1.0` → `0.1.1`：Bug 修复
- `0.1.0` → `0.2.0`：新增功能
- `0.1.0` → `1.0.0`：重大更新

## 代理说明

### 为什么需要代理？

GitHub 在中国大陆访问速度较慢，使用代理可以加速下载。

### 使用的代理

- **主代理**：`https://hk.gh-proxy.org/`
- **二进制代理**：`https://ghfast.top/`

### 如何修改代理？

编辑 `app/server/routes/update.js`：

```javascript
const MAIN_PROXY = 'https://your-proxy.com/';
const BINARY_PROXY = 'https://your-binary-proxy.com/';
```

## 安全考虑

1. **HTTPS**：GitHub API 和下载都使用 HTTPS
2. **权限控制**：更新操作需要用户认证
3. **审计日志**：记录所有更新操作
4. **数据保留**：更新时自动保留用户数据

## 测试更新

### 本地测试

1. 修改版本号为较低版本：
   ```bash
   # manifest
   version = 0.1.0
   ```

2. 构建并安装应用

3. 在 GitHub 上发布更高版本的 Release

4. 触发更新检查

### 生产环境

1. 确保 GitHub Releases 正确发布
2. 测试代理是否可用
3. 准备好回滚方案
4. 监控更新成功率

## 故障排查

### 更新失败

1. 检查网络连接
2. 检查 GitHub API 是否可访问
3. 检查代理是否可用
4. 检查应用日志：`/var/log/apps/logmanager/`
5. 检查更新目录：`/vol1/@appshare/logmanager/update/`

### 版本检查失败

1. 检查 GitHub 仓库地址是否正确
2. 检查 GitHub API 限流（未认证限制 60 次/小时）
3. 检查 Release 是否正确发布
4. 检查 FPK 文件名是否包含 `logmanager` 且以 `.fpk` 结尾

### 下载失败

1. 检查代理是否可用
2. 尝试更换代理地址
3. 检查 GitHub Release 文件是否存在

## GitHub API 限流

GitHub API 对未认证请求有限流：
- **未认证**：60 次/小时
- **认证**：5000 次/小时

建议：
- 使用缓存减少 API 调用
- 每天只检查一次更新
- 如需更高配额，可添加 GitHub Token

## 相关文件

- `app/server/routes/update.js` - 后端更新 API
- `app/ui/src/composables/useUpdate.js` - 前端更新逻辑（composable）
- `app/ui/src/components/UpdateNotification.vue` - 更新通知组件
- `app/ui/src/components/UpdateChecker.vue` - 更新检查组件

## 支持

如有问题，请提交 Issue：https://github.com/sushazhi/fnos-logmanager/issues

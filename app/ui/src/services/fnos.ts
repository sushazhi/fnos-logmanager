/**
 * ============================================================================
 * fnOS Open Platform JS SDK 完整封装
 * ============================================================================
 *
 * 基于 @trimjs/web-app v0.4.2，封装 fnOS 开放平台全部前端能力。
 *
 * 用途：供 fnOS 生态应用开发使用，可直接复制此文件到你的项目中。
 *
 * 依赖：npm install @trimjs/web-app
 * 文档：https://developer.fnnas.com/
 *
 * 使用方法：
 *   import { sdk, setTitle, pickUserFile, onThemeChange, ... } from './services/fnos'
 *
 * 环境兼容：
 *   - fnOS Web 宿主环境：所有 API 正常工作
 *   - 独立浏览器环境：自动降级，不报错
 *
 * ============================================================================
 */

import { TrimApp } from '@trimjs/web-app'

// ============================================================================
// 从 SDK 包导出类型（方便外部引用）
// ============================================================================

import type {
  AppBridgeResponse,
  FilePickerParams,
  PlatformConfig,
  FileDetailsOptions,
  TrimAppOptions,
  HostSnapshot,
  AppAuthResult,
  AppAuthMethod,
  AppAuthCallbackStatus,
  AppAuthCallbackError,
  AppAuthMethodParamsMap,
  AppAuthBaseParams,
  AppAuthPickFileParams,
  AppAuthPickSharedFileParams,
  AppAuthAuthorizeParams,
} from '@trimjs/web-app'

export type {
  AppBridgeResponse,
  FilePickerParams,
  PlatformConfig,
  FileDetailsOptions,
  TrimAppOptions,
  HostSnapshot,
  AppAuthResult,
  AppAuthMethod,
  AppAuthCallbackStatus,
  AppAuthCallbackError,
  AppAuthMethodParamsMap,
  AppAuthBaseParams,
  AppAuthPickFileParams,
  AppAuthPickSharedFileParams,
  AppAuthAuthorizeParams,
}

// ============================================================================
// SDK 实例（可直接导出给外部使用）
// ============================================================================

/** TrimApp 实例，可直接用于调用底层 SDK 方法 */
export const sdk = new TrimApp()

/** 当前是否在 fnOS Web 宿主环境中 */
export const isWeb = sdk.isWeb

/** 当前是否在独立浏览器环境中（Standalone Web） */
export const isStandaloneWeb = sdk.isStandaloneWeb

// ============================================================================
// JSSDK 错误码
// ============================================================================

export const JSSDKErrorCode = {
  SUCCESS: 0,
  INTERNAL_ERROR: 1000000,
  AUTH_FAILED: 1000001,
  ADMIN_REQUIRED: 1000002,
  INVALID_REQUEST: 1000030,
  APP_NOT_FOUND: 1000300,
  PATH_NOT_FOUND: 1000701,
  APP_PERMISSION_FAILED: 1003103,
  USER_AUTH_DISABLED: 1003201,
} as const

export type JSSDKErrorCodeValue = (typeof JSSDKErrorCode)[keyof typeof JSSDKErrorCode]

// ============================================================================
// API 错误码
// ============================================================================

export const APIErrorCode = {
  INVALID_PARAMS: 200001,
  UNAUTHORIZED: 200004,
  FORBIDDEN: 200003,
  NOT_FOUND: 200005,
  INTERNAL_ERROR: 200006,
} as const

export type APIErrorCodeValue = (typeof APIErrorCode)[keyof typeof APIErrorCode]

// ============================================================================
// 环境判断
// ============================================================================

/** 是否在 fnOS 宿主环境中运行 */
export function isFnosEnvironment(): boolean {
  return sdk.isWeb
}

/** 是否在独立浏览器中运行 */
export function isStandaloneEnvironment(): boolean {
  return sdk.isStandaloneWeb
}

// ============================================================================
// 一、文件选择器（授权入口）
// ============================================================================

/**
 * 打开宿主文件选择器（仅选择，不授权）。
 *
 * @param options - FilePickerParams
 * @returns AppBridgeResponse<string[]> 或 null（非 fnOS 环境）
 *
 * @example
 * ```ts
 * const result = await pickFile({ directory: true, title: '选择文件' })
 * if (result?.code === 0) console.log(result.data)
 * ```
 */
export async function pickFile(options: FilePickerParams): Promise<AppBridgeResponse<string[]> | null> {
  if (!sdk.isWeb) return null
  try {
    const result = await sdk.pickFile(options)
    return result as unknown as AppBridgeResponse<string[]> | null
  } catch {
    return null
  }
}

/**
 * 用户个人授权：选择文件或目录并完成授权。
 *
 * @param options - FilePickerParams
 * @returns AppBridgeResponse<string[]> 或 null
 *
 * @example
 * ```ts
 * const result = await pickUserFile({ directory: true, title: '选择日志目录' })
 * if (result?.code === 0) console.log('已授权:', result.data)
 * ```
 */
export async function pickUserFile(options: FilePickerParams): Promise<AppBridgeResponse<string[]> | null> {
  if (!sdk.isWeb) return null
  try {
    return (await sdk.pickUserFile(options)) || null
  } catch {
    return null
  }
}

/**
 * 应用共享授权：管理员选择共享目录。
 *
 * @param options - FilePickerParams（host 控制 directory，调用方不应传 directory 参数）
 * @returns AppBridgeResponse<string[]> 或 null
 *
 * @example
 * ```ts
 * const result = await pickSharedFile({ title: '选择共享目录' })
 * if (result?.code === 0) console.log('共享目录:', result.data)
 * ```
 */
export async function pickSharedFile(options: FilePickerParams): Promise<AppBridgeResponse<string[]> | null> {
  if (!sdk.isWeb) return null
  try {
    return (await sdk.pickSharedFile(options)) || null
  } catch {
    return null
  }
}

/**
 * 按已知路径重新申请用户文件授权。
 *
 * @param path - 文件或目录路径
 * @returns AppBridgeResponse<string[]> 或 null
 *
 * @example
 * ```ts
 * const result = await authorizeUserFile('/vol1/@appdata/myapp')
 * ```
 */
export async function authorizeUserFile(path: string): Promise<AppBridgeResponse<string[]> | null> {
  if (!sdk.isWeb) return null
  try {
    return (await sdk.authorizeUserFile(path)) || null
  } catch {
    return null
  }
}

/**
 * 按已知路径重新申请共享文件授权。
 *
 * @param path - 文件或目录路径
 * @returns AppBridgeResponse<string[]> 或 null
 *
 * @example
 * ```ts
 * const result = await authorizeSharedFile('/vol1/@appdata/myapp')
 * ```
 */
export async function authorizeSharedFile(path: string): Promise<AppBridgeResponse<string[]> | null> {
  if (!sdk.isWeb) return null
  try {
    return (await sdk.authorizeSharedFile(path)) || null
  } catch {
    return null
  }
}

// ============================================================================
// 二、页面路由
// ============================================================================

/**
 * 通过宿主系统打开文件。
 *
 * @param path - 文件路径
 *
 * @example
 * ```ts
 * await openFile('/vol1/@appdata/myapp/readme.txt')
 * ```
 */
export async function openFile(path: string): Promise<void> {
  if (!sdk.isWeb) return
  await sdk.openFile(path)
}

/**
 * 显示文件详情面板。
 *
 * @param paths - 文件路径数组
 * @param options - 可选，如 { admin: true }
 *
 * @example
 * ```ts
 * await showFileDetails(['/vol1/@appdata/myapp/readme.txt'])
 * ```
 */
export async function showFileDetails(paths: string[], options?: FileDetailsOptions): Promise<void> {
  if (!sdk.isWeb) return
  await sdk.showFileDetails(paths, options)
}

/**
 * 打开文件管理器并导航到指定路径。
 *
 * @param path - 目标路径
 *
 * @example
 * ```ts
 * await openFileManager('/vol1/@appdata')
 * ```
 */
export async function openFileManager(path: string): Promise<void> {
  if (!sdk.isWeb) return
  await sdk.openFileManager(path)
}

/**
 * 打开当前应用的设置页面。
 *
 * @example
 * ```ts
 * await openAppSetting()
 * ```
 */
export async function openAppSetting(): Promise<void> {
  if (!sdk.isWeb) return
  await sdk.openAppSetting()
}

/**
 * 在宿主系统浏览器中打开 URL。
 *
 * @param url - 目标 URL
 * @param target - 窗口 target，默认 '_blank'
 * @param features - 窗口特性（仅 Web 宿主有效，对应 window.open 的 features）
 *
 * @example
 * ```ts
 * await openURL('https://example.com/docs')
 * ```
 */
export async function openURL(url: string, target?: string, features?: string): Promise<void> {
  if (!sdk.isWeb) {
    window.open(url, target || '_blank', features)
    return
  }
  await sdk.openURL(url, target, features)
}

// ============================================================================
// 三、页面交互
// ============================================================================

/**
 * 设置当前应用页面的标题。
 *
 * @param title - 页面标题
 *
 * @example
 * ```ts
 * await setTitle('我的应用')
 * ```
 */
export async function setTitle(title: string): Promise<void> {
  if (sdk.isWeb) {
    await sdk.setTitle(title)
  } else {
    document.title = title
  }
}

/**
 * 设置或清除用户离开页面时的确认提示。
 *
 * @param params - { title?, content? }，传 undefined 清除提示
 *
 * @example
 * ```ts
 * // 设置提示
 * await setExitPageTips({ title: '确认离开', content: '有未保存的数据' })
 * // 清除提示
 * await setExitPageTips()
 * ```
 */
export async function setExitPageTips(params?: {
  title?: string
  content?: string
}): Promise<void> {
  if (sdk.isWeb) {
    await sdk.setExitPageTips(params)
  } else {
    if (params) {
      const msg = params.content || params.title || ''
      window.onbeforeunload = msg ? () => msg : null
    } else {
      window.onbeforeunload = null
    }
  }
}

/**
 * 关闭当前应用页面。
 *
 * @example
 * ```ts
 * await close()
 * ```
 */
export async function close(): Promise<void> {
  if (!sdk.isWeb) return
  await sdk.close()
}

// ============================================================================
// 四、事件监听
// ============================================================================

/**
 * 监听宿主系统主题变化。
 *
 * @param callback - 主题变化回调，theme: 'dark' | 'light'
 * @returns 取消监听的函数
 *
 * @example
 * ```ts
 * const off = onThemeChange((theme) => {
 *   document.documentElement.classList.toggle('dark', theme === 'dark')
 * })
 * // 取消监听
 * off()
 * ```
 */
export function onThemeChange(callback: (theme: 'dark' | 'light') => void): () => void {
  if (!sdk.isWeb) return () => {}
  const handler = (payload: unknown) => {
    const theme = (payload as Record<string, unknown>)?.theme
    callback(theme === 'dark' ? 'dark' : 'light')
  }
  sdk.$on('os/theme', handler)
  return () => {
    try { sdk.$off('os/theme', handler) } catch { /* ignore */ }
  }
}

/**
 * 监听宿主系统语言变化。
 *
 * @param callback - 语言变化回调，lang: string（如 'zh-CN', 'en-US'）
 * @returns 取消监听的函数
 *
 * @example
 * ```ts
 * const off = onLanguageChange((lang) => {
 *   i18n.global.locale.value = lang
 * })
 * // 取消监听
 * off()
 * ```
 */
export function onLanguageChange(callback: (lang: string) => void): () => void {
  if (!sdk.isWeb) return () => {}
  const handler = (payload: unknown) => {
    const lang = (payload as Record<string, unknown>)?.language as string || 'zh-CN'
    callback(lang)
  }
  sdk.$on('os/language', handler)
  return () => {
    try { sdk.$off('os/language', handler) } catch { /* ignore */ }
  }
}

// ============================================================================
// 五、平台配置
// ============================================================================

/**
 * 获取平台配置（主题、语言、版本等）。
 *
 * @returns PlatformConfig 或 null
 *
 * @example
 * ```ts
 * const config = await getPlatformConfig()
 * console.log(config?.theme, config?.language, config?.systemVersion)
 * ```
 */
export async function getPlatformConfig(): Promise<PlatformConfig | null> {
  if (!sdk.isWeb) return null
  try {
    return await sdk.getPlatformConfig()
  } catch {
    return null
  }
}

// ============================================================================
// 六、授权跳转（独立浏览器环境）
// ============================================================================

/**
 * 在独立浏览器中打开授权页面。
 *
 * @param method - 授权方法名，如 'pickFile', 'pickUserFile' 等
 * @param params - 授权参数（含 appName, redirectUri 等）
 * @param options - 可选 { target?, features? }
 * @returns 回调 URL 字符串或 null
 *
 * @example
 * ```ts
 * const callbackUrl = await openAppAuth('pickFile', {
 *   appName: 'logmanager',
 *   directory: true,
 *   title: '选择文件',
 *   redirectUri: window.location.origin + '/auth-callback'
 * })
 * if (callbackUrl) {
 *   const result = parseAppAuthCallback(callbackUrl)
 *   console.log(result.path)
 * }
 * ```
 */
export async function openAppAuth(
  method: AppAuthMethod,
  params: Record<string, unknown>,
  options?: { target?: string; features?: string }
): Promise<string | null> {
  if (!sdk.isWeb && !sdk.isStandaloneWeb) return null
  try {
    // sdk.openAppAuth 接受泛型参数，使用 unknown 绕过类型约束
    return await sdk.openAppAuth(method, params as unknown as AppAuthBaseParams, options)
  } catch {
    return null
  }
}

/**
 * 解析授权回调 URL。
 *
 * @param url - 回调 URL（不传则从当前页面 URL 解析）
 * @returns AppAuthResult
 *
 * @example
 * ```ts
 * const result = parseAppAuthCallback()
 * if (result.status === 'success') {
 *   console.log('选中的路径:', result.path)
 * }
 * ```
 */
export function parseAppAuthCallback(url?: string): AppAuthResult {
  return sdk.parseAppAuthCallback(url)
}

// ============================================================================
// 七、错误处理辅助
// ============================================================================

/**
 * 检查 AppBridgeResponse 是否成功。
 *
 * @example
 * ```ts
 * const result = await pickUserFile({ directory: true, title: '选择' })
 * if (isSuccess(result)) {
 *   console.log('选中:', result.data)
 * }
 * ```
 */
export function isSuccess<T>(response: AppBridgeResponse<T> | null | undefined): response is AppBridgeResponse<T> {
  return response !== null && response !== undefined && response.code === JSSDKErrorCode.SUCCESS
}

/**
 * 获取 AppBridgeResponse 的错误信息。
 *
 * @example
 * ```ts
 * const result = await pickUserFile(...)
 * if (!isSuccess(result)) {
 *   console.error('操作失败:', getErrorMsg(result))
 * }
 * ```
 */
export function getErrorMsg(response: AppBridgeResponse<unknown> | null | undefined): string {
  if (!response) return '非 fnOS 环境'
  if (response.code === 0) return ''
  return response.msg || `错误码: ${response.code}`
}

// ============================================================================
// 八、后端 API 辅助（通过服务端代理调用）
//
// 以下方法需要后端实现对应的代理端点。
// 如果你的项目没有对应后端 API，可以自行实现或忽略。
//
// 使用前先设置后端 API 基础地址：
//   setBackendApiBase('http://localhost:8080')
//
// 在 fnOS 宿主环境中，建议优先使用 query() 方法通过 SDK 代理请求。
// ============================================================================

let backendApiBase = ''

/**
 * 设置后端 API 基础地址。
 * 在应用初始化时调用一次即可。
 *
 * @param baseUrl - 后端 API 基础地址，如 'http://localhost:8080' 或 '/api'
 *
 * @example
 * ```ts
 * // 在 main.ts 或 App.vue 的 onMounted 中调用
 * setBackendApiBase('/api')
 * ```
 */
export function setBackendApiBase(baseUrl: string): void {
  backendApiBase = baseUrl.replace(/\/$/, '') // 去掉末尾斜杠
}

/**
 * 通过后端 API 进行路径转换（trim.file.convertPath）。
 * 需要后端实现 POST /api/utils/convert-path 代理端点。
 * 使用前需先调用 setBackendApiBase() 设置 API 地址。
 *
 * @param path - 内部路径，如 /vol1/@appdata/myapp
 * @param language - 展示语言，如 'zh-CN'、'en-US'，默认取当前界面语言（zh-CN）
 * @returns 语义化路径，如 "存储空间1/应用数据/myapp"
 *
 * @example
 * ```ts
 * const friendly = await convertPathViaBackend('/vol1/@appdata/myapp/log.txt')
 * const en = await convertPathViaBackend('/vol1/@appdata/myapp/log.txt', 'en-US')
 * ```
 */
export async function convertPathViaBackend(path: string, language?: string): Promise<string> {
  if (!backendApiBase) {
    console.warn('[fnos] convertPathViaBackend: 请先调用 setBackendApiBase() 设置 API 地址')
    return path
  }
  try {
    const lang = language || 'zh-CN'
    const response = await fetch(`${backendApiBase}/api/utils/convert-path`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, language: lang }),
      credentials: 'include'
    })
    if (response.ok) {
      const data = await response.json()
      return data.semanticPath || data.displayPath || path
    }
  } catch {
    // ignore
  }
  return path
}

/**
 * 批量路径转换（通过后端代理）。
 *
 * @param paths - 内部路径数组
 * @param language - 展示语言，如 'zh-CN'、'en-US'，默认取当前界面语言（zh-CN）
 */
export async function convertPathsViaBackend(paths: string[], language?: string): Promise<Map<string, string>> {
  const result = new Map<string, string>()
  for (const p of paths) {
    result.set(p, await convertPathViaBackend(p, language))
  }
  return result
}

// ============================================================================
// 九、API Scope 常量（便于在 config/resource 中声明）
// ============================================================================

export const API_SCOPES = {
  USER_ACCESS: 'trim.file.userAccess',
  USER_ACL: 'trim.file.userAcl',
  PATH: 'trim.file.path',
  SHARED_ACCESS: 'trim.file.sharedAccess',
  SYSTEM_PLATFORM_CONFIG: 'trim.system.getPlatformConfig',
} as const

export type APIScope = (typeof API_SCOPES)[keyof typeof API_SCOPES]

// ============================================================================
// 十、SDK 初始化与用户信息（P0）
// ============================================================================

/** SDK 初始化完成标志 */
let sdkReady = false

/**
 * 等待 SDK 初始化完成。
 * 应在应用启动时尽早调用，确保后续所有 SDK 方法调用安全。
 *
 * @example
 * ```ts
 * await waitForReady()
 * // SDK 就绪，可以安全调用其他 API
 * ```
 */
export async function waitForReady(): Promise<void> {
  if (sdkReady) return
  try {
    await sdk.ready()
    sdkReady = true
  } catch {
    // 非 fnOS 环境或初始化失败，标记为就绪以继续运行
    sdkReady = true
  }
}

/**
 * 获取当前宿主会话快照。
 * 包含当前用户信息（status, username, sessions 等）。
 *
 * @returns HostSnapshot 或 null
 *
 * @example
 * ```ts
 * const snapshot = await getHostSnapshot()
 * if (snapshot) {
 *   console.log('当前用户:', snapshot.username)
 *   console.log('会话数:', snapshot.sessions?.length)
 * }
 * ```
 */
export async function getHostSnapshot(): Promise<HostSnapshot | null> {
  if (!sdk.isWeb) return null
  try {
    return await sdk.getHostSnapshot()
  } catch {
    return null
  }
}

// ============================================================================
// 十一、SDK 代理请求（P0 - 替代 fetch 调后端）
// ============================================================================

import type { QueryConfig, ResponseData } from '@trimjs/web-app'

/**
 * 通过 SDK 代理向后端 API 发请求。
 *
 * 相比直接用 fetch，sdk.query 的优点：
 *   1. 自动携带认证 Token，无需手动管理 CSRF / Session
 *   2. fnOS 宿主环境下走宿主代理，性能更好
 *   3. 支持 observable 流式响应（实时日志推送等场景）
 *   4. 统一的错误处理和重试
 *
 * @param params - 请求参数 { path, method?, body?, headers? }
 * @param config - 可选配置 { retry?, timeout?, observable? }
 * @returns ResponseData<T> 或 null
 *
 * @example
 * ```ts
 * // 普通请求
 * const data = await query({ path: '/api/dirs', method: 'GET' })
 * console.log(data)
 *
 * // 流式请求
 * const observable = await query(
 *   { path: '/api/logs/tail', method: 'GET' },
 *   { observable: true }
 * )
 * if (observable) {
 *   observable.subscribe({ next: (chunk) => console.log(chunk) })
 * }
 * ```
 */
export async function query<T = unknown>(
  params: { path: string; method?: string; body?: unknown; headers?: Record<string, string> },
  config?: QueryConfig & { observable?: boolean }
): Promise<ResponseData<T> | null> {
  if (!sdk.isWeb) return null
  try {
    // 构建与当前 api.ts 兼容的请求格式
    const requestParams = {
      path: params.path,
      method: params.method || 'GET',
      body: params.body,
      headers: params.headers,
    }
    return (await sdk.query<T>(requestParams, config)) || null
  } catch {
    return null
  }
}

/**
 * 刷新认证 Token。
 * 在 SDK 代理请求返回 401 时调用。
 *
 * @example
 * ```ts
 * await refreshToken()
 * // Token 已刷新，可重试请求
 * ```
 */
export async function refreshToken(): Promise<void> {
  if (!sdk.isWeb) return
  try {
    await sdk.refreshToken()
  } catch {
    // ignore
  }
}

/**
 * 获取授权基础 URL。
 * 用于独立浏览器环境下拼装授权链接。
 *
 * @example
 * ```ts
 * const baseUrl = await getAppAuthBaseUrl()
 * const authUrl = `${baseUrl}?appName=logmanager&method=pickFile&...`
 * ```
 */
export async function getAppAuthBaseUrl(): Promise<string | null> {
  if (!sdk.isWeb && !sdk.isStandaloneWeb) return null
  try {
    return await sdk.getAppAuthBaseUrl()
  } catch {
    return null
  }
}

/**
 * 构建授权 URL（不打开窗口）。
 *
 * @param method - 授权方法
 * @param params - 授权参数
 * @returns 完整的授权 URL 或 null
 *
 * @example
 * ```ts
 * const url = await buildAppAuthUrl('pickFile', {
 *   appName: 'logmanager',
 *   directory: true,
 *   title: '选择文件',
 *   redirectUri: 'https://example.com/callback'
 * })
 * ```
 */
export async function buildAppAuthUrl<TMethod extends AppAuthMethod>(
  method: TMethod,
  params: AppAuthBaseParams & Record<string, unknown>
): Promise<string | null> {
  if (!sdk.isWeb && !sdk.isStandaloneWeb) return null
  try {
    return await sdk.buildAppAuthUrl(method, params as any)
  } catch {
    return null
  }
}

/**
 * 打开其他 fnOS 应用。
 *
 * @param anchor - 应用锚点，如 'files' 打开文件管理器
 *
 * @example
 * ```ts
 * await openApp('files')   // 打开文件管理器
 * await openApp('settings') // 打开系统设置
 * ```
 */
export async function openApp(anchor: string): Promise<void> {
  if (!sdk.isWeb) return
  try {
    await sdk.openApp(anchor)
  } catch {
    // ignore
  }
}

/**
 * 打开自定义应用。
 *
 * @param appName - 应用标识
 * @param options - 打开参数
 *
 * @example
 * ```ts
 * await openCustomApp('my-app', { path: '/settings' })
 * ```
 */
export async function openCustomApp(appName: string, options: Record<string, unknown>): Promise<void> {
  if (!sdk.isWeb) return
  try {
    await sdk.openCustomApp(appName, options as any)
  } catch {
    // ignore
  }
}

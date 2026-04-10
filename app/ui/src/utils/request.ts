/**
 * 错误类型定义
 */
export class NetworkError extends Error {
  constructor(message: string = '网络错误') {
    super(message)
    this.name = 'NetworkError'
  }
}

export class AuthenticationError extends Error {
  constructor(message: string = '认证失败') {
    super(message)
    this.name = 'AuthenticationError'
  }
}

export class ValidationError extends Error {
  constructor(message: string = '验证失败') {
    super(message)
    this.name = 'ValidationError'
  }
}

export class ServerError extends Error {
  constructor(message: string = '服务器错误') {
    super(message)
    this.name = 'ServerError'
  }
}

/**
 * 请求重试装饰器
 */
export async function withRetry<T>(
  fn: () => Promise<T>,
  maxRetries: number = 3,
  delay: number = 1000
): Promise<T> {
  let lastError: Error | null = null
  
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn()
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err))
      
      // 如果是认证错误或验证错误，不重试
      if (err instanceof AuthenticationError || err instanceof ValidationError) {
        throw err
      }
      
      // 如果不是最后一次尝试，等待后重试
      if (i < maxRetries - 1) {
        await new Promise(resolve => setTimeout(resolve, delay * (i + 1)))
      }
    }
  }
  
  throw lastError
}

/**
 * 请求取消管理器
 */
export class RequestCanceller {
  private controllers: Map<string, AbortController> = new Map()

  createController(key: string): AbortController {
    // 取消之前的请求
    this.cancel(key)
    
    const controller = new AbortController()
    this.controllers.set(key, controller)
    return controller
  }

  cancel(key: string): void {
    const controller = this.controllers.get(key)
    if (controller) {
      controller.abort()
      this.controllers.delete(key)
    }
  }

  cancelAll(): void {
    this.controllers.forEach(controller => controller.abort())
    this.controllers.clear()
  }
}

/**
 * 请求去重器
 */
export class RequestDeduper {
  private pending: Map<string, Promise<any>> = new Map()

  async dedupe<T>(key: string, fn: () => Promise<T>): Promise<T> {
    // 如果有相同的请求正在进行，返回该 Promise
    const pending = this.pending.get(key)
    if (pending) {
      return pending
    }

    // 执行请求
    const promise = fn().finally(() => {
      this.pending.delete(key)
    })

    this.pending.set(key, promise)
    return promise
  }

  clear(): void {
    this.pending.clear()
  }
}

/**
 * 过滤错误信息中的敏感内容（公共工具函数）
 * @param message - 原始错误消息
 * @returns 过滤后的安全消息
 */
export function filterSensitiveInfo(message: unknown): string {
  if (typeof message !== 'string') return String(message)

  // 过滤路径信息
  let filtered = message.replace(/\/[\w\-./]+/g, '[PATH]')

  // 过滤IP地址
  filtered = filtered.replace(/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/g, '[IP]')

  // 过滤端口号
  filtered = filtered.replace(/:\d{2,5}/g, ':[PORT]')

  return filtered
}

/**
 * 安全获取错误消息：从 Error 对象中提取消息并过滤敏感信息
 * @param err - 错误对象
 * @param fallback - 默认错误消息
 * @returns 过滤后的安全错误消息
 */
export function safeErrorMessage(err: unknown, fallback: string = '操作失败'): string {
  const message = err instanceof Error ? err.message : String(err)
  return filterSensitiveInfo(message || fallback)
}

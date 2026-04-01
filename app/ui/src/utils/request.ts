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

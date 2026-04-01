/**
 * 应用错误基类
 */
export class AppError extends Error {
  constructor(
    public code: string,
    public message: string,
    public statusCode: number = 500,
    public details?: any
  ) {
    super(message)
    this.name = 'AppError'
    Error.captureStackTrace(this, this.constructor)
  }

  toJSON() {
    return {
      success: false,
      error: {
        code: this.code,
        message: this.message,
        details: this.details
      }
    }
  }
}

/**
 * 验证错误
 */
export class ValidationError extends AppError {
  constructor(message: string, details?: any) {
    super('VALIDATION_ERROR', message, 400, details)
    this.name = 'ValidationError'
  }
}

/**
 * 认证错误
 */
export class AuthenticationError extends AppError {
  constructor(message: string = '认证失败') {
    super('AUTH_ERROR', message, 401)
    this.name = 'AuthenticationError'
  }
}

/**
 * 授权错误
 */
export class AuthorizationError extends AppError {
  constructor(message: string = '权限不足') {
    super('AUTHORIZATION_ERROR', message, 403)
    this.name = 'AuthorizationError'
  }
}

/**
 * 资源未找到错误
 */
export class NotFoundError extends AppError {
  constructor(message: string = '资源未找到') {
    super('NOT_FOUND_ERROR', message, 404)
    this.name = 'NotFoundError'
  }
}

/**
 * 冲突错误
 */
export class ConflictError extends AppError {
  constructor(message: string, details?: any) {
    super('CONFLICT_ERROR', message, 409, details)
    this.name = 'ConflictError'
  }
}

/**
 * 服务器错误
 */
export class ServerError extends AppError {
  constructor(message: string = '服务器内部错误', details?: any) {
    super('SERVER_ERROR', message, 500, details)
    this.name = 'ServerError'
  }
}

/**
 * 服务不可用错误
 */
export class ServiceUnavailableError extends AppError {
  constructor(message: string = '服务暂时不可用') {
    super('SERVICE_UNAVAILABLE', message, 503)
    this.name = 'ServiceUnavailableError'
  }
}

/**
 * 错误工厂函数
 */
export function createError(type: string, message: string, details?: any): AppError {
  switch (type) {
    case 'VALIDATION':
      return new ValidationError(message, details)
    case 'AUTH':
      return new AuthenticationError(message)
    case 'AUTHORIZATION':
      return new AuthorizationError(message)
    case 'NOT_FOUND':
      return new NotFoundError(message)
    case 'CONFLICT':
      return new ConflictError(message, details)
    case 'SERVICE_UNAVAILABLE':
      return new ServiceUnavailableError(message)
    default:
      return new ServerError(message, details)
  }
}

import { Request, Response, NextFunction } from 'express';
import { AppError } from '../errors';
import Logger from '../utils/logger';

const logger = Logger.child({ module: 'ErrorHandler' });

/**
 * 统一响应格式
 */
interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
    details?: any;
  };
  meta?: {
    timestamp: number;
    requestId: string;
  };
}

/**
 * 生成请求 ID
 */
function generateRequestId(): string {
  return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * 404 处理器
 */
export function notFoundHandler(req: Request, res: Response, _next: NextFunction): void {
  const requestId = generateRequestId();
  
  res.status(404).json({
    success: false,
    error: {
      code: 'NOT_FOUND',
      message: '资源不存在',
      details: { path: req.path }
    },
    meta: {
      timestamp: Date.now(),
      requestId
    }
  } as ApiResponse);
}

/**
 * 全局错误处理器
 */
export function errorHandler(err: Error, req: Request, res: Response, _next: NextFunction): void {
  const requestId = generateRequestId();
  
  // 记录错误日志
  logger.error({
    err,
    path: req.path,
    method: req.method,
    requestId,
    body: req.body
  }, '请求处理错误');

  // 处理 AppError
  if (err instanceof AppError) {
    const response: ApiResponse = {
      success: false,
      error: {
        code: err.code,
        message: err.message,
        details: err.details
      },
      meta: {
        timestamp: Date.now(),
        requestId
      }
    };

    res.status(err.statusCode).json(response);
    return;
  }

  // 处理 JSON 解析错误
  if ((err as any).type === 'entity.parse.failed') {
    res.status(400).json({
      success: false,
      error: {
        code: 'INVALID_JSON',
        message: '无效的JSON格式'
      },
      meta: {
        timestamp: Date.now(),
        requestId
      }
    } as ApiResponse);
    return;
  }

  // 处理验证错误 (express-validator)
  if ((err as any).name === 'ValidationError') {
    res.status(400).json({
      success: false,
      error: {
        code: 'VALIDATION_ERROR',
        message: '请求参数验证失败',
        details: (err as any).errors
      },
      meta: {
        timestamp: Date.now(),
        requestId
      }
    } as ApiResponse);
    return;
  }

  // 未知错误
  logger.error({ err, requestId }, '未预期的错误');

  // 生产环境隐藏错误详情
  const isProduction = process.env.NODE_ENV === 'production' || process.env.FORCE_HTTPS === 'true';
  
  res.status(500).json({
    success: false,
    error: {
      code: 'INTERNAL_ERROR',
      message: isProduction ? '服务器内部错误' : err.message || '服务器内部错误',
      details: isProduction ? undefined : err.stack
    },
    meta: {
      timestamp: Date.now(),
      requestId
    }
  } as ApiResponse);
}

/**
 * 成功响应包装器
 */
export function successResponse<T>(data: T, message?: string): ApiResponse<T> {
  return {
    success: true,
    data,
    meta: {
      timestamp: Date.now(),
      requestId: generateRequestId()
    }
  };
}

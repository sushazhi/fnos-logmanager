/**
 * @fileoverview 自定义错误类
 */

class AppError extends Error {
    /**
     * @param {string} message - 错误消息
     * @param {number} statusCode - HTTP状态码
     * @param {string} [code] - 错误代码
     */
    constructor(message, statusCode, code = null) {
        super(message);
        this.name = 'AppError';
        this.statusCode = statusCode;
        this.code = code;
        this.isOperational = true;
        Error.captureStackTrace(this, this.constructor);
    }
}

class ValidationError extends AppError {
    /**
     * @param {string} message - 错误消息
     * @param {Array} [errors] - 验证错误详情
     */
    constructor(message, errors = null) {
        super(message, 400, 'VALIDATION_ERROR');
        this.name = 'ValidationError';
        this.errors = errors;
    }
}

class AuthenticationError extends AppError {
    /**
     * @param {string} message - 错误消息
     */
    constructor(message = '需要认证') {
        super(message, 401, 'AUTHENTICATION_ERROR');
        this.name = 'AuthenticationError';
    }
}

class AuthorizationError extends AppError {
    /**
     * @param {string} message - 错误消息
     */
    constructor(message = '权限不足') {
        super(message, 403, 'AUTHORIZATION_ERROR');
        this.name = 'AuthorizationError';
    }
}

class NotFoundError extends AppError {
    /**
     * @param {string} message - 错误消息
     */
    constructor(message = '资源不存在') {
        super(message, 404, 'NOT_FOUND_ERROR');
        this.name = 'NotFoundError';
    }
}

class RateLimitError extends AppError {
    /**
     * @param {string} message - 错误消息
     */
    constructor(message = '请求过于频繁') {
        super(message, 429, 'RATE_LIMIT_ERROR');
        this.name = 'RateLimitError';
    }
}

class CSRFError extends AppError {
    /**
     * @param {string} message - 错误消息
     */
    constructor(message = 'CSRF验证失败') {
        super(message, 403, 'CSRF_ERROR');
        this.name = 'CSRFError';
    }
}

module.exports = {
    AppError,
    ValidationError,
    AuthenticationError,
    AuthorizationError,
    NotFoundError,
    RateLimitError,
    CSRFError
};

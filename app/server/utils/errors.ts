interface ValidationErrorDetail {
    msg: string;
    param?: string;
    location?: string;
    value?: unknown;
}

export class AppError extends Error {
    statusCode: number;
    code: string | null;
    isOperational: boolean;

    constructor(message: string, statusCode: number, code: string | null = null) {
        super(message);
        this.name = 'AppError';
        this.statusCode = statusCode;
        this.code = code;
        this.isOperational = true;
        Error.captureStackTrace(this, this.constructor);
    }
}

export class ValidationError extends AppError {
    errors: ValidationErrorDetail[] | null;

    constructor(message: string, errors: ValidationErrorDetail[] | null = null) {
        super(message, 400, 'VALIDATION_ERROR');
        this.name = 'ValidationError';
        this.errors = errors;
    }
}

export class AuthenticationError extends AppError {
    constructor(message: string = '需要认证') {
        super(message, 401, 'AUTHENTICATION_ERROR');
        this.name = 'AuthenticationError';
    }
}

export class AuthorizationError extends AppError {
    constructor(message: string = '权限不足') {
        super(message, 403, 'AUTHORIZATION_ERROR');
        this.name = 'AuthorizationError';
    }
}

export class NotFoundError extends AppError {
    constructor(message: string = '资源不存在') {
        super(message, 404, 'NOT_FOUND_ERROR');
        this.name = 'NotFoundError';
    }
}

export class RateLimitError extends AppError {
    constructor(message: string = '请求过于频繁') {
        super(message, 429, 'RATE_LIMIT_ERROR');
        this.name = 'RateLimitError';
    }
}

export class CSRFError extends AppError {
    constructor(message: string = 'CSRF验证失败') {
        super(message, 403, 'CSRF_ERROR');
        this.name = 'CSRFError';
    }
}

/**
 * @fileoverview 错误处理中间件
 */

const { AppError } = require('../utils/errors');

/**
 * 404 处理中间件
 * @param {import('express').Request} req - Express请求对象
 * @param {import('express').Response} res - Express响应对象
 * @param {import('express').NextFunction} next - 下一个中间件
 */
function notFoundHandler(req, res, next) {
    res.status(404).json({
        error: '资源不存在',
        code: 'NOT_FOUND',
        path: req.path
    });
}

/**
 * 全局错误处理中间件
 * @param {Error} err - 错误对象
 * @param {import('express').Request} req - Express请求对象
 * @param {import('express').Response} res - Express响应对象
 * @param {import('express').NextFunction} next - 下一个中间件
 */
function errorHandler(err, req, res, next) {
    console.error('[LogManager] Error:', err.message);
    
    if (err instanceof AppError) {
        const response = {
            error: err.message,
            code: err.code
        };
        
        if (err.name === 'ValidationError' && err.errors) {
            response.errors = err.errors;
        }
        
        return res.status(err.statusCode).json(response);
    }
    
    if (err.type === 'entity.parse.failed') {
        return res.status(400).json({
            error: '无效的JSON格式',
            code: 'INVALID_JSON'
        });
    }
    
    console.error('[LogManager] Unexpected error:', err.stack || err);
    
    res.status(500).json({
        error: '服务器内部错误',
        code: 'INTERNAL_ERROR'
    });
}

module.exports = {
    notFoundHandler,
    errorHandler
};

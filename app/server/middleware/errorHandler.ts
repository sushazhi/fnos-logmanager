import { Request, Response, NextFunction } from 'express';
import { AppError } from '../utils/errors';

export function notFoundHandler(req: Request, res: Response, _next: NextFunction): void {
    res.status(404).json({
        error: '资源不存在',
        code: 'NOT_FOUND',
        path: req.path
    });
}

export function errorHandler(err: Error, req: Request, res: Response, _next: NextFunction): void {
    console.error('[LogManager] Error:', err.message);

    if (err instanceof AppError) {
        const response: {
            error: string;
            code: string | null;
            errors?: unknown;
        } = {
            error: err.message,
            code: err.code
        };

        if (err.name === 'ValidationError' && 'errors' in err) {
            response.errors = (err as { errors: unknown }).errors;
        }

        res.status(err.statusCode).json(response);
        return;
    }

    if ((err as { type?: string }).type === 'entity.parse.failed') {
        res.status(400).json({
            error: '无效的JSON格式',
            code: 'INVALID_JSON'
        });
        return;
    }

    console.error('[LogManager] Unexpected error:', err.stack || err);

    res.status(500).json({
        error: '服务器内部错误',
        code: 'INTERNAL_ERROR'
    });
}

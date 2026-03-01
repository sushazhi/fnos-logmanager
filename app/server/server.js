/**
 * @fileoverview 飞牛日志管理服务 - 主入口
 */

const express = require('express');
const path = require('path');
const morgan = require('morgan');
const cookieParser = require('cookie-parser');

const config = require('./utils/config');
const { notFoundHandler, errorHandler } = require('./middleware/errorHandler');
const { securityHeaders } = require('./middleware/security');
const rateLimitMiddleware = require('./middleware/rateLimit');

const authRoutes = require('./routes/auth');
const logRoutes = require('./routes/logs');
const dockerRoutes = require('./routes/docker');

const app = express();

app.set('trust proxy', true);

app.use(morgan('combined'));
app.use(securityHeaders);
app.use(cookieParser());
app.use(express.json());
app.use(rateLimitMiddleware.rateLimit);

app.use(express.static(path.join(__dirname, '../ui')));

app.use('/api/auth', authRoutes);
app.use('/api', logRoutes);
app.use('/api', dockerRoutes);

app.use(notFoundHandler);
app.use(errorHandler);

app.listen(config.port, '0.0.0.0', () => {
    console.log(`飞牛日志管理服务已启动，端口: ${config.port}`);
});

module.exports = app;

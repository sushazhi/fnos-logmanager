/**
 * Event Logger API Routes
 */

import { Router, Request, Response } from 'express';
import { body, query, param, validationResult } from 'express-validator';
import { validateToken, validateCSRF } from '../middleware/auth';
import * as eventLoggerService from '../services/eventLogger';
import {
    UpdateEventLoggerConfigRequest,
    GetEventsRequest,
    CreateEventNotificationRuleRequest
} from '../types/eventLogger';

const router = Router();

/**
 * GET /api/eventlogger/status
 * Get service status
 */
router.get('/status', validateToken, async (req: Request, res: Response) => {
    try {
        const status = eventLoggerService.getStatus();
        res.json(status);
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

/**
 * GET /api/eventlogger/config
 * Get current configuration
 */
router.get('/config', validateToken, async (req: Request, res: Response) => {
    try {
        const config = eventLoggerService.getConfig();
        res.json(config);
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

/**
 * PUT /api/eventlogger/config
 * Update configuration
 */
router.put('/config',
    validateToken,
    validateCSRF,
    body('dbPath').optional().isString(),
    body('enabled').optional().isBoolean(),
    body('checkInterval').optional().isInt({ min: 5000 }),
    body('eventTypes').optional().isArray(),
    body('minSeverity').optional().isIn(['debug', 'info', 'warning', 'error', 'critical']),
    body('notificationChannels').optional().isArray(),
    body('appFilter').optional().isArray(),
    body('excludeSources').optional().isArray(),
    async (req: Request, res: Response) => {
        try {
            const errors = validationResult(req);
            if (!errors.isEmpty()) {
                return res.status(400).json({ errors: errors.array() });
            }

            const updates = req.body as UpdateEventLoggerConfigRequest;
            await eventLoggerService.updateConfig(updates);
            
            const config = eventLoggerService.getConfig();
            res.json(config);
        } catch (err) {
            res.status(500).json({ error: (err as Error).message });
        }
    }
);

/**
 * GET /api/eventlogger/stats
 * Get event statistics
 */
router.get('/stats', validateToken, async (req: Request, res: Response) => {
    try {
        const stats = eventLoggerService.getStats();
        res.json(stats);
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

/**
 * GET /api/eventlogger/events
 * Get events with filtering
 */
router.get('/events',
    validateToken,
    query('limit').optional().isInt({ min: 1, max: 1000 }),
    query('offset').optional().isInt({ min: 0 }),
    query('startTime').optional().isISO8601(),
    query('endTime').optional().isISO8601(),
    query('severity').optional().isIn(['debug', 'info', 'warning', 'error', 'critical']),
    query('source').optional().isString(),
    query('eventType').optional().isString(),
    query('search').optional().isString(),
    async (req: Request, res: Response) => {
        try {
            const errors = validationResult(req);
            if (!errors.isEmpty()) {
                return res.status(400).json({ errors: errors.array() });
            }

            const request: GetEventsRequest = {
                limit: req.query.limit ? parseInt(req.query.limit as string) : 100,
                offset: req.query.offset ? parseInt(req.query.offset as string) : 0,
                startTime: req.query.startTime as string,
                endTime: req.query.endTime as string,
                severity: req.query.severity as any,
                source: req.query.source as string,
                eventType: req.query.eventType as string,
                search: req.query.search as string
            };

            const result = eventLoggerService.getEvents(request);
            res.json(result);
        } catch (err) {
            res.status(500).json({ error: (err as Error).message });
        }
    }
);

/**
 * POST /api/eventlogger/check
 * Force a check (manual trigger)
 */
router.post('/check', validateToken, validateCSRF, async (req: Request, res: Response) => {
    try {
        await eventLoggerService.forceCheck();
        res.json({ success: true, message: 'Check completed' });
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

/**
 * POST /api/eventlogger/start
 * Start monitoring
 */
router.post('/start', validateToken, validateCSRF, async (req: Request, res: Response) => {
    try {
        await eventLoggerService.start();
        const status = eventLoggerService.getStatus();
        res.json(status);
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

/**
 * POST /api/eventlogger/stop
 * Stop monitoring
 */
router.post('/stop', validateToken, validateCSRF, async (req: Request, res: Response) => {
    try {
        eventLoggerService.stop();
        const status = eventLoggerService.getStatus();
        res.json(status);
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

/**
 * POST /api/eventlogger/restart
 * Restart monitoring
 */
router.post('/restart', validateToken, validateCSRF, async (req: Request, res: Response) => {
    try {
        await eventLoggerService.restart();
        const status = eventLoggerService.getStatus();
        res.json(status);
    } catch (err) {
        res.status(500).json({ error: (err as Error).message });
    }
});

export default router;

/**
 * EventLogger DB 测试
 */
import {
    mapLogLevelToSeverity,
    parseSeverity,
    SEVERITY_ORDER
} from './db';
import { EventSeverity } from './types';

describe('EventLoggerDb', () => {
    describe('mapLogLevelToSeverity', () => {
        it('should map debug level', () => {
            expect(mapLogLevelToSeverity('debug')).toBe('debug');
            expect(mapLogLevelToSeverity('DEBUG')).toBe('debug');
            expect(mapLogLevelToSeverity('trace')).toBe('debug');
        });

        it('should map info level', () => {
            expect(mapLogLevelToSeverity('info')).toBe('info');
            expect(mapLogLevelToSeverity('INFO')).toBe('info');
            expect(mapLogLevelToSeverity('information')).toBe('info');
        });

        it('should map warning level', () => {
            expect(mapLogLevelToSeverity('warn')).toBe('warning');
            expect(mapLogLevelToSeverity('WARN')).toBe('warning');
            expect(mapLogLevelToSeverity('warning')).toBe('warning');
        });

        it('should map error level', () => {
            expect(mapLogLevelToSeverity('error')).toBe('error');
            expect(mapLogLevelToSeverity('ERROR')).toBe('error');
        });

        it('should map critical level', () => {
            expect(mapLogLevelToSeverity('fatal')).toBe('critical');
            expect(mapLogLevelToSeverity('critical')).toBe('critical');
            expect(mapLogLevelToSeverity('CRITICAL')).toBe('critical');
        });

        it('should default to info for unknown level', () => {
            expect(mapLogLevelToSeverity('unknown')).toBe('info');
            expect(mapLogLevelToSeverity('')).toBe('info');
        });
    });

    describe('parseSeverity', () => {
        it('should parse numeric severity', () => {
            expect(parseSeverity(0)).toBe('debug');
            expect(parseSeverity(1)).toBe('info');
            expect(parseSeverity(2)).toBe('warning');
            expect(parseSeverity(3)).toBe('error');
            expect(parseSeverity(4)).toBe('critical');
            expect(parseSeverity(5)).toBe('critical');
        });

        it('should parse string severity', () => {
            expect(parseSeverity('debug')).toBe('debug');
            expect(parseSeverity('info')).toBe('info');
            expect(parseSeverity('warning')).toBe('warning');
            expect(parseSeverity('error')).toBe('error');
            expect(parseSeverity('critical')).toBe('critical');
        });

        it('should be case insensitive', () => {
            expect(parseSeverity('DEBUG')).toBe('debug');
            expect(parseSeverity('INFO')).toBe('info');
            expect(parseSeverity('WARNING')).toBe('warning');
            expect(parseSeverity('ERROR')).toBe('error');
            expect(parseSeverity('CRITICAL')).toBe('critical');
        });

        it('should default to info for unknown severity', () => {
            expect(parseSeverity('unknown')).toBe('info');
            expect(parseSeverity(-1)).toBe('debug');
        });
    });

    describe('SEVERITY_ORDER', () => {
        it('should have correct order', () => {
            expect(SEVERITY_ORDER['debug']).toBeLessThan(SEVERITY_ORDER['info']);
            expect(SEVERITY_ORDER['info']).toBeLessThan(SEVERITY_ORDER['warning']);
            expect(SEVERITY_ORDER['warning']).toBeLessThan(SEVERITY_ORDER['error']);
            expect(SEVERITY_ORDER['error']).toBeLessThan(SEVERITY_ORDER['critical']);
        });

        it('should have all severity levels', () => {
            const levels: EventSeverity[] = ['debug', 'info', 'warning', 'error', 'critical'];
            for (const level of levels) {
                expect(SEVERITY_ORDER[level]).toBeDefined();
                expect(typeof SEVERITY_ORDER[level]).toBe('number');
            }
        });
    });
});

/**
 * TemplateHandler 测试
 */
import {
    formatTimestampToLocal,
    formatTemplateMessage,
    formatEventMessage
} from './templateHandler';
import { EventLogEntry } from './types';

describe('TemplateHandler', () => {
    describe('formatTimestampToLocal', () => {
        it('should format Unix timestamp in seconds', () => {
            const timestamp = 1609459200; // 2021-01-01 00:00:00 UTC
            const result = formatTimestampToLocal(timestamp);
            expect(result).toMatch(/2021-01-01/);
        });

        it('should format Unix timestamp in milliseconds', () => {
            const timestamp = 1609459200000; // 2021-01-01 00:00:00 UTC
            const result = formatTimestampToLocal(timestamp);
            expect(result).toMatch(/2021-01-01/);
        });

        it('should format ISO string', () => {
            const timestamp = '2021-01-01T00:00:00Z';
            const result = formatTimestampToLocal(timestamp);
            expect(result).toMatch(/2021-01-01/);
        });

        it('should return formatted string with correct format', () => {
            const result = formatTimestampToLocal(1609459200);
            // 格式: YYYY-MM-DD HH:mm:ss
            expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
        });
    });

    describe('formatTemplateMessage', () => {
        describe('user templates', () => {
            it('should format LoginSucc template', () => {
                const result = formatTemplateMessage({
                    template: 'LoginSucc',
                    user: 'admin',
                    IP: '192.168.1.1'
                });
                expect(result).toBe('admin登录成功 IP:192.168.1.1');
            });

            it('should format LoginFail template', () => {
                const result = formatTemplateMessage({
                    template: 'LoginFail',
                    user: 'admin',
                    IP: '192.168.1.1'
                });
                expect(result).toBe('admin登录失败 IP:192.168.1.1');
            });

            it('should format Logout template', () => {
                const result = formatTemplateMessage({
                    template: 'Logout',
                    user: 'admin'
                });
                expect(result).toBe('admin登出');
            });

            it('should handle missing user', () => {
                const result = formatTemplateMessage({
                    template: 'LoginSucc',
                    IP: '192.168.1.1'
                });
                expect(result).toBe('用户登录成功 IP:192.168.1.1');
            });
        });

        describe('storage templates', () => {
            it('should format DiskWakeup template', () => {
                const result = formatTemplateMessage({
                    template: 'DiskWakeup',
                    disk: '/dev/sda'
                });
                expect(result).toBe('硬盘唤醒 /dev/sda');
            });

            it('should format DiskSpindown template', () => {
                const result = formatTemplateMessage({
                    template: 'DiskSpindown',
                    disk: '/dev/sda'
                });
                expect(result).toBe('硬盘休眠 /dev/sda');
            });
        });

        describe('file templates', () => {
            it('should format CreateFile template', () => {
                const result = formatTemplateMessage({
                    template: 'CreateFile',
                    path: '/tmp/test.txt'
                });
                expect(result).toBe('创建文件 /tmp/test.txt');
            });

            it('should format DeleteFile template', () => {
                const result = formatTemplateMessage({
                    template: 'DeleteFile',
                    path: '/tmp/test.txt'
                });
                expect(result).toBe('删除文件 /tmp/test.txt');
            });
        });

        describe('app templates', () => {
            it('should format AppInstall template', () => {
                const result = formatTemplateMessage({
                    template: 'AppInstall',
                    app: 'nginx'
                });
                expect(result).toBe('安装应用 nginx');
            });

            it('should format AppUninstall template', () => {
                const result = formatTemplateMessage({
                    template: 'AppUninstall',
                    app: 'nginx'
                });
                expect(result).toBe('卸载应用 nginx');
            });
        });

        describe('unknown template', () => {
            it('should return JSON string for unknown template', () => {
                const result = formatTemplateMessage({
                    template: 'UnknownTemplate',
                    data: 'test'
                });
                expect(result).toContain('UnknownTemplate');
            });

            it('should use message field if available', () => {
                const result = formatTemplateMessage({
                    message: 'Custom message'
                });
                expect(result).toBe('Custom message');
            });
        });
    });

    describe('formatEventMessage', () => {
        it('should format event with param', () => {
            const event: EventLogEntry = {
                id: 1,
                timestamp: 1609459200,
                param: JSON.stringify({
                    template: 'LoginSucc',
                    user: 'admin',
                    IP: '192.168.1.1'
                })
            };

            const result = formatEventMessage(event);
            expect(result).toBe('admin登录成功 IP:192.168.1.1');
        });

        it('should format event with template', () => {
            const event: EventLogEntry = {
                id: 1,
                timestamp: 1609459200,
                template: 'Logout',
                user: 'admin'
            };

            const result = formatEventMessage(event);
            expect(result).toBe('admin登出');
        });

        it('should format event with message', () => {
            const event: EventLogEntry = {
                id: 1,
                timestamp: 1609459200,
                message: 'Custom event message'
            };

            const result = formatEventMessage(event);
            expect(result).toBe('Custom event message');
        });

        it('should return JSON for event without template/message', () => {
            const event: EventLogEntry = {
                id: 1,
                timestamp: 1609459200,
                source: 'test'
            };

            const result = formatEventMessage(event);
            expect(result).toContain('id');
            expect(result).toContain('timestamp');
        });
    });
});

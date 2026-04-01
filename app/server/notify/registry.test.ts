/**
 * ChannelRegistry 测试
 */
import { registry } from '../registry';
import { NotifyChannel, NotifyResult } from '../types';

describe('ChannelRegistry', () => {
    // 每个测试前清空注册表
    beforeEach(() => {
        // 获取所有渠道并注销
        const channels = registry.getAll();
        for (const channel of channels) {
            registry.unregister(channel.name);
        }
    });

    describe('register', () => {
        it('should register a channel', () => {
            const channel: NotifyChannel = {
                name: 'test',
                enabled: true,
                send: jest.fn()
            };

            registry.register(channel);

            expect(registry.has('test')).toBe(true);
            expect(registry.get('test')).toBe(channel);
        });

        it('should overwrite existing channel with same name', () => {
            const channel1: NotifyChannel = {
                name: 'test',
                enabled: true,
                send: jest.fn()
            };

            const channel2: NotifyChannel = {
                name: 'test',
                enabled: false,
                send: jest.fn()
            };

            registry.register(channel1);
            registry.register(channel2);

            expect(registry.get('test')).toBe(channel2);
        });
    });

    describe('unregister', () => {
        it('should unregister a channel', () => {
            const channel: NotifyChannel = {
                name: 'test',
                enabled: true,
                send: jest.fn()
            };

            registry.register(channel);
            registry.unregister('test');

            expect(registry.has('test')).toBe(false);
            expect(registry.get('test')).toBeUndefined();
        });

        it('should not throw when unregistering non-existent channel', () => {
            expect(() => registry.unregister('nonexistent')).not.toThrow();
        });
    });

    describe('get', () => {
        it('should return channel by name', () => {
            const channel: NotifyChannel = {
                name: 'test',
                enabled: true,
                send: jest.fn()
            };

            registry.register(channel);
            expect(registry.get('test')).toBe(channel);
        });

        it('should return undefined for non-existent channel', () => {
            expect(registry.get('nonexistent')).toBeUndefined();
        });
    });

    describe('getAll', () => {
        it('should return all channels', () => {
            const channel1: NotifyChannel = {
                name: 'test1',
                enabled: true,
                send: jest.fn()
            };

            const channel2: NotifyChannel = {
                name: 'test2',
                enabled: false,
                send: jest.fn()
            };

            registry.register(channel1);
            registry.register(channel2);

            const all = registry.getAll();
            expect(all).toHaveLength(2);
            expect(all).toContain(channel1);
            expect(all).toContain(channel2);
        });

        it('should return empty array when no channels', () => {
            expect(registry.getAll()).toEqual([]);
        });
    });

    describe('getEnabled', () => {
        it('should return only enabled channels', () => {
            const channel1: NotifyChannel = {
                name: 'enabled',
                enabled: true,
                send: jest.fn()
            };

            const channel2: NotifyChannel = {
                name: 'disabled',
                enabled: false,
                send: jest.fn()
            };

            registry.register(channel1);
            registry.register(channel2);

            const enabled = registry.getEnabled();
            expect(enabled).toHaveLength(1);
            expect(enabled).toContain(channel1);
            expect(enabled).not.toContain(channel2);
        });
    });

    describe('enable/disable', () => {
        it('should enable a channel', () => {
            const channel: NotifyChannel = {
                name: 'test',
                enabled: false,
                send: jest.fn()
            };

            registry.register(channel);
            registry.enable('test');

            expect(channel.enabled).toBe(true);
        });

        it('should disable a channel', () => {
            const channel: NotifyChannel = {
                name: 'test',
                enabled: true,
                send: jest.fn()
            };

            registry.register(channel);
            registry.disable('test');

            expect(channel.enabled).toBe(false);
        });

        it('should not throw when enabling non-existent channel', () => {
            expect(() => registry.enable('nonexistent')).not.toThrow();
        });
    });

    describe('size', () => {
        it('should return correct count', () => {
            expect(registry.size()).toBe(0);

            registry.register({
                name: 'test1',
                enabled: true,
                send: jest.fn()
            });
            expect(registry.size()).toBe(1);

            registry.register({
                name: 'test2',
                enabled: true,
                send: jest.fn()
            });
            expect(registry.size()).toBe(2);
        });
    });
});

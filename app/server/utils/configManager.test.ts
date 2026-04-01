import configManager, { ConfigDefinition } from './configManager';

describe('ConfigManager', () => {
    beforeEach(() => {
        // 重置配置管理器
        configManager.reset();
    });

    describe('define', () => {
        it('should register a config definition', () => {
            const definition: ConfigDefinition<number> = {
                key: 'test.value',
                defaultValue: 100
            };
            configManager.define(definition);

            expect(configManager.has('test.value')).toBe(true);
            expect(configManager.get('test.value')).toBe(100);
        });

        it('should set default value when defining', () => {
            configManager.define({
                key: 'test.string',
                defaultValue: 'hello'
            });

            expect(configManager.get('test.string')).toBe('hello');
        });
    });

    describe('get and set', () => {
        it('should get config value', () => {
            configManager.define({
                key: 'test.number',
                defaultValue: 42
            });

            expect(configManager.get('test.number')).toBe(42);
        });

        it('should throw error for unknown key', () => {
            expect(() => configManager.get('unknown.key')).toThrow('Unknown config key');
        });

        it('should set config value with source', () => {
            configManager.define({
                key: 'test.override',
                defaultValue: 'default'
            });

            configManager.set('test.override', 'overridden', 'env');
            expect(configManager.get('test.override')).toBe('overridden');
            expect(configManager.getSource('test.override')).toBe('env');
        });

        it('should respect priority: env > file > default', () => {
            configManager.define({
                key: 'test.priority',
                defaultValue: 'default'
            });

            // 设置 file 源
            configManager.set('test.priority', 'file-value', 'file');
            expect(configManager.get('test.priority')).toBe('file-value');

            // 设置 env 源（更高优先级）
            configManager.set('test.priority', 'env-value', 'env');
            expect(configManager.get('test.priority')).toBe('env-value');

            // 再次设置 file 源（应该被忽略）
            configManager.set('test.priority', 'file-value-2', 'file');
            expect(configManager.get('test.priority')).toBe('env-value');
        });
    });

    describe('getOrDefault', () => {
        it('should return config value if exists', () => {
            configManager.define({
                key: 'test.exists',
                defaultValue: 'value'
            });

            expect(configManager.getOrDefault('test.exists', 'fallback')).toBe('value');
        });

        it('should return default value if not exists', () => {
            expect(configManager.getOrDefault('unknown.key', 'fallback')).toBe('fallback');
        });
    });

    describe('loadFromEnv', () => {
        it('should load config from environment variables', () => {
            process.env.TEST_ENV_KEY = 'env-value';

            configManager.define({
                key: 'test.env',
                defaultValue: 'default',
                envKey: 'TEST_ENV_KEY'
            });

            configManager.loadFromEnv();
            expect(configManager.get('test.env')).toBe('env-value');

            delete process.env.TEST_ENV_KEY;
        });

        it('should parse number from env', () => {
            process.env.TEST_NUMBER = '12345';

            configManager.define({
                key: 'test.number',
                defaultValue: 0,
                envKey: 'TEST_NUMBER'
            });

            configManager.loadFromEnv();
            expect(configManager.get('test.number')).toBe(12345);

            delete process.env.TEST_NUMBER;
        });

        it('should parse boolean from env', () => {
            process.env.TEST_BOOL = 'true';

            configManager.define({
                key: 'test.bool',
                defaultValue: false,
                envKey: 'TEST_BOOL'
            });

            configManager.loadFromEnv();
            expect(configManager.get('test.bool')).toBe(true);

            delete process.env.TEST_BOOL;
        });
    });

    describe('toObject', () => {
        it('should export all config as object', () => {
            configManager.define({
                key: 'test.export1',
                defaultValue: 'value1'
            });
            configManager.define({
                key: 'test.export2',
                defaultValue: 'value2'
            });

            const obj = configManager.toObject();
            expect(obj['test.export1']).toBe('value1');
            expect(obj['test.export2']).toBe('value2');
        });
    });
});

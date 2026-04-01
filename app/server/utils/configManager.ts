import fs from 'fs';
import path from 'path';
import Logger from './logger';

const logger = Logger.child({ module: 'ConfigManager' });

export type ConfigSource = 'default' | 'file' | 'env';

export interface ConfigDefinition<T> {
    key: string;
    defaultValue: T;
    envKey?: string;
    validator?: (value: unknown) => value is T;
    description?: string;
}

class ConfigManager {
    private config: Map<string, unknown> = new Map();
    private sources: Map<string, ConfigSource> = new Map();
    private definitions: Map<string, ConfigDefinition<unknown>> = new Map();

    /**
     * 注册配置项定义
     */
    define<T>(definition: ConfigDefinition<T>): void {
        this.definitions.set(definition.key, definition as ConfigDefinition<unknown>);
        this.set(definition.key, definition.defaultValue, 'default');
        logger.debug({ key: definition.key, defaultValue: definition.defaultValue }, 'Config definition registered');
    }

    /**
     * 设置配置值
     */
    set<T>(key: string, value: T, source: ConfigSource): void {
        const definition = this.definitions.get(key);

        // 验证配置值
        if (definition?.validator && !definition.validator(value)) {
            logger.warn({ key, value, source }, 'Invalid config value, ignored');
            return;
        }

        // 检查优先级
        const currentSource = this.sources.get(key);
        const priority: ConfigSource[] = ['env', 'file', 'default'];

        if (currentSource && priority.indexOf(currentSource) > priority.indexOf(source)) {
            logger.debug({ key, source, currentSource }, 'Config ignored due to lower priority');
            return;
        }

        this.config.set(key, value);
        this.sources.set(key, source);
        logger.debug({ key, value, source }, 'Config set');
    }

    /**
     * 获取配置值
     */
    get<T>(key: string): T {
        const value = this.config.get(key);
        if (value === undefined) {
            const definition = this.definitions.get(key);
            if (definition) {
                return definition.defaultValue as T;
            }
            throw new Error(`Unknown config key: ${key}`);
        }
        return value as T;
    }

    /**
     * 获取配置值，如果不存在则返回默认值
     */
    getOrDefault<T>(key: string, defaultValue: T): T {
        try {
            return this.get<T>(key);
        } catch {
            return defaultValue;
        }
    }

    /**
     * 检查配置项是否存在
     */
    has(key: string): boolean {
        return this.config.has(key) || this.definitions.has(key);
    }

    /**
     * 获取配置来源
     */
    getSource(key: string): ConfigSource | undefined {
        return this.sources.get(key);
    }

    /**
     * 从环境变量加载
     */
    loadFromEnv(): void {
        let loaded = 0;
        for (const [key, definition] of this.definitions) {
            if (definition.envKey && process.env[definition.envKey] !== undefined) {
                const envValue = process.env[definition.envKey];
                const parsedValue = this.parseValue(envValue, definition);
                this.set(key, parsedValue, 'env');
                loaded++;
            }
        }
        logger.info({ count: loaded }, 'Config loaded from environment');
    }

    /**
     * 从 JSON 文件加载
     */
    loadFromFile(filePath: string): void {
        try {
            if (fs.existsSync(filePath)) {
                const content = fs.readFileSync(filePath, 'utf8');
                const fileConfig = JSON.parse(content);
                let loaded = 0;
                for (const [key, value] of Object.entries(fileConfig)) {
                    if (this.definitions.has(key)) {
                        this.set(key, value, 'file');
                        loaded++;
                    }
                }
                logger.info({ filePath, count: loaded }, 'Config loaded from file');
            }
        } catch (error) {
            logger.warn({ error, filePath }, 'Failed to load config file');
        }
    }

    /**
     * 导出所有配置为对象
     */
    toObject(): Record<string, unknown> {
        const result: Record<string, unknown> = {};
        for (const [key, value] of this.config) {
            result[key] = value;
        }
        return result;
    }

    /**
     * 获取所有配置定义
     */
    getDefinitions(): Map<string, ConfigDefinition<unknown>> {
        return new Map(this.definitions);
    }

    /**
     * 解析环境变量值
     */
    private parseValue(value: string | undefined, definition: ConfigDefinition<unknown>): unknown {
        if (value === undefined) return undefined;

        // 根据默认值类型进行转换
        const defaultType = typeof definition.defaultValue;

        switch (defaultType) {
            case 'number':
                const num = parseInt(value, 10);
                return isNaN(num) ? definition.defaultValue : num;
            case 'boolean':
                return value === 'true';
            case 'string':
            default:
                return value;
        }
    }

    /**
     * 重置配置（用于测试）
     */
    reset(): void {
        this.config.clear();
        this.sources.clear();
        this.definitions.clear();
    }
}

export const configManager = new ConfigManager();
export default configManager;

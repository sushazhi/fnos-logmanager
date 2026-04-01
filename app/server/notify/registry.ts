/**
 * 通知渠道注册机制
 */
import Logger from '../utils/logger';
import { NotifyChannel } from './types';

const logger = Logger.child({ module: 'ChannelRegistry' });

class ChannelRegistry {
    private channels: Map<string, NotifyChannel> = new Map();

    /**
     * 注册通知渠道
     */
    register(channel: NotifyChannel): void {
        this.channels.set(channel.name, channel);
        logger.info({ channel: channel.name, enabled: channel.enabled }, 'Channel registered');
    }

    /**
     * 注销通知渠道
     */
    unregister(name: string): void {
        this.channels.delete(name);
        logger.info({ channel: name }, 'Channel unregistered');
    }

    /**
     * 获取指定渠道
     */
    get(name: string): NotifyChannel | undefined {
        return this.channels.get(name);
    }

    /**
     * 获取所有渠道
     */
    getAll(): NotifyChannel[] {
        return Array.from(this.channels.values());
    }

    /**
     * 获取已启用的渠道
     */
    getEnabled(): NotifyChannel[] {
        return this.getAll().filter(c => c.enabled);
    }

    /**
     * 启用渠道
     */
    enable(name: string): void {
        const channel = this.channels.get(name);
        if (channel) {
            channel.enabled = true;
            logger.info({ channel: name }, 'Channel enabled');
        }
    }

    /**
     * 禁用渠道
     */
    disable(name: string): void {
        const channel = this.channels.get(name);
        if (channel) {
            channel.enabled = false;
            logger.info({ channel: name }, 'Channel disabled');
        }
    }

    /**
     * 检查渠道是否存在
     */
    has(name: string): boolean {
        return this.channels.has(name);
    }

    /**
     * 获取渠道数量
     */
    size(): number {
        return this.channels.size;
    }
}

export const registry = new ChannelRegistry();
export default registry;

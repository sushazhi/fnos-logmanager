/**
 * 模板消息处理器
 */
import Logger from '../../utils/logger';
import { EventLogEntry } from './types';

const logger = Logger.child({ module: 'TemplateHandler' });

/**
 * 将时间戳转换为本地时间字符串
 */
export function formatTimestampToLocal(timestamp: number | string): string {
    let date: Date;

    if (typeof timestamp === 'number') {
        // Unix 时间戳
        if (timestamp > 1000000000000) {
            // 毫秒
            date = new Date(timestamp);
        } else if (timestamp > 1000000000) {
            // 秒
            date = new Date(timestamp * 1000);
        } else {
            date = new Date(timestamp);
        }
    } else {
        date = new Date(timestamp);
    }

    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');

    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

/**
 * 格式化 template 格式的日志消息
 */
export function formatTemplateMessage(param: Record<string, unknown>): string {
    const template = param.template as string;
    const cat = param.cat as number;

    // 模板翻译映射
    const templateMessages: Record<string, (p: Record<string, unknown>) => string> = {
        // 用户相关
        'LoginSucc': (p) => `${p.user || '用户'}登录成功 IP:${p.IP || '未知'}`,
        'LoginFail': (p) => `${p.user || '用户'}登录失败 IP:${p.IP || '未知'}`,
        'Logout': (p) => `${p.user || '用户'}登出`,
        'UserAdd': (p) => `添加用户 ${p.user || '未知'}`,
        'UserDel': (p) => `删除用户 ${p.user || '未知'}`,
        'UserMod': (p) => `修改用户 ${p.user || '未知'}`,
        'PassMod': (p) => `${p.user || '用户'}修改密码`,

        // 存储相关
        'DiskWakeup': (p) => `硬盘唤醒 ${p.disk || ''}`,
        'DiskSpindown': (p) => `硬盘休眠 ${p.disk || ''}`,
        'DiskAdd': (p) => `添加硬盘 ${p.disk || ''}`,
        'DiskRemove': (p) => `移除硬盘 ${p.disk || ''}`,
        'RaidCreate': (p) => `创建RAID ${p.raid || ''}`,
        'RaidDelete': (p) => `删除RAID ${p.raid || ''}`,
        'VolumeCreate': (p) => `创建存储卷 ${p.volume || ''}`,
        'VolumeDelete': (p) => `删除存储卷 ${p.volume || ''}`,
        'VolumeExpand': (p) => `扩展存储卷 ${p.volume || ''}`,

        // 文件操作
        'CreateFile': (p) => `创建文件 ${p.path || ''}`,
        'DeleteFile': (p) => `删除文件 ${p.path || ''}`,
        'MoveFile': (p) => `移动文件 ${p.src || ''} -> ${p.dst || ''}`,
        'CopyFile': (p) => `复制文件 ${p.src || ''} -> ${p.dst || ''}`,
        'RenameFile': (p) => `重命名文件 ${p.old || ''} -> ${p.new || ''}`,

        // 共享相关
        'ShareCreate': (p) => `创建共享 ${p.share || ''}`,
        'ShareDelete': (p) => `删除共享 ${p.share || ''}`,
        'ShareMod': (p) => `修改共享 ${p.share || ''}`,

        // 应用相关
        'AppInstall': (p) => `安装应用 ${p.app || ''}`,
        'AppUninstall': (p) => `卸载应用 ${p.app || ''}`,
        'AppUpdate': (p) => `更新应用 ${p.app || ''}`,
        'AppStart': (p) => `启动应用 ${p.app || ''}`,
        'AppStop': (p) => `停止应用 ${p.app || ''}`,
        'AppRestart': (p) => `重启应用 ${p.app || ''}`,

        // 系统相关
        'SystemStart': () => '系统启动',
        'SystemShutdown': () => '系统关机',
        'SystemReboot': () => '系统重启',
        'SystemUpdate': () => '系统更新',
        'NetworkChange': (p) => `网络配置变更 ${p.iface || ''}`,
        'ServiceStart': (p) => `启动服务 ${p.service || ''}`,
        'ServiceStop': (p) => `停止服务 ${p.service || ''}`,

        // 备份相关
        'BackupCreate': (p) => `创建备份 ${p.name || ''}`,
        'BackupRestore': (p) => `恢复备份 ${p.name || ''}`,
        'BackupDelete': (p) => `删除备份 ${p.name || ''}`,

        // Docker 相关
        'ContainerCreate': (p) => `创建容器 ${p.container || ''}`,
        'ContainerDelete': (p) => `删除容器 ${p.container || ''}`,
        'ContainerStart': (p) => `启动容器 ${p.container || ''}`,
        'ContainerStop': (p) => `停止容器 ${p.container || ''}`,

        // 权限相关
        'PermissionChange': (p) => `权限变更 ${p.path || ''}`,
        'AclChange': (p) => `ACL变更 ${p.path || ''}`
    };

    // 查找匹配的模板
    if (template && templateMessages[template]) {
        return templateMessages[template](param);
    }

    // 分类默认消息
    const categoryMessages: Record<number, string> = {
        1: '存储事件',
        2: '网络事件',
        3: '用户事件',
        4: '系统事件',
        5: '应用事件',
        6: '备份事件',
        7: 'Docker事件'
    };

    if (cat && categoryMessages[cat]) {
        return `${categoryMessages[cat]}: ${JSON.stringify(param)}`;
    }

    // 未知模板，返回原始信息
    if (template) {
        logger.debug({ template }, 'Unknown template type');
    }

    return param.message as string || JSON.stringify(param);
}

/**
 * 格式化事件为可读消息
 */
export function formatEventMessage(event: EventLogEntry): string {
    // 如果有 param 字段，尝试解析
    if (event.param) {
        try {
            const param = typeof event.param === 'string' ? JSON.parse(event.param) : event.param;
            return formatTemplateMessage(param);
        } catch {
            // 解析失败，使用原始消息
        }
    }

    // 如果有 template 字段
    if (event.template) {
        return formatTemplateMessage({ template: event.template, ...event });
    }

    // 使用 message 字段
    if (event.message) {
        return event.message;
    }

    // 返回 JSON 格式
    return JSON.stringify(event);
}

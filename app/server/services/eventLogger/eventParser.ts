/**
 * 事件日志行解析工具
 * 提取自 services/eventLogger.ts 中 queryEvents 和 getNewEvents 的重复逻辑
 */

import type { EventLogEntry, EventSeverity } from '../../types/eventLogger';

/**
 * 计算本地时区与 UTC 的偏移量（秒）
 */
function getLocalTimezoneOffsetSeconds(): number {
    const now = new Date();
    return -now.getTimezoneOffset() * 60;
}

const TIMEZONE_OFFSET_SECONDS = getLocalTimezoneOffsetSeconds();

/**
 * 格式化 template 格式的日志消息
 */
function formatTemplateMessage(param: any): string {
    const template = param.template;
    
    const templateMessages: Record<string, (p: any) => string> = {
        'LoginSucc': (p) => `${p.user || '用户'}登录成功 IP:${p.IP || '未知'}`,
        'LoginFail': (p) => `${p.user || '用户'}登录失败 IP:${p.IP || '未知'}`,
        'Logout': (p) => `${p.user || '用户'}登出`,
        'UserAdd': (p) => `添加用户 ${p.user || '未知'}`,
        'UserDel': (p) => `删除用户 ${p.user || '未知'}`,
        'UserMod': (p) => `修改用户 ${p.user || '未知'}`,
        'DiskWakeup': (p) => `硬盘${p.disk || '未知'}已被唤醒 型号:${p.model || '未知'} 序列号:${p.serial || '未知'}`,
        'DiskSpindown': (p) => `硬盘${p.disk || '未知'}已休眠 型号:${p.model || '未知'} 序列号:${p.serial || '未知'}`,
        'DiskAdd': (p) => `硬盘${p.disk || '未知'}已添加 型号:${p.model || '未知'}`,
        'DiskRemove': (p) => `硬盘${p.disk || '未知'}已移除`,
        'DiskError': (p) => `硬盘${p.disk || '未知'}错误 ${p.error || ''}`,
        'NetUp': (p) => `网络接口${p.iface || '未知'}已启动`,
        'NetDown': (p) => `网络接口${p.iface || '未知'}已停止`,
        'NetChange': (p) => `网络配置已更改`,
        'Shutdown': (p) => `系统关机`,
        'Reboot': (p) => `系统重启`,
        'Startup': (p) => `系统启动`,
        'ServiceStart': (p) => `服务${p.service || '未知'}已启动`,
        'ServiceStop': (p) => `服务${p.service || '未知'}已停止`,
        'ServiceRestart': (p) => `服务${p.service || '未知'}已重启`,
        'AppInstall': (p) => `应用${p.app || '未知'}已安装`,
        'AppUninstall': (p) => `应用${p.app || '未知'}已卸载`,
        'AppUpdate': (p) => `应用${p.app || '未知'}已更新`,
        'AppStart': (p) => `应用${p.app || '未知'}已启动`,
        'AppStop': (p) => `应用${p.app || '未知'}已停止`,
        'DeleteFile': (p) => `删除"${p.FILE || p.file || '未知'}"`,
        'CreateFile': (p) => `创建"${p.FILE || p.file || '未知'}"`,
        'MoveFile': (p) => `移动"${p.SRC || p.src || '未知'}" 到 "${p.DST || p.dst || '未知'}"`,
        'CopyFile': (p) => `复制"${p.SRC || p.src || '未知'}" 到 "${p.DST || p.dst || '未知'}"`,
        'RenameFile': (p) => `重命名"${p.OLD || p.old || '未知'}" 为 "${p.NEW || p.new || '未知'}"`,
        'Mkdir': (p) => `创建目录"${p.FILE || p.file || p.dir || '未知'}"`,
        'UploadFile': (p) => `上传"${p.FILE || p.file || '未知'}"`,
        'DownloadFile': (p) => `下载"${p.FILE || p.file || '未知'}"`,
    };
    
    const formatter = templateMessages[template];
    if (formatter) {
        return formatter(param);
    }
    return `${template}: ${JSON.stringify(param)}`;
}

/**
 * 应用事件动作映射
 */
const APP_ACTION_MAP: Record<string, string> = {
    'APP_STARTED': '启用成功',
    'APP_STOPPED': '停止成功',
    'APP_INSTALLED': '安装成功',
    'APP_UNINSTALLED': '卸载成功',
    'APP_UPDATED': '更新成功',
    'APP_UPGRADED': '升级成功',
    'APP_CRASH': '异常退出',
    'APP_START_FAILED_LOCAL_APP_RUN_EXCEPTION': '启用失败。原因：执行应用启动脚本失败。'
};

/**
 * Parse numeric loglevel to severity
 */
function parseLoglevel(level: string | number): EventSeverity {
    if (typeof level === 'number') {
        switch (level) {
            case 0: return 'info';
            case 1: return 'warning';
            case 2: return 'error';
            case 3: return 'critical';
            default: return level >= 4 ? 'critical' : 'info';
        }
    }
    const s = String(level).toLowerCase();
    if (s.includes('crit') || s.includes('emerg') || s.includes('panic')) return 'critical';
    if (s.includes('err')) return 'error';
    if (s.includes('warn')) return 'warning';
    if (s.includes('debug') || s.includes('trace')) return 'debug';
    return 'info';
}

/**
 * 识别行中的关键列名
 */
function identifyColumns(keys: string[]) {
    const timestampKey = keys.find(k => 
        k.toLowerCase() === 'logtime' || k.toLowerCase() === 'timestamp' || 
        k.toLowerCase().includes('time') || k.toLowerCase().includes('date')
    );
    const sourceKey = keys.find(k => 
        k.toLowerCase() === 'serviceid' || k.toLowerCase() === 'service' ||
        k.toLowerCase().includes('source') || k.toLowerCase().includes('app')
    );
    const messageKey = keys.find(k => 
        k.toLowerCase() === 'message' || k.toLowerCase() === 'msg' || 
        k.toLowerCase() === 'content' || k.toLowerCase().includes('description') ||
        k.toLowerCase() === 'log'
    );
    const severityKey = keys.find(k => 
        k.toLowerCase() === 'loglevel' || k.toLowerCase() === 'level' || 
        k.toLowerCase().includes('severity') || k.toLowerCase().includes('priority')
    );
    const typeKey = keys.find(k => 
        k.toLowerCase() === 'type' || k.toLowerCase() === 'category' ||
        k.toLowerCase().includes('event')
    );
    const idKey = keys.find(k => k.toLowerCase() === 'id');
    const metadataKey = keys.find(k => 
        k.toLowerCase().includes('metadata') || k.toLowerCase().includes('extra')
    );
    const userKey = keys.find(k => 
        k.toLowerCase() === 'uid' || k.toLowerCase() === 'uname' ||
        k.toLowerCase().includes('user')
    );

    return { timestampKey, sourceKey, messageKey, severityKey, typeKey, idKey, metadataKey, userKey };
}

/**
 * 处理时间戳值
 * 将飞牛系统数据库中的本地时间戳转换为 ISO 格式
 */
function processTimestamp(timestampValue: any): string {
    if (typeof timestampValue === 'number' && timestampValue > 1000000000) {
        const adjustedTimestamp = timestampValue + TIMEZONE_OFFSET_SECONDS;
        const date = new Date(adjustedTimestamp * 1000);
        return date.toISOString();
    } else if (typeof timestampValue === 'number' && timestampValue > 1000000000000) {
        const adjustedTimestamp = timestampValue + (TIMEZONE_OFFSET_SECONDS * 1000);
        const date = new Date(adjustedTimestamp);
        return date.toISOString();
    } else if (typeof timestampValue === 'string') {
        const parsed = new Date(timestampValue);
        if (!isNaN(parsed.getTime())) {
            return parsed.toISOString();
        }
    }
    return timestampValue || new Date().toISOString();
}

/**
 * 处理消息内容
 * 从 parameter 字段提取有用信息，支持 template 格式和应用事件格式
 */
function processMessage(row: any, messageKey: string | undefined, keys: string[], 
    timestampKey: string | undefined, sourceKey: string | undefined, 
    severityKey: string | undefined, typeKey: string | undefined, 
    metadataKey: string | undefined, userKey: string | undefined): { message: string; eventCode: string } {
    
    let message = row[messageKey || 'message'];
    let eventCode = '';
    
    if (!message || message === 'null') {
        const parameterStr = row['parameter'];
        if (parameterStr && parameterStr !== 'null') {
            try {
                const param = typeof parameterStr === 'string' ? JSON.parse(parameterStr) : parameterStr;
                
                if (param.template) {
                    message = formatTemplateMessage(param);
                } else if (param.data) {
                    const appName = param.data.APP_NAME;
                    const displayName = param.data.DISPLAY_NAME;
                    const eventId = param.eventId;
                    eventCode = eventId || '';
                    
                    const appNameCn = displayName || appName || '';
                    const action = eventId ? (APP_ACTION_MAP[eventId] || eventId) : '';
                    
                    if (appNameCn && action) {
                        message = `应用 ${appNameCn} ${action}`;
                    } else if (appNameCn) {
                        message = appNameCn;
                    } else if (eventId) {
                        message = eventId;
                    }
                }
                if (!message) {
                    message = parameterStr.substring(0, 500);
                }
            } catch {
                message = String(parameterStr).substring(0, 500);
            }
        }
    }
    
    if (!message || message === 'null') {
        const excludedKeys = [timestampKey, sourceKey, severityKey, typeKey, metadataKey, userKey, 'parameter'];
        const idKey = keys.find(k => k.toLowerCase() === 'id');
        if (idKey) excludedKeys.push(idKey);
        const otherFields = keys.filter(k => !excludedKeys.includes(k));
        message = otherFields.map(k => `${k}: ${row[k]}`).join(', ').substring(0, 500);
    }

    return { message, eventCode };
}

/**
 * 解析事件数据库行为 EventLogEntry
 * 统一了 queryEvents 和 getNewEvents 中的重复解析逻辑
 */
export function parseEventRow(row: any): EventLogEntry {
    const keys = Object.keys(row);
    const { timestampKey, sourceKey, messageKey, severityKey, typeKey, idKey, metadataKey, userKey } = identifyColumns(keys);
    
    const timestampValue = processTimestamp(row[timestampKey || 'timestamp']);
    const { message, eventCode } = processMessage(row, messageKey, keys, timestampKey, sourceKey, severityKey, typeKey, metadataKey, userKey);
    
    let userValue = row[userKey || 'user'];
    if (userValue === null || userValue === 'null') {
        userValue = undefined;
    }

    return {
        id: row[idKey || 'id'] || 0,
        timestamp: timestampValue,
        source: row[sourceKey || 'source'] || 'unknown',
        eventType: row[typeKey || 'event_type'] || 'system',
        severity: parseLoglevel(row[severityKey || 'level']),
        message: message,
        metadata: row[metadataKey || 'metadata'],
        user: userValue,
        eventCode: eventCode
    } as EventLogEntry;
}

/**
 * 将时间戳转换为本地时间字符串
 */
export function formatTimestampToLocal(timestamp: number | string): string {
    let date: Date;
    
    if (typeof timestamp === 'number') {
        if (timestamp > 1000000000000) {
            date = new Date(timestamp);
        } else if (timestamp > 1000000000) {
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

// Re-export for backward compatibility
export { formatTemplateMessage, parseLoglevel, TIMEZONE_OFFSET_SECONDS };

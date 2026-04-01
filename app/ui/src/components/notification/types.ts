/**
 * 通知面板组件类型定义
 */

export interface GlobalSettingsProps {
  enabled: boolean;
  checkInterval: number;
}

export interface MonitorStatusProps {
  running: boolean;
  watchedFiles: number;
  activeRules: number;
  lastCheckTime: Date | null;
  errors: string[];
}

export interface ChannelItem {
  name: string;
  channel: string;
  enabled: boolean;
  config: Record<string, unknown>;
}

export interface RuleItem {
  id: string;
  name: string;
  enabled: boolean;
  appName: string;
  logLevel: string;
  keywords: string[];
  excludeKeywords: string[];
  cooldown: number;
  quietHoursStart: string;
  quietHoursEnd: string;
  channels: string[];
}

export const CHANNEL_TYPES: Record<string, string> = {
  bark: 'Bark',
  dingtalk: '钉钉机器人',
  feishu: '飞书机器人',
  telegram: 'Telegram',
  serverchan: 'Server酱',
  pushplus: 'PushPlus',
  wechat: '企业微信',
  ntfy: 'Ntfy',
  gotify: 'Gotify',
  webhook: '自定义Webhook'
};

export const LOG_LEVELS = [
  { value: 'all', label: '全部' },
  { value: 'error', label: '错误' },
  { value: 'warn', label: '警告' },
  { value: 'info', label: '信息' },
  { value: 'debug', label: '调试' }
];

export const CHECK_INTERVALS = [
  { value: 10000, label: '10秒' },
  { value: 30000, label: '30秒' },
  { value: 60000, label: '1分钟' },
  { value: 300000, label: '5分钟' }
];

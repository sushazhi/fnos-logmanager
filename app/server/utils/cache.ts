/**
 * 内存缓存实现（支持 LRU 淘汰策略）
 */
export class Cache {
  private cache: Map<string, { value: any; expires: number; lastAccessed: number }>;
  private cleanupInterval: NodeJS.Timeout;
  private maxSize: number;

  constructor(private defaultTTL: number = 600000, maxSize: number = 1000) { // 默认 10 分钟，最大 1000 条
    this.cache = new Map();
    this.maxSize = maxSize;
    
    // 定期清理过期缓存
    this.cleanupInterval = setInterval(() => {
      this.cleanup();
    }, 60000); // 每分钟清理一次
  }

  /**
   * 设置缓存
   */
  set<T>(key: string, value: T, ttl?: number): void {
    // 如果超过最大容量，执行 LRU 淘汰
    if (this.cache.size >= this.maxSize && !this.cache.has(key)) {
      this.evictLRU();
    }

    const expires = Date.now() + (ttl || this.defaultTTL);
    this.cache.set(key, { value, expires, lastAccessed: Date.now() });
  }

  /**
   * 获取缓存
   */
  get<T>(key: string): T | null {
    const item = this.cache.get(key);
    
    if (!item) {
      return null;
    }

    // 检查是否过期
    if (Date.now() > item.expires) {
      this.cache.delete(key);
      return null;
    }

    // 更新访问时间（LRU）
    item.lastAccessed = Date.now();

    return item.value as T;
  }

  /**
   * 检查缓存是否存在
   */
  has(key: string): boolean {
    const item = this.cache.get(key);
    
    if (!item) {
      return false;
    }

    // 检查是否过期
    if (Date.now() > item.expires) {
      this.cache.delete(key);
      return false;
    }

    return true;
  }

  /**
   * 删除缓存
   */
  delete(key: string): boolean {
    return this.cache.delete(key);
  }

  /**
   * 清空所有缓存
   */
  clear(): void {
    this.cache.clear();
  }

  /**
   * 获取缓存大小
   */
  size(): number {
    return this.cache.size;
  }

  /**
   * LRU 淘汰：移除最久未访问的条目
   */
  private evictLRU(): void {
    let oldestKey: string | null = null;
    let oldestTime = Infinity;

    for (const [key, item] of this.cache.entries()) {
      if (item.lastAccessed < oldestTime) {
        oldestTime = item.lastAccessed;
        oldestKey = key;
      }
    }

    if (oldestKey) {
      this.cache.delete(oldestKey);
    }
  }

  /**
   * 清理过期缓存
   */
  private cleanup(): void {
    const now = Date.now();
    
    for (const [key, item] of this.cache.entries()) {
      if (now > item.expires) {
        this.cache.delete(key);
      }
    }
  }

  /**
   * 销毁缓存
   */
  destroy(): void {
    clearInterval(this.cleanupInterval);
    this.cache.clear();
  }
}

/**
 * 装饰器：缓存函数结果
 */
export function cached(ttl?: number) {
  const cache = new Cache(ttl);
  
  return function (
    target: any,
    propertyKey: string,
    descriptor: PropertyDescriptor
  ) {
    const originalMethod = descriptor.value;
    
    descriptor.value = async function (...args: any[]) {
      const key = `${propertyKey}:${JSON.stringify(args)}`;
      
      // 尝试从缓存获取
      const cachedResult = cache.get(key);
      if (cachedResult !== null) {
        return cachedResult;
      }
      
      // 执行原方法
      const result = await originalMethod.apply(this, args);
      
      // 缓存结果
      cache.set(key, result);
      
      return result;
    };
    
    return descriptor;
  };
}

/**
 * 全局缓存实例
 */
export const globalCache = new Cache();

/**
 * 缓存键生成器
 */
export function createCacheKey(...parts: (string | number | boolean)[]): string {
  return parts.map(part => String(part)).join(':');
}

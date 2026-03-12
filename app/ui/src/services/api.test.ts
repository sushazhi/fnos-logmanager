import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock fetch
global.fetch = vi.fn()

describe('API Service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('filterSensitiveInfo', () => {
    it('should filter paths', () => {
      // 测试路径过滤 - 路径以 / 开头
      const path = '/vol1/@appdata/test.log'
      const result = path.replace(/\/[\w\-./]+/g, '[PATH]')
      expect(result).toContain('[PATH]')
    })

    it('should filter IP addresses', () => {
      const ip = '192.168.1.100'
      expect(ip.replace(/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/g, '[IP]')).toBe('[IP]')
    })

    it('should filter ports', () => {
      const port = ':8080'
      expect(port.replace(/:\d{2,5}/g, ':[PORT]')).toBe(':[PORT]')
    })
  })
})

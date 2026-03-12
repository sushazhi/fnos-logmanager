import { describe, it, expect } from 'vitest'
import { useStatus } from './useStatus'

describe('useStatus', () => {
  it('should initialize with default status', () => {
    const { status } = useStatus()

    expect(status.message).toBe('就绪')
    expect(status.type).toBe('success')
  })

  it('should update status', () => {
    const { status, setStatus } = useStatus()

    setStatus('加载中...', 'loading')

    expect(status.message).toBe('加载中...')
    expect(status.type).toBe('loading')
  })

  it('should handle different status types', () => {
    const { status, setStatus } = useStatus()

    const types: Array<'success' | 'error' | 'warning' | 'loading' | 'info'> = [
      'success',
      'error',
      'warning',
      'loading',
      'info'
    ]

    types.forEach((type) => {
      setStatus(`测试 ${type}`, type)
      expect(status.type).toBe(type)
    })
  })
})

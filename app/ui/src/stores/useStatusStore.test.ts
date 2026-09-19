import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useStatusStore } from '../stores/useStatusStore'

// 本文件替代原先的 useStatus.test.ts。
//
// 原测试针对 composables/useStatus.ts 的 useStatus() 兼容层，该兼容层是一个
// setup store 的透传（返回 store.status 这个 ref），而测试按普通对象断言
// status.message —— 在 v0.3.12 迁移到 Pinia 后即失效：既缺 active pinia，
// 也漏了 .value。由于 useStatus() 在生产代码中已无调用方，该兼容层已随本次
// 改动删除，测试改为直接覆盖真正承载逻辑的 useStatusStore。

describe('useStatusStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('初始状态为「就绪 / success」', () => {
    const store = useStatusStore()

    expect(store.status.message).toBe('就绪')
    expect(store.status.type).toBe('success')
  })

  it('setStatus 更新消息与类型', () => {
    const store = useStatusStore()

    store.setStatus('加载中...', 'loading')

    expect(store.status.message).toBe('加载中...')
    expect(store.status.type).toBe('loading')
  })

  it('setStatus 省略类型时默认为 success', () => {
    const store = useStatusStore()

    store.setStatus('加载中...', 'error')
    store.setStatus('完成')

    expect(store.status.type).toBe('success')
  })

  it.each(['success', 'error', 'warning', 'loading', 'info'] as const)(
    '支持 %s 状态类型',
    type => {
      const store = useStatusStore()

      store.setStatus('x', type)

      expect(store.status.type).toBe(type)
    }
  )

  it('confirm 为缺省字段补上默认值', async () => {
    const store = useStatusStore()
    let received: Record<string, unknown> | null = null
    store.setConfirmFn(async options => {
      received = options as unknown as Record<string, unknown>
      return true
    })

    await store.confirm({ message: '确定要删除吗？' })

    expect(received).toEqual({
      title: '确认',
      message: '确定要删除吗？',
      type: 'warning',
      confirmText: '确定'
    })
  })

  it('confirm 保留调用方显式传入的字段', async () => {
    const store = useStatusStore()
    let received: Record<string, unknown> | null = null
    store.setConfirmFn(async options => {
      received = options as unknown as Record<string, unknown>
      return false
    })

    const result = await store.confirm({
      title: '危险操作',
      message: '清空该日志？',
      type: 'danger',
      confirmText: '清空'
    })

    expect(received).toMatchObject({
      title: '危险操作',
      message: '清空该日志？',
      type: 'danger',
      confirmText: '清空'
    })
    expect(result).toBe(false)
  })
})

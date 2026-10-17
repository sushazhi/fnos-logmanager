import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// 系统空间清理面板的分类折叠行为。
//
// 关注点只有一个：扫描完成后「清理分类」下的子项是否默认折叠。
// 分类头本身已经给出名称、条目数与可释放体积，默认铺开所有子项会把汇总信息挤出视野。
//
// 打开面板的代价由服务端保证：不带 refresh 的请求读快照，陈旧时服务端后台重扫并置
// refreshing=true。因此下面折叠相关用例的默认夹具必须是 refreshing=false，
// 否则会多出后台请求，把「点击重新扫描」这类断言打乱。

const getUsage = vi.fn()

vi.mock('../services/api', () => ({
  spaceApi: {
    getUsage: (...args: unknown[]) => getUsage(...args),
    clean: vi.fn()
  }
}))

import SpaceCleanPanel from './SpaceCleanPanel.vue'

function item(path: string, over: Record<string, unknown> = {}) {
  return {
    path,
    name: path.split('/').pop() || path,
    size: 1024,
    sizeFormatted: '1.0 KB',
    isDir: false,
    fileCount: 1,
    reason: '可再生缓存',
    safe: true,
    inUse: false,
    strategy: 'delete',
    modified: '',
    ...over
  }
}

function usage(over: Record<string, unknown> = {}) {
  const categories = [
    {
      id: 'pkg-cache',
      name: '包管理器缓存',
      description: 'apt/pip 缓存',
      risk: 'low',
      icon: 'box',
      itemCount: 2,
      totalSize: 2048,
      totalSizeFormatted: '2.0 KB',
      reclaimable: 2048,
      reclaimableFormatted: '2.0 KB',
      items: [item('/var/cache/apt/pkg1'), item('/var/cache/apt/pkg2')]
    },
    {
      id: 'thumb-cache',
      name: '缩略图缓存',
      description: '媒体缩略图',
      risk: 'low',
      icon: 'image',
      itemCount: 1,
      totalSize: 1024,
      totalSizeFormatted: '1.0 KB',
      reclaimable: 1024,
      reclaimableFormatted: '1.0 KB',
      items: [item('/vol1/@appdata/media/thumb')]
    }
  ] as Record<string, unknown>[]
  return {
    scannedAt: new Date().toISOString(),
    durationMs: 12,
    volumes: [],
    categories,
    notices: [],
    reclaimable: 3072,
    reclaimableFormatted: '3.0 KB',
    categoriesWithItems: 2,
    // 服务端恒定给出的时效字段。默认值取「刚扫完、新鲜、没有后台任务」，
    // 这样默认夹具不会触发任何后台轮询，折叠相关的用例才不会被多余请求干扰。
    cacheAgeMs: 0,
    cacheAgeSeconds: 0,
    stale: false,
    refreshing: false,
    ...over
  }
}

async function mountPanel(data: Record<string, unknown> = usage()) {
  getUsage.mockResolvedValue(data)
  const wrapper = mount(SpaceCleanPanel, { attachTo: document.body })
  await flushPromises()
  return wrapper
}

describe('SpaceCleanPanel 清理分类折叠', () => {
  beforeEach(() => {
    getUsage.mockReset()
  })

  it('扫描完成后所有分类的子项默认折叠', async () => {
    const wrapper = await mountPanel()

    expect(wrapper.findAll('.category-card').length).toBe(2)
    expect(wrapper.findAll('.category-card.open').length).toBe(0)
    expect(wrapper.findAll('.item-list').length).toBe(0)
    // 分类头仍然可见，折叠不能把分类本身藏起来
    expect(wrapper.text()).toContain('包管理器缓存')
    expect(wrapper.text()).toContain('缩略图缓存')
  })

  it('点击分类头展开该分类，再次点击收起', async () => {
    const wrapper = await mountPanel()
    const heads = wrapper.findAll('.category-head')

    await heads[0].trigger('click')
    expect(wrapper.findAll('.item-list').length).toBe(1)
    expect(wrapper.findAll('.category-card')[0].classes()).toContain('open')
    expect(wrapper.text()).toContain('pkg1')
    // 只展开点击的那一个
    expect(wrapper.findAll('.category-card')[1].classes()).not.toContain('open')

    await heads[0].trigger('click')
    expect(wrapper.findAll('.item-list').length).toBe(0)
  })

  it('重新扫描不会把分类自动展开', async () => {
    const wrapper = await mountPanel()
    await wrapper.findAll('.category-head')[1].trigger('click')
    expect(wrapper.findAll('.item-list').length).toBe(1)

    await wrapper.findAll('.ghost-btn')[0].trigger('click')
    await flushPromises()

    // 重扫后保持用户此前的手工展开状态，且不新增任何自动展开
    expect(getUsage).toHaveBeenLastCalledWith(true)
    expect(wrapper.findAll('.item-list').length).toBe(1)
    expect(wrapper.findAll('.category-card')[0].classes()).not.toContain('open')
  })

  it('折叠状态下仍可通过分类勾选框批量选中', async () => {
    const wrapper = await mountPanel()
    const catCheck = wrapper.findAll('.cat-check input')[0]

    await catCheck.setValue(true)

    expect(wrapper.text()).toContain('已选 2 项')
    expect(wrapper.findAll('.item-list').length).toBe(0)
  })
})

// 真机缺陷：磁盘与分区使用情况里出现了 /vol00、/vol02 两行原始路径。
//
// 真机（fnOS OECT）实测：/vol00 是外接存储的容器目录，/vol02 是残留的普通目录，
// 两者都不是分区；外接盘真正挂在 /vol00/WDC WD10SDRW-34A0XS0（1 TB）。
describe('SpaceCleanPanel 磁盘与分区显示名', () => {
  beforeEach(() => {
    getUsage.mockReset()
  })

  function volume(mount: string, over: Record<string, unknown> = {}) {
    return {
      mount,
      total: 100 * 1024 ** 3,
      used: 40 * 1024 ** 3,
      free: 60 * 1024 ** 3,
      usedPercent: 40,
      totalFormatted: '100 GB',
      usedFormatted: '40 GB',
      freeFormatted: '60 GB',
      ...over
    }
  }

  it('按服务端语义名显示，不再显示原始挂载路径', async () => {
    const wrapper = await mountPanel(
      usage({
        volumes: [
          volume('/', { kind: 'system', label: '系统空间' }),
          volume('/vol1', { kind: 'storage', label: '存储空间1' })
        ]
      })
    )

    const rows = wrapper.findAll('.volume-row')
    expect(rows.length).toBe(2)
    expect(wrapper.find('.volume-mount').text()).toBe('系统空间')
    expect(wrapper.text()).toContain('存储空间1')
    // 原始路径不出现在正文里，只留在 title 中供排查
    expect(wrapper.find('.volume-mount').attributes('title')).toBe('/')
    expect(wrapper.text()).not.toContain('/vol1')
  })

  // 外接存储不属于「系统空间清理」：清理规则本来就不扫外接盘，列出来只会让人
  // 以为这一屏能管到外接盘的空间。
  it('不显示外接存储行', async () => {
    const wrapper = await mountPanel(
      usage({
        volumes: [
          volume('/', { kind: 'system', label: '系统空间' }),
          volume('/vol1', { kind: 'storage', label: '存储空间1' }),
          volume('/vol00/WDC WD10SDRW-34A0XS0', {
            kind: 'external',
            label: '外接存储（WDC WD10SDRW-34A0XS0）'
          })
        ]
      })
    )

    expect(wrapper.findAll('.volume-row').length).toBe(2)
    expect(wrapper.text()).not.toContain('外接存储')
    expect(wrapper.text()).not.toContain('/vol00')
  })

  // 旧服务端 / 旧快照缓存不会带 kind，只能按挂载路径识别外接盘。
  it('旧数据里没有 kind 时也按 /vol00 路径过滤外接存储', async () => {
    const wrapper = await mountPanel(
      usage({
        volumes: [
          volume('/', { kind: 'system', label: '系统空间' }),
          volume('/vol00/WDC WD10SDRW-34A0XS0', { label: '外接存储（WDC WD10SDRW-34A0XS0）' })
        ]
      })
    )

    const rows = wrapper.findAll('.volume-row')
    expect(rows.length).toBe(1)
    expect(wrapper.find('.volume-mount').text()).toBe('系统空间')
  })

  it('服务端未提供 label 时也不显示裸斜杠', async () => {
    // 兼容旧服务端/旧缓存：本地兜底只认系统分区，不猜其它语义名。
    const wrapper = await mountPanel(usage({ volumes: [volume('/'), volume('/vol1')] }))
    const mounts = wrapper.findAll('.volume-mount')

    expect(mounts[0].text()).toBe('系统空间')
    expect(mounts[1].text()).toBe('/vol1')
  })

  it('系统分区行按 tempOnSystemDisk 给出 /tmp 归属提示', async () => {
    const wrapper = await mountPanel(
      usage({ volumes: [volume('/', { kind: 'system', label: '系统空间' })], tempOnSystemDisk: true })
    )
    expect(wrapper.text()).toContain('/tmp 与 /var/tmp 位于此处')

    // 服务端判定不了时不显示任何断言
    const unknown = await mountPanel(
      usage({ volumes: [volume('/', { kind: 'system', label: '系统空间' })] })
    )
    expect(unknown.text()).not.toContain('/tmp')
  })
})

// 数据时效与后台重扫收敛。
//
// 背景：服务端拿到陈旧快照时不会让请求等一次全盘扫描，而是立刻返回旧数据并置
// refreshing=true，自己在后台重扫。前端因此有两条独立的要求：
//   1) 页头要如实说明「这份数据有多旧、此刻有没有在更新」——旧数据也是可用数据，
//      所以用弱化文字而不是错误横幅；
//   2) 看到 refreshing=true 后要用非强制请求回来取结果，直到 refreshing 变回 false，
//      并且必须有上限、必须能在关闭面板时停下。
// 若轮询误用 refresh=true，就等于把「打开面板不再等全盘扫描」重新变成一次同步全盘扫描，
// 所以下面每条用例都顺带断言了轮询调用参数。
describe('SpaceCleanPanel 数据时效与后台更新', () => {
  beforeEach(() => {
    getUsage.mockReset()
  })

  afterEach(() => {
    // 用例内安装的假定时器必须还原，否则会污染后续用例里的 Promise 与 DOM 更新。
    vi.useRealTimers()
  })

  /**
   * 只伪造 setTimeout/clearTimeout。
   *
   * 不能按默认全量伪造：flushPromises 与 Vue 的调度都依赖真实 setImmediate/微任务，
   * 把 setImmediate 一起伪造掉之后 await flushPromises() 会永远不 resolve。
   */
  function usePollTimers() {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  }

  /** 依次返回给定的响应，最后一次会重复用于后续轮询。 */
  async function mountSeq(...datas: Record<string, unknown>[]) {
    for (let i = 0; i < datas.length - 1; i++) getUsage.mockResolvedValueOnce(datas[i])
    getUsage.mockResolvedValue(datas[datas.length - 1])
    const wrapper = mount(SpaceCleanPanel, { attachTo: document.body })
    await flushPromises()
    return wrapper
  }

  /** 推过一个轮询周期。 */
  async function advancePoll(rounds = 1) {
    for (let i = 0; i < rounds; i++) await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
  }

  it('页头展示数据年龄与扫描耗时，且不写死毫秒', async () => {
    const wrapper = await mountPanel(usage({ cacheAgeSeconds: 90, durationMs: 4500 }))
    const sub = wrapper.find('.header-sub').text()

    expect(sub).toContain('数据更新于 1 分钟前')
    // 4.5 秒写成「4500 ms」要用户自己换算，秒级以上换单位
    expect(sub).toContain('扫描耗时 4.5 秒')
    expect(sub).toContain('发现 2 类可清理项')
  })

  it('刚扫完的数据写「刚刚更新」，年龄单位随量级切换', async () => {
    const justNow = await mountPanel(usage({ cacheAgeSeconds: 0 }))
    expect(justNow.find('.header-sub').text()).toContain('数据刚刚更新')

    const hours = await mountPanel(usage({ cacheAgeSeconds: 7200, stale: true }))
    expect(hours.find('.header-sub').text()).toContain('数据更新于 2 小时前')
  })

  it('新鲜数据不显示任何时效提示', async () => {
    const wrapper = await mountPanel(usage({ stale: false, refreshing: false }))

    expect(wrapper.find('.header-freshness').exists()).toBe(false)
    // 无提示时模板不能留下多余的分隔符
    expect(wrapper.find('.header-sub').text()).not.toContain(' ·  ·')
  })

  it('只旧不扫时提示可以手动重扫，且不会自动发请求', async () => {
    usePollTimers()
    // 真机上会出现「快照偏旧、但此刻没有重扫在跑」，这两个标志必须能独立成立。
    const wrapper = await mountSeq(usage({ cacheAgeSeconds: 400, stale: true, refreshing: false }))

    expect(wrapper.find('.header-freshness').text()).toContain('数据已偏旧')
    expect(wrapper.find('.header-freshness').text()).toContain('重新扫描')
    expect(wrapper.text()).not.toContain('正在后台更新')

    await advancePoll(3)
    // 没有 refreshing 就不该有任何后台请求
    expect(getUsage).toHaveBeenCalledTimes(1)
  })

  it('后台更新中自动取回结果，取到新数据后停止轮询', async () => {
    usePollTimers()
    const fresh = usage({ cacheAgeSeconds: 0, stale: false, refreshing: false })
    const wrapper = await mountSeq(usage({ cacheAgeSeconds: 400, stale: true, refreshing: true }), fresh)

    expect(wrapper.find('.header-freshness').text()).toContain('正在后台更新')
    expect(getUsage).toHaveBeenCalledTimes(1)
    expect(getUsage).toHaveBeenLastCalledWith(false)

    await advancePoll()
    expect(getUsage).toHaveBeenCalledTimes(2)
    // 服务端自己在重扫，客户端只能取结果，不能传 refresh=true 再触发一次全量扫描
    expect(getUsage).toHaveBeenLastCalledWith(false)
    // 新结果落到界面上，提示随之消失
    expect(wrapper.find('.header-sub').text()).toContain('数据刚刚更新')
    expect(wrapper.find('.header-freshness').exists()).toBe(false)

    await advancePoll(3)
    // 已经收敛，定时器必须停掉
    expect(getUsage).toHaveBeenCalledTimes(2)
  })

  it('后台一直不收敛时轮询有上限，不会永远请求下去', async () => {
    usePollTimers()
    // 服务端始终报告 refreshing：模拟重扫失败或卡住，前端必须自己收敛。
    const wrapper = await mountSeq(usage({ cacheAgeSeconds: 400, stale: true, refreshing: true }))

    await advancePoll(20)

    // 首次取数 1 次 + 最多 12 次轮询
    expect(getUsage).toHaveBeenCalledTimes(13)
    expect(wrapper.find('.header-freshness').text()).toContain('正在后台更新')

    await advancePoll(5)
    expect(getUsage).toHaveBeenCalledTimes(13)
  })

  it('重扫不会留下两个并存的轮询者', async () => {
    usePollTimers()
    const stuck = usage({ cacheAgeSeconds: 400, stale: true, refreshing: true })
    // 每次调用都返回 refreshing，这样旧的轮询者若还活着就会多出一次请求。
    getUsage.mockResolvedValue(stuck)
    const wrapper = mount(SpaceCleanPanel, { attachTo: document.body })
    await flushPromises()
    expect(getUsage).toHaveBeenCalledTimes(1)

    // 用户在上一次轮询到点前手动重扫
    await wrapper.findAll('.ghost-btn')[0].trigger('click')
    await flushPromises()
    expect(getUsage).toHaveBeenCalledTimes(2)
    expect(getUsage).toHaveBeenLastCalledWith(true)

    await advancePoll()
    // 1 次挂载 + 1 次手动重扫 + 1 次轮询；若旧定时器没被清掉这里会是 4
    expect(getUsage).toHaveBeenCalledTimes(3)
    expect(getUsage).toHaveBeenLastCalledWith(false)
  })

  it('关闭面板即停止轮询', async () => {
    usePollTimers()
    const wrapper = await mountSeq(usage({ cacheAgeSeconds: 400, stale: true, refreshing: true }))
    expect(getUsage).toHaveBeenCalledTimes(1)

    await wrapper.find('.close-btn').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()

    await advancePoll(3)
    // 面板关了还在后台要数据是纯粹的浪费
    expect(getUsage).toHaveBeenCalledTimes(1)
  })

  it('卸载后迟到的响应不再安排下一次轮询', async () => {
    usePollTimers()
    const wrapper = await mountSeq(usage({ cacheAgeSeconds: 400, stale: true, refreshing: true }))
    expect(getUsage).toHaveBeenCalledTimes(1)

    wrapper.unmount()
    await advancePoll(3)
    expect(getUsage).toHaveBeenCalledTimes(1)
  })

  it('挂载时按非强制请求取数，不会触发全盘同步扫描', async () => {
    usePollTimers()
    await mountSeq(usage())
    // 打开面板的代价必须是一次读快照，而不是一次全盘扫描
    expect(getUsage).toHaveBeenCalledTimes(1)
    expect(getUsage).toHaveBeenLastCalledWith(false)
  })

  it('页头所有时效文案都有定义，渲染时不产生 Vue 警告', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const collect = () =>
      warn.mock.calls
        .map((c) => String(c[0]))
        .filter((m) => m.includes('[Vue warn]'))

    // 覆盖四种组合：新鲜、只旧、只扫、又旧又扫。模板无条件引用 durationLabel/
    // dataFreshness/freshnessHint，任一组合下少一个定义都会退化成 undefined 而不是报错，
    // 所以这里靠 Vue 警告来兜底（曾经模板已改、脚本未改，测试全绿但页面是坏的）。
    const fresh = await mountPanel(usage({ stale: false, refreshing: false }))
    const staleOnly = await mountPanel(usage({ cacheAgeSeconds: 400, stale: true, refreshing: false }))
    const refreshing = await mountPanel(usage({ cacheAgeSeconds: 400, stale: true, refreshing: true }))

    for (const w of [fresh, staleOnly, refreshing]) {
      expect(w.find('.header-sub').text()).toContain('扫描耗时')
      w.unmount()
    }

    expect(collect()).toEqual([])
    warn.mockRestore()
  })
})
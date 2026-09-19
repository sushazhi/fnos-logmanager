import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import LogListCard from './LogListCard.vue'

const COLLAPSE_KEY = 'logmanager:logListDefaultCollapsed'
const EXCEPTIONS_KEY = 'logmanager:logListCollapseExceptions'

// 测试环境（src/test/setup.ts）把 localStorage mock 成了永远返回 null 的空壳，
// 无法验证持久化。这里换成基于 Map 的真实实现，并在每个用例后还原。
function installLocalStorage() {
  const store = new Map<string, string>()
  const real = {
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => { store.set(k, String(v)) },
    removeItem: (k: string) => { store.delete(k) },
    clear: () => store.clear(),
    key: (i: number) => [...store.keys()][i] ?? null,
    get length() { return store.size }
  }
  Object.defineProperty(window, 'localStorage', { value: real, configurable: true })
  return real
}

// 分组视图测试。
//
// 覆盖三种数据形态：
//   1) 带 appName（ListLogFiles / SearchLogFilesByName）→ 按应用分组
//   2) 不带 appName（ListLargeLogFiles）→ 保持扁平，不出现组头
//   3) Docker → 保持扁平
// 以及收起/展开、搜索时强制展开、未归类兜底。

function log(over: Record<string, unknown> = {}) {
  return {
    path: '/vol1/@appdata/nginx/access.log',
    size: 1024,
    sizeFormatted: '1.0 KB',
    showActions: true,
    ...over
  }
}

function mountCard(logs: unknown[], type = 'logs') {
  return mount(LogListCard, {
    props: { logs: logs as never, type },
    global: { stubs: { Teleport: true } }
  })
}

describe('LogListCard 分组视图', () => {
  it('带 appName 时按应用分组，组头显示文件名数与总大小', () => {
    const wrapper = mountCard([
      log({ path: '/vol1/@appdata/nginx/a.log', appName: 'nginx', size: 1024 }),
      log({ path: '/vol1/@appdata/nginx/b.log', appName: 'nginx', size: 1024 }),
      log({ path: '/vol1/@appdata/redis/c.log', appName: 'redis', size: 512 })
    ])

    const headers = wrapper.findAll('.group-header')
    expect(headers).toHaveLength(2)

    // 组间按文件数降序：nginx(2) 在前，redis(1) 在后
    expect(headers[0].find('.group-name').text()).toBe('nginx')
    expect(headers[1].find('.group-name').text()).toBe('redis')

    // 组头元信息：文件数 + 累计大小
    expect(headers[0].find('.group-meta').text()).toContain('2 个文件')
    expect(headers[0].find('.group-meta').text()).toContain('2.0 KB')
  })

  it('没有 appName 的数据保持扁平，不渲染组头', () => {
    const wrapper = mountCard([
      log({ path: '/vol1/@appdata/nginx/a.log', size: 2048 }),
      log({ path: '/vol1/@appdata/redis/b.log', size: 2048 })
    ])

    expect(wrapper.findAll('.group-header')).toHaveLength(0)
    // 行本身仍然照常渲染
    expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(2)
  })

  it('Docker 类型不分组', () => {
    const wrapper = mountCard(
      [
        log({ path: 'c1', appName: 'nginx' }),
        log({ path: 'c2', appName: 'redis' })
      ],
      'docker'
    )

    expect(wrapper.findAll('.group-header')).toHaveLength(0)
  })

  it('appName 为空的条目归入「未归类」组', () => {
    const wrapper = mountCard([
      log({ path: '/vol1/@appdata/nginx/a.log', appName: 'nginx' }),
      log({ path: '/var/log/apps/orphan.log', appName: '' }),
      log({ path: '/tmp/loose.log' })
    ])

    const names = wrapper.findAll('.group-header .group-name').map(n => n.text())
    expect(names).toContain('未归类')
  })

  it('点击组头收起该组，组内行消失；再次点击展开', async () => {
    const wrapper = mountCard([
      log({ path: '/vol1/@appdata/nginx/a.log', appName: 'nginx' }),
      log({ path: '/vol1/@appdata/nginx/b.log', appName: 'nginx' })
    ])

    expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(2)

    await wrapper.find('.group-header').trigger('click')
    expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(0)
    // 组头本身仍在，只是折叠了
    expect(wrapper.findAll('.group-header')).toHaveLength(1)

    await wrapper.find('.group-header').trigger('click')
    expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(2)
  })

  it('搜索时无视手动收起，命中结果始终可见', async () => {
    const wrapper = mountCard([
      log({ path: '/vol1/@appdata/nginx/access.log', appName: 'nginx' }),
      log({ path: '/vol1/@appdata/redis/dump.log', appName: 'redis' })
    ])

    // 先收起 nginx 组
    await wrapper.find('.group-header').trigger('click')
    expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(1)

    // 搜索仍然能看到 nginx 组内的命中项
    await wrapper.find('.search-input').setValue('access.log')
    const rows = wrapper.findAll('.log-item:not(.header)')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('access.log')
  })

  it('搜索只保留命中项所在的组', async () => {
    const wrapper = mountCard([
      log({ path: '/vol1/@appdata/nginx/access.log', appName: 'nginx' }),
      log({ path: '/vol1/@appdata/redis/dump.log', appName: 'redis' })
    ])

    await wrapper.find('.search-input').setValue('dump.log')

    const names = wrapper.findAll('.group-header .group-name').map(n => n.text())
    expect(names).toEqual(['redis'])
  })

  it('空列表不渲染任何组', () => {
    const wrapper = mountCard([])
    expect(wrapper.findAll('.group-header')).toHaveLength(0)
  })

  describe('全部折叠 / 全部展开', () => {
    const threeGroups = () => [
      log({ path: '/vol1/@appdata/nginx/a.log', appName: 'nginx' }),
      log({ path: '/vol1/@appdata/nginx/b.log', appName: 'nginx' }),
      log({ path: '/vol1/@appdata/redis/c.log', appName: 'redis' }),
      log({ path: '/vol1/@appdata/mysql/d.log', appName: 'mysql' })
    ]

    function btn(wrapper: ReturnType<typeof mountCard>, label: string) {
      const found = wrapper.findAll('.collapse-all-btn').find(b => b.text() === label)
      if (!found) throw new Error(`button not found: ${label}`)
      return found
    }

    it('无 appName 数据时不显示这两个按钮', () => {
      const wrapper = mountCard([log({ path: '/vol1/@appdata/nginx/a.log' })])
      expect(wrapper.findAll('.collapse-all-btn')).toHaveLength(0)
    })

    it('分组模式下显示两个按钮', () => {
      const wrapper = mountCard(threeGroups())
      const labels = wrapper.findAll('.collapse-all-btn').map(b => b.text())
      expect(labels).toEqual(['全部折叠', '全部展开'])
    })

    it('点击「全部折叠」收起所有组，组头保留', async () => {
      const wrapper = mountCard(threeGroups())
      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(4)

      await btn(wrapper, '全部折叠').trigger('click')

      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(0)
      expect(wrapper.findAll('.group-header')).toHaveLength(3)
    })

    it('点击「全部展开」恢复所有组', async () => {
      const wrapper = mountCard(threeGroups())

      await btn(wrapper, '全部折叠').trigger('click')
      await btn(wrapper, '全部展开').trigger('click')

      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(4)
    })

    it('已全折叠时「全部折叠」禁用，全展开时「全部展开」禁用', async () => {
      const wrapper = mountCard(threeGroups())

      // 初始：全部展开 → 展开按钮禁用
      expect(btn(wrapper, '全部展开').attributes('disabled')).toBeDefined()
      expect(btn(wrapper, '全部折叠').attributes('disabled')).toBeUndefined()

      await btn(wrapper, '全部折叠').trigger('click')

      // 全折叠后反过来
      expect(btn(wrapper, '全部折叠').attributes('disabled')).toBeDefined()
      expect(btn(wrapper, '全部展开').attributes('disabled')).toBeUndefined()
    })

    it('搜索时两个按钮均禁用（搜索强制展开）', async () => {
      const wrapper = mountCard(threeGroups())

      await wrapper.find('.search-input').setValue('a.log')

      expect(btn(wrapper, '全部折叠').attributes('disabled')).toBeDefined()
      expect(btn(wrapper, '全部展开').attributes('disabled')).toBeDefined()
    })

    it('「全部折叠」只作用于当前渲染的组，不残留历史 key', async () => {
      const wrapper = mountCard(threeGroups())

      // 先全折叠（写入 3 个 key），再搜索到只剩一组
      await btn(wrapper, '全部折叠').trigger('click')
      await wrapper.find('.search-input').setValue('c.log')

      // 搜索期间强制展开，唯一命中的组必须可见
      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(1)

      // 清空搜索后回到「全折叠」状态（3 个组都还收着）
      await wrapper.find('.search-input').setValue('')
      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(0)
      expect(wrapper.findAll('.group-header')).toHaveLength(3)
    })
  })

  describe('折叠状态持久化', () => {
    const twoGroups = () => [
      log({ path: '/vol1/@appdata/nginx/a.log', appName: 'nginx' }),
      log({ path: '/vol1/@appdata/redis/b.log', appName: 'redis' })
    ]

    function btn(wrapper: ReturnType<typeof mountCard>, label: string) {
      const found = wrapper.findAll('.collapse-all-btn').find(b => b.text() === label)
      if (!found) throw new Error(`button not found: ${label}`)
      return found
    }

    afterEach(() => {
      try {
        localStorage.removeItem(COLLAPSE_KEY)
        localStorage.removeItem(EXCEPTIONS_KEY)
      } catch { /* ignore */ }
    })

    it('「全部折叠」写入 localStorage，刷新后仍保持折叠', async () => {
      installLocalStorage()
      const first = mountCard(twoGroups())

      await btn(first, '全部折叠').trigger('click')
      expect(localStorage.getItem(COLLAPSE_KEY)).toBe('1')

      // 模拟刷新：重新挂载一个全新组件实例
      const second = mountCard(twoGroups())
      expect(second.findAll('.log-item:not(.header)')).toHaveLength(0)
      expect(second.findAll('.group-header')).toHaveLength(2)
      // 按钮状态也跟着恢复
      expect(btn(second, '全部折叠').attributes('disabled')).toBeDefined()
    })

    it('「全部展开」写入 localStorage，刷新后仍保持展开', async () => {
      installLocalStorage()
      localStorage.setItem(COLLAPSE_KEY, '1')

      const first = mountCard(twoGroups())
      await btn(first, '全部展开').trigger('click')
      expect(localStorage.getItem(COLLAPSE_KEY)).toBe('0')

      const second = mountCard(twoGroups())
      expect(second.findAll('.log-item:not(.header)')).toHaveLength(2)
    })

    it('未设置过时默认展开', () => {
      installLocalStorage()
      const wrapper = mountCard(twoGroups())
      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(2)
    })

    it('默认折叠时，新出现的应用组也跟随折叠', async () => {
      installLocalStorage()
      localStorage.setItem(COLLAPSE_KEY, '1')

      // 后一次挂载多了一个此前没见过的应用组
      const wrapper = mountCard([
        ...twoGroups(),
        log({ path: '/vol1/@appdata/mysql/c.log', appName: 'mysql' })
      ])

      // 新组必须同样是收起的，而不是因为「记录里没有这个 key」就展开
      expect(wrapper.findAll('.group-header')).toHaveLength(3)
      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(0)
    })

    it('默认折叠时单独展开某组，刷新后该组保持展开、其余仍折叠', async () => {
      installLocalStorage()
      localStorage.setItem(COLLAPSE_KEY, '1')

      const first = mountCard(twoGroups())
      const headers = first.findAll('.group-header')
      // 组间按文件数降序，同数量时按名称：nginx 在前
      expect(headers[0].find('.group-name').text()).toBe('nginx')

      await headers[0].trigger('click')   // 单独展开 nginx
      expect(first.findAll('.log-item:not(.header)')).toHaveLength(1)

      // 刷新后：nginx 展开，redis 仍收起
      const second = mountCard(twoGroups())
      expect(second.findAll('.log-item:not(.header)')).toHaveLength(1)

      const rows = second.findAll('.log-item:not(.header)')
      expect(rows[0].text()).toContain('a.log')
    })

    it('localStorage 抛异常时不崩溃，退化为不持久化', async () => {
      Object.defineProperty(window, 'localStorage', {
        configurable: true,
        get() { throw new Error('SecurityError: localStorage disabled') }
      })

      const wrapper = mountCard(twoGroups())
      expect(wrapper.findAll('.log-item:not(.header)')).toHaveLength(2)

      // 点击不应抛出
      await btn(wrapper, '全部折叠').trigger('click')
      expect(wrapper.findAll('.group-header')).toHaveLength(2)
    })
  })
})

import { describe, it, expect } from 'vitest'
import { collectDpkgResidue, dpkgPurgeCommands } from './kernelResidue'
import type { KernelVersion } from '../services/api'

function kv(over: Partial<KernelVersion> = {}): KernelVersion {
  return {
    version: '6.18.18.c1090-trim',
    isCurrent: false,
    bootSize: 0,
    bootSizeFormatted: '0 B',
    modulesSize: 0,
    modulesSizeFormatted: '0 B',
    srcSize: 0,
    srcSizeFormatted: '0 B',
    imageSize: 0,
    imageSizeFormatted: '0 B',
    totalSize: 0,
    totalSizeFormatted: '0 B',
    hasModules: false,
    hasSrc: false,
    hasImage: false,
    ...over
  }
}

// 这些测试锁定的核心事实：内核删除只删文件、不经包管理器，
// 因此 dpkg 空壳记录必然残留，界面必须把它显示出来而不能静默。
describe('collectDpkgResidue', () => {
  it('收集仍被 dpkg 记录的内核包名', () => {
    const got = collectDpkgResidue([
      kv({ version: 'a', dpkgPackages: ['linux-image-a', 'linux-headers-a'] })
    ])
    expect(got.sort()).toEqual(['linux-headers-a', 'linux-image-a'])
  })

  it('跨多个内核去重', () => {
    const got = collectDpkgResidue([
      kv({ version: 'a', dpkgPackages: ['linux-image-a'] }),
      kv({ version: 'b', dpkgPackages: ['linux-image-a', 'linux-image-b'] })
    ])
    expect(got.sort()).toEqual(['linux-image-a', 'linux-image-b'])
  })

  // 说明：`undefined`（无法查询 dpkg）与 `[]`（已查询、确实没有）在本函数里
  // 都会产出"没有包名"这一结果，因此单看返回值无法区分二者。真正的区分点在
  // 界面层：提示块用 `dpkgResidue.length` 判断，两种情形都不显示提示块，
  // 即**不会**因此谎称"已清理干净"。下面按实际可观测行为断言。
  it('字段缺失（无法查询 dpkg）时不会凭空产出包名', () => {
    expect(collectDpkgResidue([kv({ version: 'a' })])).toEqual([])
  })

  it('字段缺失时不影响其它内核的残留统计', () => {
    const got = collectDpkgResidue([
      kv({ version: 'a' }), // dpkgPackages 未定义
      kv({ version: 'b', dpkgPackages: ['linux-image-b'] })
    ])
    expect(got).toEqual(['linux-image-b'])
  })

  // 这条是能真正区分两种实现的：字段存在但为 null（后端异常序列化）时必须
  // 跳过而不是抛错，同一条目里的其它包仍要被收集。
  it('dpkgPackages 为 null 时跳过该条目且不抛错', () => {
    const got = collectDpkgResidue([
      { ...kv({ version: 'a' }), dpkgPackages: null as unknown as string[] },
      kv({ version: 'b', dpkgPackages: ['linux-image-b'] })
    ])
    expect(got).toEqual(['linux-image-b'])
  })

  it('dpkgPackages 不是数组时跳过且不抛错', () => {
    const got = collectDpkgResidue([
      { ...kv({ version: 'a' }), dpkgPackages: 'linux-image-a' as unknown as string[] }
    ])
    expect(got).toEqual([])
  })

  it('空数组表示已查询且确实没有记录', () => {
    expect(collectDpkgResidue([kv({ dpkgPackages: [] })])).toEqual([])
  })

  it('全部字段缺失时返回空（不是断言干净，而是无从得知）', () => {
    expect(collectDpkgResidue([kv({ version: 'a' }), kv({ version: 'b' })])).toEqual([])
  })

  it('剔除空串，避免渲染出无效的 apt purge 命令', () => {
    const got = collectDpkgResidue([kv({ dpkgPackages: ['', 'linux-image-a', ''] })])
    expect(got).toEqual(['linux-image-a'])
  })

  it('容忍 null / undefined 输入', () => {
    expect(collectDpkgResidue(null)).toEqual([])
    expect(collectDpkgResidue(undefined)).toEqual([])
  })

  it('容忍元素本身为 null 而不抛错', () => {
    const got = collectDpkgResidue([
      null as unknown as KernelVersion,
      kv({ dpkgPackages: ['linux-image-a'] })
    ])
    expect(got).toEqual(['linux-image-a'])
  })
})

describe('dpkgPurgeCommands', () => {
  it('无残留时不产生命令', () => {
    expect(dpkgPurgeCommands([])).toEqual([])
  })

  it('生成可直接粘贴的 apt purge 命令', () => {
    expect(dpkgPurgeCommands(['linux-image-a', 'linux-headers-a'])).toEqual([
      'apt purge -y linux-image-a linux-headers-a'
    ])
  })
})

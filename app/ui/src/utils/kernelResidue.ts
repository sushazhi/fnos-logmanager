import type { KernelVersion } from '../services/api'

/**
 * 汇总仍被 dpkg 记录着的内核包名（去重、剔除空串）。
 *
 * 内核删除功能只删文件、不经包管理器，所以删除后会留下 dpkg 空壳记录
 * （半安装态）。这个函数的作用就是把那部分残留算出来给界面显示。
 *
 * 关键约定 —— `dpkgPackages` 的「缺失」与「空数组」含义不同：
 *   - `undefined`（字段不存在）：无法查询 dpkg（例如非 Debian 系）。
 *     此时**不能**断言"没有残留"，因此跳过该条目。
 *   - `[]`：已成功查询，且确实没有记录。
 *
 * 把这个区分写成显式测试，是因为一旦写成 `v.dpkgPackages?.length` 之类的
 * 真值判断，两种情形就会被混为一谈，界面会对"未知"谎称"干净"。
 */
export function collectDpkgResidue(versions: KernelVersion[] | null | undefined): string[] {
  const seen = new Set<string>()
  if (!Array.isArray(versions)) return []
  for (const v of versions) {
    // 注意：必须用 Array.isArray 而不是真值判断 —— 见上方约定。
    if (!Array.isArray(v?.dpkgPackages)) continue
    for (const p of v.dpkgPackages) {
      if (typeof p === 'string' && p.length > 0) seen.add(p)
    }
  }
  return Array.from(seen)
}

/**
 * 把残留包名渲染成可直接粘贴的 apt purge 命令。
 * 无残留时返回空数组（界面据此不显示提示块）。
 */
export function dpkgPurgeCommands(packages: string[]): string[] {
  if (!Array.isArray(packages) || packages.length === 0) return []
  return [`apt purge -y ${packages.join(' ')}`]
}

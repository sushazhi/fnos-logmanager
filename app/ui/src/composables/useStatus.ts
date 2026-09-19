/**
 * setConfirmFn - 把 UI 层的确认对话框注册到 status store。
 *
 * 曾在此处导出一个 useStatus() 兼容层（返回 store.status/setStatus/confirm）。
 * 它是 v0.3.12 迁移到 Pinia 时的过渡产物：生产代码从未调用（useStore() 直接
 * 使用 useStatusStore），唯一使用者是同样过时的 useStatus.test.ts，且它直接
 * 透出 setup store 的 ref，外部读取需要 .value 而调用方按普通对象使用。
 * 现已删除；确认框逻辑本身由 useStatusStore 提供并已被 useStore 覆盖测试。
 *
 * 注意：本函数的导入方是 composables/useStore.ts，它再 re-export 给 App.vue
 * （App.vue 用它注入 showConfirm）。不要把它当作死代码删除。
 */
import { useStatusStore } from '../stores/useStatusStore'

export function setConfirmFn(fn: (options: any) => Promise<boolean>): void {
  useStatusStore().setConfirmFn(fn)
}

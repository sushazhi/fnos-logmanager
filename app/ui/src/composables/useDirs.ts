/**
 * useDirs - 代理到 Pinia useDirsStore
 */
import { useDirsStore } from '../stores/useDirsStore'

export function useDirs() {
  const store = useDirsStore()
  return {
    dirs: store.dirs,
    selectedDir: store.selectedDir,
    loadDirs: store.loadDirs,
    selectDir: store.selectDir
  }
}

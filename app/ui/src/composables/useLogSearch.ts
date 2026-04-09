/**
 * 日志搜索 Composable
 * 使用 Web Worker 在后台线程执行搜索，避免阻塞主线程
 * 对小量数据（< 1000 行）直接在主线程搜索，避免 Worker 通信开销
 */

import { ref, watch, type Ref } from 'vue'

interface MatchPosition {
  lineIndex: number
  charIndex: number
}

const WORKER_THRESHOLD = 1000 // 超过此行数使用 Worker

export function useLogSearch(allLines: Ref<string[]>, searchQuery: Ref<string>) {
  const matchPositions = ref<MatchPosition[]>([])
  const isSearching = ref(false)

  let worker: Worker | null = null
  let searchTimer: ReturnType<typeof setTimeout> | null = null

  function getWorker(): Worker {
    if (!worker) {
      worker = new Worker(
        new URL('../workers/logSearch.worker.ts', import.meta.url),
        { type: 'module' }
      )
    }
    return worker
  }

  function terminateWorker(): void {
    if (worker) {
      worker.terminate()
      worker = null
    }
  }

  /**
   * 在主线程中搜索（小量数据）
   */
  function searchInMainThread(lines: string[], query: string): MatchPosition[] {
    const lowerQuery = query.toLowerCase()
    const positions: MatchPosition[] = []
    for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
      const line = lines[lineIndex]
      const lowerLine = line.toLowerCase()
      let searchPos = 0
      let pos = lowerLine.indexOf(lowerQuery, searchPos)
      while (pos !== -1) {
        positions.push({ lineIndex, charIndex: pos })
        searchPos = pos + 1
        pos = lowerLine.indexOf(lowerQuery, searchPos)
      }
    }
    return positions
  }

  /**
   * 在 Worker 中搜索（大量数据）
   */
  function searchInWorker(lines: string[], query: string): void {
    isSearching.value = true
    const w = getWorker()

    w.onmessage = (e: MessageEvent) => {
      if (e.data.type === 'searchResult') {
        matchPositions.value = e.data.positions
        isSearching.value = false
      }
    }

    w.onerror = () => {
      // Worker 出错时降级到主线程搜索
      matchPositions.value = searchInMainThread(lines, query)
      isSearching.value = false
    }

    w.postMessage({ type: 'search', lines, query })
  }

  /**
   * 执行搜索（带 debounce）
   */
  function performSearch(): void {
    if (searchTimer) {
      clearTimeout(searchTimer)
    }

    const query = searchQuery.value.trim()
    if (!query) {
      matchPositions.value = []
      isSearching.value = false
      return
    }

    // 300ms debounce，避免频繁搜索
    searchTimer = setTimeout(() => {
      const lines = allLines.value

      if (lines.length < WORKER_THRESHOLD) {
        // 小量数据直接在主线程搜索
        matchPositions.value = searchInMainThread(lines, query)
      } else {
        // 大量数据使用 Worker
        searchInWorker(lines, query)
      }
    }, 300)
  }

  // 监听搜索关键词变化
  watch(searchQuery, () => {
    performSearch()
  })

  // 监听内容变化时重新搜索
  watch(allLines, () => {
    if (searchQuery.value.trim()) {
      performSearch()
    }
  }, { deep: false })

  function cleanup(): void {
    if (searchTimer) {
      clearTimeout(searchTimer)
    }
    terminateWorker()
  }

  return {
    matchPositions,
    isSearching,
    performSearch,
    cleanup
  }
}

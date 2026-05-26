/**
 * 日志搜索 Composable
 * 使用 Web Worker 在后台线程执行搜索，避免阻塞主线程
 * 对小量数据（< 1000 行）直接在主线程搜索，避免 Worker 通信开销
 * 支持关键词/正则模式，搜索总是不区分大小写
 */

import { ref, watch, type Ref } from 'vue'

type SearchMode = 'keyword' | 'regex'

interface MatchPosition {
  lineIndex: number
  charIndex: number
}

interface SearchOptions {
  mode: SearchMode
}

const WORKER_THRESHOLD = 1000

export function useLogSearch(
  allLines: Ref<string[]>,
  searchQuery: Ref<string>,
  searchOptions: Ref<SearchOptions>
) {
  const matchPositions = ref<MatchPosition[]>([])
  const isSearching = ref(false)
  const searchError = ref('')

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

  function searchInMainThread(
    lines: string[],
    query: string,
    options: SearchOptions
  ): MatchPosition[] {
    const { mode } = options
    searchError.value = ''

    const positions: MatchPosition[] = []

    if (mode === 'regex') {
      if (query.length > 500) {
        searchError.value = '搜索内容过长'
        return []
      }
      // ReDoS 防护
      if (/\(.+[+*?]\{[0-9]|\([^)]+\)[+*?]+\(|[+*?]\{[0-9]{3,}/.test(query)) {
        searchError.value = '正则表达式过于复杂'
        return []
      }
      let regex: RegExp
      try {
        regex = new RegExp(query, 'gi')
      } catch (err) {
        searchError.value = `无效的正则表达式: ${query}`
        return []
      }

      for (let i = 0; i < lines.length; i++) {
        const line = lines[i]
        regex.lastIndex = 0
        let match = regex.exec(line)
        while (match !== null) {
          positions.push({ lineIndex: i, charIndex: match.index })
          if (match[0].length === 0) regex.lastIndex++
          match = regex.exec(line)
        }
      }
    } else {
      const searchQueryStr = query.toLowerCase()
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i]
        const searchLine = line.toLowerCase()
        let searchPos = 0
        let pos = searchLine.indexOf(searchQueryStr, searchPos)
        while (pos !== -1) {
          positions.push({ lineIndex: i, charIndex: pos })
          searchPos = pos + 1
          pos = searchLine.indexOf(searchQueryStr, searchPos)
        }
      }
    }

    return positions
  }

  function searchInWorker(
    lines: string[],
    query: string,
    options: SearchOptions
  ): void {
    isSearching.value = true
    searchError.value = ''
    const w = getWorker()

    w.onmessage = (e: MessageEvent) => {
      if (e.data.type === 'searchResult') {
        matchPositions.value = e.data.positions
        isSearching.value = false
      } else if (e.data.type === 'error') {
        searchError.value = e.data.message
        matchPositions.value = []
        isSearching.value = false
      }
    }

    w.onerror = () => {
      matchPositions.value = searchInMainThread(lines, query, options)
      isSearching.value = false
    }

    w.postMessage({
      type: 'search',
      lines,
      query,
      mode: options.mode
    })
  }

  function performSearch(): void {
    if (searchTimer) clearTimeout(searchTimer)

    const query = searchQuery.value.trim()
    if (!query) {
      matchPositions.value = []
      isSearching.value = false
      searchError.value = ''
      return
    }

    searchTimer = setTimeout(() => {
      const lines = allLines.value
      const options = searchOptions.value

      if (lines.length < WORKER_THRESHOLD) {
        matchPositions.value = searchInMainThread(lines, query, options)
      } else {
        searchInWorker(lines, query, options)
      }
    }, 300)
  }

  watch([searchQuery, searchOptions], () => {
    performSearch()
  }, { deep: true })

  watch(allLines, () => {
    if (searchQuery.value.trim()) {
      performSearch()
    }
  }, { deep: false })

  function cleanup(): void {
    if (searchTimer) clearTimeout(searchTimer)
    terminateWorker()
  }

  return {
    matchPositions,
    isSearching,
    searchError,
    performSearch,
    cleanup
  }
}

/**
 * 日志搜索 Web Worker
 * 在后台线程中执行搜索，避免阻塞主线程
 * 支持关键词/正则模式，搜索总是不区分大小写
 */

type SearchMode = 'keyword' | 'regex'

interface SearchMessage {
  type: 'search'
  lines: string[]
  query: string
  mode?: SearchMode
}

interface SearchResponse {
  type: 'searchResult'
  positions: Array<{ lineIndex: number; charIndex: number }>
  totalMatches: number
}

interface ErrorResponse {
  type: 'error'
  message: string
}

interface CancelMessage {
  type: 'cancel'
}

self.onmessage = function (e: MessageEvent<SearchMessage | CancelMessage>) {
  const message = e.data

  if (message.type === 'search') {
    const {
      lines,
      query,
      mode = 'keyword'
    } = message

    const positions: Array<{ lineIndex: number; charIndex: number }> = []

    if (mode === 'regex') {
      let regex: RegExp
      try {
        regex = new RegExp(query, 'gi')
      } catch {
        const errorResp: ErrorResponse = {
          type: 'error',
          message: `无效的正则表达式: ${query}`
        }
        self.postMessage(errorResp)
        return
      }

      for (let i = 0; i < lines.length; i++) {
        const line = lines[i]
        regex.lastIndex = 0
        let match = regex.exec(line)
        while (match !== null) {
          positions.push({ lineIndex: i, charIndex: match.index })
          if (match[0].length === 0) {
            regex.lastIndex++
          }
          match = regex.exec(line)
        }

        if (i > 0 && i % 5000 === 0) {
          const response: SearchResponse = {
            type: 'searchResult',
            positions: [...positions],
            totalMatches: positions.length
          }
          self.postMessage(response)
        }
      }
    } else {
      const searchQuery = query.toLowerCase()
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i]
        const searchLine = line.toLowerCase()
        let searchPos = 0
        let pos = searchLine.indexOf(searchQuery, searchPos)
        while (pos !== -1) {
          positions.push({ lineIndex: i, charIndex: pos })
          searchPos = pos + 1
          pos = searchLine.indexOf(searchQuery, searchPos)
        }

        if (i > 0 && i % 5000 === 0) {
          const response: SearchResponse = {
            type: 'searchResult',
            positions: [...positions],
            totalMatches: positions.length
          }
          self.postMessage(response)
        }
      }
    }

    const response: SearchResponse = {
      type: 'searchResult',
      positions,
      totalMatches: positions.length
    }
    self.postMessage(response)
  }
}

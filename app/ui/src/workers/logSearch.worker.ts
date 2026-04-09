/**
 * 日志搜索 Web Worker
 * 在后台线程中执行搜索，避免阻塞主线程
 */

interface SearchMessage {
  type: 'search'
  lines: string[]
  query: string
}

interface SearchResponse {
  type: 'searchResult'
  positions: Array<{ lineIndex: number; charIndex: number }>
}

interface CancelMessage {
  type: 'cancel'
}

self.onmessage = function (e: MessageEvent<SearchMessage | CancelMessage>) {
  const message = e.data

  if (message.type === 'search') {
    const { lines, query } = message
    const lowerQuery = query.toLowerCase()
    const positions: Array<{ lineIndex: number; charIndex: number }> = []

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

      // 每处理 5000 行发送一次中间结果，避免长时间无响应
      if (lineIndex > 0 && lineIndex % 5000 === 0) {
        const response: SearchResponse = {
          type: 'searchResult',
          positions: [...positions]
        }
        self.postMessage(response)
      }
    }

    const response: SearchResponse = {
      type: 'searchResult',
      positions
    }
    self.postMessage(response)
  }
}

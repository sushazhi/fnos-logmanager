/**
 * 实时日志流 Composable
 * 使用 WebSocket 实现日志实时跟踪（类似 tail -f）
 */

import { ref, onUnmounted } from 'vue'

interface StreamMessage {
  type: 'connected' | 'subscribed' | 'unsubscribed' | 'data' | 'file_rotated' | 'file_deleted' | 'error'
  content?: string
  filePath?: string
  offset?: number
  totalSize?: number
  initialSize?: number
  message?: string
}

export function useLogStream() {
  const ws = ref<WebSocket | null>(null)
  const isConnected = ref(false)
  const isSubscribed = ref(false)
  const streamingContent = ref<string[]>([])
  const streamError = ref<string | null>(null)
  const currentFile = ref<string | null>(null)

  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  const MAX_RECONNECT_ATTEMPTS = 5
  const RECONNECT_BASE_DELAY = 1000

  function getWsUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/api/logs/stream`
  }

  function connect(): void {
    if (ws.value && ws.value.readyState === WebSocket.OPEN) return

    try {
      const socket = new WebSocket(getWsUrl())

      socket.onopen = () => {
        isConnected.value = true
        streamError.value = null
        reconnectAttempts = 0
      }

      socket.onmessage = (event) => {
        try {
          const message: StreamMessage = JSON.parse(event.data)
          handleMessage(message)
        } catch {
          // Ignore invalid messages
        }
      }

      socket.onclose = () => {
        isConnected.value = false
        isSubscribed.value = false
        ws.value = null
        attemptReconnect()
      }

      socket.onerror = () => {
        streamError.value = 'WebSocket 连接失败'
      }

      ws.value = socket
    } catch (err) {
      streamError.value = '无法建立 WebSocket 连接'
    }
  }

  function handleMessage(message: StreamMessage): void {
    switch (message.type) {
      case 'connected':
        isConnected.value = true
        break

      case 'subscribed':
        isSubscribed.value = true
        currentFile.value = message.filePath || null
        streamingContent.value = []
        break

      case 'unsubscribed':
        isSubscribed.value = false
        currentFile.value = null
        break

      case 'data':
        if (message.content) {
          const newLines = message.content.split('\n').filter(line => line.length > 0)
          streamingContent.value.push(...newLines)
          // 限制内存中的行数
          if (streamingContent.value.length > 10000) {
            streamingContent.value = streamingContent.value.slice(-5000)
          }
        }
        break

      case 'file_rotated':
        // 文件被轮转，清空当前内容
        streamingContent.value = ['--- 文件已轮转 ---']
        break

      case 'file_deleted':
        isSubscribed.value = false
        currentFile.value = null
        streamError.value = '日志文件已被删除'
        break

      case 'error':
        streamError.value = message.message || '未知错误'
        break
    }
  }

  function attemptReconnect(): void {
    if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) return
    if (reconnectTimer) return

    const delay = RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempts)
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      reconnectAttempts++
      connect()
    }, delay)
  }

  function subscribe(filePath: string, pollInterval: number = 1000): void {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      connect()
      // 等待连接建立后订阅
      const waitConnect = setInterval(() => {
        if (ws.value && ws.value.readyState === WebSocket.OPEN) {
          clearInterval(waitConnect)
          ws.value!.send(JSON.stringify({
            type: 'subscribe',
            filePath,
            pollInterval
          }))
        }
      }, 100)
      // 5 秒超时
      setTimeout(() => clearInterval(waitConnect), 5000)
      return
    }

    ws.value.send(JSON.stringify({
      type: 'subscribe',
      filePath,
      pollInterval
    }))
  }

  function unsubscribe(): void {
    if (ws.value && ws.value.readyState === WebSocket.OPEN) {
      ws.value.send(JSON.stringify({ type: 'unsubscribe' }))
    }
    isSubscribed.value = false
    currentFile.value = null
    streamingContent.value = []
  }

  function disconnect(): void {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    reconnectAttempts = MAX_RECONNECT_ATTEMPTS // 阻止自动重连

    if (ws.value) {
      ws.value.close()
      ws.value = null
    }
    isConnected.value = false
    isSubscribed.value = false
    currentFile.value = null
    streamingContent.value = []
  }

  function clearContent(): void {
    streamingContent.value = []
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    isConnected,
    isSubscribed,
    streamingContent,
    streamError,
    currentFile,
    connect,
    subscribe,
    unsubscribe,
    disconnect,
    clearContent
  }
}

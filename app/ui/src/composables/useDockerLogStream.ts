/**
 * Docker 日志实时流 Composable
 * 使用 WebSocket 实现 Docker 容器日志实时跟踪（docker logs -f）
 */

import { ref, onUnmounted } from 'vue'
import api from '../services/api'
import { buildWsUrl } from '../utils/env'

interface DockerStreamMessage {
  type: 'connected' | 'subscribed' | 'data' | 'error' | 'pong'
  content?: string
  container?: string
  message?: string
}

export function useDockerLogStream() {
  const ws = ref<WebSocket | null>(null)
  const isConnected = ref(false)
  const isSubscribed = ref(false)
  const streamingContent = ref<string[]>([])
  const streamError = ref<string | null>(null)
  const currentContainer = ref<string | null>(null)

  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  const MAX_RECONNECT_ATTEMPTS = 5
  const RECONNECT_BASE_DELAY = 1000
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let intentionalClose = false

  function getWsUrl(): string {
    return buildWsUrl('/api/docker/stream', api.getSessionToken())
  }

  function connect(): void {
    if (ws.value && ws.value.readyState === WebSocket.OPEN) return
    intentionalClose = false

    try {
      const socket = new WebSocket(getWsUrl())

      socket.onopen = () => {
        isConnected.value = true
        streamError.value = null
        reconnectAttempts = 0
        startPing()
      }

      socket.onmessage = (event) => {
        try {
          const message: DockerStreamMessage = JSON.parse(event.data)
          handleMessage(message)
        } catch {
          // Ignore invalid messages
        }
      }

      socket.onclose = () => {
        isConnected.value = false
        isSubscribed.value = false
        ws.value = null
        stopPing()
        if (!intentionalClose) {
          attemptReconnect()
        }
      }

      socket.onerror = () => {
        streamError.value = 'WebSocket 连接失败'
      }

      ws.value = socket
    } catch (err) {
      streamError.value = '无法建立 WebSocket 连接'
    }
  }

  function handleMessage(message: DockerStreamMessage): void {
    switch (message.type) {
      case 'connected':
        isConnected.value = true
        break

      case 'subscribed':
        isSubscribed.value = true
        currentContainer.value = message.container || null
        streamingContent.value = []
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

      case 'error':
        streamError.value = message.message || '未知错误'
        break

      case 'pong':
        break
    }
  }

  function startPing(): void {
    stopPing()
    pingTimer = setInterval(() => {
      if (ws.value && ws.value.readyState === WebSocket.OPEN) {
        ws.value.send('ping')
      }
    }, 25000)
  }

  function stopPing(): void {
    if (pingTimer) {
      clearInterval(pingTimer)
      pingTimer = null
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

  function subscribe(container: string): void {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      connect()
      // 等待连接建立后订阅
      const waitConnect = setInterval(() => {
        if (ws.value && ws.value.readyState === WebSocket.OPEN) {
          clearInterval(waitConnect)
          ws.value!.send(JSON.stringify({
            type: 'subscribe',
            container
          }))
        }
      }, 100)
      // 5 秒超时
      setTimeout(() => clearInterval(waitConnect), 5000)
      return
    }

    ws.value.send(JSON.stringify({
      type: 'subscribe',
      container
    }))
  }

  function unsubscribe(): void {
    if (ws.value && ws.value.readyState === WebSocket.OPEN) {
      ws.value.send(JSON.stringify({ type: 'unsubscribe' }))
    }
    isSubscribed.value = false
    currentContainer.value = null
    streamingContent.value = []
  }

  function disconnect(): void {
    intentionalClose = true
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    reconnectAttempts = MAX_RECONNECT_ATTEMPTS
    stopPing()

    if (ws.value) {
      ws.value.close()
      ws.value = null
    }
    isConnected.value = false
    isSubscribed.value = false
    currentContainer.value = null
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
    currentContainer,
    connect,
    subscribe,
    unsubscribe,
    disconnect,
    clearContent
  }
}

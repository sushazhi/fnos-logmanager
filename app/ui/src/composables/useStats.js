import { reactive } from 'vue'
import api from '../services/api'

const stats = reactive({
  totalLogs: 0,
  totalSize: '0B',
  archiveCount: 0,
  largeFiles: 0
})

export function useStats() {
  async function loadStats() {
    try {
      const data = await api.get('/api/logs/stats')
      stats.totalLogs = data.totalLogs || 0
      stats.totalSize = data.totalSizeFormatted || '0B'
      stats.archiveCount = data.totalArchives || 0
      stats.largeFiles = data.largeFiles || 0
    } catch (e) {
      console.error('加载统计失败:', e)
    }
  }

  return {
    stats,
    loadStats
  }
}

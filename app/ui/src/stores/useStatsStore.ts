import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Stats, StatsResponse } from '../types'
import api from '../services/api'

export const useStatsStore = defineStore('stats', () => {
  const stats = ref<Stats>({
    totalLogs: 0,
    totalSize: '0B',
    archiveCount: 0,
    largeFiles: 0
  })

  async function loadStats(): Promise<void> {
    try {
      const data = await api.get<StatsResponse>('/api/logs/stats')
      stats.value = {
        totalLogs: data.totalLogs || 0,
        totalSize: data.totalSizeFormatted || '0B',
        archiveCount: data.totalArchives || 0,
        largeFiles: data.largeFiles || 0
      }
    } catch (e) {
      console.error('加载统计失败:', e)
    }
  }

  return {
    stats,
    loadStats
  }
})

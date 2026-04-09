/**
 * useStats - 代理到 Pinia useStatsStore
 */
import { useStatsStore } from '../stores/useStatsStore'

export function useStats() {
  const store = useStatsStore()
  return {
    stats: store.stats,
    loadStats: store.loadStats
  }
}

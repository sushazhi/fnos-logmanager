/**
 * useDocker - 代理到 Pinia useDockerStore
 */
import { useDockerStore } from '../stores/useDockerStore'

export function useDocker() {
  const store = useDockerStore()
  return {
    dockerContainers: store.dockerContainers,
    listDockerContainers: store.listDockerContainers,
    viewDockerLogs: store.viewDockerLogs
  }
}

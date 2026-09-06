import { onBeforeUnmount, onMounted, ref } from 'vue'

export type LatencyProbeState = 'idle' | 'probing' | 'ok' | 'error'

const PROBE_PATH = '/health'
const SAMPLE_COUNT = 3
const REFRESH_INTERVAL_MS = 5000
const TIMEOUT_MS = 5000

/**
 * Measures round-trip latency from the visitor's browser to this deployment's
 * `/health` endpoint while the page is visible and online. Each round starts
 * three samples with one deadline and publishes the fastest valid response.
 */
export function useLatencyProbe() {
  const latencyMs = ref<number | null>(null)
  const state = ref<LatencyProbeState>('idle')
  let disposed = false
  let activeRequest: AbortController | null = null
  let refreshTimer: ReturnType<typeof setTimeout> | undefined
  let timeoutTimer: ReturnType<typeof setTimeout> | undefined

  function canProbe() {
    return !disposed && document.visibilityState !== 'hidden' && navigator.onLine
  }

  async function probe(): Promise<void> {
    if (typeof window === 'undefined' || typeof fetch !== 'function' || !canProbe() || activeRequest) return
    clearTimeout(refreshTimer)
    // Keep the last reading visible during routine refreshes to avoid flicker.
    if (latencyMs.value === null) state.value = 'probing'
    const controller = new AbortController()
    activeRequest = controller
    const timeout = setTimeout(() => controller.abort(), TIMEOUT_MS)
    timeoutTimer = timeout
    async function sample(): Promise<number | null> {
      const start = performance.now()
      try {
        const res = await fetch(PROBE_PATH, {
          cache: 'no-store', credentials: 'omit', signal: controller.signal
        })
        if (!res.ok) return null
        const body = await res.json()
        if (body?.status !== 'ok' || controller.signal.aborted) return null
        return performance.now() - start
      } catch {
        return null
      }
    }

    try {
      const samples = await Promise.all(Array.from({ length: SAMPLE_COUNT }, sample))
      if (activeRequest !== controller || disposed) return
      const validSamples = samples.filter((value): value is number => value !== null)
      latencyMs.value = validSamples.length ? Math.max(1, Math.round(Math.min(...validSamples))) : null
      state.value = latencyMs.value === null ? 'error' : 'ok'
    } finally {
      clearTimeout(timeout)
      if (activeRequest === controller) {
        activeRequest = null
        timeoutTimer = undefined
        if (canProbe()) refreshTimer = setTimeout(() => void probe(), REFRESH_INTERVAL_MS)
      }
    }
  }

  function stop() {
    clearTimeout(refreshTimer)
    clearTimeout(timeoutTimer)
    const controller = activeRequest
    activeRequest = null
    controller?.abort()
  }

  function syncActivity() {
    if (canProbe()) {
      void probe()
    } else {
      stop()
      latencyMs.value = null
      state.value = navigator.onLine ? 'idle' : 'error'
    }
  }

  onMounted(() => {
    document.addEventListener('visibilitychange', syncActivity)
    window.addEventListener('online', syncActivity)
    window.addEventListener('offline', syncActivity)
    syncActivity()
  })

  onBeforeUnmount(() => {
    disposed = true
    stop()
    document.removeEventListener('visibilitychange', syncActivity)
    window.removeEventListener('online', syncActivity)
    window.removeEventListener('offline', syncActivity)
  })

  return { latencyMs, state, probe }
}

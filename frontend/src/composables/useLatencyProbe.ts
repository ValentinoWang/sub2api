import { onBeforeUnmount, ref } from 'vue'

export type LatencyProbeState = 'idle' | 'probing' | 'ok' | 'error'

const PROBE_PATH = '/health'
const SAMPLE_COUNT = 3

/**
 * Measures round-trip latency from the visitor's browser to this deployment's
 * `/health` endpoint. Takes a few samples and keeps the best one so a cold
 * connection does not dominate. Never throws; failures surface as `error`.
 */
export function useLatencyProbe() {
  const latencyMs = ref<number | null>(null)
  const state = ref<LatencyProbeState>('idle')
  let cancelled = false

  async function sample(): Promise<number | null> {
    const start = performance.now()
    try {
      const res = await fetch(PROBE_PATH, { cache: 'no-store', credentials: 'omit' })
      if (!res.ok) return null
      return performance.now() - start
    } catch {
      return null
    }
  }

  async function probe(): Promise<void> {
    if (typeof window === 'undefined' || typeof fetch !== 'function') return
    state.value = 'probing'
    let best: number | null = null
    for (let i = 0; i < SAMPLE_COUNT; i++) {
      if (cancelled) return
      const value = await sample()
      if (value !== null && (best === null || value < best)) best = value
    }
    if (cancelled) return
    latencyMs.value = best === null ? null : Math.max(1, Math.round(best))
    state.value = best === null ? 'error' : 'ok'
  }

  onBeforeUnmount(() => {
    cancelled = true
  })

  return { latencyMs, state, probe }
}

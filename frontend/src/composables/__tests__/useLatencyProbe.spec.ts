import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { useLatencyProbe } from '../useLatencyProbe'

describe('useLatencyProbe', () => {
  let wrapper: VueWrapper | undefined
  let probe: ReturnType<typeof useLatencyProbe>
  let delay: number
  let status: number
  let sampleDelays: number[]
  let fetchMock: ReturnType<typeof vi.fn>

  function start() {
    wrapper = mount(defineComponent({
      setup() {
        probe = useLatencyProbe()
        return probe
      },
      template: '<span>{{ state }}:{{ latencyMs }}</span>'
    }))
  }

  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(performance, 'now').mockImplementation(() => Date.now())
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true)
    delay = 313
    status = 200
    sampleDelays = []
    fetchMock = vi.fn((_url: string, options: RequestInit) => new Promise<Response>((resolve, reject) => {
      const timer = setTimeout(() => resolve({
        ok: status === 200,
        status,
        json: async () => ({ status: 'ok' })
      } as Response), sampleDelays.shift() ?? delay)
      options.signal?.addEventListener('abort', () => {
        clearTimeout(timer)
        reject(new DOMException('Aborted', 'AbortError'))
      }, { once: true })
    }))
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('starts three requests immediately and publishes the round minimum, including a slower next round', async () => {
    sampleDelays = [313, 105, 820]
    start()
    expect(fetchMock).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(0)
    expect(wrapper!.text()).toBe('probing:')
    await vi.advanceTimersByTimeAsync(820)
    expect(wrapper!.text()).toBe('ok:105')
    expect(fetchMock).toHaveBeenCalledWith('/health', expect.objectContaining({
      cache: 'no-store', credentials: 'omit', signal: expect.any(AbortSignal)
    }))
    sampleDelays = [900, 700, 1100]
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper!.text()).toBe('ok:105')
    await vi.advanceTimersByTimeAsync(1100)
    expect(wrapper!.text()).toBe('ok:700')
    expect(fetchMock).toHaveBeenCalledTimes(6)
  })

  it('deduplicates manual probes while a request is pending', async () => {
    start()
    void probe.probe()
    void probe.probe()
    expect(fetchMock).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(313)
    void probe.probe()
    await vi.advanceTimersByTimeAsync(313)
    expect(fetchMock).toHaveBeenCalledTimes(6)
    await vi.advanceTimersByTimeAsync(5000)
    expect(fetchMock).toHaveBeenCalledTimes(9)
  })

  it('clears stale RTT after HTTP failure and automatically recovers', async () => {
    start()
    await vi.advanceTimersByTimeAsync(313)
    status = 503
    await vi.advanceTimersByTimeAsync(5313)
    expect(wrapper!.text()).toBe('error:')
    status = 200
    await vi.advanceTimersByTimeAsync(5313)
    expect(wrapper!.text()).toBe('ok:313')
  })

  it('times out a stalled request and retries', async () => {
    delay = 60000
    start()
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper!.text()).toBe('error:')
    expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true)
    delay = 100
    await vi.advanceTimersByTimeAsync(5100)
    expect(wrapper!.text()).toBe('ok:100')
  })

  it('rejects HTML fallbacks and invalid health responses', async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => { throw new SyntaxError('HTML') } })
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => ({ status: 'unavailable' }) })
    fetchMock.mockRejectedValueOnce(new TypeError('Network failure'))
    start()
    await vi.advanceTimersByTimeAsync(0)
    expect(wrapper!.text()).toBe('error:')
    await vi.advanceTimersByTimeAsync(5313)
    expect(wrapper!.text()).toBe('ok:313')
  })

  it('pauses and aborts in the background, then immediately measures on return', async () => {
    start()
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(20000)
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true)
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(313)
    expect(fetchMock).toHaveBeenCalledTimes(6)
    expect(wrapper!.text()).toBe('ok:313')
  })

  it('does not probe while offline and measures immediately on reconnection', async () => {
    start()
    await vi.advanceTimersByTimeAsync(313)
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    window.dispatchEvent(new Event('offline'))
    await vi.advanceTimersByTimeAsync(20000)
    expect(wrapper!.text()).toBe('error:')
    expect(fetchMock).toHaveBeenCalledTimes(3)
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true)
    window.dispatchEvent(new Event('online'))
    await vi.advanceTimersByTimeAsync(313)
    expect(wrapper!.text()).toBe('ok:313')
    expect(fetchMock).toHaveBeenCalledTimes(6)
  })

  it('does not start in a hidden tab', async () => {
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    start()
    await vi.advanceTimersByTimeAsync(20000)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('aborts on unmount and removes polling and event listeners', async () => {
    start()
    wrapper!.unmount()
    wrapper = undefined
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new Event('online'))
    await vi.advanceTimersByTimeAsync(20000)
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('keeps successful samples when another request reaches the shared five-second deadline', async () => {
    sampleDelays = [800, 200, 60000]
    start()
    await vi.advanceTimersByTimeAsync(4999)
    expect(wrapper!.text()).toBe('probing:')
    await vi.advanceTimersByTimeAsync(1)
    expect(wrapper!.text()).toBe('ok:200')
    expect(fetchMock.mock.calls.every(call => call[1].signal.aborted)).toBe(true)
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('takes the minimum of valid samples when one request fails', async () => {
    fetchMock.mockResolvedValueOnce({ ok: false })
    sampleDelays = [450, 180]
    start()
    await vi.advanceTimersByTimeAsync(450)
    expect(wrapper!.text()).toBe('ok:180')
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })
})

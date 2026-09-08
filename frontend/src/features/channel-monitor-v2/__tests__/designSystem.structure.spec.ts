/**
 * Structure contracts: channel-monitor-v2 + studio shells must use project
 * design-system utility classes rather than isolated flat RGB skins.
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../..')

function read(rel: string) {
  return readFileSync(resolve(root, rel), 'utf8')
}

describe('channel-monitor-v2 design system structure', () => {
  it('user V1 and V2 shells consume the AppLayout surface with compact monitor panels', () => {
    const v1 = read('views/user/ChannelStatusV1View.vue')
    const v1Card = read('components/user/monitor/MonitorCard.vue')
    const v1Grid = read('components/user/monitor/MonitorCardGrid.vue')
    const v1Hero = read('components/user/monitor/MonitorHero.vue')
    const v1MetricPair = read('components/user/monitor/MonitorMetricPair.vue')
    const v1Availability = read('components/user/monitor/MonitorAvailabilityRow.vue')
    const v1Timeline = read('components/user/monitor/MonitorTimeline.vue')
    const v1ProviderIcon = read('components/user/monitor/ProviderIcon.vue')
    const v2 = read('views/user/ChannelStatusV2View.vue')

    expect(v1).toContain('monitor-v1-page')
    expect(v1Card).toContain('monitor-card')
    expect(v1Card).toContain('var(--user-surface)')
    expect(v1Grid).toContain('var(--user-border)')
    expect(v1Hero).toContain('monitor-window-tabs')
    expect(v1MetricPair).toContain('var(--user-hover)')
    expect(v1Availability).toContain('var(--user-muted)')
    expect(v1Timeline).toContain('var(--user-border)')
    expect(v1ProviderIcon).toContain('var(--user-muted)')
    expect(`${v1Card}${v1Grid}${v1Hero}${v1MetricPair}${v1Availability}${v1Timeline}${v1ProviderIcon}`)
      .not.toMatch(/rounded-(?:2xl|3xl)/)

    expect(v2).toContain('monitor-v2-page')
    expect(v2).toContain('page-header')
    expect(v2).toContain('page-title')
    expect(v2).toContain('btn btn-secondary')
    expect(v2).toContain('class="tab')
    expect(v2).toContain('tab-active')
    expect(v2).toContain('badge badge-warning')
    expect(v2).toContain('var(--user-surface)')
    expect(v2).toContain('var(--user-border)')
    expect(v2).toContain('monitor-v2-metric')
    expect(v2).toContain('monitor-v2-visual')
    expect(v2).not.toMatch(/rounded-(?:2xl|3xl)/)
    expect(v2).not.toContain('ring-1 ring-gray-900/5')
    // Compact single-row toolbar
    expect(v2).toContain('monitor-toolbar')
    expect(v2).toContain('clearFilters')
    expect(v2).toContain('healthModeOptions')
    expect(v2).toContain("'cache'")
    // Overview-first KPI strip before primary viz
    expect(v2.indexOf('summaryAria')).toBeLessThan(v2.indexOf('MonitorTrendChart'))
    // No page-level fixed min-width that forces viewport horizontal scroll
    expect(v2).not.toMatch(/min-width:\s*980px/)
    expect(v2).not.toMatch(/min-w-\[980px\]/)
    // Dense tables scroll internally
    expect(v2).toMatch(/max-h-\[min\(52vh/)
    expect(v2).toContain('overflow-auto')
    // Trend view toggle (pulse matrix / line chart) + default platform/group dimension
    expect(v2).toContain("trendView")
    expect(v2).toContain("'platform_group'")
    expect(v2).toContain('MonitorTrendChart')
  })

  it('RelayPulseMatrix uses card chrome, matrix scroll, and hover tooltips (no click modal)', () => {
    const src = read('features/channel-monitor-v2/RelayPulseMatrix.vue')
    expect(src).toContain('class="card')
    expect(src).toContain('card-header')
    expect(src).toContain('card-body')
    expect(src).toContain('matrix-scroll')
    expect(src).toMatch(/max-h-\[min\(42vh/)
    expect(src).toContain('overflow-auto')
    expect(src).toContain('pulse-tooltip')
    expect(src).toContain('rounded-3xl')
    expect(src).toContain('ring-1 ring-gray-900/5')
    expect(src).not.toContain('modal-overlay')
    expect(src).not.toContain('modal-content')
  })

  it('MetricCell uses stat-card utility', () => {
    const src = read('features/channel-monitor-v2/MetricCell.vue')
    expect(src).toContain('stat-card')
    expect(src).toContain('stat-label')
    expect(src).toContain('stat-value')
    expect(src).toContain('rounded-3xl')
  })

  it('MonitorTrendChart uses Ops chart shell tokens', () => {
    const src = read('features/channel-monitor-v2/MonitorTrendChart.vue')
    expect(src).toContain('class="card')
    expect(src).toContain('rounded-3xl')
    expect(src).toContain('ring-1 ring-gray-900/5')
    expect(src).toContain('EmptyState')
    expect(src).toContain('min-h-[360px]')
  })

  it('FilterMultiSelect uses rounded-xl input chrome and dropdown utility', () => {
    const src = read('features/channel-monitor-v2/FilterMultiSelect.vue')
    expect(src).toContain('rounded-xl')
    expect(src).toContain('dropdown')
    expect(src).toContain('dropdown-item')
  })

  it('MonitorSettingsPanel uses page-header, card, btn-primary, tabs', () => {
    const src = read('features/channel-monitor-v2/MonitorSettingsPanel.vue')
    expect(src).toContain('page-header')
    expect(src).toContain('btn btn-primary')
    expect(src).toContain('class="card')
    expect(src).toContain('tab-active')
    expect(src).toMatch(/max-h-\[min\(40vh/)
  })

  it('admin ChannelMonitorView V2 tab chrome uses project tabs', () => {
    const src = read('views/admin/ChannelMonitorView.vue')
    expect(src).toContain('page-header')
    expect(src).toContain('page-title')
    expect(src).toContain('class="tabs')
    expect(src).toContain('tab-active')
    expect(src).toContain('MonitorSettingsPanel')
  })
})

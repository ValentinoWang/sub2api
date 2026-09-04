<template>
  <div class="station" :class="{ 'station-compact': compact }">
    <div class="station-glow" aria-hidden="true"></div>

    <svg class="station-svg" viewBox="0 0 600 440" fill="none" aria-hidden="true">
      <defs>
        <linearGradient id="st-core" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stop-color="#2dd4bf" />
          <stop offset="0.55" stop-color="#0891b2" />
          <stop offset="1" stop-color="#4f46e5" />
        </linearGradient>
        <linearGradient id="st-line" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stop-color="#2dd4bf" stop-opacity="0.15" />
          <stop offset="0.5" stop-color="#2dd4bf" stop-opacity="0.9" />
          <stop offset="1" stop-color="#818cf8" stop-opacity="0.15" />
        </linearGradient>
        <filter id="st-glow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="8" result="b" />
          <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
        <filter id="st-soft" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="3" result="b" />
          <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
      </defs>

      <!-- orbit rings -->
      <g class="station-rings" transform="translate(280 220)">
        <circle r="150" class="station-ring station-ring-1" />
        <circle r="110" class="station-ring station-ring-2" />
        <circle r="72" class="station-ring station-ring-3" />
      </g>

      <!-- links -->
      <path id="st-p-in" d="M92 220 C 160 220, 190 220, 236 220" class="station-link" />
      <path id="st-p-1" d="M324 220 C 380 220, 400 120, 462 120" class="station-link" />
      <path id="st-p-2" d="M324 220 C 380 220, 400 220, 462 220" class="station-link" />
      <path id="st-p-3" d="M324 220 C 380 220, 400 320, 462 320" class="station-link" />

      <!-- packets -->
      <circle r="4" class="station-packet">
        <animateMotion dur="1.8s" repeatCount="indefinite"><mpath href="#st-p-in" /></animateMotion>
      </circle>
      <circle r="4" class="station-packet station-packet-a">
        <animateMotion dur="2.2s" begin="0.4s" repeatCount="indefinite"><mpath href="#st-p-1" /></animateMotion>
      </circle>
      <circle r="4" class="station-packet station-packet-b">
        <animateMotion dur="2.2s" begin="1.1s" repeatCount="indefinite"><mpath href="#st-p-2" /></animateMotion>
      </circle>
      <circle r="4" class="station-packet station-packet-c">
        <animateMotion dur="2.2s" begin="1.7s" repeatCount="indefinite"><mpath href="#st-p-3" /></animateMotion>
      </circle>

      <!-- left node: you, resting -->
      <g transform="translate(62 220)">
        <circle r="30" class="station-node" />
        <path d="M6 -12 a13 13 0 1 0 8 22 a11 11 0 0 1 -8 -22z" fill="url(#st-core)" filter="url(#st-soft)" />
        <text x="0" y="52" class="station-label" text-anchor="middle">{{ leftLabel }}</text>
      </g>

      <!-- core: the .lol relay -->
      <g transform="translate(280 220)" class="station-core-group">
        <polygon points="0,-58 50,-29 50,29 0,58 -50,29 -50,-29" class="station-core-halo" />
        <polygon points="0,-48 42,-24 42,24 0,48 -42,24 -42,-24" fill="url(#st-core)" filter="url(#st-glow)" />
        <text x="0" y="6" class="station-core-text" text-anchor="middle">.lol</text>
        <text x="0" y="26" class="station-core-sub" text-anchor="middle">{{ coreSub }}</text>
        <text x="0" y="84" class="station-label station-label-strong" text-anchor="middle">{{ coreLabel }}</text>
      </g>

      <!-- right nodes: upstream models -->
      <g transform="translate(492 120)">
        <circle r="26" class="station-node" />
        <circle r="14" fill="#f97316" filter="url(#st-soft)" />
        <text x="0" y="5" class="station-node-letter" text-anchor="middle">C</text>
        <text x="40" y="5" class="station-label">Claude</text>
      </g>
      <g transform="translate(492 220)">
        <circle r="26" class="station-node" />
        <circle r="14" fill="#22c55e" filter="url(#st-soft)" />
        <text x="0" y="5" class="station-node-letter" text-anchor="middle">G</text>
        <text x="40" y="5" class="station-label">GPT</text>
      </g>
      <g transform="translate(492 320)">
        <circle r="26" class="station-node" />
        <circle r="14" fill="#3b82f6" filter="url(#st-soft)" />
        <text x="0" y="5" class="station-node-letter" text-anchor="middle">G</text>
        <text x="40" y="5" class="station-label">Gemini</text>
      </g>
    </svg>

    <!-- live latency badge -->
    <div class="station-latency" :class="`is-${latencyState}`">
      <span class="station-latency-dot"></span>
      <span class="station-latency-title">{{ latencyTitle }}</span>
      <span class="station-latency-value">
        <template v-if="latencyState === 'ok' && latencyMs !== null">{{ latencyMs }}<i>ms</i></template>
        <template v-else-if="latencyState === 'error'">{{ latencyUnavailable }}</template>
        <template v-else>{{ latencyProbing }}</template>
      </span>
    </div>

    <!-- "never runs away" seal -->
    <div class="station-stamp" aria-hidden="true">{{ stampText }}</div>
  </div>
</template>

<script setup lang="ts">
import type { LatencyProbeState } from '@/composables/useLatencyProbe'

withDefaults(
  defineProps<{
    compact?: boolean
    leftLabel: string
    coreLabel: string
    coreSub?: string
    latencyTitle: string
    latencyProbing: string
    latencyUnavailable: string
    latencyMs?: number | null
    latencyState?: LatencyProbeState
    stampText: string
  }>(),
  {
    compact: false,
    coreSub: 'RELAY',
    latencyMs: null,
    latencyState: 'idle'
  }
)
</script>

<style scoped>
.station {
  position: relative;
  width: 100%;
  max-width: 600px;
  margin: 0 auto;
  --station-line: rgba(15, 23, 42, 0.12);
  --station-node: rgba(255, 255, 255, 0.85);
  --station-node-border: rgba(15, 23, 42, 0.12);
  --station-label: rgb(71 85 105);
  --station-label-strong: rgb(15 23 42);
}
.dark .station {
  --station-line: rgba(148, 163, 184, 0.16);
  --station-node: rgba(10, 18, 32, 0.9);
  --station-node-border: rgba(148, 163, 184, 0.22);
  --station-label: rgb(148 163 184);
  --station-label-strong: rgb(241 245 249);
}

.station-glow {
  position: absolute;
  inset: 10% 15%;
  border-radius: 9999px;
  background: radial-gradient(closest-side, rgba(20, 184, 166, 0.35), rgba(99, 102, 241, 0.12) 60%, transparent);
  filter: blur(30px);
  animation: station-breathe 5s ease-in-out infinite;
}
@keyframes station-breathe {
  0%,
  100% {
    opacity: 0.7;
    transform: scale(0.96);
  }
  50% {
    opacity: 1;
    transform: scale(1.04);
  }
}

.station-svg {
  position: relative;
  display: block;
  width: 100%;
  height: auto;
}

.station-ring {
  fill: none;
  stroke: var(--station-line);
  stroke-width: 1;
  stroke-dasharray: 4 10;
  transform-origin: center;
}
.station-ring-1 {
  animation: station-spin 70s linear infinite;
}
.station-ring-2 {
  stroke-dasharray: 2 8;
  animation: station-spin 45s linear infinite reverse;
}
.station-ring-3 {
  stroke: rgba(45, 212, 191, 0.45);
  stroke-dasharray: 6 6;
  animation: station-spin 24s linear infinite;
}
@keyframes station-spin {
  to {
    transform: rotate(360deg);
  }
}

.station-link {
  stroke: url(#st-line);
  stroke-width: 1.5;
  stroke-dasharray: 5 7;
  animation: station-dash 1.4s linear infinite;
}
@keyframes station-dash {
  to {
    stroke-dashoffset: -24;
  }
}

.station-packet {
  fill: #2dd4bf;
  filter: drop-shadow(0 0 6px rgba(45, 212, 191, 0.9));
}
.station-packet-a {
  fill: #fb923c;
  filter: drop-shadow(0 0 6px rgba(251, 146, 60, 0.9));
}
.station-packet-b {
  fill: #4ade80;
  filter: drop-shadow(0 0 6px rgba(74, 222, 128, 0.9));
}
.station-packet-c {
  fill: #60a5fa;
  filter: drop-shadow(0 0 6px rgba(96, 165, 250, 0.9));
}

.station-node {
  fill: var(--station-node);
  stroke: var(--station-node-border);
  stroke-width: 1;
}
.station-node-letter {
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 13px;
  font-weight: 800;
  fill: #fff;
}
.station-label {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  letter-spacing: 0.04em;
  fill: var(--station-label);
}
.station-label-strong {
  font-size: 13px;
  font-weight: 700;
  fill: var(--station-label-strong);
}

.station-core-halo {
  fill: rgba(45, 212, 191, 0.12);
  stroke: rgba(45, 212, 191, 0.5);
  stroke-width: 1;
  transform-origin: center;
  animation: station-core-pulse 2.8s ease-in-out infinite;
}
@keyframes station-core-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.8;
  }
  50% {
    transform: scale(1.12);
    opacity: 0.35;
  }
}
.station-core-text {
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 30px;
  font-weight: 900;
  letter-spacing: -0.04em;
  fill: #fff;
}
.station-core-sub {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 9px;
  letter-spacing: 0.32em;
  fill: rgba(255, 255, 255, 0.8);
}

/* latency badge */
.station-latency {
  position: absolute;
  top: 6%;
  left: 4%;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border-radius: 12px;
  border: 1px solid var(--station-node-border);
  background: var(--station-node);
  backdrop-filter: blur(12px);
  padding: 8px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: var(--station-label);
  box-shadow: 0 14px 30px -18px rgba(2, 6, 23, 0.6);
}
.station-latency-dot {
  width: 7px;
  height: 7px;
  border-radius: 9999px;
  background: #94a3b8;
}
.station-latency.is-probing .station-latency-dot {
  background: #f59e0b;
  animation: station-blink 1s ease-in-out infinite;
}
.station-latency.is-ok .station-latency-dot {
  background: #2dd4bf;
  box-shadow: 0 0 10px #2dd4bf;
}
.station-latency.is-error .station-latency-dot {
  background: #f43f5e;
}
@keyframes station-blink {
  50% {
    opacity: 0.3;
  }
}
.station-latency-value {
  font-size: 15px;
  font-weight: 700;
  color: var(--station-label-strong);
}
.station-latency-value i {
  margin-left: 2px;
  font-size: 10px;
  font-style: normal;
  font-weight: 500;
  color: var(--station-label);
}

/* seal */
.station-stamp {
  position: absolute;
  right: 3%;
  bottom: 4%;
  transform: rotate(-12deg);
  border: 2.5px solid rgba(244, 63, 94, 0.85);
  border-radius: 8px;
  padding: 4px 12px;
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 18px;
  font-weight: 900;
  letter-spacing: 0.22em;
  color: rgba(244, 63, 94, 0.9);
  background: rgba(255, 255, 255, 0.04);
  box-shadow: inset 0 0 0 1px rgba(244, 63, 94, 0.25);
  -webkit-mask-image: radial-gradient(circle at 30% 40%, #000 60%, rgba(0, 0, 0, 0.7) 100%);
  mask-image: radial-gradient(circle at 30% 40%, #000 60%, rgba(0, 0, 0, 0.7) 100%);
}
.station-stamp::after {
  content: '';
  position: absolute;
  inset: 3px;
  border: 1px solid rgba(244, 63, 94, 0.5);
  border-radius: 5px;
}

/* compact mode (auth page brand panel) */
.station-compact {
  max-width: 420px;
}
.station-compact .station-latency {
  padding: 6px 10px;
}
.station-compact .station-stamp {
  font-size: 14px;
  padding: 3px 10px;
}

@media (prefers-reduced-motion: reduce) {
  .station-glow,
  .station-ring,
  .station-link,
  .station-core-halo,
  .station-latency-dot {
    animation: none;
  }
}
</style>

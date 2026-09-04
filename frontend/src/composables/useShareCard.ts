import QRCode from 'qrcode'

export type ShareCardSize = 'og' | 'square'

export interface ShareCardOptions {
  size: ShareCardSize
  /** Big wordmark, e.g. "rest2build" (rendered as rest · 2 · build when it matches xxx2yyy). */
  brand: string
  domain: string
  headline: string
  subline: string
  /** Optional free-form line written by the user. */
  note?: string
  /** e.g. "实时延迟 12 ms · 2026-09-04" */
  meta?: string
  /** Text encoded in the QR code (site URL or invite link). Omit to skip the QR. */
  qrText?: string
  qrCaption?: string
  footer?: string
}

const SIZES: Record<ShareCardSize, { w: number; h: number }> = {
  og: { w: 1200, h: 630 },
  square: { w: 1080, h: 1080 }
}

function roundedRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

function wrapLines(ctx: CanvasRenderingContext2D, text: string, maxWidth: number): string[] {
  const lines: string[] = []
  let current = ''
  for (const ch of Array.from(text)) {
    const candidate = current + ch
    if (ctx.measureText(candidate).width > maxWidth && current) {
      lines.push(current)
      current = ch
    } else {
      current = candidate
    }
  }
  if (current) lines.push(current)
  return lines
}

/**
 * Draws a ".lol"-styled share card onto a canvas. Pure client-side; nothing is uploaded.
 */
export async function renderShareCard(canvas: HTMLCanvasElement, opts: ShareCardOptions): Promise<void> {
  const { w, h } = SIZES[opts.size]
  canvas.width = w
  canvas.height = h
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const square = opts.size === 'square'
  const pad = square ? 84 : 72

  // background
  const bg = ctx.createLinearGradient(0, 0, w, h)
  bg.addColorStop(0, '#07111f')
  bg.addColorStop(1, '#050b14')
  ctx.fillStyle = bg
  ctx.fillRect(0, 0, w, h)

  // aurora blobs
  const blob = (x: number, y: number, r: number, c: string) => {
    const g = ctx.createRadialGradient(x, y, 0, x, y, r)
    g.addColorStop(0, c)
    g.addColorStop(1, 'rgba(0,0,0,0)')
    ctx.fillStyle = g
    ctx.fillRect(0, 0, w, h)
  }
  blob(w * 0.85, h * 0.1, w * 0.45, 'rgba(20,184,166,0.35)')
  blob(w * 0.1, h * 0.95, w * 0.5, 'rgba(99,102,241,0.3)')

  // grid
  ctx.strokeStyle = 'rgba(148,163,184,0.08)'
  ctx.lineWidth = 1
  for (let x = 0; x <= w; x += 56) {
    ctx.beginPath()
    ctx.moveTo(x, 0)
    ctx.lineTo(x, h)
    ctx.stroke()
  }
  for (let y = 0; y <= h; y += 56) {
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(w, y)
    ctx.stroke()
  }

  // domain pill
  ctx.font = `600 ${square ? 30 : 26}px ui-monospace, SFMono-Regular, Menlo, monospace`
  const pillText = `● ${opts.domain}`
  const pillW = ctx.measureText(pillText).width + 44
  roundedRect(ctx, pad, pad, pillW, square ? 58 : 50, 999)
  ctx.fillStyle = 'rgba(20,184,166,0.14)'
  ctx.fill()
  ctx.strokeStyle = 'rgba(45,212,191,0.6)'
  ctx.stroke()
  ctx.fillStyle = '#5eead4'
  ctx.textBaseline = 'middle'
  ctx.fillText(pillText, pad + 22, pad + (square ? 29 : 25))

  // wordmark
  let y = pad + (square ? 170 : 150)
  const brandSize = square ? 132 : 118
  ctx.textBaseline = 'alphabetic'
  const match = /^([A-Za-z]+)2([A-Za-z]+)$/.exec(opts.brand)
  let x = pad
  if (match) {
    ctx.font = `500 ${brandSize}px ui-sans-serif, system-ui, -apple-system, sans-serif`
    ctx.fillStyle = '#94a3b8'
    ctx.fillText(match[1], x, y)
    x += ctx.measureText(match[1]).width
    ctx.font = `800 ${brandSize}px ui-sans-serif, system-ui, -apple-system, sans-serif`
    const g2 = ctx.createLinearGradient(x, 0, x + brandSize, 0)
    g2.addColorStop(0, '#2dd4bf')
    g2.addColorStop(1, '#38bdf8')
    ctx.fillStyle = g2
    ctx.fillText('2', x, y)
    x += ctx.measureText('2').width
    const g3 = ctx.createLinearGradient(x, 0, x + brandSize * 3, 0)
    g3.addColorStop(0, '#5eead4')
    g3.addColorStop(0.55, '#67e8f9')
    g3.addColorStop(1, '#a5b4fc')
    ctx.fillStyle = g3
    ctx.fillText(match[2], x, y)
  } else {
    ctx.font = `800 ${brandSize}px ui-sans-serif, system-ui, -apple-system, sans-serif`
    ctx.fillStyle = '#ffffff'
    ctx.fillText(opts.brand, x, y)
  }

  // headline + subline
  y += square ? 96 : 84
  ctx.font = `700 ${square ? 54 : 46}px ui-sans-serif, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif`
  ctx.fillStyle = '#f8fafc'
  ctx.fillText(opts.headline, pad, y)
  y += square ? 64 : 56
  ctx.font = `400 ${square ? 34 : 28}px ui-sans-serif, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif`
  ctx.fillStyle = '#cbd5e1'
  ctx.fillText(opts.subline, pad, y)

  // note (wrapped)
  const qrSize = opts.qrText ? (square ? 220 : 180) : 0
  const textMaxW = w - pad * 2 - (qrSize ? qrSize + 40 : 0)
  if (opts.note) {
    y += square ? 78 : 64
    ctx.font = `500 ${square ? 32 : 26}px ui-sans-serif, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif`
    ctx.fillStyle = '#5eead4'
    for (const line of wrapLines(ctx, `“${opts.note}”`, textMaxW).slice(0, 3)) {
      ctx.fillText(line, pad, y)
      y += square ? 42 : 34
    }
  }

  // meta chip (latency / date)
  if (opts.meta) {
    ctx.font = `600 ${square ? 26 : 22}px ui-monospace, SFMono-Regular, Menlo, monospace`
    const mw = ctx.measureText(opts.meta).width + 36
    const my = h - pad - (square ? 120 : 96)
    roundedRect(ctx, pad, my, mw, square ? 48 : 42, 12)
    ctx.fillStyle = 'rgba(15,23,42,0.85)'
    ctx.fill()
    ctx.strokeStyle = 'rgba(148,163,184,0.25)'
    ctx.stroke()
    ctx.fillStyle = '#e2e8f0'
    ctx.textBaseline = 'middle'
    ctx.fillText(opts.meta, pad + 18, my + (square ? 24 : 21))
    ctx.textBaseline = 'alphabetic'
  }

  // footer
  if (opts.footer) {
    ctx.font = `400 ${square ? 24 : 20}px ui-sans-serif, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif`
    ctx.fillStyle = 'rgba(148,163,184,0.85)'
    ctx.fillText(opts.footer, pad, h - pad + (square ? 4 : 0))
  }

  // QR
  if (opts.qrText && qrSize) {
    const qx = w - pad - qrSize
    const qy = h - pad - qrSize - (square ? 36 : 28)
    roundedRect(ctx, qx - 14, qy - 14, qrSize + 28, qrSize + 28, 18)
    ctx.fillStyle = '#ffffff'
    ctx.fill()
    const dataUrl = await QRCode.toDataURL(opts.qrText, { margin: 0, width: qrSize, color: { dark: '#0f172a', light: '#ffffff' } })
    await new Promise<void>((resolve) => {
      const img = new Image()
      img.onload = () => {
        ctx.drawImage(img, qx, qy, qrSize, qrSize)
        resolve()
      }
      img.onerror = () => resolve()
      img.src = dataUrl
    })
    if (opts.qrCaption) {
      ctx.font = `500 ${square ? 22 : 18}px ui-sans-serif, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif`
      ctx.fillStyle = '#94a3b8'
      ctx.textAlign = 'center'
      ctx.fillText(opts.qrCaption, qx + qrSize / 2, h - pad + (square ? 4 : 0))
      ctx.textAlign = 'left'
    }
  }

  // stamp
  ctx.save()
  ctx.translate(w - pad - (square ? 40 : 30), pad + (square ? 30 : 24))
  ctx.rotate(-0.2)
  ctx.font = `900 ${square ? 30 : 26}px ui-sans-serif, system-ui, -apple-system, sans-serif`
  const st = '.lol'
  const sw = ctx.measureText(st).width + 30
  roundedRect(ctx, -sw, -22, sw, 44, 8)
  ctx.strokeStyle = 'rgba(244,63,94,0.85)'
  ctx.lineWidth = 3
  ctx.stroke()
  ctx.fillStyle = 'rgba(244,63,94,0.9)'
  ctx.textBaseline = 'middle'
  ctx.fillText(st, -sw + 15, 0)
  ctx.restore()
}

export function canvasToBlob(canvas: HTMLCanvasElement): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob((b) => resolve(b), 'image/png'))
}

export async function downloadCanvas(canvas: HTMLCanvasElement, filename: string): Promise<void> {
  const blob = await canvasToBlob(canvas)
  if (!blob) return
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

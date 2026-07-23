import { describe, expect, it, vi } from 'vitest'
import {
  fetchCodexMemoryReleaseManifest,
  formatAssetSize,
  parseCodexMemoryReleaseManifest,
} from '@/utils/codexMemoryRelease'

const manifest = {
  schema_version: 1,
  version: '0.1.0',
  tag: 'codex-memory-v0.1.0',
  repository: 'owner/sub2api',
  python_minimum: '3.11',
  released_at: null,
  assets: ['macos', 'windows', 'linux'].map((platform) => ({
    platform,
    filename: `codex-memory_0.1.0_${platform}.zip`,
    url: `https://github.com/owner/sub2api/releases/download/codex-memory-v0.1.0/codex-memory_0.1.0_${platform}.zip`,
    sha256: 'a'.repeat(64),
    size: 2048,
  })),
  checksums: { filename: 'codex-memory-checksums.txt', url: 'https://github.com/owner/sub2api/releases/download/codex-memory-v0.1.0/codex-memory-checksums.txt', sha256: 'b'.repeat(64) },
}

describe('Codex Memory release manifest', () => {
  it('accepts one immutable asset for every supported platform', () => {
    expect(parseCodexMemoryReleaseManifest(manifest).assets).toHaveLength(3)
    expect(formatAssetSize(2048)).toBe('2.0 KiB')
  })

  it('rejects duplicate platforms and invalid hashes', () => {
    const duplicate = structuredClone(manifest)
    duplicate.assets[1].platform = 'macos'
    expect(() => parseCodexMemoryReleaseManifest(duplicate)).toThrow('platform set')
    const invalid = structuredClone(manifest)
    invalid.assets[0].sha256 = 'bad'
    expect(() => parseCodexMemoryReleaseManifest(invalid)).toThrow('integrity')
    const unsafeChecksums = structuredClone(manifest)
    unsafeChecksums.checksums.url = 'http://example.com/checksums.txt'
    expect(() => parseCodexMemoryReleaseManifest(unsafeChecksums)).toThrow('checksum')

    const wrongTag = structuredClone(manifest)
    wrongTag.tag = 'v0.1.0'
    expect(() => parseCodexMemoryReleaseManifest(wrongTag)).toThrow('metadata')

    const crossRepository = structuredClone(manifest)
    crossRepository.assets[0].url = crossRepository.assets[0].url.replace('owner/sub2api', 'attacker/sub2api')
    expect(() => parseCodexMemoryReleaseManifest(crossRepository)).toThrow('asset URL')
  })

  it('does not silently use stale data when the manifest is unavailable', async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: false, status: 404 })
    await expect(fetchCodexMemoryReleaseManifest(fetcher)).rejects.toThrow('unavailable')
  })
})

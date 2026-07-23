export interface CodexMemoryReleaseAsset {
  platform: 'macos' | 'windows' | 'linux'
  filename: string
  url: string
  sha256: string
  size: number
}

export interface CodexMemoryReleaseManifest {
  schema_version: 1
  version: string
  tag: string
  repository: string
  python_minimum: string
  released_at: string | null
  assets: CodexMemoryReleaseAsset[]
  checksums: {
    filename: string
    url: string
    sha256: string
  }
}

const platforms = new Set(['macos', 'windows', 'linux'])
const versionPattern = /^[0-9]+\.[0-9]+\.[0-9]+(?:[-.][0-9A-Za-z]+)*$/
const repositoryPattern = /^[0-9A-Za-z_.-]+\/[0-9A-Za-z_.-]+$/

export function parseCodexMemoryReleaseManifest(value: unknown): CodexMemoryReleaseManifest {
  if (!value || typeof value !== 'object') throw new Error('Release manifest is not an object')
  const manifest = value as Partial<CodexMemoryReleaseManifest>
  if (manifest.schema_version !== 1) throw new Error('Unsupported release manifest schema')
  if (
    !manifest.version ||
    !versionPattern.test(manifest.version) ||
    manifest.tag !== `codex-memory-v${manifest.version}` ||
    !manifest.repository ||
    !repositoryPattern.test(manifest.repository) ||
    !manifest.python_minimum
  ) {
    throw new Error('Release manifest metadata is incomplete')
  }
  if (!Array.isArray(manifest.assets) || manifest.assets.length !== 3) {
    throw new Error('Release manifest must contain exactly three platform assets')
  }
  const seen = new Set<string>()
  for (const asset of manifest.assets) {
    if (!platforms.has(asset.platform) || seen.has(asset.platform)) {
      throw new Error('Release manifest platform set is invalid')
    }
    seen.add(asset.platform)
    const expectedFilename = `codex-memory_${manifest.version}_${asset.platform}.zip`
    const expectedUrl = `https://github.com/${manifest.repository}/releases/download/${manifest.tag}/${expectedFilename}`
    if (asset.filename !== expectedFilename || asset.url !== expectedUrl) {
      throw new Error('Release asset URL is invalid')
    }
    if (!/^[a-f0-9]{64}$/.test(asset.sha256) || !Number.isSafeInteger(asset.size) || asset.size < 1) {
      throw new Error('Release asset integrity metadata is invalid')
    }
  }
  if (
    !manifest.checksums ||
    manifest.checksums.filename !== 'codex-memory-checksums.txt' ||
    manifest.checksums.url !==
      `https://github.com/${manifest.repository}/releases/download/${manifest.tag}/codex-memory-checksums.txt` ||
    !/^[a-f0-9]{64}$/.test(manifest.checksums.sha256)
  ) {
    throw new Error('Release checksum metadata is invalid')
  }
  return manifest as CodexMemoryReleaseManifest
}

export async function fetchCodexMemoryReleaseManifest(
  fetcher: typeof fetch = fetch,
): Promise<CodexMemoryReleaseManifest> {
  const response = await fetcher('/codex-memory-release-manifest.json', { cache: 'no-store' })
  if (!response.ok) throw new Error(`Release manifest unavailable (${response.status})`)
  return parseCodexMemoryReleaseManifest(await response.json())
}

export function formatAssetSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KiB`
}

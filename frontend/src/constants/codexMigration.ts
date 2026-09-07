import prompt from '../content/codexSessionMigrationPrompt.txt?raw'

export const CODEX_SESSION_MIGRATION = {
  route: '/error-experiences/codex-session-migration',
  promptDownload: '/codex-session-migrate-prompt.txt',
  packageDownload: '/codex-session-migrate-1.0.0.zip',
  manifestDownload: '/codex-session-migrate-manifest.json',
  toolVersion: '1.0.0',
  title: '切换接入后，为什么 Codex 旧对话无法继续？',
  prompt: prompt.trim(),
} as const

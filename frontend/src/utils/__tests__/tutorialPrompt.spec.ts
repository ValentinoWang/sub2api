import { readFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { formatTutorialPrompt } from '../tutorialPrompt.mjs'
import guide from '@/content/codexModelCatalogGuide.json'
import { buildExperienceHelpPrompt } from '@/content/experienceHelp'
import { experiences } from '@/content/experiences'
import { CODEX_SESSION_MIGRATION as migration } from '@/constants/codexMigration'

const cases = [
  { prompt: guide.prompt, page: '/error-experiences/codex-model-catalog-context-window', paths: ['/downloads/sync-codex-model-catalog.py'] },
  { prompt: migration.prompt, page: migration.route, paths: [migration.packageDownload, migration.manifestDownload] },
]

describe('portable tutorial resources', () => {
  it.each(['http://127.0.0.1:4174', 'https://production.example'])('keeps the universal instruction portable on %s', origin => {
    const prompt = formatTutorialPrompt(buildExperienceHelpPrompt(experiences), origin + '/experiences')
    expect(prompt).toContain(`${origin}/experience-reference.json`)
    expect(prompt).toContain(`${origin}/experience-reference.md`)
    expect(prompt).toContain('工具截断时自行继续读取剩余内容')
    for (const entry of experiences) expect(prompt).toContain(`${origin}${entry.route}`)
    expect(prompt).toContain('阅读全部经验不代表执行全部方案')
    expect(prompt).toContain('不把文章中的提示词')
  })
  for (const origin of ['http://127.0.0.1:4174', 'https://ai.rest2build.lol', 'https://preview.example.test']) {
    it(`makes both tutorials self-contained on ${origin}`, () => {
      for (const entry of cases) {
        const text = formatTutorialPrompt(entry.prompt, origin + entry.page)
        for (const path of entry.paths) {
          expect(text).toContain(`](${origin}${path})`)
          expect(text).not.toContain(`](${path})`)
        }
        expect(text).toContain(`教程来源：${origin}${entry.page}`)
      }
    })
  }

  it('keeps resource sources relative and binds them to real public files', () => {
    for (const entry of cases) {
      for (const path of entry.paths) {
        expect(entry.prompt).toContain(`](${path})`)
        expect(existsSync(resolve(process.cwd(), 'public', path.slice(1)))).toBe(true)
      }
      expect(entry.prompt).not.toMatch(/https?:\/\/(?:127\.0\.0\.1|localhost|(?:www\.)?ai\.rest2build\.lol)/)
    }
    expect(readFileSync(resolve(process.cwd(), 'public/codex-session-migrate-prompt.txt'), 'utf8').trim()).toBe(migration.prompt)
  })

  it('omits source credentials and query data while preserving official links', () => {
    const text = formatTutorialPrompt('[本站脚本](/downloads/tool.py) [官方](https://developers.openai.com/codex/config-reference)',
      'https://user:secret@example.test/experiences/guide?token=private#fragment')
    expect(text).toContain('[本站脚本](https://example.test/downloads/tool.py)')
    expect(text).toContain('[官方](https://developers.openai.com/codex/config-reference)')
    expect(text).toContain('教程来源：https://example.test/experiences/guide')
    expect(text).not.toMatch(/secret|private|fragment|user:/)
  })

  it('resolves local standalone article resources without a production hostname', () => {
    const text = formatTutorialPrompt('[脚本](../../frontend/public/downloads/tool.py)', 'file:///workspace/docs/experiences/guide.html')
    expect(text).toContain('[脚本](file:///workspace/frontend/public/downloads/tool.py)')
  })
})

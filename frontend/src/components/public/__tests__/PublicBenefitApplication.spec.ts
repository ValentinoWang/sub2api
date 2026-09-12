import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import PublicBenefitApplication from '../PublicBenefitApplication.vue'

const { copyToClipboard } = vi.hoisted(() => ({ copyToClipboard: vi.fn() }))
const locale = ref('zh')
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))

let wrapper: VueWrapper

async function fillRequired() {
  await wrapper.get('[name="category"]').setValue('student')
  await wrapper.get('[name="field"]').setValue('计算数学')
  await wrapper.get('[name="tool"]').setValue('codex')
  await wrapper.get('[name="purpose"]').setValue('为课程项目开发可复现的数值计算示例。')
  await wrapper.get('[name="consent"]').setValue(true)
}

function action(icon: string) {
  return wrapper.findAll('button').find((button) => button.find(`icon-stub[name="${icon}"]`).exists())!
}

beforeEach(() => {
  vi.clearAllMocks()
  locale.value = 'zh'
  copyToClipboard.mockResolvedValue(true)
  wrapper = mount(PublicBenefitApplication, {
    attachTo: document.body,
    global: { stubs: { Icon: true } }
  })
})

afterEach(() => {
  wrapper.unmount()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('PublicBenefitApplication', () => {
  it('requires applicant facts and consent, focuses the first invalid field, and does not ask for credentials', async () => {
    await wrapper.get('form').trigger('submit')

    expect(wrapper.get('[role="alert"]').text()).toBe('请检查并补全标记的项目。')
    expect(document.activeElement).toBe(wrapper.get('[name="category"]').element)
    expect(wrapper.findAll('[aria-invalid="true"]')).toHaveLength(5)
    expect(wrapper.find('.benefit-result').exists()).toBe(false)
    expect(wrapper.find('input[type="password"], input[type="email"], input[type="file"]').exists()).toBe(false)
    expect(wrapper.findAll('input').map((input) => input.attributes('name'))).toEqual(['field', 'organization', 'projectUrl', 'consent'])
  })

  it('generates a local unsubmitted application without optional personal information, network requests, or storage writes', async () => {
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    const storageWrite = vi.spyOn(Storage.prototype, 'setItem')
    const xhrOpen = vi.spyOn(XMLHttpRequest.prototype, 'open')
    await fillRequired()
    await wrapper.get('form').trigger('submit')

    const text = (wrapper.get('.benefit-output').element as HTMLTextAreaElement).value
    expect(text).toContain('申请类型: 相关专业学生')
    expect(text).toContain('学习／研究领域: 计算数学')
    expect(text).toContain('拟使用的工具: Codex')
    expect(text).toContain('为课程项目开发可复现的数值计算示例。')
    expect(text).toContain('仅用于本次申请审核与必要联系')
    expect(text).not.toContain('学校／组织:')
    expect(text).not.toContain('公开项目链接:')
    expect(wrapper.get('.benefit-generated-status').text()).toBe('申请说明已生成，尚未提交')
    expect(wrapper.text()).toContain('当前未配置申请联系信息，申请尚无法递交。')
    expect(fetch).not.toHaveBeenCalled()
    expect(xhrOpen).not.toHaveBeenCalled()
    expect(storageWrite).not.toHaveBeenCalled()
  })

  it.each(['javascript:alert(1)', 'https://user:password@example.org/project', 'not-a-url'])('rejects a nonpublic or malformed project URL: %s', async (url) => {
    await fillRequired()
    await wrapper.get('[name="projectUrl"]').setValue(url)
    await wrapper.get('form').trigger('submit')

    expect(wrapper.get('[name="projectUrl"]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.find('.benefit-result').exists()).toBe(false)
  })

  it('includes optional public facts and renders configured contact as text', async () => {
    await wrapper.setProps({ contactInfo: '申请联系：@rest2build\n<script>alert(1)</script>' })
    await fillRequired()
    await wrapper.get('[name="organization"]').setValue('示例大学')
    await wrapper.get('[name="projectUrl"]').setValue('https://example.org/project')
    await wrapper.get('form').trigger('submit')

    const text = (wrapper.get('.benefit-output').element as HTMLTextAreaElement).value
    expect(text).toContain('学校／组织: 示例大学')
    expect(text).toContain('公开项目链接: https://example.org/project')
    expect(wrapper.get('.benefit-contact-info').text()).toContain('<script>alert(1)</script>')
    expect(wrapper.find('script').exists()).toBe(false)
  })

  it.each(['field', 'organization', 'purpose', 'projectUrl', 'category', 'tool', 'consent'])('invalidates generated output when %s changes', async (name) => {
    await fillRequired()
    await wrapper.get('form').trigger('submit')
    const value = name === 'consent' ? false : name === 'category' ? 'developer' : name === 'tool' ? 'both' : 'changed'
    await wrapper.get(`[name="${name}"]`).setValue(value)

    expect(wrapper.find('.benefit-result').exists()).toBe(false)
  })

  it('invalidates generated text on locale changes and regenerates in English', async () => {
    await fillRequired()
    await wrapper.get('form').trigger('submit')
    locale.value = 'en'
    await nextTick()

    expect(wrapper.find('.benefit-result').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.get('.benefit-generated-status').text()).toBe('Application text generated. Not submitted.')
    expect((wrapper.get('.benefit-output').element as HTMLTextAreaElement).value).toContain('Applicant category: Student in a relevant field')
  })

  it.each(['false', 'throw'])('shows a copy failure when the clipboard reports %s', async (failure) => {
    if (failure === 'throw') copyToClipboard.mockRejectedValue(new Error('Clipboard denied'))
    else copyToClipboard.mockResolvedValue(false)
    await fillRequired()
    await wrapper.get('form').trigger('submit')
    await action('copy').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('复制失败，申请尚未提交')
    expect(wrapper.get('.benefit-generated-status').text()).toBe('申请说明已生成，尚未提交')
  })

  it('copies the generated text and keeps a late copy result from marking revised text as copied', async () => {
    await fillRequired()
    await wrapper.get('form').trigger('submit')
    await action('copy').trigger('click')
    await flushPromises()
    expect(copyToClipboard).toHaveBeenCalledWith((wrapper.get('.benefit-output').element as HTMLTextAreaElement).value, '申请说明已复制，尚未提交。')
    expect(wrapper.get('.benefit-feedback').text()).toBe('申请说明已复制，尚未提交。')

    let resolveCopy!: (value: boolean) => void
    copyToClipboard.mockReturnValue(new Promise<boolean>((resolve) => { resolveCopy = resolve }))
    await action('copy').trigger('click')
    await wrapper.get('[name="purpose"]').setValue('另一项研究计划')
    await wrapper.get('form').trigger('submit')
    resolveCopy(true)
    await flushPromises()

    expect(wrapper.find('.benefit-feedback').exists()).toBe(false)
    expect(wrapper.get('.benefit-generated-status').text()).toBe('申请说明已生成，尚未提交')
  })

  it('downloads the exact application as TXT and releases its object URL', async () => {
    vi.useFakeTimers()
    const createObjectURL = vi.fn().mockReturnValue('blob:application')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal('URL', Object.assign(class extends URL {}, { createObjectURL, revokeObjectURL }))
    let filename = ''
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) { filename = this.download })
    await fillRequired()
    await wrapper.get('form').trigger('submit')
    await action('download').trigger('click')

    const blob = createObjectURL.mock.calls[0]![0] as Blob
    expect(blob.type).toBe('text/plain;charset=utf-8')
    expect(revokeObjectURL).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1000)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:application')
    vi.useRealTimers()
    const reader = new FileReader()
    const content = new Promise((resolve) => { reader.onload = () => resolve(reader.result) })
    reader.readAsText(blob)
    expect(await content).toBe((wrapper.get('.benefit-output').element as HTMLTextAreaElement).value)
    expect(filename).toBe('rest2build-public-benefit-application.txt')
    expect(wrapper.get('.benefit-feedback').text()).toContain('申请尚未提交')
  })

  it('reports a failed download without marking the application as submitted', async () => {
    vi.stubGlobal('URL', Object.assign(class extends URL {}, { createObjectURL: vi.fn(() => { throw new Error('Download unavailable') }) }))
    await fillRequired()
    await wrapper.get('form').trigger('submit')
    await action('download').trigger('click')

    expect(wrapper.get('[role="alert"]').text()).toContain('下载未能启动，申请尚未提交')
    expect(wrapper.find('a[download]').exists()).toBe(false)
  })
})

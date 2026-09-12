<template>
  <section class="benefit-application" aria-labelledby="benefit-application-title">
    <header class="benefit-header">
      <h2 id="benefit-application-title">{{ copy.title }}</h2>
      <p>{{ copy.intro }}</p>
      <p class="benefit-privacy">{{ copy.privacy }}</p>
    </header>

    <form ref="applicationForm" novalidate autocomplete="off" @submit.prevent="generateApplication">
      <div class="benefit-fields">
        <div class="benefit-field">
          <label for="benefit-category">{{ copy.category }}</label>
          <select id="benefit-category" v-model="form.category" name="category" required :aria-invalid="hasError('category')" :aria-describedby="hasError('category') ? 'benefit-category-error' : undefined">
            <option disabled value="">{{ copy.choose }}</option>
            <option v-for="option in copy.categories" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
          <p v-if="hasError('category')" id="benefit-category-error" class="benefit-error">{{ errors.category }}</p>
        </div>

        <div class="benefit-field">
          <label for="benefit-field">{{ copy.field }}</label>
          <input id="benefit-field" v-model="form.field" name="field" type="text" required maxlength="120" :aria-invalid="hasError('field')" :aria-describedby="hasError('field') ? 'benefit-field-error' : undefined" />
          <p v-if="hasError('field')" id="benefit-field-error" class="benefit-error">{{ errors.field }}</p>
        </div>

        <div class="benefit-field">
          <label for="benefit-organization">{{ copy.organization }} <span>{{ copy.optional }}</span></label>
          <input id="benefit-organization" v-model="form.organization" name="organization" type="text" maxlength="160" :aria-invalid="hasError('organization')" :aria-describedby="hasError('organization') ? 'benefit-organization-error' : undefined" />
          <p v-if="hasError('organization')" id="benefit-organization-error" class="benefit-error">{{ errors.organization }}</p>
        </div>

        <div class="benefit-field">
          <label for="benefit-tool">{{ copy.tool }}</label>
          <select id="benefit-tool" v-model="form.tool" name="tool" required :aria-invalid="hasError('tool')" :aria-describedby="hasError('tool') ? 'benefit-tool-error' : undefined">
            <option disabled value="">{{ copy.choose }}</option>
            <option v-for="option in copy.tools" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
          <p v-if="hasError('tool')" id="benefit-tool-error" class="benefit-error">{{ errors.tool }}</p>
        </div>

        <div class="benefit-field benefit-field-wide">
          <label for="benefit-purpose">{{ copy.purpose }}</label>
          <textarea id="benefit-purpose" v-model="form.purpose" name="purpose" required rows="4" maxlength="1000" :aria-invalid="hasError('purpose')" :aria-describedby="hasError('purpose') ? 'benefit-purpose-error' : undefined"></textarea>
          <p v-if="hasError('purpose')" id="benefit-purpose-error" class="benefit-error">{{ errors.purpose }}</p>
        </div>

        <div class="benefit-field benefit-field-wide">
          <label for="benefit-project-url">{{ copy.projectUrl }} <span>{{ copy.optional }}</span></label>
          <input id="benefit-project-url" v-model="form.projectUrl" name="projectUrl" type="url" inputmode="url" maxlength="500" :aria-invalid="hasError('projectUrl')" :aria-describedby="hasError('projectUrl') ? 'benefit-project-url-error' : undefined" />
          <p v-if="hasError('projectUrl')" id="benefit-project-url-error" class="benefit-error">{{ errors.projectUrl }}</p>
        </div>
      </div>

      <label class="benefit-consent" for="benefit-consent">
        <input id="benefit-consent" v-model="form.consent" name="consent" type="checkbox" required :aria-invalid="hasError('consent')" :aria-describedby="hasError('consent') ? 'benefit-consent-error' : undefined" />
        <span>{{ copy.consent }}</span>
      </label>
      <p v-if="hasError('consent')" id="benefit-consent-error" class="benefit-error">{{ errors.consent }}</p>
      <p v-if="attempted && Object.keys(errors).length" class="benefit-error" role="alert">{{ copy.checkFields }}</p>

      <button class="benefit-button benefit-button-primary" type="submit">
        <Icon name="document" size="sm" aria-hidden="true" />
        {{ copy.generate }}
      </button>
    </form>

    <section v-if="generatedText" class="benefit-result" aria-labelledby="benefit-result-title">
      <h3 id="benefit-result-title">{{ copy.resultTitle }}</h3>
      <p class="benefit-generated-status" role="status">{{ copy.generated }}</p>
      <textarea class="benefit-output" :value="generatedText" :aria-label="copy.resultTitle" readonly rows="12"></textarea>
      <div class="benefit-actions">
        <button type="button" class="benefit-button" :disabled="copying" @click="copyApplication">
          <Icon name="copy" size="sm" aria-hidden="true" />
          {{ copying ? copy.copying : copy.copy }}
        </button>
        <button type="button" class="benefit-button" @click="downloadApplication">
          <Icon name="download" size="sm" aria-hidden="true" />
          {{ copy.download }}
        </button>
      </div>
      <p v-if="feedback" :class="feedback.endsWith('Failed') ? 'benefit-error' : 'benefit-feedback'" :role="feedback.endsWith('Failed') ? 'alert' : 'status'">{{ copy[feedback] }}</p>
    </section>

    <div class="benefit-contact">
      <h3>{{ copy.contactTitle }}</h3>
      <p v-if="contactInfo.trim()" class="benefit-contact-info">{{ contactInfo }}</p>
      <p v-else>{{ copy.contactUnavailable }}</p>
      <p>{{ copy.reviewNotice }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

withDefaults(defineProps<{ contactInfo?: string }>(), { contactInfo: '' })

const messages = {
  zh: {
    title: '公益支持申请',
    intro: '面向相关专业学生、开发者和开源项目。请填写实际学习、研究或开发用途。',
    privacy: '申请说明仅在当前页面生成，不会自动上传，也不会保存在浏览器本地存储中。无需提供身份证件、密码、API Key 或其他凭证。',
    category: '申请类型',
    categories: [
      { value: 'student', label: '相关专业学生' },
      { value: 'developer', label: '开发者' },
      { value: 'open-source', label: '开源项目' }
    ],
    field: '学习／研究领域',
    organization: '学校／组织',
    tool: '拟使用的工具',
    tools: [
      { value: 'codex', label: 'Codex' },
      { value: 'claude-code', label: 'Claude Code' },
      { value: 'both', label: 'Codex 与 Claude Code' },
      { value: 'other', label: '其他工具' }
    ],
    purpose: '项目与使用计划',
    projectUrl: '公开项目链接',
    optional: '选填',
    choose: '请选择',
    consent: '我同意将我主动提供的申请信息仅用于本次申请审核与必要联系。',
    checkFields: '请检查并补全标记的项目。',
    required: '请填写此项。',
    selectRequired: '请选择一项。',
    tooLong: '内容超出允许长度，请精简后重试。',
    invalidUrl: '请填写不含账号密码的完整 http:// 或 https:// 公开链接。',
    consentRequired: '请先确认申请信息的使用范围。',
    generate: '生成申请说明',
    resultTitle: '申请说明',
    generated: '申请说明已生成，尚未提交',
    copy: '复制说明',
    copying: '正在复制',
    copied: '申请说明已复制，尚未提交。',
    download: '下载 TXT',
    downloadStarted: '已发起文件下载，申请尚未提交。',
    copyFailed: '复制失败，申请尚未提交。可选中上方说明手动复制，或下载 TXT。',
    downloadFailed: '下载未能启动，申请尚未提交。可复制上方说明。',
    contactTitle: '申请联系渠道',
    contactUnavailable: '当前未配置申请联系信息，申请尚无法递交。',
    reviewNotice: '生成、复制或下载说明均不代表申请已提交或获批。支持范围与结果需由运营方审核确认。'
  },
  en: {
    title: 'Public-benefit support application',
    intro: 'For students in relevant fields, developers, and open-source projects. Describe your actual study, research, or development plans.',
    privacy: 'Your application text is generated on this page only. It is not automatically uploaded or saved in browser storage. No identity documents, passwords, API keys, or other credentials are required.',
    category: 'Applicant category',
    categories: [
      { value: 'student', label: 'Student in a relevant field' },
      { value: 'developer', label: 'Developer' },
      { value: 'open-source', label: 'Open-source project' }
    ],
    field: 'Study / research field',
    organization: 'School / organization',
    tool: 'Intended tool',
    tools: [
      { value: 'codex', label: 'Codex' },
      { value: 'claude-code', label: 'Claude Code' },
      { value: 'both', label: 'Codex and Claude Code' },
      { value: 'other', label: 'Other tool' }
    ],
    purpose: 'Project and intended use',
    projectUrl: 'Public project URL',
    optional: 'Optional',
    choose: 'Select an option',
    consent: 'I agree that the application information I voluntarily provide may be used only to review this application and contact me as necessary.',
    checkFields: 'Please check and complete the marked fields.',
    required: 'Complete this field.',
    selectRequired: 'Select an option.',
    tooLong: 'This exceeds the allowed length. Please shorten it.',
    invalidUrl: 'Enter a complete public http:// or https:// URL without a username or password.',
    consentRequired: 'Please confirm the permitted use of your application information.',
    generate: 'Generate application text',
    resultTitle: 'Application text',
    generated: 'Application text generated. Not submitted.',
    copy: 'Copy text',
    copying: 'Copying',
    copied: 'Application text copied. Not submitted.',
    download: 'Download TXT',
    downloadStarted: 'File download requested. Application not submitted.',
    copyFailed: 'Copy failed. Application not submitted. Select the text above to copy it manually, or download the TXT file.',
    downloadFailed: 'The download could not start. Application not submitted. You can copy the text above.',
    contactTitle: 'Application contact',
    contactUnavailable: 'No application contact is currently configured. Delivery is not yet available.',
    reviewNotice: 'Generating, copying, or downloading this text does not submit or approve an application. The operator must review and confirm the available support and outcome.'
  }
}

const { locale } = useI18n()
const copy = computed(() => locale.value.startsWith('zh') ? messages.zh : messages.en)
const { copyToClipboard } = useClipboard()
const applicationForm = ref<HTMLFormElement | null>(null)
const form = reactive({ category: '', field: '', organization: '', tool: '', purpose: '', projectUrl: '', consent: false })
type Field = keyof typeof form
type Feedback = '' | 'copied' | 'downloadStarted' | 'copyFailed' | 'downloadFailed'
const attempted = ref(false)
const generatedText = ref('')
const copying = ref(false)
const feedback = ref<Feedback>('')
let generation = 0

const errors = computed(() => {
  const result: Partial<Record<Field, string>> = {}
  if (!copy.value.categories.some((option) => option.value === form.category)) result.category = copy.value.selectRequired
  if (!form.field.trim()) result.field = copy.value.required
  else if (form.field.length > 120) result.field = copy.value.tooLong
  if (form.organization.length > 160) result.organization = copy.value.tooLong
  if (!copy.value.tools.some((option) => option.value === form.tool)) result.tool = copy.value.selectRequired
  if (!form.purpose.trim()) result.purpose = copy.value.required
  else if (form.purpose.length > 1000) result.purpose = copy.value.tooLong
  if (form.projectUrl.trim()) {
    try {
      const url = new URL(form.projectUrl.trim())
      if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password) result.projectUrl = copy.value.invalidUrl
    } catch {
      result.projectUrl = copy.value.invalidUrl
    }
  }
  if (form.projectUrl.length > 500) result.projectUrl = copy.value.tooLong
  if (!form.consent) result.consent = copy.value.consentRequired
  return result
})

function hasError(field: Field): boolean {
  return attempted.value && Boolean(errors.value[field])
}

watch([form, locale], () => {
  generation++
  generatedText.value = ''
  feedback.value = ''
}, { deep: true, flush: 'sync' })

onBeforeUnmount(() => { generation++ })

function generateApplication() {
  attempted.value = true
  const firstError = Object.keys(errors.value)[0]
  if (firstError) {
    const input = applicationForm.value?.elements.namedItem(firstError)
    if (input instanceof HTMLElement) input.focus()
    return
  }

  generation++
  feedback.value = ''
  generatedText.value = [
    `rest2build | ${copy.value.title}`,
    copy.value.generated,
    '',
    `${copy.value.category}: ${copy.value.categories.find((option) => option.value === form.category)!.label}`,
    `${copy.value.field}: ${form.field.trim()}`,
    ...(form.organization.trim() ? [`${copy.value.organization}: ${form.organization.trim()}`] : []),
    `${copy.value.tool}: ${copy.value.tools.find((option) => option.value === form.tool)!.label}`,
    `${copy.value.purpose}:`,
    form.purpose.trim(),
    ...(form.projectUrl.trim() ? [`${copy.value.projectUrl}: ${form.projectUrl.trim()}`] : []),
    '',
    copy.value.consent
  ].join('\n')
}

async function copyApplication() {
  if (!generatedText.value || copying.value) return
  const currentGeneration = generation
  copying.value = true
  feedback.value = ''
  try {
    const success = await copyToClipboard(generatedText.value, copy.value.copied)
    if (currentGeneration === generation) feedback.value = success ? 'copied' : 'copyFailed'
  } catch {
    if (currentGeneration === generation) feedback.value = 'copyFailed'
  } finally {
    copying.value = false
  }
}

function downloadApplication() {
  if (!generatedText.value) return
  feedback.value = ''
  let objectUrl: string | undefined
  const anchor = document.createElement('a')
  try {
    objectUrl = URL.createObjectURL(new Blob([generatedText.value], { type: 'text/plain;charset=utf-8' }))
    anchor.href = objectUrl
    anchor.download = 'rest2build-public-benefit-application.txt'
    anchor.hidden = true
    document.body.appendChild(anchor)
    anchor.click()
    feedback.value = 'downloadStarted'
  } catch {
    feedback.value = 'downloadFailed'
  } finally {
    anchor.remove()
    if (objectUrl) {
      const url = objectUrl
      // Allow the browser to consume the download before releasing its object URL.
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    }
  }
}
</script>

<style scoped>
.benefit-application { margin-top: 32px; border-top: 1px solid #c7d5cf; padding-top: 28px; color: #16312d; letter-spacing: 0; }
.benefit-header h2 { margin: 0; font-size: 23px; font-weight: 750; line-height: 1.4; overflow-wrap: anywhere; }
.benefit-header p, .benefit-contact p { margin: 10px 0 0; color: #52615c; font-size: 14px; line-height: 1.75; overflow-wrap: anywhere; }
.benefit-header .benefit-privacy { font-size: 13px; }
.benefit-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; margin-top: 24px; }
.benefit-field { display: flex; min-width: 0; flex-direction: column; gap: 8px; }
.benefit-field-wide { grid-column: 1 / -1; }
.benefit-field label { font-size: 14px; font-weight: 650; line-height: 1.5; }
.benefit-field label span { margin-left: 5px; color: #6a7770; font-size: 12px; font-weight: 400; }
.benefit-field input, .benefit-field select, .benefit-field textarea, .benefit-output { box-sizing: border-box; width: 100%; min-width: 0; border: 1px solid #aabfb5; border-radius: 6px; background: #fff; padding: 10px 12px; color: #16312d; font: inherit; font-size: 14px; line-height: 1.6; }
.benefit-field input, .benefit-field select { min-height: 44px; }
.benefit-field textarea, .benefit-output { resize: vertical; overflow-wrap: anywhere; }
.benefit-field [aria-invalid="true"] { border-color: #b42318; }
.benefit-field :focus-visible, .benefit-output:focus-visible, .benefit-consent input:focus-visible, .benefit-button:focus-visible { outline: 2px solid #0f766e; outline-offset: 3px; }
.benefit-consent { display: flex; align-items: flex-start; gap: 10px; margin: 22px 0 16px; font-size: 13px; line-height: 1.7; cursor: pointer; }
.benefit-consent input { flex: 0 0 auto; width: 18px; height: 18px; margin-top: 2px; accent-color: #0f766e; }
.benefit-button { display: inline-flex; justify-content: center; align-items: center; gap: 8px; max-width: 100%; min-height: 44px; border: 1px solid #aabfb5; border-radius: 6px; background: #fff; padding: 10px 16px; color: #31574b; font: inherit; font-size: 14px; font-weight: 650; line-height: 1.5; overflow-wrap: anywhere; cursor: pointer; }
.benefit-button svg { flex: 0 0 16px; }
.benefit-button:hover { border-color: #0f766e; background: #eef9f4; }
.benefit-button-primary { border-color: #0f766e; background: #0f766e; color: #fff; }
.benefit-button-primary:hover { background: #115e59; }
.benefit-button:disabled { opacity: 0.6; cursor: wait; }
.benefit-error { margin: 0 0 8px; color: #b42318; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.benefit-result, .benefit-contact { margin-top: 28px; border-top: 1px solid #c7d5cf; padding-top: 22px; }
.benefit-result h3, .benefit-contact h3 { margin: 0; font-size: 17px; font-weight: 700; line-height: 1.5; }
.benefit-generated-status, .benefit-feedback { margin: 10px 0; color: #0f766e; font-size: 14px; line-height: 1.6; }
.benefit-output { min-height: 260px; background: #f5f9f7; }
.benefit-actions { display: flex; flex-wrap: wrap; gap: 10px; margin: 12px 0; }
.benefit-contact-info { white-space: pre-wrap; }
.dark .benefit-application { border-color: #38574d; color: #ecfdf5; }
.dark .benefit-header p, .dark .benefit-contact p { color: #c3d2ca; }
.dark .benefit-field label span { color: #a6bab0; }
.dark .benefit-field input, .dark .benefit-field select, .dark .benefit-field textarea, .dark .benefit-output, .dark .benefit-button { border-color: #59796b; background: #10271f; color: #ecfdf5; }
.dark .benefit-button:hover { border-color: #5eead4; background: #164535; }
.dark .benefit-button-primary { background: #0f766e; border-color: #2dd4bf; }
.dark .benefit-button-primary:hover { background: #115e59; }
.dark .benefit-result, .dark .benefit-contact { border-color: #38574d; }
.dark .benefit-generated-status, .dark .benefit-feedback { color: #5eead4; }
.dark .benefit-error { color: #fca5a5; }
.dark .benefit-field [aria-invalid="true"] { border-color: #fca5a5; }
@media (max-width: 640px) {
  .benefit-fields { grid-template-columns: minmax(0, 1fr); gap: 18px; }
  .benefit-header h2 { font-size: 21px; }
  .benefit-field input, .benefit-field select, .benefit-field textarea { font-size: 16px; }
}
</style>

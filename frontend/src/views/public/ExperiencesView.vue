<template>
  <PublicPageLayout>
    <section class="experiences-page" aria-labelledby="experiences-page-title">
      <header class="experiences-intro" data-experience-assistant>
        <p class="assistant-brand">rest2build · {{ t('experiences.kicker') }}</p>
        <h1 id="experiences-page-title">{{ t('experiences.assistantTitle') }}</h1>
        <p class="experiences-description">{{ t('experiences.assistantDescription') }}</p>
        <ol class="assistant-steps">
          <li><span>01</span>{{ t('experiences.assistantStepOne') }}</li>
          <li><span>02</span>{{ t('experiences.assistantStepTwo') }}</li>
        </ol>
        <label class="assistant-label" for="experience-problem">{{ t('experiences.assistantProblem') }}</label>
        <input id="experience-problem" v-model="problem" class="assistant-problem" :placeholder="t('experiences.assistantPlaceholder')" />
        <div class="assistant-actions">
          <button type="button" class="assistant-copy" @click="copyPrompt">
            <Icon :name="copyState === 'copied' ? 'check' : 'copy'" size="md" />
            {{ t(copyState === 'copied' ? 'experiences.assistantCopied' : 'experiences.assistantCopy') }}
          </button>
          <p>{{ t('experiences.assistantCoverage') }}</p>
        </div>
        <p class="assistant-status" role="status" aria-live="polite">{{ copyState ? t(copyState === 'copied' ? 'experiences.assistantNext' : 'experiences.assistantFallback') : t('experiences.assistantPrerequisite') }}</p>
        <details ref="promptDetails" class="assistant-details">
          <summary>{{ t('experiences.assistantPreview') }}</summary>
          <textarea ref="promptField" readonly :aria-label="t('experiences.assistantPreview')" :value="prompt"></textarea>
        </details>
      </header>

      <h2 class="experience-list-title">{{ t('experiences.indexTitle') }}</h2>
      <div class="experience-filters" role="group" :aria-label="t('experiences.filterLabel')">
        <button
          v-for="category in categories"
          :key="category.value"
          type="button"
          class="experience-filter"
          :class="{ 'is-active': selectedCategory === category.value }"
          :aria-pressed="selectedCategory === category.value"
          :data-category-filter="category.value"
          @click="selectCategory(category.value)"
        >
          {{ category.label }}
        </button>
      </div>

      <div v-if="filteredExperiences.length" class="experiences-grid">
        <ExperienceCard v-for="experience in filteredExperiences" :key="experience.id" :experience="experience" />
      </div>
      <p v-else class="experiences-empty">{{ t('experiences.empty') }}</p>
    </section>

    <template #footer>
      <Rest2BuildBrandFooter />
    </template>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import ExperienceCard from '@/components/experiences/ExperienceCard.vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { experienceCategories, experiences, type ExperienceCategory } from '@/content/experiences'
import { buildExperienceHelpPrompt } from '@/content/experienceHelp'
import { formatTutorialPrompt } from '@/utils/tutorialPrompt.mjs'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const problem = ref('')
const copyState = ref('')
const promptField = ref<HTMLTextAreaElement | null>(null)
const promptDetails = ref<HTMLDetailsElement | null>(null)
const prompt = computed(() => formatTutorialPrompt(buildExperienceHelpPrompt(experiences, problem.value), new URL('/experiences', window.location.origin).href))
watch(problem, () => { copyState.value = '' })
async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(prompt.value)
    copyState.value = 'copied'
  } catch {
    copyState.value = 'failed'
    if (promptDetails.value) promptDetails.value.open = true
    promptField.value?.focus()
    promptField.value?.select()
  }
}
function toCategory(value: unknown): 'all' | ExperienceCategory {
  return experienceCategories.some((category) => category.id === value) ? value as ExperienceCategory : 'all'
}

const selectedCategory = ref<'all' | ExperienceCategory>(toCategory(route.query.category))
const categories = computed(() => [
  { value: 'all' as const, label: t('experiences.allCategories') },
  ...experienceCategories.map((category) => ({ value: category.id, label: t(category.labelKey) })),
])
watch(() => route.query.category, (category) => {
  selectedCategory.value = toCategory(category)
})
function selectCategory(category: 'all' | ExperienceCategory) {
  selectedCategory.value = category
  const query = { ...route.query }
  if (category === 'all') delete query.category
  else query.category = category
  void router.replace({ query })
}
const filteredExperiences = computed(() => selectedCategory.value === 'all'
  ? experiences
  : experiences.filter((experience) => experience.category === selectedCategory.value))
</script>

<style scoped>
.experiences-page { min-width: 0; padding: 8px 0 0; }

.experiences-intro {
  padding: 38px 0 28px;
  border-bottom: 1px solid #bdcec8;
}

.experiences-intro > p:first-child {
  margin: 0 0 10px;
  color: #0f766e;
  font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0;
}

.experiences-intro h1 {
  margin: 0;
  overflow-wrap: anywhere;
  color: #16312d;
  font-size: 44px;
  font-weight: 800;
  line-height: 1.25;
  text-wrap: balance;
}

.dark .experiences-intro { border-color: #2dd4bf; }
.dark .experiences-intro > p:first-child { color: #5eead4; }
.dark .experiences-intro h1 { color: #f0fdfa; }

.experiences-description {
  margin: 14px 0 0;
  overflow-wrap: anywhere;
  color: #52615c;
  font-size: 16px;
  line-height: 1.7;
}

.dark .experiences-description { color: #cbd5e1; }
.assistant-steps{display:flex;flex-wrap:wrap;gap:18px 32px;margin:24px 0;padding:0;list-style:none;color:#22352f;font-size:15px;font-weight:650}.assistant-steps li{display:flex;gap:10px;align-items:center}.assistant-steps span{color:#0f766e;font:700 13px ui-monospace,monospace}.assistant-label{display:block;margin-bottom:8px;color:#45534e;font-size:13px}.assistant-problem{display:block;width:100%;max-width:760px;min-height:48px;border:1px solid #b5c8c0;border-radius:6px;background:#fff;padding:12px 14px;color:#22352f;font-size:15px}.assistant-problem:focus{outline:2px solid #0f766e;outline-offset:2px}.assistant-actions{display:flex;align-items:center;flex-wrap:wrap;gap:16px 24px;margin-top:18px}.assistant-copy{display:inline-flex;justify-content:center;align-items:center;gap:10px;min-height:54px;border:0;border-radius:6px;background:#0f766e;padding:14px 24px;color:white;font-size:17px;font-weight:750}.assistant-copy:hover{background:#115e59}.assistant-copy:focus-visible{outline:3px solid #38bdf8;outline-offset:3px}.assistant-actions p{margin:0;color:#61736b;font-size:13px}.assistant-status{min-height:24px;margin:12px 0 6px;color:#52615c;font-size:13px;line-height:1.65}.assistant-details summary{width:fit-content;cursor:pointer;color:#52615c;font-size:13px}.assistant-details textarea{display:block;width:100%;min-height:280px;margin-top:12px;resize:vertical;border:1px solid #b5c8c0;border-radius:6px;background:#fff;padding:16px;color:#26352e;font:13px/1.8 ui-monospace,monospace}.experience-list-title{margin:28px 0 0;color:#22352f;font-size:20px;font-weight:750}.dark .assistant-steps,.dark .experience-list-title{color:#e2e8f0}.dark .assistant-label,.dark .assistant-actions p,.dark .assistant-status,.dark .assistant-details summary{color:#aebfb8}.dark .assistant-problem,.dark .assistant-details textarea{background:#13201c;border-color:#466159;color:#e2e8f0}.dark .assistant-steps span{color:#5eead4}

.experience-filters { display: flex; flex-wrap: wrap; gap: 8px; margin: 30px 0 22px; }

.experience-filter {
  min-height: 36px;
  border: 1px solid #b9cbc4;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.58);
  padding: 7px 12px;
  color: #45534e;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.35;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
}

.experience-filter:hover { border-color: #0f766e; color: #0f766e; }
.experience-filter:focus-visible { outline: 2px solid #0f766e; outline-offset: 3px; }
.experience-filter.is-active { border-color: #0f766e; background: #0f766e; color: #fff; }
.dark .experience-filter { border-color: #466159; background: rgba(15, 36, 41, 0.68); color: #cbd5e1; }
.dark .experience-filter:hover { border-color: #5eead4; color: #5eead4; }
.dark .experience-filter.is-active { border-color: #2dd4bf; background: #115e59; color: #ecfdf5; }

.experiences-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }

.experiences-empty {
  margin: 0;
  border: 1px dashed #8eaaa1;
  border-radius: 8px;
  padding: 32px 16px;
  color: #64748b;
  text-align: center;
}

.dark .experiences-empty { border-color: #466159; color: #94a3b8; }

@media (max-width: 960px) { .experiences-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }

@media (max-width: 640px) {
  .experiences-page { padding-top: 0; }
  .experiences-intro { padding: 20px 0 24px; }
  .experiences-intro h1 { font-size: 32px; }
  .assistant-steps{gap:12px;margin:20px 0;font-size:14px;flex-direction:column}
  .assistant-copy{width:100%}
  .assistant-actions{gap:10px}
  .experience-filters { margin-top: 24px; }
  .experiences-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>

<template>
  <PublicPageLayout>
    <section class="experiences-page" aria-labelledby="experiences-page-title">
      <header class="experiences-intro">
        <p>{{ t('experiences.kicker') }}</p>
        <h1 id="experiences-page-title">{{ t('experiences.indexTitle') }}</h1>
        <p class="experiences-description">{{ t('experiences.indexDescription') }}</p>
      </header>

      <div class="experience-filters" role="group" :aria-label="t('experiences.filterLabel')">
        <button
          v-for="category in categories"
          :key="category.value"
          type="button"
          class="experience-filter"
          :class="{ 'is-active': selectedCategory === category.value }"
          :aria-pressed="selectedCategory === category.value"
          @click="selectedCategory = category.value"
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
import { useRoute } from 'vue-router'
import ExperienceCard from '@/components/experiences/ExperienceCard.vue'
import Rest2BuildBrandFooter from '@/components/common/Rest2BuildBrandFooter.vue'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import { experienceCategories, experiences, type ExperienceCategory } from '@/content/experiences'

const { t } = useI18n()
const route = useRoute()
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
const filteredExperiences = computed(() => selectedCategory.value === 'all'
  ? experiences
  : experiences.filter((experience) => experience.category === selectedCategory.value))
</script>

<style scoped>
.experiences-page { padding: 32px; border: 1px solid rgba(15, 23, 42, 0.1); border-radius: 8px; background: rgba(255, 255, 255, 0.8); box-shadow: 0 20px 40px -38px rgba(15, 23, 42, 0.45); }.dark .experiences-page { border-color: rgba(255, 255, 255, 0.12); background: rgba(10, 18, 32, 0.7); }
.experiences-intro > p:first-child { margin: 0 0 10px; color: #0f766e; font: 600 12px ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; }.experiences-intro h1 { margin: 0; color: #1f2937; font-size: 32px; line-height: 1.35; }.dark .experiences-intro h1 { color: #f8fafc; }.experiences-description { margin: 14px 0 0; color: #52615c; font-size: 16px; line-height: 1.7; }.dark .experiences-description { color: #cbd5e1; }
.experience-filters { display: flex; flex-wrap: wrap; gap: 8px; margin: 28px 0 20px; }.experience-filter { min-height: 36px; border: 1px solid #b9cbc4; border-radius: 6px; background: transparent; padding: 7px 12px; color: #45534e; font-size: 14px; font-weight: 600; }.experience-filter:hover { border-color: #0f766e; color: #0f766e; }.experience-filter.is-active { border-color: #0f766e; background: #0f766e; color: #fff; }.dark .experience-filter { border-color: #466159; color: #cbd5e1; }.dark .experience-filter.is-active { border-color: #2dd4bf; background: #115e59; color: #ecfdf5; }
.experiences-grid { display: grid; gap: 14px; }.experiences-empty { margin: 0; border: 1px dashed #b9cbc4; padding: 32px 16px; color: #64748b; text-align: center; }.dark .experiences-empty { border-color: #466159; color: #94a3b8; }
@media (max-width: 640px) { .experiences-page { padding: 24px 20px; }.experiences-intro h1 { font-size: 27px; } }
</style>

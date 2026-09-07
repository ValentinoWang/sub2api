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

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
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
  max-width: 760px;
  padding-left: 16px;
  border-left: 3px solid #14b8a6;
}

.experiences-intro > p:first-child {
  margin: 0 0 10px;
  color: #0f766e;
  font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.experiences-intro h1 {
  margin: 0;
  overflow-wrap: anywhere;
  color: #16312d;
  font-size: 34px;
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
  .experiences-intro { padding-left: 12px; }
  .experiences-intro h1 { font-size: 27px; }
  .experience-filters { margin-top: 24px; }
  .experiences-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>

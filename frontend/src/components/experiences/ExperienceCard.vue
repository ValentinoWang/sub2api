<template>
  <article class="experience-card">
    <div class="experience-card-meta">
      <span class="experience-card-category">{{ t(`experiences.categories.${experience.category}`) }}</span>
      <span>{{ experience.series }}</span>
      <time :datetime="experience.updatedAt">{{ t('experiences.updated', { date: experience.updatedAt }) }}</time>
    </div>
    <h3>{{ experience.title }}</h3>
    <p class="experience-card-summary">{{ experience.summary }}</p>
    <p class="experience-card-audience">{{ experience.applicableTo }}</p>
    <router-link :to="experience.route" class="experience-card-link">
      {{ t('experiences.readExperience') }}
      <Icon name="arrowRight" size="sm" />
    </router-link>
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ExperienceContent } from '@/content/experiences'

defineProps<{ experience: ExperienceContent }>()

const { t } = useI18n()
</script>

<style scoped>
.experience-card { display: flex; min-width: 0; flex-direction: column; align-items: flex-start; border: 1px solid rgba(15, 23, 42, 0.1); border-radius: 8px; background: rgba(255, 255, 255, 0.82); padding: 22px; box-shadow: 0 12px 25px -24px rgba(15, 23, 42, 0.48); }
.dark .experience-card { border-color: rgba(255, 255, 255, 0.12); background: rgba(10, 18, 32, 0.68); }
.experience-card-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; color: #64748b; font-size: 12px; }
.experience-card-category { color: #0f766e; font-weight: 650; }
.dark .experience-card-category { color: #5eead4; }.dark .experience-card-meta { color: #94a3b8; }
.experience-card h3 { margin: 12px 0 0; color: #1f2937; font-size: 18px; line-height: 1.45; font-weight: 700; }.dark .experience-card h3 { color: #f8fafc; }
.experience-card-summary { margin: 10px 0 0; color: #52615c; font-size: 14px; line-height: 1.7; }.dark .experience-card-summary { color: #cbd5e1; }
.experience-card-audience { margin: 14px 0 0; color: #64748b; font-size: 12px; line-height: 1.5; }.dark .experience-card-audience { color: #94a3b8; }
.experience-card-link { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; margin-top: 18px; color: #0f766e; font-size: 14px; font-weight: 650; text-decoration: none; }.experience-card-link:hover { color: #0b5d57; text-decoration: underline; text-underline-offset: 4px; }.dark .experience-card-link { color: #5eead4; }
</style>

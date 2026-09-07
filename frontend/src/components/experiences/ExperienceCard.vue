<template>
  <article class="experience-card" :data-experience-id="experience.id">
    <header class="experience-card-header">
      <span class="experience-card-icon" aria-hidden="true">
        <Icon :name="experience.icon" size="md" />
      </span>
      <div class="experience-card-meta">
        <span class="experience-card-topic">{{ experience.series }}</span>
        <span class="experience-card-category">{{ t(`experiences.categories.${experience.category}`) }}</span>
      </div>
      <time :datetime="experience.updatedAt">{{ t('experiences.updated', { date: experience.updatedAt }) }}</time>
    </header>
    <h3 class="experience-card-title">{{ experience.title }}</h3>
    <p class="experience-card-summary">{{ experience.summary }}</p>
    <p class="experience-card-audience">
      <span class="experience-card-audience-label">{{ t('experiences.appliesTo') }}</span>
      <span>{{ experience.applicableTo }}</span>
    </p>
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
.experience-card {
  display: flex;
  min-width: 0;
  min-height: 276px;
  flex-direction: column;
  border: 1px solid rgba(15, 118, 110, 0.18);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.86);
  padding: 20px;
  box-shadow: 0 16px 32px -28px rgba(15, 118, 110, 0.52);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.experience-card:hover {
  transform: translateY(-2px);
  border-color: rgba(13, 148, 136, 0.52);
  box-shadow: 0 20px 32px -24px rgba(13, 148, 136, 0.38);
}

.experience-card:focus-within {
  border-color: #0f766e;
  box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.18);
}

.dark .experience-card {
  border-color: rgba(94, 234, 212, 0.2);
  background: rgba(10, 28, 35, 0.88);
  box-shadow: 0 16px 32px -28px rgba(2, 132, 199, 0.64);
}

.dark .experience-card:hover { border-color: rgba(94, 234, 212, 0.52); }

.experience-card-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.experience-card-icon {
  display: inline-flex;
  width: 36px;
  height: 36px;
  flex: none;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(13, 148, 136, 0.22);
  border-radius: 8px;
  color: #0f766e;
  background: rgba(20, 184, 166, 0.1);
}

.dark .experience-card-icon {
  border-color: rgba(94, 234, 212, 0.24);
  color: #5eead4;
  background: rgba(45, 212, 191, 0.12);
}

.experience-card-meta { display: grid; min-width: 0; gap: 2px; }

.experience-card-topic {
  overflow-wrap: anywhere;
  color: #0f766e;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.experience-card-category {
  overflow-wrap: anywhere;
  color: #64748b;
  font-size: 12px;
  line-height: 1.35;
}

.experience-card time {
  align-self: start;
  color: #64748b;
  font-size: 11px;
  line-height: 1.35;
  text-align: right;
  white-space: nowrap;
}

.dark .experience-card-topic { color: #5eead4; }
.dark .experience-card-category,
.dark .experience-card time { color: #94a3b8; }

.experience-card-title {
  margin: 18px 0 0;
  overflow-wrap: anywhere;
  color: #16312d;
  font-size: 20px;
  font-weight: 750;
  line-height: 1.35;
}

.dark .experience-card-title { color: #f0fdfa; }

.experience-card-summary {
  margin: 10px 0 0;
  overflow-wrap: anywhere;
  color: #52615c;
  font-size: 14px;
  line-height: 1.7;
}

.dark .experience-card-summary { color: #cbd5e1; }

.experience-card-audience {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 6px;
  margin: 16px 0 0;
  padding-top: 12px;
  border-top: 1px solid rgba(15, 118, 110, 0.12);
  color: #64748b;
  font-size: 12px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.experience-card-audience-label {
  color: #0f766e;
  font-weight: 700;
}

.dark .experience-card-audience { border-color: rgba(94, 234, 212, 0.16); color: #94a3b8; }
.dark .experience-card-audience-label { color: #5eead4; }

.experience-card-link {
  display: inline-flex;
  width: fit-content;
  min-height: 36px;
  align-items: center;
  gap: 6px;
  margin-top: auto;
  padding-top: 16px;
  color: #0f766e;
  font-size: 14px;
  font-weight: 700;
  text-decoration: none;
}

.experience-card-link:hover { color: #0b5d57; text-decoration: underline; text-underline-offset: 4px; }
.experience-card-link:focus-visible { outline: 2px solid #0f766e; outline-offset: 3px; border-radius: 4px; }
.dark .experience-card-link { color: #5eead4; }

@media (max-width: 640px) {
  .experience-card { min-height: 0; padding: 18px; }
  .experience-card-title { font-size: 19px; }
}
</style>

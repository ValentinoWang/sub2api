<template>
  <section class="experience-collection" :class="{ 'experience-collection-compact': compact }" aria-labelledby="experience-sharing-title">
    <div class="experience-collection-heading">
      <div>
        <p class="experience-collection-kicker">{{ t('experiences.kicker') }}</p>
        <h2 id="experience-sharing-title">{{ t('experiences.featuredTitle') }}</h2>
        <p v-if="!compact" class="experience-collection-description">{{ t('experiences.featuredDescription') }}</p>
      </div>
      <router-link v-if="showBrowseLink" to="/experiences" class="experience-browse-link">
        {{ t('experiences.browseAll') }}
        <Icon name="arrowRight" size="sm" />
      </router-link>
    </div>

    <div class="experience-grid">
      <ExperienceCard v-for="experience in items" :key="experience.id" :experience="experience" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ExperienceCard from './ExperienceCard.vue'
import Icon from '@/components/icons/Icon.vue'
import { experiences } from '@/content/experiences'

const props = withDefaults(defineProps<{
  compact?: boolean
  showBrowseLink?: boolean
  limit?: number
}>(), {
  compact: false,
  showBrowseLink: true,
  limit: 1,
})

const { t } = useI18n()
const items = computed(() => experiences.slice(0, props.limit))
</script>

<style scoped>
.experience-collection { padding: 32px; border: 1px solid rgba(15, 118, 110, 0.18); border-radius: 8px; background: rgba(240, 253, 250, 0.62); }
.dark .experience-collection { border-color: rgba(45, 212, 191, 0.2); background: rgba(15, 35, 33, 0.5); }
.experience-collection-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 20px; }
.experience-collection-kicker { margin: 0 0 8px; color: #0f766e; font: 600 12px ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; }
.experience-collection h2 { margin: 0; color: #16312d; font-size: 24px; line-height: 1.3; font-weight: 700; }
.dark .experience-collection h2 { color: #ecfdf5; }
.experience-collection-description { max-width: 680px; margin: 8px 0 0; color: #52615c; font-size: 14px; line-height: 1.65; }
.dark .experience-collection-description { color: #cbd5e1; }
.experience-browse-link { display: inline-flex; flex: none; align-items: center; gap: 6px; color: #0f766e; font-size: 14px; font-weight: 650; text-decoration: none; }
.experience-browse-link:hover { color: #0b5d57; text-decoration: underline; text-underline-offset: 4px; }
.dark .experience-browse-link { color: #5eead4; }
.experience-grid { display: grid; gap: 14px; }
.experience-collection-compact { padding: 20px; }
.experience-collection-compact .experience-collection-heading { margin-bottom: 14px; }
.experience-collection-compact h2 { font-size: 18px; }
@media (max-width: 640px) { .experience-collection { padding: 22px 18px; }.experience-collection-heading { align-items: flex-start; flex-direction: column; gap: 12px; }.experience-browse-link { min-height: 32px; }.experience-collection h2 { font-size: 21px; } }
</style>

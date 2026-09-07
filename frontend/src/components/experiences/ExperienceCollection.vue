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
.experience-collection { min-width: 0; padding: 0; }
.experience-collection-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 20px; }
.experience-collection-kicker { margin: 0 0 8px; color: #0f766e; font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.06em; text-transform: uppercase; }
.experience-collection h2 { margin: 0; overflow-wrap: anywhere; color: #16312d; font-size: 24px; line-height: 1.3; font-weight: 750; text-wrap: balance; }
.dark .experience-collection h2 { color: #ecfdf5; }
.experience-collection-description { max-width: 680px; margin: 8px 0 0; overflow-wrap: anywhere; color: #52615c; font-size: 14px; line-height: 1.65; }
.dark .experience-collection-description { color: #cbd5e1; }
.experience-browse-link { display: inline-flex; flex: none; min-height: 36px; align-items: center; gap: 6px; color: #0f766e; font-size: 14px; font-weight: 700; text-decoration: none; }
.experience-browse-link:hover { color: #0b5d57; text-decoration: underline; text-underline-offset: 4px; }
.experience-browse-link:focus-visible { outline: 2px solid #0f766e; outline-offset: 3px; border-radius: 4px; }
.dark .experience-browse-link { color: #5eead4; }
.experience-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.experience-collection-compact { padding: 18px; }
.experience-collection-compact .experience-collection-heading { margin-bottom: 14px; }
.experience-collection-compact h2 { font-size: 18px; }
@media (max-width: 640px) { .experience-collection-heading { align-items: flex-start; flex-direction: column; gap: 8px; }.experience-collection h2 { font-size: 21px; }.experience-grid { grid-template-columns: minmax(0, 1fr); }.experience-collection-compact { padding: 16px; } }
</style>

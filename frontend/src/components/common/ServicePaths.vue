<template>
  <section id="service-paths" class="service-paths" aria-labelledby="service-paths-title">
    <h2 id="service-paths-title">{{ copy.title }}</h2>
    <p class="section-description">{{ copy.description }}</p>
    <div class="service-lanes">
      <article v-for="lane in copy.lanes" :key="lane.id" class="service-lane" :class="`service-lane--${lane.id}`">
        <div class="lane-heading">
          <Icon :name="lane.icon" size="lg" />
          <div><h3>{{ lane.title }}</h3><p>{{ lane.subtitle }}</p></div>
        </div>
        <ol class="lane-steps">
          <li v-for="(step, index) in lane.steps" :key="step">
            <span class="step-number">{{ index + 1 }}</span>
            <span>{{ step }}</span>
            <Icon v-if="index < lane.steps.length - 1" name="arrowRight" size="sm" class="step-arrow" aria-hidden="true" />
          </li>
        </ol>
        <router-link v-if="lane.href.startsWith('/')" :to="lane.href" class="service-link">{{ lane.action }}<Icon name="arrowRight" size="sm" /></router-link>
        <a v-else :href="lane.href" class="service-link">{{ lane.action }}<Icon name="arrowRight" size="sm" /></a>
      </article>
    </div>
    <p class="service-note">{{ copy.note }}</p>
    <h2 class="practice-title">{{ copy.practiceTitle }}</h2>
    <div class="practice-resources">
      <article v-for="resource in copy.resources" :key="resource.href" class="practice-resource">
        <Icon :name="resource.icon" size="lg" />
        <h3>{{ resource.title }}</h3>
        <p>{{ resource.description }}</p>
        <router-link :to="resource.href" class="service-link">{{ resource.action }}<Icon name="arrowRight" size="sm" /></router-link>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { servicePaths } from '@/content/servicePaths'

const { locale } = useI18n()
const copy = computed(() => servicePaths[locale.value === 'en' ? 'en' : 'zh'])
</script>

<style scoped>
.service-paths { min-width: 0; color: #203b32; letter-spacing: 0; }
h2 { margin: 0; font-size: 26px; font-weight: 750; line-height: 1.4; }
.section-description { margin: 12px 0 28px; color: #596960; font-size: 15px; line-height: 1.75; }
.service-lane { display: grid; grid-template-columns: 230px minmax(0, 1fr); align-items: center; gap: 20px 32px; padding: 26px 0; border-top: 1px solid #cbded3; }
.lane-heading { display: flex; align-items: center; gap: 14px; color: #146d52; }
h3 { margin: 0; font-size: 18px; font-weight: 700; line-height: 1.5; }
.lane-heading p { margin: 4px 0 0; color: #596960; font-size: 13px; line-height: 1.6; }
.lane-steps { list-style: none; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); padding: 0; margin: 0; gap: 20px; }
.lane-steps li { display: flex; align-items: center; gap: 10px; min-width: 0; font-size: 14px; line-height: 1.65; overflow-wrap: anywhere; }
.step-number { display: grid; place-items: center; flex: none; width: 28px; height: 28px; border: 1px solid #b7d8c8; border-radius: 50%; color: #146d52; font-size: 12px; font-weight: 700; }
.step-arrow { flex: none; margin-left: auto; color: #719b89; }
.service-link { display: inline-flex; align-items: center; gap: 8px; width: fit-content; max-width: 100%; min-height: 40px; font-size: 14px; line-height: 1.6; color: #146d52; font-weight: 700; text-decoration: none; }
.service-link:hover { text-decoration: underline; text-underline-offset: 4px; }
.service-link:focus-visible { outline: 2px solid currentColor; outline-offset: 4px; }
.service-lane > .service-link { grid-column: 2; }
.service-lane--account .lane-heading, .service-lane--account .service-link { color: #365aab; }
.service-lane--account .step-number { color: #365aab; border-color: #bdcce8; }
.service-note { border-bottom: 1px solid #cbded3; padding-bottom: 24px; margin: 0; font-size: 13px; line-height: 1.8; color: #596960; }
.practice-title { margin-top: 40px; font-size: 22px; }
.practice-resources { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 28px; margin-top: 24px; }
.practice-resource { display: flex; min-width: 0; flex-direction: column; align-items: flex-start; }
.practice-resource > svg { color: #146d52; }
.practice-resource h3 { margin-top: 14px; }
.practice-resource p { flex: 1; margin: 10px 0; font-size: 14px; line-height: 1.8; color: #596960; }
.dark .service-paths { color: #e3ede6; }
.dark .section-description, .dark .lane-heading p, .dark .service-note, .dark .practice-resource p { color: #b6c6bc; }
.dark .service-lane, .dark .service-note { border-color: #345344; }
.dark .lane-heading, .dark .service-link, .dark .practice-resource > svg { color: #8ed7b2; }
.dark .step-number { color: #8ed7b2; border-color: #406855; }
.dark .service-lane--account .lane-heading, .dark .service-lane--account .service-link, .dark .service-lane--account .step-number { color: #a9c2ff; }
.dark .service-lane--account .step-number { border-color: #455c8b; }
@media (max-width: 850px) {
  .service-lane { grid-template-columns: minmax(0, 1fr); gap: 20px; }
  .service-lane > .service-link { grid-column: 1; }
}
@media (max-width: 600px) {
  h2 { font-size: 22px; }
  .lane-steps { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .step-arrow { display: none; }
  .practice-resources { grid-template-columns: minmax(0, 1fr); }
  .practice-resource { padding-bottom: 20px; border-bottom: 1px solid #cbded3; }
  .dark .practice-resource { border-color: #345344; }
}
</style>

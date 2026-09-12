<template>
  <div
    class="app-layout min-h-screen bg-gray-50 dark:bg-dark-950"
    :class="{ 'user-brand-shell': usesUserBrandShell }"
  >
    <div aria-hidden="true" class="app-layout-backdrop pointer-events-none fixed inset-0"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="app-layout-main relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="app-layout-content p-4 md:p-6 lg:p-8">
        <div class="app-layout-page">
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const usesUserBrandShell = computed(
  () => (route.meta.requiresAuth === true || authStore.isAuthenticated) && route.meta.requiresAdmin !== true
)

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
/* Authenticated user routes inherit the public home palette. */
.app-layout.user-brand-shell {
  --user-canvas: #f6f9fc;
  --user-grid-line: rgba(15, 23, 42, 0.06);
  --user-surface: rgba(255, 255, 255, 0.72);
  --user-border: rgba(15, 23, 42, 0.08);
  --user-foreground: #0f172a;
  --user-muted: #64748b;
  --user-hover: rgba(15, 23, 42, 0.06);
  --user-content-viewport-height: calc(100vh - 64px - 4rem);
  --dashboard-surface: var(--user-surface);
  --dashboard-border: var(--user-border);
  --dashboard-hover: var(--user-hover);
  --dashboard-accent: #0f766e;
  min-width: 0;
  background:
    radial-gradient(1200px 600px at 50% -10%, rgba(20, 184, 166, 0.14), transparent 60%),
    var(--user-canvas);
  color: var(--user-foreground);
}

:global(.dark) .app-layout.user-brand-shell {
  --user-canvas: #050b14;
  --user-grid-line: rgba(148, 163, 184, 0.08);
  --user-surface: rgba(10, 18, 32, 0.6);
  --user-border: rgba(148, 163, 184, 0.12);
  --user-foreground: #f8fafc;
  --user-muted: #94a3b8;
  --user-hover: rgba(148, 163, 184, 0.1);
  --dashboard-accent: #5eead4;
  background:
    radial-gradient(1200px 600px at 50% -10%, rgba(20, 184, 166, 0.18), transparent 60%),
    var(--user-canvas);
}

.app-layout-backdrop {
  display: none;
}

.user-brand-shell .app-layout-backdrop {
  display: block;
  background-image:
    linear-gradient(var(--user-grid-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--user-grid-line) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(ellipse 80% 70% at 50% 20%, #000 30%, transparent 100%);
  -webkit-mask-image: radial-gradient(ellipse 80% 70% at 50% 20%, #000 30%, transparent 100%);
}

.app-layout-main,
.app-layout-content,
.app-layout-page {
  min-width: 0;
}

.user-brand-shell .app-layout-page {
  width: min(100%, 80rem);
  margin-inline: auto;
}

.user-brand-shell :deep(.glass),
.user-brand-shell :deep(.sidebar),
.user-brand-shell :deep(.dropdown),
.user-brand-shell :deep(.input),
.user-brand-shell :deep(.table-container),
.user-brand-shell :deep(.table-scroll-container),
.user-brand-shell :deep(.btn-secondary) {
  background-color: var(--user-surface);
  border-color: var(--user-border);
}

.user-brand-shell :deep(.card:not([class*=' bg-'])) {
  background-color: var(--user-surface);
  border-color: var(--user-border);
}

.user-brand-shell :deep(.sidebar-header),
.user-brand-shell :deep(.glass),
.user-brand-shell :deep(.card-header),
.user-brand-shell :deep(.card-footer) {
  border-color: var(--user-border);
}

.user-brand-shell :deep(.sidebar-link:not(.sidebar-link-active)) {
  color: var(--user-muted);
}

.user-brand-shell :deep(.sidebar-link:not(.sidebar-link-active):hover) {
  background-color: var(--user-hover);
  color: var(--user-foreground);
}

.user-brand-shell :deep(.page-title),
.user-brand-shell :deep(.dashboard-section-heading) {
  color: var(--user-foreground);
}

.user-brand-shell :deep(.page-description) {
  color: var(--user-muted);
}

@media (max-width: 639px) {
  .user-brand-shell .app-layout-content {
    padding: 1rem 0.875rem 2rem;
  }
}
</style>

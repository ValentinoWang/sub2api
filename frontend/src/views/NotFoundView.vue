<template>
  <div
    class="nf-root relative flex min-h-screen items-center justify-center overflow-hidden px-4"
  >
    <!-- Background Decoration -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <div class="nf-aurora nf-aurora-1"></div>
      <div class="nf-aurora nf-aurora-2"></div>
      <div class="nf-grid"></div>
    </div>

    <div class="relative z-10 w-full max-w-md text-center">
      <!-- 404 Display -->
      <div class="mb-8">
        <div class="relative inline-block">
          <span class="nf-code text-[12rem] font-extrabold leading-none tracking-tight">404</span>
          <div class="absolute inset-0 flex items-center justify-center">
            <div
              class="nf-badge flex h-24 w-24 items-center justify-center rounded-2xl"
            >
              <svg
                class="h-12 w-12 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.5"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
                />
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Text Content -->
      <div class="mb-8">
        <h1 class="mb-3 text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('errors.pageNotFound') }}
        </h1>
        <p class="text-gray-500 dark:text-dark-400">
          The page you are looking for doesn't exist or has been moved.
        </p>
      </div>

      <!-- Action Buttons -->
      <div class="flex flex-col justify-center gap-3 sm:flex-row">
        <button @click="goBack" class="btn btn-secondary">
          <Icon name="arrowLeft" size="md" class="mr-2" />
          Go Back
        </button>
        <router-link to="/dashboard" class="btn btn-primary">
          <Icon name="home" size="md" class="mr-2" />
          Go to Dashboard
        </router-link>
      </div>

      <!-- Help Link -->
      <p class="mt-8 text-sm text-gray-400 dark:text-dark-500">
        Need help?
        <a
          href="#"
          class="text-primary-600 transition-colors hover:text-primary-500 dark:text-primary-400 dark:hover:text-primary-300"
        >
          Contact support
        </a>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()

function goBack(): void {
  router.back()
}
</script>

<style scoped>
.nf-root {
  --nf-line: rgba(15, 23, 42, 0.06);
  background:
    radial-gradient(900px 450px at 50% -10%, rgba(20, 184, 166, 0.14), transparent 60%),
    #f6f9fc;
}
.dark .nf-root {
  --nf-line: rgba(148, 163, 184, 0.08);
  background:
    radial-gradient(900px 450px at 50% -10%, rgba(20, 184, 166, 0.18), transparent 60%),
    #050b14;
}
.nf-aurora {
  position: absolute;
  border-radius: 9999px;
  filter: blur(90px);
  opacity: 0.5;
}
.nf-aurora-1 {
  top: -200px;
  right: -160px;
  width: 520px;
  height: 520px;
  background: radial-gradient(circle at 30% 30%, rgba(20, 184, 166, 0.5), rgba(34, 211, 238, 0.2) 45%, transparent 70%);
}
.nf-aurora-2 {
  bottom: -220px;
  left: -180px;
  width: 540px;
  height: 540px;
  background: radial-gradient(circle at 60% 40%, rgba(99, 102, 241, 0.35), transparent 70%);
}
.nf-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--nf-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--nf-line) 1px, transparent 1px);
  background-size: 56px 56px;
  -webkit-mask-image: radial-gradient(ellipse 80% 75% at 50% 45%, #000 25%, transparent 100%);
  mask-image: radial-gradient(ellipse 80% 75% at 50% 45%, #000 25%, transparent 100%);
}
.nf-code {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.12), rgba(15, 23, 42, 0.02));
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.dark .nf-code {
  background-image: linear-gradient(180deg, rgba(148, 163, 184, 0.28), rgba(148, 163, 184, 0.04));
}
.nf-badge {
  background: linear-gradient(135deg, #14b8a6 0%, #0891b2 60%, #4f46e5 140%);
  box-shadow:
    0 0 0 8px rgba(20, 184, 166, 0.1),
    0 20px 50px -16px rgba(20, 184, 166, 0.8);
}
</style>

<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="home-root relative flex min-h-screen flex-col overflow-hidden text-gray-900 dark:text-white"
  >
    <div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <div class="home-aurora home-aurora-1"></div>
      <div class="home-aurora home-aurora-2"></div>
      <div class="home-grid"></div>
    </div>

    <header class="relative z-20 px-4 py-4 sm:px-6">
      <nav class="home-nav mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 rounded-2xl px-3 py-2 sm:gap-4 sm:px-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold"><BrandWordmark :name="siteName" /></span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="home-nav-btn flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="home-nav-btn flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="home-nav-btn flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="home-cta-primary inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg px-4 py-2 text-sm font-semibold"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <div class="home-logo-halo mx-auto mb-6 h-20 w-20">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-20 w-20 rounded-2xl object-contain"
          />
        </div>
        <p class="home-domain mb-3">{{ BRAND_DOMAIN }}</p>
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold tracking-tight md:text-4xl"><BrandWordmark :name="siteName" /></h1>
        <p class="mt-3 text-lg font-semibold text-gray-800 dark:text-gray-100">{{ t('home.meme.tagline') }}</p>
        <p class="mt-2 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="home-cta-primary mt-8 inline-flex min-h-10 items-center justify-center rounded-lg px-5 py-2.5 text-sm font-semibold"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="relative z-10 min-w-0 border-t border-gray-200/60 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-white/5 dark:text-dark-400">
      &copy; {{ currentYear }} {{ BRAND_DOMAIN }} · {{ t('home.meme.footer') }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="home-root relative flex min-h-screen flex-col overflow-hidden text-gray-900 dark:text-white"
  >
    <!-- Background Layers -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <div class="home-aurora home-aurora-1"></div>
      <div class="home-aurora home-aurora-2"></div>
      <div class="home-aurora home-aurora-3"></div>
      <div class="home-grid"></div>
      <div class="home-beam"></div>
      <div class="home-particles">
        <span
          v-for="particle in particles"
          :key="particle.id"
          class="home-particle"
          :style="particle.style"
        ></span>
      </div>
    </div>

    <!-- Header -->
    <header class="sticky top-0 z-30 px-4 pt-4 sm:px-6">
      <nav
        class="home-nav mx-auto flex max-w-6xl items-center justify-between gap-3 rounded-2xl px-3 py-2 sm:px-4"
      >
        <!-- Logo -->
        <div class="flex min-w-0 items-center gap-3">
          <div class="home-logo-halo h-9 w-9 shrink-0">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-9 w-9 rounded-xl object-contain" />
          </div>
          <span class="hidden min-w-0 truncate text-base font-semibold tracking-tight sm:inline"><BrandWordmark :name="siteName" /></span>
          <span class="home-mode-pill hidden md:inline-flex" :title="t('home.meme.tagline')">
            <span class="home-mode-seg home-mode-rest"><Icon name="moon" size="xs" />{{ t('home.meme.youRest') }}</span>
            <span class="home-mode-seg home-mode-build"><span class="home-pulse-dot"></span>{{ t('home.meme.aiBuild') }}</span>
          </span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-1.5 sm:gap-2">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="home-nav-btn rounded-lg p-2"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Model Plaza Link -->
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="home-nav-btn inline-flex items-center gap-1.5 rounded-lg p-2 text-sm"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="home-nav-btn rounded-lg p-2"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="home-cta-primary inline-flex items-center gap-1.5 rounded-full py-1 pl-1 pr-3"
          >
            <span
              class="flex h-6 w-6 items-center justify-center rounded-full bg-white/20 text-[11px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-semibold">{{ t('home.dashboard') }}</span>
            <Icon name="arrowRight" size="xs" :stroke-width="2" />
          </router-link>
          <router-link
            v-else
            to="/login"
            class="home-cta-primary inline-flex items-center rounded-full px-4 py-1.5 text-xs font-semibold"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-4 pb-16 pt-12 sm:px-6 lg:pt-20">
      <div class="mx-auto max-w-6xl">
        <!-- Hero Section -->
        <section class="mb-16 grid min-w-0 grid-cols-1 items-center gap-12 lg:mb-20 lg:grid-cols-[1fr_1.05fr] lg:gap-8">
          <!-- Left: Text Content -->
          <div class="min-w-0 text-center lg:text-left" data-reveal>
            <div class="home-eyebrow mb-6">
              <span class="home-pulse-dot"></span>
              <span class="font-mono tracking-wide">{{ BRAND_DOMAIN }}</span>
              <span class="home-eyebrow-sep"></span>
              <span class="font-mono text-[11px] uppercase tracking-[0.18em] opacity-70">{{ t('home.sell.kicker') }}</span>
            </div>
            <p class="home-positioning mb-3">{{ t('marketing.positioning') }} · {{ t('marketing.nonOfficialShort') }}</p>

            <h1
              class="home-hero-title mb-4 text-5xl font-extrabold leading-[1.05] tracking-tight md:text-6xl lg:text-7xl"
            >
              <BrandWordmark :name="siteName" />
            </h1>
            <p class="mb-3 text-2xl font-bold text-gray-900 dark:text-white md:text-3xl">
              {{ t('home.meme.tagline') }}
            </p>
            <p class="mx-auto mb-7 max-w-xl text-base leading-relaxed text-gray-600 dark:text-dark-300 md:text-lg lg:mx-0">
              {{ t('home.meme.taglineSub') }}
            </p>

            <!-- CTA Buttons -->
            <div class="mb-8 flex flex-col items-center gap-3 sm:flex-row sm:justify-center lg:justify-start">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="home-cta-primary home-cta-lg inline-flex items-center justify-center gap-2 rounded-xl px-7 py-3.5 text-base font-semibold"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="md" :stroke-width="2" />
              </router-link>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="home-cta-secondary inline-flex items-center justify-center gap-2 rounded-xl px-6 py-3.5 text-base font-medium"
              >
                <Icon name="book" size="md" />
                {{ t('home.viewDocs') }}
              </a>
              <router-link
                v-else-if="showModelPlazaEntry"
                to="/model-plaza"
                class="home-cta-secondary inline-flex items-center justify-center gap-2 rounded-xl px-6 py-3.5 text-base font-medium"
              >
                <Icon name="grid" size="md" />
                {{ t('nav.modelPlaza') }}
              </router-link>
            </div>

            <!-- API base URL card -->
            <div class="home-address text-left">
              <div class="home-address-head">
                <span class="home-address-title">{{ t('home.address.title') }}</span>
                <span class="home-address-sub">{{ siteSubtitle }}</span>
              </div>
              <div class="home-address-row">
                <code class="home-address-url">{{ apiBaseUrl }}</code>
                <button type="button" class="home-address-copy" :class="{ 'is-copied': copied }" @click="copyBaseUrl">
                  <Icon :name="copied ? 'check' : 'copy'" size="sm" />
                  {{ copied ? t('home.address.copied') : t('home.address.copy') }}
                </button>
              </div>
              <p class="home-address-hint">{{ t('home.address.hint') }}</p>
              <div class="mt-3 flex flex-wrap items-center gap-2">
                <span class="home-endpoint"><i class="home-endpoint-method">POST</i>/v1/messages</span>
                <span class="home-endpoint"><i class="home-endpoint-method">POST</i>/v1/chat/completions</span>
                <span class="home-endpoint"><i class="home-endpoint-method">POST</i>/v1/responses</span>
                <span class="home-endpoint"><i class="home-endpoint-method">GET</i>/v1/models</span>
              </div>
            </div>
          </div>

          <!-- Right: Key visual -->
          <div class="flex min-w-0 justify-center lg:justify-end" data-reveal data-reveal-delay="1">
            <RelayStationVisual
              :left-label="t('home.station.you')"
              :core-label="t('home.station.core')"
              :latency-title="t('home.station.latencyLive')"
              :latency-probing="t('home.station.probing')"
              :latency-unavailable="t('home.station.unavailable')"
              :latency-ms="latencyMs"
              :latency-state="latencyState"
              :stamp-text="t('home.station.stamp')"
            />
          </div>
        </section>

        <!-- Selling points -->
        <section class="mb-20 grid gap-4 sm:grid-cols-2 lg:grid-cols-4" data-reveal>
          <div class="home-sell">
            <div class="home-sell-icon" style="--icon-from: #2dd4bf; --icon-to: #0891b2"><Icon name="bolt" size="md" class="text-white" /></div>
            <div class="home-sell-body">
              <p class="home-sell-title">{{ t('home.sell.latency') }}<span v-if="latencyState === 'ok' && latencyMs !== null" class="home-sell-live">{{ latencyMs }} ms</span></p>
              <p class="home-sell-desc">{{ t('home.sell.latencyDesc') }}</p>
            </div>
          </div>
          <div class="home-sell">
            <div class="home-sell-icon" style="--icon-from: #fb7185; --icon-to: #e11d48"><Icon name="shield" size="md" class="text-white" /></div>
            <div class="home-sell-body">
              <p class="home-sell-title">{{ t('home.sell.stable') }}</p>
              <p class="home-sell-desc">{{ t('home.sell.stableDesc') }}</p>
            </div>
          </div>
          <div class="home-sell">
            <div class="home-sell-icon" style="--icon-from: #60a5fa; --icon-to: #4f46e5"><Icon name="swap" size="md" class="text-white" /></div>
            <div class="home-sell-body">
              <p class="home-sell-title">{{ t('home.sell.relay') }}</p>
              <p class="home-sell-desc">{{ t('home.sell.relayDesc') }}</p>
            </div>
          </div>
          <div class="home-sell">
            <div class="home-sell-icon" style="--icon-from: #a78bfa; --icon-to: #7c3aed"><Icon name="calculator" size="md" class="text-white" /></div>
            <div class="home-sell-body">
              <p class="home-sell-title">{{ t('home.sell.billing') }}</p>
              <p class="home-sell-desc">{{ t('home.sell.billingDesc') }}</p>
            </div>
          </div>
        </section>

        <!-- Two access modes -->
        <section class="mb-20">
          <div class="mb-8 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('marketing.modes.kicker') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">{{ t('marketing.modes.title') }}</h2>
          </div>
          <div class="grid gap-5 md:grid-cols-2">
            <div class="home-mode" data-reveal>
              <div class="home-mode-head">
                <span class="home-mode-badge">MANAGED</span>
                <h3 class="text-lg font-bold">{{ t('marketing.modes.managed.title') }}</h3>
              </div>
              <p class="home-mode-desc">{{ t('marketing.modes.managed.desc') }}</p>
              <ul class="home-mode-points">
                <li v-for="(pt, i) in stringList('marketing.modes.managed.points')" :key="i"><Icon name="checkCircle" size="sm" class="text-primary-500" />{{ pt }}</li>
              </ul>
            </div>
            <div class="home-mode home-mode-byok" data-reveal data-reveal-delay="1">
              <div class="home-mode-head">
                <span class="home-mode-badge home-mode-badge-byok">BYOK</span>
                <h3 class="text-lg font-bold">{{ t('marketing.modes.byok.title') }}</h3>
              </div>
              <p class="home-mode-desc">{{ t('marketing.modes.byok.desc') }}</p>
              <ul class="home-mode-points">
                <li v-for="(pt, i) in stringList('marketing.modes.byok.points')" :key="i"><Icon name="shield" size="sm" class="text-indigo-400" />{{ pt }}</li>
              </ul>
              <div class="mt-4 flex flex-wrap gap-2">
                <router-link :to="PUBLIC_PAGES.codex" class="home-mode-link">Codex CLI →</router-link>
                <router-link :to="PUBLIC_PAGES.claudeCode" class="home-mode-link">Claude Code →</router-link>
                <router-link :to="PUBLIC_PAGES.openaiCompat" class="home-mode-link">OpenAI SDK →</router-link>
              </div>
            </div>
          </div>
        </section>

        <!-- Free tier vs business -->
        <section class="mb-20">
          <div class="mb-8 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('marketing.lines.kicker') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">{{ t('marketing.lines.title') }}</h2>
          </div>
          <div class="grid gap-5 md:grid-cols-2">
            <router-link :to="PUBLIC_PAGES.publicBenefit" class="home-line home-line-benefit" data-reveal>
              <Icon name="gift" size="lg" class="home-line-icon" />
              <h3 class="text-xl font-bold">{{ t('marketing.lines.benefit.title') }}</h3>
              <p class="home-line-desc">{{ t('marketing.lines.benefit.desc') }}</p>
              <span class="home-line-cta">{{ t('marketing.lines.benefit.cta') }} →</span>
            </router-link>
            <router-link :to="PUBLIC_PAGES.business" class="home-line home-line-business" data-reveal data-reveal-delay="1">
              <Icon name="document" size="lg" class="home-line-icon" />
              <h3 class="text-xl font-bold">{{ t('marketing.lines.business.title') }}</h3>
              <p class="home-line-desc">{{ t('marketing.lines.business.desc') }}</p>
              <span class="home-line-cta">{{ t('marketing.lines.business.cta') }} →</span>
            </router-link>
          </div>
        </section>

        <!-- Night shift terminal -->
        <section class="mb-20">
          <div class="mb-8 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('home.nightShift.subtitle') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">
              {{ t('home.nightShift.title') }}
            </h2>
          </div>
          <div class="flex min-w-0 justify-center" data-reveal>
            <div class="terminal-container">
              <div class="terminal-glow" aria-hidden="true"></div>

              <!-- Floating provider chips -->
              <div class="terminal-orbit terminal-orbit-1" aria-hidden="true">
                <span class="terminal-orbit-dot" style="background: #f97316"></span>Claude
              </div>
              <div class="terminal-orbit terminal-orbit-2" aria-hidden="true">
                <span class="terminal-orbit-dot" style="background: #22c55e"></span>GPT
              </div>
              <div class="terminal-orbit terminal-orbit-3" aria-hidden="true">
                <span class="terminal-orbit-dot" style="background: #3b82f6"></span>Gemini
              </div>

              <div class="terminal-window">
                <!-- Window header -->
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">agent — {{ t('home.meme.nightShift') }}</span>
                  <span class="terminal-status">
                    <span class="terminal-status-dot"></span>{{ t('home.terminal.online') }}
                  </span>
                </div>
                <!-- Terminal content -->
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-time">23:58</span>
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">you:</span>
                    <span class="code-string">{{ t('home.terminal.youSleep') }}</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-time">23:58</span>
                    <span class="code-arrow">→</span>
                    <span class="code-cmd">agent:</span>
                    <span class="code-comment">{{ t('home.terminal.agentAck') }}</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-time">00:02</span>
                    <span class="code-flag">POST</span>
                    <span class="code-url">/v1/messages</span>
                    <span class="code-tag">claude-sonnet-4</span>
                    <span class="code-tag">account #3</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-time">03:41</span>
                    <span class="code-arrow">▍</span>
                    <span class="code-comment">{{ t('home.terminal.building') }}</span>
                    <span class="code-stream"></span>
                  </div>
                  <div class="code-line line-5">
                    <span class="code-time">06:12</span>
                    <span class="code-success">✓ done</span>
                    <span class="code-response">{{ t('home.terminal.done') }}</span>
                  </div>
                  <div class="code-line line-6">
                    <span class="code-time">07:30</span>
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">you:</span>
                    <span class="code-string">{{ t('home.terminal.goodMorning') }}</span>
                    <span class="cursor"></span>
                  </div>
                </div>
                <div class="terminal-footer">
                  <span class="terminal-footer-item"><span class="terminal-footer-key">in</span> 21 tok</span>
                  <span class="terminal-footer-item"><span class="terminal-footer-key">out</span> 38 tok</span>
                  <span class="terminal-footer-item"><span class="terminal-footer-key">cost</span> $0.0007</span>
                  <span class="terminal-progress"><span class="terminal-progress-bar"></span></span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- Request Flow -->
        <section class="mb-20" data-reveal>
          <div class="home-flow">
            <div class="home-flow-node">
              <div class="home-flow-icon">
                <Icon name="terminal" size="md" />
              </div>
              <span class="home-flow-label">{{ t('home.flow.client') }}</span>
              <span class="home-flow-sub">{{ t('home.flow.clientSub') }}</span>
            </div>
            <div class="home-flow-link" aria-hidden="true"><span class="home-flow-packet"></span></div>
            <div class="home-flow-node home-flow-node-core">
              <div class="home-flow-icon home-flow-icon-core">
                <Icon name="cpu" size="md" />
              </div>
              <span class="home-flow-label">{{ t('home.flow.gateway') }}</span>
              <span class="home-flow-sub">{{ t('home.flow.gatewaySub') }}</span>
            </div>
            <div class="home-flow-link" aria-hidden="true"><span class="home-flow-packet home-flow-packet-delay"></span></div>
            <div class="home-flow-node">
              <div class="home-flow-icon">
                <Icon name="users" size="md" />
              </div>
              <span class="home-flow-label">{{ t('home.flow.pool') }}</span>
            </div>
            <div class="home-flow-link" aria-hidden="true"><span class="home-flow-packet home-flow-packet-delay-2"></span></div>
            <div class="home-flow-node">
              <div class="home-flow-icon">
                <Icon name="sparkles" size="md" />
              </div>
              <span class="home-flow-label">{{ t('home.flow.upstream') }}</span>
              <span class="home-flow-sub">{{ t('home.flow.upstreamSub') }}</span>
            </div>
          </div>
        </section>

        <!-- Features Grid -->
        <section class="mb-20">
          <div class="mb-10 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('home.solutions.subtitle') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">
              {{ t('home.solutions.title') }}
            </h2>
          </div>

          <div class="grid gap-5 md:grid-cols-3">
            <!-- Feature 1: Unified Gateway -->
            <div class="home-card" data-reveal @mousemove="onCardMove">
              <span class="home-card-corner home-card-corner-tl"></span>
              <span class="home-card-corner home-card-corner-br"></span>
              <div class="home-card-icon" style="--icon-from: #38bdf8; --icon-to: #2563eb">
                <Icon name="server" size="lg" class="text-white" />
              </div>
              <h3 class="mb-2 text-lg font-semibold">
                {{ t('home.features.unifiedGateway') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-300">
                {{ t('home.features.unifiedGatewayDesc') }}
              </p>
              <span class="home-card-index">01</span>
            </div>

            <!-- Feature 2: Account Pool -->
            <div class="home-card" data-reveal data-reveal-delay="1" @mousemove="onCardMove">
              <span class="home-card-corner home-card-corner-tl"></span>
              <span class="home-card-corner home-card-corner-br"></span>
              <div class="home-card-icon" style="--icon-from: #2dd4bf; --icon-to: #0d9488">
                <Icon name="users" size="lg" class="text-white" />
              </div>
              <h3 class="mb-2 text-lg font-semibold">
                {{ t('home.features.multiAccount') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-300">
                {{ t('home.features.multiAccountDesc') }}
              </p>
              <span class="home-card-index">02</span>
            </div>

            <!-- Feature 3: Billing & Quota -->
            <div class="home-card" data-reveal data-reveal-delay="2" @mousemove="onCardMove">
              <span class="home-card-corner home-card-corner-tl"></span>
              <span class="home-card-corner home-card-corner-br"></span>
              <div class="home-card-icon" style="--icon-from: #a78bfa; --icon-to: #7c3aed">
                <Icon name="calculator" size="lg" class="text-white" />
              </div>
              <h3 class="mb-2 text-lg font-semibold">
                {{ t('home.features.balanceQuota') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-300">
                {{ t('home.features.balanceQuotaDesc') }}
              </p>
              <span class="home-card-index">03</span>
            </div>
          </div>
        </section>

        <!-- Supported Providers -->
        <section class="mb-20">
          <div class="mb-8 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('home.providers.description') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">
              {{ t('home.providers.title') }}
            </h2>
          </div>

          <div class="flex flex-wrap items-center justify-center gap-3 md:gap-4" data-reveal>
            <!-- Claude - Supported -->
            <div class="home-provider">
              <div class="home-provider-logo" style="--logo-from: #fb923c; --logo-to: #ea580c">C</div>
              <span class="home-provider-name">{{ t('home.providers.claude') }}</span>
              <span class="home-provider-badge">{{ t('home.providers.supported') }}</span>
            </div>
            <!-- GPT - Supported -->
            <div class="home-provider">
              <div class="home-provider-logo" style="--logo-from: #4ade80; --logo-to: #16a34a">G</div>
              <span class="home-provider-name">GPT</span>
              <span class="home-provider-badge">{{ t('home.providers.supported') }}</span>
            </div>
            <!-- Gemini - Supported -->
            <div class="home-provider">
              <div class="home-provider-logo" style="--logo-from: #60a5fa; --logo-to: #2563eb">G</div>
              <span class="home-provider-name">{{ t('home.providers.gemini') }}</span>
              <span class="home-provider-badge">{{ t('home.providers.supported') }}</span>
            </div>
            <!-- Antigravity - Supported -->
            <div class="home-provider">
              <div class="home-provider-logo" style="--logo-from: #fb7185; --logo-to: #db2777">A</div>
              <span class="home-provider-name">{{ t('home.providers.antigravity') }}</span>
              <span class="home-provider-badge">{{ t('home.providers.supported') }}</span>
            </div>
            <!-- More - Coming Soon -->
            <div class="home-provider home-provider-muted">
              <div class="home-provider-logo" style="--logo-from: #94a3b8; --logo-to: #64748b">+</div>
              <span class="home-provider-name">{{ t('home.providers.more') }}</span>
              <span class="home-provider-badge home-provider-badge-muted">{{ t('home.providers.soon') }}</span>
            </div>
          </div>
        </section>

        <!-- Trust principles -->
        <section class="mb-20">
          <div class="mb-8 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('marketing.trust.kicker') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">{{ t('marketing.trust.title') }}</h2>
          </div>
          <ul class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3" data-reveal>
            <li v-for="(item, i) in stringList('marketing.trust.items')" :key="i" class="home-trust">
              <Icon name="lock" size="sm" class="text-primary-500" />{{ item }}
            </li>
          </ul>
          <p class="mt-6 text-center text-xs text-gray-500 dark:text-dark-400">
            {{ t('marketing.disclaimer') }}
            <router-link :to="PUBLIC_PAGES.security" class="underline decoration-dotted underline-offset-4">{{ t('marketing.nav.security') }}</router-link>
          </p>
        </section>

        <!-- FAQ (static, crawlable) -->
        <section class="mb-20">
          <div class="mb-8 text-center" data-reveal>
            <span class="home-section-kicker">{{ t('marketing.faq.kicker') }}</span>
            <h2 class="mt-3 text-3xl font-bold tracking-tight md:text-4xl">{{ t('marketing.faq.title') }}</h2>
          </div>
          <div class="mx-auto max-w-3xl space-y-3" data-reveal>
            <details v-for="(item, i) in faqItems" :key="i" class="home-faq" :open="i === 0">
              <summary class="home-faq-q">{{ item.q }}</summary>
              <p class="home-faq-a">{{ item.a }}</p>
            </details>
          </div>
        </section>

        <!-- CTA -->
        <section class="home-cta-panel" data-reveal>
          <div class="home-cta-panel-grid" aria-hidden="true"></div>
          <div class="relative z-10 flex flex-col items-center gap-6 text-center md:flex-row md:justify-between md:text-left">
            <div>
              <h2 class="text-2xl font-bold tracking-tight text-white md:text-3xl">
                {{ t('home.meme.ctaTitle') }}
              </h2>
              <p class="mt-2 text-sm text-white/75 md:text-base">
                {{ t('home.meme.ctaDesc') }}
              </p>
            </div>
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex shrink-0 items-center gap-2 rounded-xl bg-white px-6 py-3 text-sm font-semibold text-primary-700 shadow-lg shadow-black/20 transition-transform hover:-translate-y-0.5"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="sm" :stroke-width="2" />
            </router-link>
          </div>
        </section>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/60 px-6 py-10 dark:border-white/5">
      <div class="mx-auto grid max-w-6xl gap-8 text-sm md:grid-cols-[1.2fr_1fr_1fr]">
        <div>
          <p class="text-base font-semibold"><BrandWordmark :name="siteName" /></p>
          <p class="mt-1 text-gray-600 dark:text-dark-300">{{ t('marketing.lab') }} · {{ t('marketing.positioning') }}</p>
          <p class="mt-3 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('marketing.disclaimer') }}</p>
          <router-link :to="PUBLIC_PAGES.verify" class="home-verify mt-4">
            <Icon name="badge" size="sm" />{{ t('marketing.footer.verifyHint') }}：{{ XIANYU_STORE_NAME }}
          </router-link>
        </div>
        <div>
          <p class="home-footer-h">{{ t('marketing.nav.publicInfo') }}</p>
          <ul class="mt-3 space-y-2 text-gray-500 dark:text-dark-400">
            <li v-for="link in publicLinks" :key="link.to"><router-link :to="link.to" class="hover:text-gray-900 dark:hover:text-white">{{ link.label }}</router-link></li>
          </ul>
        </div>
        <div>
          <p class="home-footer-h">{{ t('marketing.nav.legal') }}</p>
          <ul class="mt-3 space-y-2 text-gray-500 dark:text-dark-400">
            <li v-for="doc in legalDocuments" :key="doc.id"><router-link :to="`/legal/${doc.id}`" class="hover:text-gray-900 dark:hover:text-white">{{ doc.title }}</router-link></li>
            <li v-if="docUrl"><a :href="docUrl" target="_blank" rel="noopener noreferrer" class="hover:text-gray-900 dark:hover:text-white">{{ t('home.docs') }}</a></li>
            <li><a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="hover:text-gray-900 dark:hover:text-white">{{ t('marketing.footer.builtOn') }}</a></li>
          </ul>
        </div>
      </div>
      <div
        class="mx-auto mt-8 flex max-w-6xl flex-col items-center justify-between gap-4 border-t border-gray-200/60 pt-6 text-center dark:border-white/5 sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} <span class="font-mono">{{ BRAND_DOMAIN }}</span> · {{ t('home.meme.footer') }}
        </p>
        <div class="flex items-center gap-5">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import BrandWordmark from '@/components/common/BrandWordmark.vue'
import RelayStationVisual from '@/components/common/RelayStationVisual.vue'
import { useLatencyProbe } from '@/composables/useLatencyProbe'
import { BRAND_DOMAIN, PUBLIC_PAGES, XIANYU_STORE_NAME, resolveBrandName } from '@/constants/brand'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t, tm, rt } = useI18n()

// vue-i18n tm() returns raw resources; normalise to plain strings.
function i18nStr(value: unknown): string {
  if (typeof value === 'string') return value
  try {
    return rt(value as never)
  } catch {
    return ''
  }
}
function stringList(key: string): string[] {
  const raw = tm(key) as unknown
  return Array.isArray(raw) ? raw.map(i18nStr).filter(Boolean) : []
}
const faqItems = computed(() => {
  const raw = tm('marketing.faq.items') as unknown
  if (!Array.isArray(raw)) return [] as Array<{ q: string; a: string }>
  return raw.map((entry) => {
    const item = entry as Record<string, unknown>
    return { q: i18nStr(item.q), a: i18nStr(item.a) }
  })
})
const publicLinks = computed(() => [
  { to: PUBLIC_PAGES.codex, label: t('marketing.nav.codex') },
  { to: PUBLIC_PAGES.claudeCode, label: t('marketing.nav.claudeCode') },
  { to: PUBLIC_PAGES.openaiCompat, label: t('marketing.nav.openaiCompat') },
  { to: PUBLIC_PAGES.publicBenefit, label: t('marketing.nav.publicBenefit') },
  { to: PUBLIC_PAGES.business, label: t('marketing.nav.business') },
  { to: PUBLIC_PAGES.security, label: t('marketing.nav.security') },
  { to: PUBLIC_PAGES.benchmarks, label: t('marketing.nav.benchmarks') },
  { to: PUBLIC_PAGES.models, label: t('marketing.nav.models') },
  { to: PUBLIC_PAGES.keyUsage, label: t('marketing.nav.keyUsage') }
])
const legalDocuments = computed(() => appStore.cachedPublicSettings?.login_agreement_documents ?? [])

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => resolveBrandName(appStore.cachedPublicSettings?.site_name || appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// API base URL shown on the cover: the real origin this page is served from
const apiBaseUrl = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return origin && !/^https?:\/\/(localhost|127\.0\.0\.1)/.test(origin) ? origin : `https://${BRAND_DOMAIN}`
})
const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null
async function copyBaseUrl() {
  try {
    await navigator.clipboard.writeText(apiBaseUrl.value)
    copied.value = true
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = false), 1600)
  } catch {
    /* clipboard unavailable: leave the URL selectable */
  }
}

// Live latency probe against this deployment's /health
const { latencyMs, state: latencyState, probe: probeLatency } = useLatencyProbe()

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Background particles (deterministic layout, no per-render randomness)
const particles = Array.from({ length: 18 }, (_, index) => {
  const seed = index + 1
  const left = (seed * 37) % 100
  const top = (seed * 53) % 100
  const size = 2 + (seed % 3)
  const duration = 14 + (seed % 7) * 2
  const delay = -((seed * 3) % 14)
  return {
    id: seed,
    style: {
      left: `${left}%`,
      top: `${top}%`,
      width: `${size}px`,
      height: `${size}px`,
      animationDuration: `${duration}s`,
      animationDelay: `${delay}s`
    }
  }
})

// Spotlight hover for feature cards
function onCardMove(event: MouseEvent) {
  const card = event.currentTarget as HTMLElement | null
  if (!card) return
  const rect = card.getBoundingClientRect()
  card.style.setProperty('--spot-x', `${event.clientX - rect.left}px`)
  card.style.setProperty('--spot-y', `${event.clientY - rect.top}px`)
}

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  // Dark-first: only an explicit "light" choice opts out.
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme !== 'light') {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

// Reveal-on-scroll
let revealObserver: IntersectionObserver | null = null

function initReveal() {
  const targets = Array.from(document.querySelectorAll<HTMLElement>('[data-reveal]'))
  if (targets.length === 0) return

  const reduceMotion =
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)')?.matches

  if (reduceMotion || typeof IntersectionObserver === 'undefined') {
    targets.forEach((el) => el.classList.add('is-visible'))
    return
  }

  revealObserver = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
          revealObserver?.unobserve(entry.target)
        }
      })
    },
    { threshold: 0.12, rootMargin: '0px 0px -40px 0px' }
  )
  targets.forEach((el) => revealObserver?.observe(el))
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  initReveal()
  void probeLatency()
})

onBeforeUnmount(() => {
  revealObserver?.disconnect()
  revealObserver = null
  if (copiedTimer) clearTimeout(copiedTimer)
})
</script>

<style scoped>
/* ============ Page canvas ============ */
.home-root {
  --home-bg: #f6f9fc;
  --home-line: rgba(15, 23, 42, 0.06);
  --home-glass: rgba(255, 255, 255, 0.72);
  --home-glass-border: rgba(15, 23, 42, 0.08);
  --home-accent: #14b8a6;
  --home-accent-2: #22d3ee;
  --home-accent-3: #6366f1;
  background:
    radial-gradient(1200px 600px at 50% -10%, rgba(20, 184, 166, 0.14), transparent 60%),
    var(--home-bg);
}

.dark .home-root {
  --home-bg: #050b14;
  --home-line: rgba(148, 163, 184, 0.08);
  --home-glass: rgba(10, 18, 32, 0.6);
  --home-glass-border: rgba(148, 163, 184, 0.12);
  background:
    radial-gradient(1200px 600px at 50% -10%, rgba(20, 184, 166, 0.18), transparent 60%),
    var(--home-bg);
}

/* Aurora blobs */
.home-aurora {
  position: absolute;
  border-radius: 9999px;
  filter: blur(90px);
  opacity: 0.55;
  will-change: transform;
  animation: home-aurora-drift 22s ease-in-out infinite alternate;
}
.home-aurora-1 {
  top: -220px;
  right: -160px;
  width: 620px;
  height: 620px;
  background: radial-gradient(circle at 30% 30%, rgba(20, 184, 166, 0.55), rgba(34, 211, 238, 0.25) 45%, transparent 70%);
}
.home-aurora-2 {
  bottom: -260px;
  left: -200px;
  width: 640px;
  height: 640px;
  background: radial-gradient(circle at 60% 40%, rgba(99, 102, 241, 0.42), rgba(20, 184, 166, 0.2) 50%, transparent 72%);
  animation-delay: -8s;
  animation-duration: 26s;
}
.home-aurora-3 {
  top: 35%;
  left: 40%;
  width: 420px;
  height: 420px;
  background: radial-gradient(circle, rgba(34, 211, 238, 0.35), transparent 70%);
  opacity: 0.35;
  animation-delay: -14s;
  animation-duration: 30s;
}
.dark .home-aurora {
  opacity: 0.5;
}

@keyframes home-aurora-drift {
  0% {
    transform: translate3d(0, 0, 0) scale(1);
  }
  50% {
    transform: translate3d(40px, -30px, 0) scale(1.08);
  }
  100% {
    transform: translate3d(-30px, 40px, 0) scale(0.96);
  }
}

/* Grid with radial mask */
.home-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--home-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--home-line) 1px, transparent 1px);
  background-size: 56px 56px;
  -webkit-mask-image: radial-gradient(ellipse 80% 70% at 50% 20%, #000 30%, transparent 100%);
  mask-image: radial-gradient(ellipse 80% 70% at 50% 20%, #000 30%, transparent 100%);
}

/* Vertical light beam behind the hero */
.home-beam {
  position: absolute;
  top: -10%;
  left: 50%;
  width: 2px;
  height: 60%;
  transform: translateX(-50%);
  background: linear-gradient(180deg, transparent, rgba(20, 184, 166, 0.5), transparent);
  opacity: 0.6;
  filter: blur(1px);
}
.dark .home-beam {
  background: linear-gradient(180deg, transparent, rgba(45, 212, 191, 0.7), transparent);
}

/* Particles */
.home-particles {
  position: absolute;
  inset: 0;
}
.home-particle {
  position: absolute;
  border-radius: 9999px;
  background: rgba(20, 184, 166, 0.45);
  box-shadow: 0 0 12px rgba(20, 184, 166, 0.55);
  animation: home-particle-float linear infinite;
}
.dark .home-particle {
  background: rgba(94, 234, 212, 0.6);
  box-shadow: 0 0 14px rgba(94, 234, 212, 0.65);
}
@keyframes home-particle-float {
  0% {
    transform: translate3d(0, 0, 0);
    opacity: 0;
  }
  10% {
    opacity: 0.8;
  }
  90% {
    opacity: 0.8;
  }
  100% {
    transform: translate3d(0, -140px, 0);
    opacity: 0;
  }
}

/* ============ Navigation ============ */
.home-nav {
  background: var(--home-glass);
  border: 1px solid var(--home-glass-border);
  backdrop-filter: blur(18px) saturate(160%);
  -webkit-backdrop-filter: blur(18px) saturate(160%);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.5) inset,
    0 10px 30px -18px rgba(2, 6, 23, 0.35);
}
.dark .home-nav {
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.06) inset,
    0 20px 40px -24px rgba(0, 0, 0, 0.8);
}

.home-nav-btn {
  color: rgb(100 116 139);
  transition: color 0.2s ease, background-color 0.2s ease;
}
.home-nav-btn:hover {
  color: rgb(17 24 39);
  background: rgba(15, 23, 42, 0.06);
}
.dark .home-nav-btn {
  color: rgb(148 163 184);
}
.dark .home-nav-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.07);
}


.home-logo-halo {
  position: relative;
  display: inline-flex;
  border-radius: 14px;
}
.home-logo-halo::before {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: inherit;
  background: conic-gradient(from 180deg, rgba(20, 184, 166, 0.7), rgba(34, 211, 238, 0.4), rgba(99, 102, 241, 0.5), rgba(20, 184, 166, 0.7));
  filter: blur(8px);
  opacity: 0.55;
  z-index: -1;
}

.home-mode-pill {
  align-items: center;
  overflow: hidden;
  border-radius: 9999px;
  border: 1px solid var(--home-glass-border);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  letter-spacing: 0.06em;
}
.home-mode-seg {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
}
.home-mode-rest {
  color: rgb(100 116 139);
  background: rgba(148, 163, 184, 0.12);
}
.home-mode-build {
  color: #0f766e;
  background: rgba(20, 184, 166, 0.12);
}
.dark .home-mode-rest {
  color: rgb(148 163 184);
}
.dark .home-mode-build {
  color: #5eead4;
}

.home-domain {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  letter-spacing: 0.08em;
  color: #0d9488;
}
.dark .home-domain {
  color: #5eead4;
}

.home-caption {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  letter-spacing: 0.02em;
  color: rgb(107 114 128);
}
.dark .home-caption {
  color: rgb(100 116 139);
}

/* ============ Buttons ============ */
.home-cta-primary {
  position: relative;
  overflow: hidden;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6 0%, #0891b2 60%, #4f46e5 140%);
  box-shadow:
    0 10px 30px -10px rgba(20, 184, 166, 0.55),
    0 0 0 1px rgba(255, 255, 255, 0.08) inset;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.home-cta-primary::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(110deg, transparent 30%, rgba(255, 255, 255, 0.35) 50%, transparent 70%);
  transform: translateX(-120%);
  transition: transform 0.6s ease;
}
.home-cta-primary:hover {
  transform: translateY(-1px);
  box-shadow:
    0 16px 40px -12px rgba(20, 184, 166, 0.7),
    0 0 0 1px rgba(255, 255, 255, 0.12) inset;
}
.home-cta-primary:hover::after {
  transform: translateX(120%);
}
.home-cta-lg {
  box-shadow:
    0 18px 45px -14px rgba(20, 184, 166, 0.7),
    0 0 40px -10px rgba(34, 211, 238, 0.45),
    0 0 0 1px rgba(255, 255, 255, 0.1) inset;
}

.home-cta-secondary {
  color: rgb(31 41 55);
  background: var(--home-glass);
  border: 1px solid var(--home-glass-border);
  backdrop-filter: blur(12px);
  transition: transform 0.2s ease, border-color 0.2s ease, background-color 0.2s ease;
}
.home-cta-secondary:hover {
  transform: translateY(-1px);
  border-color: rgba(20, 184, 166, 0.5);
}
.dark .home-cta-secondary {
  color: rgb(226 232 240);
}

/* ============ Hero ============ */
.home-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  border-radius: 9999px;
  border: 1px solid rgba(20, 184, 166, 0.35);
  background: rgba(20, 184, 166, 0.08);
  padding: 6px 14px 6px 10px;
  font-size: 13px;
  font-weight: 500;
  color: #0f766e;
}
.dark .home-eyebrow {
  color: #99f6e4;
  background: rgba(20, 184, 166, 0.12);
}
.home-eyebrow-sep {
  width: 1px;
  height: 12px;
  background: currentColor;
  opacity: 0.3;
}
.home-pulse-dot {
  position: relative;
  width: 8px;
  height: 8px;
  border-radius: 9999px;
  background: #14b8a6;
}
.home-pulse-dot::after {
  content: '';
  position: absolute;
  inset: -4px;
  border-radius: inherit;
  border: 1px solid #14b8a6;
  animation: home-ping 1.8s cubic-bezier(0, 0, 0.2, 1) infinite;
}
@keyframes home-ping {
  0% {
    transform: scale(0.6);
    opacity: 1;
  }
  100% {
    transform: scale(1.8);
    opacity: 0;
  }
}


.home-hero-title {
  text-wrap: balance;
  overflow-wrap: anywhere;
}

.home-endpoint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 8px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  padding: 4px 10px 4px 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  color: rgb(55 65 81);
}
.dark .home-endpoint {
  color: rgb(203 213 225);
}
.home-endpoint-method {
  font-style: normal;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  border-radius: 4px;
  padding: 1px 5px;
  background: rgba(20, 184, 166, 0.15);
  color: #0f766e;
}
.dark .home-endpoint-method {
  color: #5eead4;
}

/* ============ API address card ============ */
.home-address {
  border-radius: 18px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  backdrop-filter: blur(14px);
  padding: 16px 18px;
}
.home-address-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
.home-address-title {
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.02em;
}
.home-address-sub {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: rgb(107 114 128);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.home-address-row {
  display: flex;
  align-items: stretch;
  gap: 8px;
}
.home-address-url {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-radius: 10px;
  border: 1px solid rgba(20, 184, 166, 0.35);
  background: rgba(20, 184, 166, 0.08);
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 14px;
  font-weight: 600;
  color: #0f766e;
}
.dark .home-address-url {
  color: #5eead4;
}
.home-address-copy {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 10px;
  padding: 0 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6 0%, #0891b2 60%, #4f46e5 140%);
  box-shadow: 0 8px 20px -10px rgba(20, 184, 166, 0.6);
  transition: transform 0.15s ease, filter 0.15s ease;
  white-space: nowrap;
}
.home-address-copy:hover {
  transform: translateY(-1px);
  filter: brightness(1.05);
}
.home-address-copy.is-copied {
  background: linear-gradient(135deg, #22c55e, #16a34a);
}
@media (max-width: 639px) {
  .home-address-row {
    flex-direction: column;
  }
  .home-address-url {
    white-space: normal;
    overflow-wrap: anywhere;
    font-size: 13px;
  }
  .home-address-copy {
    justify-content: center;
    padding: 10px 14px;
  }
}
.home-address-hint {
  margin-top: 8px;
  font-size: 12px;
  color: rgb(107 114 128);
}
.dark .home-address-hint {
  color: rgb(148 163 184);
}

/* ============ Selling points ============ */
.home-sell {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  border-radius: 18px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  backdrop-filter: blur(14px);
  padding: 18px;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}
.home-sell:hover {
  transform: translateY(-3px);
  border-color: rgba(20, 184, 166, 0.45);
  box-shadow: 0 18px 40px -22px rgba(20, 184, 166, 0.55);
}
.home-sell-icon {
  display: flex;
  height: 40px;
  width: 40px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: linear-gradient(135deg, var(--icon-from), var(--icon-to));
  box-shadow: 0 10px 22px -12px var(--icon-to);
}
.home-sell-body {
  min-width: 0;
}
.home-sell-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 700;
}
.home-sell-live {
  border-radius: 6px;
  padding: 1px 7px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  font-weight: 600;
  color: #0f766e;
  background: rgba(20, 184, 166, 0.14);
}
.dark .home-sell-live {
  color: #5eead4;
}
.home-sell-desc {
  margin-top: 3px;
  font-size: 12.5px;
  line-height: 1.5;
  color: rgb(107 114 128);
}
.dark .home-sell-desc {
  color: rgb(148 163 184);
}

/* ============ Positioning / modes / lines / trust / FAQ / footer ============ */
.home-positioning {
  font-size: 13px;
  color: rgb(107 114 128);
}
.dark .home-positioning {
  color: rgb(148 163 184);
}
.home-mode {
  border-radius: 20px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  backdrop-filter: blur(14px);
  padding: 24px;
}
.home-mode-byok {
  border-color: rgba(99, 102, 241, 0.35);
}
.home-mode-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.home-mode-badge {
  border-radius: 6px;
  padding: 2px 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 10px;
  letter-spacing: 0.18em;
  color: #0f766e;
  background: rgba(20, 184, 166, 0.14);
}
.home-mode-badge-byok {
  color: #4338ca;
  background: rgba(99, 102, 241, 0.14);
}
.dark .home-mode-badge {
  color: #5eead4;
}
.dark .home-mode-badge-byok {
  color: #a5b4fc;
}
.home-mode-desc {
  margin-top: 10px;
  font-size: 14px;
  line-height: 1.65;
  color: rgb(75 85 99);
}
.dark .home-mode-desc {
  color: rgb(203 213 225);
}
.home-mode-points {
  margin-top: 12px;
  display: grid;
  gap: 6px;
  font-size: 13.5px;
}
.home-mode-points li {
  display: flex;
  align-items: center;
  gap: 8px;
}
.home-mode-link {
  border-radius: 8px;
  border: 1px solid var(--home-glass-border);
  padding: 4px 10px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  transition: border-color 0.2s ease;
}
.home-mode-link:hover {
  border-color: rgba(99, 102, 241, 0.6);
}
.home-line {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-radius: 20px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  backdrop-filter: blur(14px);
  padding: 26px;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}
.home-line:hover {
  transform: translateY(-3px);
}
.home-line-benefit:hover {
  border-color: rgba(20, 184, 166, 0.5);
  box-shadow: 0 18px 40px -22px rgba(20, 184, 166, 0.55);
}
.home-line-business:hover {
  border-color: rgba(99, 102, 241, 0.5);
  box-shadow: 0 18px 40px -22px rgba(99, 102, 241, 0.55);
}
.home-line-icon {
  color: #0d9488;
}
.home-line-business .home-line-icon {
  color: #6366f1;
}
.home-line-desc {
  font-size: 14px;
  line-height: 1.65;
  color: rgb(75 85 99);
}
.dark .home-line-desc {
  color: rgb(203 213 225);
}
.home-line-cta {
  margin-top: 4px;
  font-size: 13px;
  font-weight: 600;
  color: #0f766e;
}
.home-line-business .home-line-cta {
  color: #4f46e5;
}
.dark .home-line-cta {
  color: #5eead4;
}
.dark .home-line-business .home-line-cta {
  color: #a5b4fc;
}
.home-trust {
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: 14px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  padding: 12px 14px;
  font-size: 14px;
  font-weight: 500;
}
.home-faq {
  border-radius: 14px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  padding: 4px 18px;
}
.home-faq-q {
  cursor: pointer;
  padding: 12px 0;
  font-size: 15px;
  font-weight: 600;
  list-style: none;
}
.home-faq-q::-webkit-details-marker {
  display: none;
}
.home-faq-q::before {
  content: '+';
  display: inline-block;
  width: 18px;
  color: #0d9488;
  font-weight: 700;
}
.home-faq[open] .home-faq-q::before {
  content: '−';
}
.home-faq-a {
  padding: 0 0 14px 18px;
  font-size: 14px;
  line-height: 1.7;
  color: rgb(75 85 99);
}
.dark .home-faq-a {
  color: rgb(203 213 225);
}
.home-footer-h {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: rgb(107 114 128);
}
.home-verify {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 9999px;
  border: 1px solid rgba(20, 184, 166, 0.4);
  background: rgba(20, 184, 166, 0.08);
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #0f766e;
}
.dark .home-verify {
  color: #5eead4;
}

/* ============ Chips ============ */

.home-section-kicker {
  display: inline-block;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: #0d9488;
}
.dark .home-section-kicker {
  color: #5eead4;
}

/* ============ Request Flow ============ */
.home-flow {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
  align-items: center;
  border-radius: 24px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  backdrop-filter: blur(16px);
  padding: 28px 24px;
}
@media (min-width: 768px) {
  .home-flow {
    grid-template-columns: 1fr auto 1.2fr auto 1fr auto 1fr;
    gap: 0;
    padding: 32px 36px;
  }
}
.home-flow-node {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
}
.home-flow-icon {
  display: flex;
  height: 52px;
  width: 52px;
  align-items: center;
  justify-content: center;
  border-radius: 16px;
  border: 1px solid var(--home-glass-border);
  background: rgba(255, 255, 255, 0.7);
  color: #0d9488;
  box-shadow: 0 8px 20px -12px rgba(2, 6, 23, 0.3);
}
.dark .home-flow-icon {
  background: rgba(15, 23, 42, 0.8);
  color: #5eead4;
}
.home-flow-icon-core {
  height: 64px;
  width: 64px;
  color: #fff;
  border-color: transparent;
  background: linear-gradient(135deg, #14b8a6, #0891b2 60%, #4f46e5 140%);
  box-shadow:
    0 0 0 6px rgba(20, 184, 166, 0.12),
    0 16px 40px -14px rgba(20, 184, 166, 0.8);
  animation: home-core-pulse 3s ease-in-out infinite;
}
@keyframes home-core-pulse {
  0%,
  100% {
    box-shadow:
      0 0 0 6px rgba(20, 184, 166, 0.12),
      0 16px 40px -14px rgba(20, 184, 166, 0.8);
  }
  50% {
    box-shadow:
      0 0 0 12px rgba(20, 184, 166, 0.06),
      0 16px 50px -12px rgba(20, 184, 166, 0.95);
  }
}
.home-flow-label {
  font-size: 14px;
  font-weight: 600;
}
.home-flow-sub {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: rgb(107 114 128);
}
.dark .home-flow-sub {
  color: rgb(148 163 184);
}
.home-flow-link {
  position: relative;
  height: 28px;
  width: 2px;
  margin: 0 auto;
  background: repeating-linear-gradient(180deg, rgba(20, 184, 166, 0.5) 0 4px, transparent 4px 8px);
}
@media (min-width: 768px) {
  .home-flow-link {
    height: 2px;
    width: 100%;
    min-width: 48px;
    margin: 0 8px;
    background: repeating-linear-gradient(90deg, rgba(20, 184, 166, 0.5) 0 6px, transparent 6px 12px);
    transform: translateY(-14px);
  }
}
.home-flow-packet {
  position: absolute;
  top: 0;
  left: 50%;
  width: 6px;
  height: 6px;
  margin-left: -3px;
  border-radius: 9999px;
  background: #2dd4bf;
  box-shadow: 0 0 12px 2px rgba(45, 212, 191, 0.8);
  animation: home-packet-v 2.4s linear infinite;
}
@media (min-width: 768px) {
  .home-flow-packet {
    top: 50%;
    left: 0;
    margin-left: 0;
    margin-top: -3px;
    animation: home-packet-h 2.4s linear infinite;
  }
}
.home-flow-packet-delay {
  animation-delay: 0.8s;
}
.home-flow-packet-delay-2 {
  animation-delay: 1.6s;
}
@keyframes home-packet-v {
  from {
    transform: translateY(-6px);
    opacity: 0;
  }
  15% {
    opacity: 1;
  }
  85% {
    opacity: 1;
  }
  to {
    transform: translateY(28px);
    opacity: 0;
  }
}
@keyframes home-packet-h {
  from {
    left: 0;
    opacity: 0;
  }
  15% {
    opacity: 1;
  }
  85% {
    opacity: 1;
  }
  to {
    left: 100%;
    opacity: 0;
  }
}

/* ============ Feature cards ============ */
.home-card {
  position: relative;
  overflow: hidden;
  border-radius: 20px;
  border: 1px solid var(--home-glass-border);
  background: var(--home-glass);
  backdrop-filter: blur(16px);
  padding: 28px;
  transition: transform 0.3s ease, border-color 0.3s ease, box-shadow 0.3s ease;
  --spot-x: 50%;
  --spot-y: 0%;
}
.home-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(360px circle at var(--spot-x) var(--spot-y), rgba(20, 184, 166, 0.16), transparent 45%);
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
}
.home-card:hover {
  transform: translateY(-4px);
  border-color: rgba(20, 184, 166, 0.45);
  box-shadow: 0 24px 50px -24px rgba(20, 184, 166, 0.45);
}
.home-card:hover::before {
  opacity: 1;
}
.home-card-corner {
  position: absolute;
  width: 14px;
  height: 14px;
  border-color: rgba(20, 184, 166, 0.6);
  border-style: solid;
  opacity: 0;
  transition: opacity 0.3s ease;
}
.home-card-corner-tl {
  top: 10px;
  left: 10px;
  border-width: 1px 0 0 1px;
}
.home-card-corner-br {
  bottom: 10px;
  right: 10px;
  border-width: 0 1px 1px 0;
}
.home-card:hover .home-card-corner {
  opacity: 1;
}
.home-card-icon {
  position: relative;
  display: flex;
  height: 48px;
  width: 48px;
  align-items: center;
  justify-content: center;
  margin-bottom: 18px;
  border-radius: 14px;
  background: linear-gradient(135deg, var(--icon-from), var(--icon-to));
  box-shadow: 0 12px 28px -12px var(--icon-to);
  transition: transform 0.3s ease;
}
.home-card:hover .home-card-icon {
  transform: scale(1.08) rotate(-3deg);
}
.home-card-index {
  position: absolute;
  top: 22px;
  right: 24px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  letter-spacing: 0.16em;
  color: rgb(148 163 184);
}

/* ============ Providers ============ */
.home-provider {
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: 14px;
  border: 1px solid rgba(20, 184, 166, 0.3);
  background: var(--home-glass);
  backdrop-filter: blur(12px);
  padding: 10px 16px 10px 10px;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}
.home-provider:hover {
  transform: translateY(-2px);
  border-color: rgba(20, 184, 166, 0.6);
  box-shadow: 0 14px 30px -18px rgba(20, 184, 166, 0.7);
}
.home-provider-muted {
  border-color: var(--home-glass-border);
  opacity: 0.65;
}
.home-provider-logo {
  display: flex;
  height: 34px;
  width: 34px;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--logo-from), var(--logo-to));
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  box-shadow: 0 8px 18px -10px var(--logo-to);
}
.home-provider-name {
  font-size: 14px;
  font-weight: 600;
  color: rgb(55 65 81);
}
.dark .home-provider-name {
  color: rgb(226 232 240);
}
.home-provider-badge {
  border-radius: 6px;
  padding: 2px 7px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  background: rgba(20, 184, 166, 0.14);
  color: #0f766e;
}
.dark .home-provider-badge {
  color: #5eead4;
}
.home-provider-badge-muted {
  background: rgba(148, 163, 184, 0.18);
  color: rgb(100 116 139);
}

/* ============ CTA panel ============ */
.home-cta-panel {
  position: relative;
  overflow: hidden;
  border-radius: 28px;
  padding: 40px 32px;
  background: linear-gradient(120deg, #0f766e 0%, #0e7490 45%, #4338ca 120%);
  box-shadow:
    0 30px 60px -30px rgba(20, 184, 166, 0.7),
    0 0 0 1px rgba(255, 255, 255, 0.08) inset;
}
@media (min-width: 768px) {
  .home-cta-panel {
    padding: 48px 56px;
  }
}
.home-cta-panel::before {
  content: '';
  position: absolute;
  top: -120px;
  right: -80px;
  width: 340px;
  height: 340px;
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(255, 255, 255, 0.35), transparent 65%);
  filter: blur(20px);
}
.home-cta-panel-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.08) 1px, transparent 1px);
  background-size: 40px 40px;
  -webkit-mask-image: radial-gradient(ellipse at 50% 50%, #000 20%, transparent 80%);
  mask-image: radial-gradient(ellipse at 50% 50%, #000 20%, transparent 80%);
}

/* ============ Reveal on scroll ============ */
[data-reveal] {
  opacity: 0;
  transform: translateY(18px);
  transition: opacity 0.7s ease, transform 0.7s cubic-bezier(0.22, 1, 0.36, 1);
}
[data-reveal][data-reveal-delay='1'] {
  transition-delay: 0.12s;
}
[data-reveal][data-reveal-delay='2'] {
  transition-delay: 0.24s;
}
[data-reveal].is-visible {
  opacity: 1;
  transform: translateY(0);
}

/* ============ Terminal ============ */
.terminal-container {
  position: relative;
  display: block;
  width: 100%;
  max-width: 500px;
}

.terminal-glow {
  position: absolute;
  inset: -24px;
  border-radius: 32px;
  background: radial-gradient(closest-side, rgba(20, 184, 166, 0.35), rgba(99, 102, 241, 0.12) 60%, transparent);
  filter: blur(24px);
  z-index: 0;
  animation: terminal-glow-breathe 5s ease-in-out infinite;
}
@keyframes terminal-glow-breathe {
  0%,
  100% {
    opacity: 0.7;
    transform: scale(0.98);
  }
  50% {
    opacity: 1;
    transform: scale(1.03);
  }
}

.terminal-orbit {
  position: absolute;
  z-index: 3;
  display: none;
  align-items: center;
  gap: 8px;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(10, 18, 32, 0.85);
  backdrop-filter: blur(10px);
  padding: 6px 12px 6px 8px;
  font-size: 12px;
  font-weight: 600;
  color: #e2e8f0;
  box-shadow: 0 12px 30px -14px rgba(0, 0, 0, 0.7);
  animation: terminal-orbit-float 6s ease-in-out infinite;
}
@media (min-width: 640px) {
  .terminal-orbit {
    display: inline-flex;
  }
}
.terminal-orbit-dot {
  width: 8px;
  height: 8px;
  border-radius: 9999px;
  box-shadow: 0 0 10px currentColor;
}
.terminal-orbit-1 {
  top: -18px;
  left: -28px;
}
.terminal-orbit-2 {
  top: 42%;
  right: -34px;
  animation-delay: -2s;
}
.terminal-orbit-3 {
  bottom: 24%;
  left: -64px;
  animation-delay: -4s;
}
@keyframes terminal-orbit-float {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-8px);
  }
}

.terminal-window {
  position: relative;
  z-index: 1;
  width: 100%;
  background: linear-gradient(160deg, #0f172a 0%, #070d19 100%);
  border-radius: 16px;
  box-shadow:
    0 30px 60px -20px rgba(0, 0, 0, 0.55),
    0 0 0 1px rgba(148, 163, 184, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  overflow: hidden;
  transform: perspective(1200px) rotateX(3deg) rotateY(-4deg);
  transition: transform 0.4s ease;
}
.terminal-window::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  padding: 1px;
  background: linear-gradient(135deg, rgba(45, 212, 191, 0.6), transparent 40%, transparent 60%, rgba(99, 102, 241, 0.5));
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  pointer-events: none;
}
.terminal-container:hover .terminal-window {
  transform: perspective(1200px) rotateX(0deg) rotateY(0deg) translateY(-4px);
}

.terminal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(15, 23, 42, 0.7);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.terminal-buttons {
  display: flex;
  gap: 8px;
}
.terminal-buttons span {
  width: 11px;
  height: 11px;
  border-radius: 50%;
}
.btn-close {
  background: #ff5f57;
}
.btn-minimize {
  background: #febc2e;
}
.btn-maximize {
  background: #28c840;
}
.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-family: ui-monospace, monospace;
  color: #64748b;
}
.terminal-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: ui-monospace, monospace;
  font-size: 10px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #5eead4;
}
.terminal-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 9999px;
  background: #2dd4bf;
  box-shadow: 0 0 8px #2dd4bf;
  animation: blink-soft 2s ease-in-out infinite;
}
@keyframes blink-soft {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.terminal-body {
  padding: 18px 22px 12px;
  font-family: ui-monospace, 'Fira Code', 'JetBrains Mono', monospace;
  font-size: 13px;
  line-height: 1.9;
}
.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}
.line-1 {
  animation-delay: 0.3s;
}
.line-2 {
  animation-delay: 0.8s;
  padding-left: 22px;
}
.line-3 {
  animation-delay: 1.5s;
}
.line-4 {
  animation-delay: 2.1s;
}
.line-5 {
  animation-delay: 3s;
}
.line-6 {
  animation-delay: 3.6s;
}
@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-time {
  width: 38px;
  flex-shrink: 0;
  font-size: 11px;
  color: #475569;
}
.code-prompt {
  color: #22c55e;
  font-weight: bold;
}
.code-cmd {
  color: #38bdf8;
}
.code-flag {
  color: #a78bfa;
}
.code-url {
  color: #2dd4bf;
}
.code-string {
  color: #fbbf24;
}
.code-arrow {
  color: #2dd4bf;
}
.code-comment {
  color: #94a3b8;
}
.code-tag {
  border-radius: 4px;
  padding: 0 6px;
  font-size: 11px;
  background: rgba(148, 163, 184, 0.12);
  color: #cbd5e1;
}
.code-success {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response {
  color: #fbbf24;
}
.code-meta {
  margin-left: auto;
  font-size: 11px;
  color: #64748b;
}
.code-stream {
  display: inline-block;
  height: 8px;
  width: 0;
  border-radius: 9999px;
  background: linear-gradient(90deg, #2dd4bf, #38bdf8);
  animation: stream-grow 1.2s ease-out 2.2s forwards;
}
@keyframes stream-grow {
  from {
    width: 0;
  }
  to {
    width: 120px;
  }
}

.cursor {
  display: inline-block;
  width: 8px;
  height: 15px;
  background: #22c55e;
  animation: blink 1s step-end infinite;
}
@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

.terminal-footer {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px 22px 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(2, 6, 23, 0.4);
  font-family: ui-monospace, monospace;
  font-size: 11px;
  color: #94a3b8;
}
.terminal-footer-key {
  color: #475569;
  margin-right: 4px;
}
.terminal-progress {
  position: relative;
  flex: 1;
  height: 3px;
  min-width: 40px;
  border-radius: 9999px;
  background: rgba(148, 163, 184, 0.15);
  overflow: hidden;
}
.terminal-progress-bar {
  position: absolute;
  inset: 0;
  width: 40%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2dd4bf, #38bdf8);
  animation: progress-slide 2.2s ease-in-out infinite;
}
@keyframes progress-slide {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(260%);
  }
}

/* ============ Reduced motion ============ */
@media (prefers-reduced-motion: reduce) {
  .home-aurora,
  .home-particle,
  .home-pulse-dot::after,
  .home-flow-packet,
  .home-flow-icon-core,
  .terminal-glow,
  .terminal-orbit,
  .terminal-progress-bar,
  .terminal-status-dot {
    animation: none;
  }
  .code-line {
    opacity: 1;
    animation: none;
  }
  .code-stream {
    width: 120px;
    animation: none;
  }
  [data-reveal] {
    opacity: 1;
    transform: none;
    transition: none;
  }
}
</style>

<template>
  <section class="brand-statement" aria-label="rest2build 公司标语与传播口号">
    <div class="brand-statement-inner">
      <a class="brand-wordmark" href="https://ai.rest2build.lol/" aria-label="访问 rest2build 网站">
        <img src="/logo.svg" alt="" />
        <span>rest2build</span>
      </a>
      <p class="brand-tagline"><span>歇一会儿，</span><span>让 AI 接着干。</span></p>
      <p class="brand-subline">rest 是你的，build 交给 AI。</p>
      <p class="brand-service">{{ BRAND_SERVICE_DESCRIPTION }}</p>
      <a class="brand-domain" href="https://ai.rest2build.lol/">ai.rest2build.lol</a>
    </div>
  </section>
</template>

<script setup lang="ts">
import { BRAND_SERVICE_DESCRIPTION } from '@/constants/brand'
</script>

<style scoped>
.brand-statement {
  border-top: 1px solid #c9ddd0;
  background: #edf7f0;
  padding: 72px 24px 28px;
  color: #3b5849;
  text-align: center;
}

.dark .brand-statement {
  border-color: #2b4639;
  background: #17221c;
  color: #c4d0c9;
}

.brand-statement-inner {
  width: min(880px, 100%);
  margin: 0 auto;
}

.brand-wordmark {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  color: #143b2a;
  font-size: 36px;
  font-weight: 750;
  line-height: 1.3;
  text-decoration: none;
}

.dark .brand-wordmark {
  color: #f7faf8;
}

.brand-wordmark img {
  width: 48px;
  height: 48px;
}

.brand-tagline {
  margin: 32px 0 14px;
  color: #143b2a;
  font-size: 52px;
  font-weight: 750;
  line-height: 1.4;
}

.dark .brand-tagline {
  color: #f7faf8;
}

.brand-tagline span {
  display: inline-block;
}

.brand-tagline span:last-child {
  color: #2c7456;
}

.dark .brand-tagline span:last-child {
  color: #d4ed9b;
}

.brand-subline {
  margin: 0 0 26px;
  color: #365847;
  font-size: 20px;
}

.dark .brand-subline {
  color: #d6dfda;
}

.brand-service {
  max-width: 760px;
  margin: 0 auto 28px;
  font-size: 16px;
  line-height: 1.9;
}

.brand-domain {
  display: inline-block;
  color: #176c52;
  font: 700 18px ui-monospace, SFMono-Regular, Menlo, monospace;
  text-underline-offset: 7px;
}

.dark .brand-domain {
  color: #d4ed9b;
}

.brand-wordmark:focus-visible,
.brand-domain:focus-visible {
  outline: 3px solid #2c7456;
  outline-offset: 5px;
}

.dark .brand-wordmark:focus-visible,
.dark .brand-domain:focus-visible {
  outline-color: #d4ed9b;
}

@media (max-width: 640px) {
  .brand-statement {
    padding: 48px 16px 24px;
  }

  .brand-wordmark {
    gap: 12px;
    font-size: 30px;
  }

  .brand-wordmark img {
    width: 42px;
    height: 42px;
  }

  .brand-tagline {
    margin-top: 24px;
    font-size: 30px;
    line-height: 1.5;
  }

  .brand-subline {
    font-size: 16px;
  }

  .brand-service {
    text-align: left;
    font-size: 14px;
  }
}
</style>

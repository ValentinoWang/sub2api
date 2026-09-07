<template>
  <section class="brand-statement" aria-label="rest2build 公司标语与传播口号">
    <div class="brand-statement-inner">
      <a class="brand-wordmark" href="https://ai.rest2build.lol/" aria-label="访问 rest2build 网站">
        <img src="/logo.svg" alt="" />
        <span>rest2build</span>
      </a>
      <p class="brand-tagline"><span>歇一会儿，</span><span>让 AI 接着干。</span></p>
      <p class="brand-subline">rest 是你的，build 交给 AI。</p>
      <p class="brand-service">rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。</p>
      <a class="brand-domain" href="https://ai.rest2build.lol/">ai.rest2build.lol</a>
    </div>
  </section>
</template>

<style scoped>
.brand-statement { background: #191f1c; padding: 72px 24px 28px; color: #c4d0c9; text-align: center; }.brand-statement-inner { width: min(880px, 100%); margin: 0 auto; }.brand-wordmark { display: inline-flex; align-items: center; gap: 14px; color: #f7faf8; font-size: 36px; font-weight: 750; line-height: 1.3; text-decoration: none; }.brand-wordmark img { width: 48px; height: 48px; }.brand-tagline { margin: 32px 0 14px; font-size: 52px; font-weight: 750; line-height: 1.4; color: #f7faf8; }.brand-tagline span { display: inline-block; }.brand-tagline span:last-child { color: #d4ed9b; }.brand-subline { margin: 0 0 26px; font-size: 20px; color: #d6dfda; }.brand-service { max-width: 760px; margin: 0 auto 28px; font-size: 16px; line-height: 1.9; }.brand-domain { display: inline-block; color: #d4ed9b; font: 700 18px ui-monospace, SFMono-Regular, Menlo, monospace; text-underline-offset: 7px; }.brand-wordmark:focus-visible, .brand-domain:focus-visible { outline: 3px solid #d4ed9b; outline-offset: 5px; }
@media (max-width: 640px) { .brand-statement { padding: 48px 16px 24px; }.brand-wordmark { font-size: 30px; gap: 12px; }.brand-wordmark img { width: 42px; height: 42px; }.brand-tagline { margin-top: 24px; font-size: 30px; line-height: 1.5; }.brand-subline { font-size: 16px; }.brand-service { text-align: left; font-size: 14px; } }
</style>

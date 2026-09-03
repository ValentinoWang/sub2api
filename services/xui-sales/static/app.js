document.addEventListener('DOMContentLoaded', () => {
  if (window.lucide) window.lucide.createIcons({ attrs: { 'stroke-width': 1.8 } });

  document.querySelectorAll('[data-copy-target]').forEach((button) => {
    button.addEventListener('click', async () => {
      const input = document.getElementById(button.dataset.copyTarget);
      if (!input) return;
      await navigator.clipboard.writeText(input.value);
      button.setAttribute('data-copied', 'true');
      setTimeout(() => button.removeAttribute('data-copied'), 1400);
    });
  });

  const body = document.body;
  const orderId = body.dataset.orderId;
  const token = body.dataset.orderToken;
  if (!orderId || !token) return;

  const titles = {
    pending: ['等待支付', '付款后请停留在此页面，系统会自动确认。'],
    paid: ['支付已确认', '正在创建专属订阅。'],
    provisioning: ['正在开通', '通常会在数秒内完成。'],
    provision_failed: ['正在重试开通', '款项已确认，系统会自动重试。'],
    active: ['订阅已开通', '请妥善保存下方订阅地址。'],
    api_pending: ['正在开通 API 套餐', '系统会继续处理 VPN 订阅。'],
    api_active: ['API 套餐已开通', '正在创建 VPN 订阅。'],
    vpn_pending: ['正在开通网络稳定器', '通常会在数秒内完成。'],
    vpn_active: ['网络稳定器已开通', 'API 套餐仍在处理中。'],
    partial_failed: ['部分服务正在恢复', '已成功的服务不会重复发放，系统会自动重试。'],
  };
  let stopped = false;
  async function poll() {
    if (stopped) return;
    try {
      const query = new URLSearchParams({ token });
      const response = await fetch(`/api/orders/${encodeURIComponent(orderId)}?${query}`, { cache: 'no-store' });
      if (!response.ok) return;
      const order = await response.json();
      const copy = titles[order.status] || ['订单处理中', '请稍后刷新。'];
      document.getElementById('status-title').textContent = copy[0];
      document.getElementById('status-copy').textContent = copy[1];
      document.querySelector('.order-status')?.setAttribute('data-order-status', order.status);
      const apiState = document.getElementById('api-state');
      const vpnState = document.getElementById('vpn-state');
      if (apiState) apiState.textContent = order.api_status;
      if (vpnState) vpnState.textContent = order.vpn_status;
      if (order.subscription_url) {
        const input = document.getElementById('subscription-url');
        if (input) input.value = order.subscription_url;
        document.getElementById('subscription-block')?.classList.remove('hidden');
      }
      if (order.status === 'active') {
        document.getElementById('pay-form')?.remove();
        stopped = true;
      }
    } finally {
      if (!stopped) setTimeout(poll, 3000);
    }
  }
  poll();
});

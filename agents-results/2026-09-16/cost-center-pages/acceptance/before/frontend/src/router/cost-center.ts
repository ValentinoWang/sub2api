import type { Router } from 'vue-router'

// Registered before initial navigation; the existing global admin guard still applies.
export function registerCostCenterRoute(router: Router): void {
  router.addRoute({
    path: '/admin/cost-center',
    name: 'AdminCostCenter',
    component: () => import('@/views/admin/CostCenterView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, title: 'Cost Workbench', titleKey: 'nav.costCenter' }
  })
}

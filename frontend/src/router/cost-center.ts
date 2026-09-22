import type { Router } from 'vue-router'

export const costCenterSections = [
  {
    path: '/admin/cost-center/accounting',
    name: 'AdminCostAccounting',
    title: 'Cost Accounting',
    titleKey: 'nav.costAccounting',
    descriptionKey: 'costCenter.accountingDescription',
    component: () => import('@/views/admin/cost-center/CostAccountingView.vue')
  },
  {
    path: '/admin/cost-center/purchases',
    name: 'AdminCostPurchases',
    title: 'Purchases and Entries',
    titleKey: 'nav.costPurchases',
    descriptionKey: 'costCenter.purchasesDescription',
    component: () => import('@/views/admin/cost-center/CostPurchasesView.vue')
  },
  {
    path: '/admin/cost-center/ledger',
    name: 'AdminCostLedger',
    title: 'Ledger Records',
    titleKey: 'nav.costLedger',
    descriptionKey: 'costCenter.ledgerDescription',
    component: () => import('@/views/admin/cost-center/CostLedgerView.vue')
  },
  {
    path: '/admin/cost-center/comparison',
    name: 'AdminCostComparison',
    title: 'Three-Tier Comparison',
    titleKey: 'nav.costComparison',
    descriptionKey: 'costCenter.comparisonDescription',
    component: () => import('@/views/admin/cost-center/CostComparisonView.vue')
  }
] as const

// Registered before initial navigation; the existing global admin guard still applies.
export function registerCostCenterRoute(router: Router): void {
  router.addRoute({
    path: '/admin/cost-center',
    name: 'AdminCostCenter',
    component: () => import('@/views/admin/CostCenterView.vue'),
    redirect: '/admin/cost-center/accounting',
    meta: { requiresAuth: true, requiresAdmin: true, title: 'Cost Workbench', titleKey: 'nav.costCenter' },
    children: costCenterSections.map(section => ({
      path: section.path,
      name: section.name,
      component: section.component,
      meta: {
        requiresAuth: true,
        requiresAdmin: true,
        title: section.title,
        titleKey: section.titleKey,
        descriptionKey: section.descriptionKey
      }
    }))
  })
}

import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'dashboard', component: () => import('@/pages/DashboardPage.vue'), meta: { title: '仪表盘' } },
  { path: '/accounts', name: 'accounts', component: () => import('@/pages/AccountsPage.vue'), meta: { title: '账号' } },
  { path: '/sources', name: 'sources', component: () => import('@/pages/SourcesPage.vue'), meta: { title: '监听源' } },
  { path: '/sinks', name: 'sinks', component: () => import('@/pages/SinksPage.vue'), meta: { title: '目标渠道' } },
  { path: '/rules', name: 'rules', component: () => import('@/pages/RulesPage.vue'), meta: { title: '规则' } },
  { path: '/templates', name: 'templates', component: () => import('@/pages/TemplatesPage.vue'), meta: { title: '模板' } },
  { path: '/deliveries', name: 'deliveries', component: () => import('@/pages/DeliveriesPage.vue'), meta: { title: '投递记录' } },
  { path: '/settings', name: 'settings', component: () => import('@/pages/SettingsPage.vue'), meta: { title: '设置' } },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

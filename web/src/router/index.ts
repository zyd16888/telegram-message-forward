import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/dashboard' },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/pages/LoginPage.vue'),
    meta: { title: '登录', public: true },
  },
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

// 全局登录守卫：识别鉴权模式后决定是否放行或跳转登录页。
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  try {
    await auth.ensureInitialized()
  } catch {
    // /auth/me 请求失败（如后端不可用）不阻断到登录页的导航。
  }

  if (to.meta.public) {
    // 已可进入后台时，登录页重定向到仪表盘。
    if (to.name === 'login' && auth.canEnter) {
      return { name: 'dashboard' }
    }
    return true
  }

  if (!auth.canEnter) {
    return { name: 'login' }
  }
  return true
})

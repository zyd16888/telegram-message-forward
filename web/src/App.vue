<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import type { MenuOption } from 'naive-ui'
import { darkTheme } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import ClayIcon from '@/components/ClayIcon.vue'
import { clayDark, clayLight } from '@/theme/clay'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const dark = shallowRef(false)
const collapsed = shallowRef(false)
const drawerOpen = shallowRef(false)
const isMobile = shallowRef(false)

const theme = computed(() => (dark.value ? darkTheme : null))
const themeOverrides = computed(() => (dark.value ? clayDark : clayLight))
const isLoginRoute = computed(() => route.name === 'login')
const activeKey = computed(() => route.name as string)
const currentTitle = computed(() => (route.meta.title as string) ?? '')
const showTopTitle = computed(() => route.name === 'dashboard')

const authTag = computed(() => {
  if (auth.devNoAuth) return { text: '开发免鉴权', dot: '#ef9a4c' }
  if (auth.authenticated) return { text: '已登录', dot: '#2fb896' }
  return { text: '未登录', dot: '#ef9a4c' }
})

let mq: MediaQueryList | null = null

watch(
  dark,
  (value) => {
    document.documentElement.dataset.clayTheme = value ? 'dark' : 'light'
  },
  { immediate: true },
)

function icon(name: string) {
  return () => h(ClayIcon, { name })
}

const menuOptions: MenuOption[] = [
  { label: '概览', key: 'dashboard', icon: icon('dashboard') },
  { label: '转发编排', key: 'flow', icon: icon('flow') },
  { label: 'TG 账号', key: 'accounts', icon: icon('accounts') },
  { label: 'Telegram 配置', key: 'telegram-config', icon: icon('telegram') },
  { label: '监听源', key: 'sources', icon: icon('sources') },
  { label: '目标渠道', key: 'sinks', icon: icon('sinks') },
  { label: '过滤器', key: 'filters', icon: icon('shield') },
  { label: '渲染模板', key: 'templates', icon: icon('templates') },
  { label: '投递记录', key: 'deliveries', icon: icon('deliveries') },
  { label: 'AI 整理', key: 'ai-digests', icon: icon('ai') },
  { label: '设置', key: 'settings', icon: icon('settings') },
]

function applyMedia() {
  isMobile.value = mq?.matches ?? false
  if (!isMobile.value) drawerOpen.value = false
}

onMounted(() => {
  mq = window.matchMedia('(max-width: 768px)')
  applyMedia()
  mq.addEventListener('change', applyMedia)
})

onBeforeUnmount(() => mq?.removeEventListener('change', applyMedia))

function toggleNav() {
  if (isMobile.value) drawerOpen.value = !drawerOpen.value
  else collapsed.value = !collapsed.value
}

function handleMenu(key: string) {
  router.push({ name: key })
  drawerOpen.value = false
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <NConfigProvider :theme="theme" :theme-overrides="themeOverrides">
    <NMessageProvider>
      <NDialogProvider>
        <RouterView v-if="isLoginRoute" />

        <NLayout v-else has-sider class="shell">
          <NLayoutSider
            v-if="!isMobile"
            :bordered="false"
            :width="248"
            :collapsed="collapsed"
            :collapsed-width="72"
            collapse-mode="width"
            :native-scrollbar="false"
            class="sider"
            :class="{ collapsed }"
          >
            <div class="brand" :class="{ collapsed }">
              <div class="brand-badge">
                <ClayIcon name="telegram" :size="22" />
              </div>
              <div v-show="!collapsed" class="brand-text">
                <span class="brand-name">TG Forward</span>
                <span class="brand-sub">消息转发中枢</span>
              </div>
            </div>

            <NMenu
              :value="activeKey"
              :options="menuOptions"
              :collapsed="collapsed"
              :collapsed-width="72"
              :collapsed-icon-size="21"
              :indent="16"
              @update:value="handleMenu"
            />
          </NLayoutSider>

          <NLayout class="main">
            <NLayoutHeader class="header">
              <div class="top-left">
                <button class="nav-toggle" type="button" aria-label="切换导航" @click="toggleNav">
                  <ClayIcon name="menu" :size="20" />
                </button>
                <div v-if="showTopTitle" class="route-heading">
                  <span class="route-title">{{ currentTitle }}</span>
                </div>
              </div>
              <div class="top-actions">
                <span class="auth-chip">
                  <span class="dot" :style="{ background: authTag.dot }" />
                  <span class="auth-text">{{ authTag.text }}</span>
                </span>
                <NSwitch v-model:value="dark" size="medium">
                  <template #checked>暗</template>
                  <template #unchecked>亮</template>
                </NSwitch>
                <NButton v-if="!auth.devNoAuth" size="small" secondary @click="logout">
                  <template #icon><ClayIcon name="logout" :size="16" /></template>
                  <span class="logout-text">退出</span>
                </NButton>
              </div>
            </NLayoutHeader>

            <NLayoutContent class="content" :native-scrollbar="false">
              <main class="content-inner">
                <RouterView />
              </main>
            </NLayoutContent>
          </NLayout>
        </NLayout>

        <NDrawer v-model:show="drawerOpen" :width="252" placement="left" class="mobile-drawer">
          <NDrawerContent :body-content-style="{ padding: '0' }" :native-scrollbar="false">
            <div class="brand drawer-brand">
              <div class="brand-badge">
                <ClayIcon name="telegram" :size="22" />
              </div>
              <div class="brand-text">
                <span class="brand-name">TG Forward</span>
                <span class="brand-sub">消息转发中枢</span>
              </div>
            </div>
            <NMenu :value="activeKey" :options="menuOptions" :indent="16" @update:value="handleMenu" />
          </NDrawerContent>
        </NDrawer>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style scoped>
.shell {
  height: 100vh;
  background: var(--clay-bg);
}

.sider {
  height: 100vh;
  border-right: 1px solid var(--clay-border);
  background: var(--clay-surface) !important;
  box-shadow: 6px 0 18px rgba(15, 23, 42, 0.04);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 64px;
  padding: 0 18px;
  border-bottom: 1px solid var(--clay-border);
  overflow: hidden;
}

.brand.collapsed {
  justify-content: center;
  padding: 0;
}

.brand-badge {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 11px;
  color: #fff;
  background: linear-gradient(135deg, #45a9ea, #237fc2);
  box-shadow: var(--clay-extruded-sm);
  animation: clay-float 3.8s ease-in-out infinite;
}

.brand-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.2;
}

.brand-name {
  color: var(--clay-text);
  font-size: 17px;
  font-weight: 800;
}

.brand-sub {
  margin-top: 2px;
  color: var(--clay-text-3);
  font-size: 12px;
}

.sider :deep(.n-menu) {
  padding: 12px 10px;
}

.sider :deep(.n-menu-item-content) {
  position: relative;
  border-radius: 10px !important;
  box-shadow: none !important;
  transform: none !important;
  transition: background-color 0.14s ease, color 0.14s ease !important;
}

.sider :deep(.n-menu-item-content:hover),
.sider :deep(.n-menu-item-content:active),
.sider :deep(.n-menu-item-content:focus),
.sider :deep(.n-menu-item-content:focus-visible),
.sider :deep(.n-menu-item-content.n-menu-item-content--selected),
.sider :deep(.n-menu-item-content.n-menu-item-content--selected:hover),
.sider :deep(.n-menu-item-content.n-menu-item-content--selected:active) {
  box-shadow: none !important;
  transform: none !important;
}

.sider :deep(.n-menu-item-content.n-menu-item-content--selected) {
  transform: none;
}

/* naive 的 ::before 是整块背景层且自带 background-color 过渡，不能挪作指示条：
   取消选中时几何覆盖立即失效，残留的颜色过渡会把整块菜单项闪成主色。
   选中态背景层保持透明，指示条改画在 ::after 上。 */
.sider :deep(.n-menu-item-content.n-menu-item-content--selected::before),
.sider :deep(.n-menu-item-content.n-menu-item-content--selected:hover::before) {
  background-color: transparent;
}

.sider :deep(.n-menu-item-content.n-menu-item-content--selected::after) {
  position: absolute;
  left: 8px;
  top: 11px;
  width: 3px;
  height: 20px;
  border-radius: 999px;
  background: var(--clay-primary);
  content: '';
}

.sider.collapsed :deep(.n-menu) {
  padding: 12px 0;
}

.sider.collapsed :deep(.n-menu-item) {
  display: flex;
  justify-content: center;
}

.sider.collapsed :deep(.n-menu-item-content) {
  /* naive-ui 的菜单项是 grid("icon content arrow")，图标列宽为 auto；
     绝对定位图标时包含块会退化为 0 宽的 icon 网格区域导致 left:50% 失效，
     因此收起态改为单区域网格并用 place-items 居中。 */
  width: 48px !important;
  height: 48px !important;
  margin: 0 auto;
  padding: 0 !important;
  grid-template-areas: 'icon' !important;
  grid-template-columns: 1fr !important;
  place-items: center !important;
}

.sider.collapsed :deep(.n-menu-item-content-header) {
  display: none;
}

.sider.collapsed :deep(.n-menu-item-content__icon) {
  width: 24px !important;
  height: 24px !important;
  margin: 0 !important;
  display: grid !important;
  place-items: center !important;
}

.sider.collapsed :deep(.n-menu-item-content__icon svg) {
  display: block;
  margin: 0;
}

.sider.collapsed :deep(.n-menu-item-content.n-menu-item-content--selected::after) {
  display: none;
}

.main {
  min-width: 0;
}

.header {
  position: sticky;
  top: 0;
  z-index: 20;
  height: 64px;
  padding: 0 22px;
  border-bottom: 1px solid var(--clay-border);
  background: color-mix(in srgb, var(--clay-surface) 92%, transparent) !important;
  backdrop-filter: blur(14px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.top-left,
.top-actions {
  display: flex;
  align-items: center;
  min-width: 0;
}

.top-left {
  gap: 12px;
}

.top-actions {
  gap: 12px;
  flex-shrink: 0;
}

.nav-toggle {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 10px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  cursor: pointer;
  color: var(--clay-text-2);
  background: var(--clay-surface);
  box-shadow: var(--clay-extruded-sm);
  transition: box-shadow 0.15s ease-out, transform 0.15s ease-out, color 0.18s ease;
}

.nav-toggle:hover {
  color: var(--clay-primary);
  box-shadow: var(--clay-extruded-hover);
  transform: translateY(-1px);
}

.nav-toggle:focus-visible {
  color: var(--clay-primary);
  outline: 2px solid var(--clay-primary);
  outline-offset: 2px;
}

.nav-toggle:active {
  color: var(--clay-primary);
  box-shadow: var(--clay-inset-deep);
  transform: scale(0.92);
  transition-duration: 0.08s;
}

.route-heading {
  min-width: 0;
}

.route-title {
  display: block;
  color: var(--clay-text);
  font-size: 17px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auth-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 30px;
  padding: 0 10px;
  border: 0;
  border-radius: 999px;
  color: var(--clay-text-2);
  background: var(--clay-surface-2);
  box-shadow: var(--clay-inset-sm);
  font-size: 13px;
  font-weight: 600;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 12%, transparent);
}

.content {
  padding: 0;
}

.content-inner {
  width: min(100% - 40px, 1480px);
  min-height: calc(100vh - 64px);
  margin: 0 auto;
  padding: 22px 0 32px;
}

.drawer-brand {
  background: var(--clay-surface);
}

.mobile-drawer :deep(.n-drawer-content) {
  background: var(--clay-surface) !important;
}

@media (max-width: 768px) {
  .header {
    height: 58px;
    padding: 0 14px;
  }

  .content-inner {
    width: calc(100% - 24px);
    min-height: calc(100vh - 58px);
    padding: 16px 0 24px;
  }

  .route-title {
    font-size: 16px;
  }

  .auth-text,
  .logout-text {
    display: none;
  }

  .auth-chip {
    padding: 0 8px;
  }
}
</style>

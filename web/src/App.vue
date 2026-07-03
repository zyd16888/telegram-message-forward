<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import type { MenuOption } from 'naive-ui'
import { darkTheme } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import ClayIcon from '@/components/ClayIcon.vue'
import { clayDark, clayLight } from '@/theme/clay'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const dark = ref(false)
const theme = computed(() => (dark.value ? darkTheme : null))
const themeOverrides = computed(() => (dark.value ? clayDark : clayLight))

// 切换暗色时同步 <html data-clay-theme>，让 clay.css 的变量整体翻转。
watch(
  dark,
  (v) => {
    document.documentElement.dataset.clayTheme = v ? 'dark' : 'light'
  },
  { immediate: true },
)

// —— 响应式：桌面折叠 / 移动抽屉 ——
const collapsed = ref(false) // 桌面侧栏收起
const drawerOpen = ref(false) // 移动端抽屉
const isMobile = ref(false)

let mq: MediaQueryList | null = null
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

// 汉堡按钮：移动端开关抽屉，桌面端收起/展开侧栏。
function toggleNav() {
  if (isMobile.value) drawerOpen.value = !drawerOpen.value
  else collapsed.value = !collapsed.value
}

// 登录页不套后台布局。
const isLoginRoute = computed(() => route.name === 'login')

function icon(name: string) {
  return () => h(ClayIcon, { name })
}

const menuOptions: MenuOption[] = [
  { label: '仪表盘', key: 'dashboard', icon: icon('dashboard') },
  { label: '账号', key: 'accounts', icon: icon('accounts') },
  { label: 'Telegram 配置', key: 'telegram-config', icon: icon('telegram') },
  { label: '监听源', key: 'sources', icon: icon('sources') },
  { label: '目标渠道', key: 'sinks', icon: icon('sinks') },
  { label: '规则', key: 'rules', icon: icon('rules') },
  { label: '模板', key: 'templates', icon: icon('templates') },
  { label: '投递记录', key: 'deliveries', icon: icon('deliveries') },
  { label: '设置', key: 'settings', icon: icon('settings') },
]

const activeKey = computed(() => route.name as string)
const currentTitle = computed(() => (route.meta.title as string) ?? '')

const authTag = computed(() => {
  if (auth.devNoAuth) return { text: '开发免鉴权', dot: '#f0a55b' }
  if (auth.authenticated) return { text: '已登录', dot: '#3fc5a0' }
  return { text: '未登录', dot: '#f0a55b' }
})

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
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <RouterView v-if="isLoginRoute" />

        <n-layout v-else has-sider class="shell">
          <!-- 桌面侧栏（可折叠） -->
          <n-layout-sider
            v-if="!isMobile"
            :bordered="false"
            :width="248"
            :collapsed="collapsed"
            :collapsed-width="86"
            collapse-mode="width"
            :native-scrollbar="false"
            class="sider"
          >
            <div class="side-panel" :class="{ collapsed }">
              <div class="brand">
                <div class="brand-badge">
                  <ClayIcon name="telegram" :size="22" />
                </div>
                <div v-show="!collapsed" class="brand-text">
                  <span class="brand-name">TG Forward</span>
                  <span class="brand-sub">消息转发中枢</span>
                </div>
              </div>

              <n-menu
                :value="activeKey"
                :options="menuOptions"
                :collapsed="collapsed"
                :collapsed-width="86"
                :collapsed-icon-size="22"
                :indent="18"
                @update:value="handleMenu"
              />
            </div>
          </n-layout-sider>

          <n-layout class="main">
            <n-layout-header class="header">
              <div class="top-bar">
                <div class="top-left">
                  <button class="nav-toggle" type="button" aria-label="切换导航" @click="toggleNav">
                    <ClayIcon name="menu" :size="20" />
                  </button>
                  <span class="title">{{ currentTitle }}</span>
                </div>
                <div class="top-actions">
                  <span class="auth-chip">
                    <span class="dot" :style="{ background: authTag.dot }" />
                    <span class="auth-text">{{ authTag.text }}</span>
                  </span>
                  <n-switch v-model:value="dark" size="medium">
                    <template #checked>暗</template>
                    <template #unchecked>亮</template>
                  </n-switch>
                  <n-button v-if="!auth.devNoAuth" size="small" secondary @click="logout">
                    <template #icon><ClayIcon name="logout" :size="16" /></template>
                    <span class="logout-text">退出</span>
                  </n-button>
                </div>
              </div>
            </n-layout-header>

            <n-layout-content class="content" :native-scrollbar="false">
              <div class="content-inner">
                <RouterView />
              </div>
            </n-layout-content>
          </n-layout>
        </n-layout>

        <!-- 移动端抽屉导航 -->
        <n-drawer
          v-model:show="drawerOpen"
          :width="252"
          placement="left"
          class="mobile-drawer"
        >
          <n-drawer-content :body-content-style="{ padding: '0' }" :native-scrollbar="false">
            <div class="side-panel drawer-panel">
              <div class="brand">
                <div class="brand-badge">
                  <ClayIcon name="telegram" :size="22" />
                </div>
                <div class="brand-text">
                  <span class="brand-name">TG Forward</span>
                  <span class="brand-sub">消息转发中枢</span>
                </div>
              </div>
              <n-menu
                :value="activeKey"
                :options="menuOptions"
                :indent="18"
                @update:value="handleMenu"
              />
            </div>
          </n-drawer-content>
        </n-drawer>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped>
.shell {
  height: 100vh;
}

/* ---------- 侧栏：悬浮黏土面板 ---------- */
.sider {
  padding: 18px 0 18px 18px;
}
.side-panel {
  height: 100%;
  border-radius: 26px;
  padding: 20px 8px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out), var(--clay-inset-hi);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.side-panel.collapsed {
  padding: 20px 0;
}
.side-panel.collapsed :deep(.n-menu) {
  padding: 0;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 14px 20px;
}
.side-panel.collapsed .brand {
  justify-content: center;
  padding: 6px 0 20px;
}
.brand-badge {
  width: 44px;
  height: 44px;
  border-radius: 15px;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(150deg, #56b0ea, #2f8fd6);
  box-shadow:
    5px 5px 12px rgba(24, 108, 170, 0.4),
    -3px -3px 8px rgba(255, 255, 255, 0.5),
    inset 2px 2px 4px rgba(255, 255, 255, 0.45),
    inset -3px -3px 6px rgba(18, 90, 150, 0.35);
  flex-shrink: 0;
}
.brand-text {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
}
.brand-name {
  font-size: 18px;
  font-weight: 900;
  letter-spacing: 0.2px;
  color: var(--n-text-color, #22364a);
}
.brand-sub {
  font-size: 12px;
  font-weight: 600;
  color: #8aa0b4;
}

.drawer-panel {
  border-radius: 0;
  box-shadow: none;
  padding-top: 20px;
}

/* ---------- 顶栏：悬浮黏土条 ---------- */
.main {
  padding: 18px 18px 0;
  min-width: 0;
}
.header {
  height: auto;
  padding: 0;
  margin-bottom: 18px;
}
.top-bar {
  height: 60px;
  border-radius: 22px;
  padding: 0 12px 0 12px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm), var(--clay-inset-hi);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.top-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.nav-toggle {
  width: 40px;
  height: 40px;
  border: none;
  border-radius: 13px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  cursor: pointer;
  color: var(--n-text-color-2, #3c5165);
  background: var(--clay-surface-2);
  box-shadow: var(--clay-out-sm);
  transition: box-shadow 0.18s ease, transform 0.12s ease;
}
.nav-toggle:hover {
  transform: translateY(-1px);
}
.nav-toggle:active {
  box-shadow: var(--clay-press);
  transform: translateY(0);
}
.title {
  font-size: 18px;
  font-weight: 800;
  letter-spacing: 0.3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.top-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.auth-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  font-weight: 700;
  padding: 7px 14px;
  border-radius: 999px;
  background: var(--clay-surface-2);
  box-shadow: inset 2px 2px 5px var(--clay-dark-soft),
    inset -2px -2px 5px var(--clay-light);
  color: var(--n-text-color-2, #3c5165);
}
.auth-chip .dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  flex-shrink: 0;
}

/* ---------- 内容区 ---------- */
.content {
  padding: 0;
}
.content-inner {
  padding: 4px 18px 28px 0;
}

/* ---------- 窄屏适配 ---------- */
@media (max-width: 768px) {
  .main {
    padding: 12px 12px 0;
  }
  .top-bar {
    height: 56px;
    padding: 0 10px;
  }
  .title {
    font-size: 16px;
  }
  .content-inner {
    padding: 2px 0 22px 0;
  }
  .auth-text {
    display: none;
  }
  .auth-chip {
    padding: 8px;
  }
  .logout-text {
    display: none;
  }
}
</style>

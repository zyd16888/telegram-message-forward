<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import type { MenuOption } from 'naive-ui'
import { NIcon, darkTheme } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const dark = ref(false)
const theme = computed(() => (dark.value ? darkTheme : null))

// 登录页不套后台布局。
const isLoginRoute = computed(() => route.name === 'login')

const menuOptions: MenuOption[] = [
  { label: '仪表盘', key: 'dashboard' },
  { label: '账号', key: 'accounts' },
  { label: '监听源', key: 'sources' },
  { label: '目标渠道', key: 'sinks' },
  { label: '规则', key: 'rules' },
  { label: '模板', key: 'templates' },
  { label: '投递记录', key: 'deliveries' },
  { label: '设置', key: 'settings' },
]

const activeKey = computed(() => route.name as string)
const currentTitle = computed(() => (route.meta.title as string) ?? '')

// 头部鉴权状态标签。
const authTag = computed(() => {
  if (auth.devNoAuth) return { type: 'warning' as const, text: '开发免鉴权', color: '#f0a020' }
  if (auth.authenticated) return { type: 'success' as const, text: '已登录', color: '#18a058' }
  return { type: 'warning' as const, text: '未登录', color: '#f0a020' }
})

function handleMenu(key: string) {
  router.push({ name: key })
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}

// 简单的连接状态图标。
function dot(color: string) {
  return () => h(NIcon, { color }, { default: () => '●' })
}
</script>

<template>
  <n-config-provider :theme="theme">
    <n-message-provider>
      <n-dialog-provider>
        <RouterView v-if="isLoginRoute" />

        <n-layout v-else has-sider style="height: 100vh">
          <n-layout-sider
            bordered
            :width="220"
            :native-scrollbar="false"
            content-style="padding-top: 12px;"
          >
            <div class="brand">TG Forward</div>
            <n-menu
              :value="activeKey"
              :options="menuOptions"
              @update:value="handleMenu"
            />
          </n-layout-sider>

          <n-layout>
            <n-layout-header bordered class="header">
              <span class="title">{{ currentTitle }}</span>
              <n-space align="center">
                <n-tag :type="authTag.type" size="small" :render-icon="dot(authTag.color)">
                  {{ authTag.text }}
                </n-tag>
                <n-switch v-model:value="dark" size="small">
                  <template #checked>暗</template>
                  <template #unchecked>亮</template>
                </n-switch>
                <n-button v-if="!auth.devNoAuth" size="small" quaternary @click="logout">
                  退出登录
                </n-button>
              </n-space>
            </n-layout-header>
            <n-layout-content class="content" :native-scrollbar="false">
              <RouterView />
            </n-layout-content>
          </n-layout>
        </n-layout>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped>
.brand {
  font-size: 18px;
  font-weight: 700;
  padding: 8px 20px 16px;
}
.header {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}
.title {
  font-size: 16px;
  font-weight: 600;
}
.content {
  padding: 20px;
}
</style>

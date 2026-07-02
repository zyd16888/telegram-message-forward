<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { errText } from '@/utils/error'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const loading = ref(false)
const ready = ref(false)

// 表单输入。
const username = ref('admin')
const password = ref('')
const bootstrapUsername = ref('admin')
const bootstrapPassword = ref('')

// 展示模式：dev（免鉴权） / bootstrap（首次初始化） / login（输入 token）。
const mode = computed<'dev' | 'bootstrap' | 'login'>(() => {
  if (auth.devNoAuth) return 'dev'
  if (auth.canBootstrap) return 'bootstrap'
  return 'login'
})

onMounted(async () => {
  try {
    await auth.ensureInitialized()
  } catch (e) {
    message.error('无法连接后端：' + errText(e))
  } finally {
    ready.value = true
  }
  // 免鉴权或已登录时直接进入后台。
  if (auth.canEnter) {
    void router.replace({ name: 'dashboard' })
  }
})

function enter() {
  void router.replace({ name: 'dashboard' })
}

async function doLogin() {
  if (!username.value.trim() || !password.value) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    message.success('登录成功')
    enter()
  } catch (e) {
    message.error('登录失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function doBootstrap() {
  if (!bootstrapUsername.value.trim() || !bootstrapPassword.value) {
    message.warning('请输入管理员用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.bootstrap(bootstrapUsername.value.trim() || 'admin', bootstrapPassword.value)
    message.success('管理员已创建')
    enter()
  } catch (e) {
    message.error('初始化失败：' + errText(e))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <n-card class="login-card" :bordered="true">
      <template #header>
        <div class="brand">TG Forward 管理后台</div>
      </template>

      <n-spin :show="!ready">
        <!-- 开发免鉴权 -->
        <div v-if="mode === 'dev'">
          <n-alert type="warning" title="开发免鉴权模式">
            当前后端已关闭管理 API 鉴权（<n-text code>auth_enabled=false</n-text>），
            仅建议在本地开发或单人使用。
          </n-alert>
          <n-button class="mt" type="primary" block @click="enter">直接进入后台</n-button>
        </div>

        <!-- 首次初始化 -->
        <div v-else-if="mode === 'bootstrap'">
          <n-alert type="info" title="首次使用：创建管理员">
            系统尚无管理员。创建后即可用用户名和密码登录后台。
          </n-alert>
          <n-space vertical class="mt">
            <n-input v-model:value="bootstrapUsername" placeholder="用户名，如 admin" />
            <n-input
              v-model:value="bootstrapPassword"
              type="password"
              show-password-on="click"
              placeholder="登录密码"
              @keyup.enter="doBootstrap"
            />
            <n-button type="primary" block :loading="loading" @click="doBootstrap">
              创建管理员
            </n-button>
          </n-space>
        </div>

        <!-- 登录 -->
        <div v-else>
          <n-space vertical>
            <n-text depth="3">输入管理员用户名和密码登录后台。</n-text>
            <n-input
              v-model:value="username"
              placeholder="用户名"
              @keyup.enter="doLogin"
            />
            <n-input
              v-model:value="password"
              type="password"
              show-password-on="click"
              placeholder="密码"
              @keyup.enter="doLogin"
            />
            <n-button type="primary" block :loading="loading" @click="doLogin">登录</n-button>
            <n-text depth="3" style="font-size: 12px">
              API Token 仍可在设置页维护，供脚本或运维调用使用。
            </n-text>
          </n-space>
        </div>
      </n-spin>
    </n-card>
  </div>
</template>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}
.login-card {
  width: 420px;
  max-width: 100%;
}
.brand {
  font-size: 18px;
  font-weight: 700;
}
.mt {
  margin-top: 16px;
}
</style>

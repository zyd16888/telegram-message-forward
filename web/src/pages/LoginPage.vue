<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { errText } from '@/utils/error'
import ClayIcon from '@/components/ClayIcon.vue'

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
    <span class="blob blob-a" />
    <span class="blob blob-b" />

    <div class="login-card">
      <div class="brand">
        <div class="brand-badge">
          <ClayIcon name="telegram" :size="30" />
        </div>
        <div class="brand-text">
          <span class="brand-name">TG Forward</span>
          <span class="brand-sub">消息转发管理后台</span>
        </div>
      </div>

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
    </div>
  </div>
</template>

<style scoped>
.login-wrap {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  overflow: hidden;
}

/* 背景柔光团 */
.blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(10px);
  pointer-events: none;
}
.blob-a {
  width: 340px;
  height: 340px;
  top: -80px;
  left: -60px;
  background: radial-gradient(circle at 35% 35%, rgba(120, 190, 245, 0.55), transparent 68%);
}
.blob-b {
  width: 380px;
  height: 380px;
  bottom: -110px;
  right: -80px;
  background: radial-gradient(circle at 60% 60%, rgba(180, 235, 215, 0.5), transparent 68%);
}

.login-card {
  position: relative;
  z-index: 1;
  width: 428px;
  max-width: 100%;
  padding: 34px 32px;
  border-radius: 30px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out), var(--clay-inset-hi);
}

.brand {
  display: flex;
  align-items: center;
  gap: 15px;
  margin-bottom: 24px;
}
.brand-badge {
  width: 60px;
  height: 60px;
  border-radius: 20px;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(150deg, #56b0ea, #2f8fd6);
  box-shadow:
    6px 6px 15px rgba(24, 108, 170, 0.4),
    -3px -3px 9px rgba(255, 255, 255, 0.5),
    inset 2px 2px 5px rgba(255, 255, 255, 0.45),
    inset -3px -3px 6px rgba(18, 90, 150, 0.35);
  flex-shrink: 0;
}
.brand-text {
  display: flex;
  flex-direction: column;
  line-height: 1.25;
}
.brand-name {
  font-size: 24px;
  font-weight: 900;
  letter-spacing: 0.3px;
  color: var(--n-text-color, #22364a);
}
.brand-sub {
  font-size: 13px;
  font-weight: 600;
  color: #8aa0b4;
}
.mt {
  margin-top: 16px;
}
</style>

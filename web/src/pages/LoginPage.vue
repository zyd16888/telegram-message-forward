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
const tokenInput = ref('')
const bootstrapName = ref('admin')
const createdToken = ref('')

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
  if (!tokenInput.value.trim()) {
    message.warning('请输入管理 Token')
    return
  }
  loading.value = true
  try {
    await auth.login(tokenInput.value.trim())
    message.success('登录成功')
    enter()
  } catch (e) {
    message.error('登录失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function doBootstrap() {
  loading.value = true
  try {
    const created = await auth.bootstrap(bootstrapName.value.trim() || 'admin')
    createdToken.value = created.token
    message.success('已创建首个管理凭证，请立即妥善保存明文 Token')
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
          <n-alert type="info" title="首次使用：创建管理凭证">
            系统尚无任何管理凭证。创建后将生成一个明文 Token，仅显示一次，请妥善保存。
          </n-alert>
          <n-space vertical class="mt">
            <n-input v-model:value="bootstrapName" placeholder="凭证名称，如 admin" />
            <n-button type="primary" block :loading="loading" @click="doBootstrap">
              创建首个管理凭证
            </n-button>
          </n-space>

          <n-alert
            v-if="createdToken"
            class="mt"
            type="success"
            title="明文 Token（仅显示一次）"
          >
            <n-space vertical>
              <n-text code>{{ createdToken }}</n-text>
              <n-button type="primary" size="small" @click="enter">已保存，进入后台</n-button>
            </n-space>
          </n-alert>
        </div>

        <!-- 登录 -->
        <div v-else>
          <n-space vertical>
            <n-text depth="3">输入管理 Token 登录后台。</n-text>
            <n-input
              v-model:value="tokenInput"
              type="password"
              show-password-on="click"
              placeholder="管理 Token"
              @keyup.enter="doLogin"
            />
            <n-button type="primary" block :loading="loading" @click="doLogin">登录</n-button>
            <n-text depth="3" style="font-size: 12px">
              提示：Token 也可用 <n-text code>go run ./cmd/token create</n-text> 从命令行生成（运维兜底）。
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

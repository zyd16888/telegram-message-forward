import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  create as createNaiveUI,
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NCode,
  NConfigProvider,
  NDataTable,
  NDialogProvider,
  NDivider,
  NDrawer,
  NDrawerContent,
  NDynamicTags,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputGroup,
  NInputNumber,
  NLayout,
  NLayoutContent,
  NLayoutHeader,
  NLayoutSider,
  NMenu,
  NMessageProvider,
  NModal,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NTooltip,
} from 'naive-ui'
import App from './App.vue'
import { router } from './router'
import { setUnauthorizedHandler } from './api/client'
import { useAuthStore } from './stores/auth'
import './theme/clay.css'

const app = createApp(App)
const pinia = createPinia()
const naive = createNaiveUI({
  components: [
    NAlert,
    NButton,
    NCard,
    NCheckbox,
    NCode,
    NConfigProvider,
    NDataTable,
    NDialogProvider,
    NDivider,
    NDrawer,
    NDrawerContent,
    NDynamicTags,
    NEmpty,
    NForm,
    NFormItem,
    NInput,
    NInputGroup,
    NInputNumber,
    NLayout,
    NLayoutContent,
    NLayoutHeader,
    NLayoutSider,
    NMenu,
    NMessageProvider,
    NModal,
    NRadioButton,
    NRadioGroup,
    NSelect,
    NSpace,
    NSpin,
    NSwitch,
    NTabPane,
    NTabs,
    NTag,
    NText,
    NTooltip,
  ],
})

app.use(pinia)
app.use(router)
app.use(naive)

setUnauthorizedHandler(() => {
  const auth = useAuthStore(pinia)
  auth.saveToken('')
  auth.authenticated = false
  if (router.currentRoute.value.name !== 'login') {
    void router.push({ name: 'login' })
  }
})

app.mount('#app')

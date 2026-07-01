import { defineStore } from 'pinia'
import { getToken, setToken } from '@/api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken(),
  }),
  getters: {
    hasToken: (state) => state.token.length > 0,
  },
  actions: {
    save(token: string) {
      this.token = token
      setToken(token)
    },
    clear() {
      this.token = ''
      setToken('')
    },
  },
})

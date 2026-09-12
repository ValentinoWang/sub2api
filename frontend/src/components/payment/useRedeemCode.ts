import { ref } from 'vue'
import { redeemAPI, type RedeemResult } from '@/api/redeem'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { extractApiErrorMessage } from '@/utils/apiError'

export function useRedeemCode(fallbackError: () => string) {
  const authStore = useAuthStore()
  const subscriptionStore = useSubscriptionStore()
  const code = ref('')
  const submitting = ref(false)
  const error = ref('')
  const refreshWarning = ref(false)
  const result = ref<RedeemResult | null>(null)

  async function redeem(): Promise<RedeemResult | null> {
    if (submitting.value || !code.value.trim()) return null

    submitting.value = true
    error.value = ''
    refreshWarning.value = false
    result.value = null

    try {
      result.value = await redeemAPI.redeem(code.value.trim())
      code.value = ''
    } catch (err) {
      error.value = extractApiErrorMessage(err, fallbackError())
      submitting.value = false
      return null
    }

    // The server result is final. Refresh failures must not reclassify a committed redemption.
    try {
      await authStore.refreshUser()
      if (result.value.type === 'subscription') {
        await subscriptionStore.fetchActiveSubscriptions(true)
      }
    } catch {
      refreshWarning.value = true
    } finally {
      submitting.value = false
    }

    return result.value
  }

  return { code, submitting, error, refreshWarning, result, redeem }
}

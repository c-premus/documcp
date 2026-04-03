<script setup lang="ts">
import { ref } from 'vue'
import { Dialog, DialogPanel, DialogTitle } from '@headlessui/vue'
import { PlusIcon, XMarkIcon } from '@heroicons/vue/24/outline'
import { toast } from 'vue-sonner'

interface CreatedClient {
  readonly id: number
  readonly client_id: string
  readonly client_secret: string
  readonly client_name: string
}

interface CreateResponse {
  readonly data: CreatedClient
  readonly message: string
}

const GRANT_TYPE_OPTIONS = [
  { value: 'authorization_code', label: 'Authorization Code' },
  { value: 'client_credentials', label: 'Client Credentials' },
  { value: 'device_code', label: 'Device Code' },
  { value: 'refresh_token', label: 'Refresh Token' },
] as const

const AUTH_METHOD_OPTIONS = [
  { value: 'client_secret_post', label: 'Client Secret Post' },
  { value: 'client_secret_basic', label: 'Client Secret Basic' },
  { value: 'none', label: 'None (Public Client)' },
] as const

defineProps<{
  readonly open: boolean
}>()

const emit = defineEmits<{
  close: []
  created: [payload: { clientId: string; clientSecret: string }]
}>()

const clientName = ref('')
const redirectUris = ref<string[]>([''])
const grantTypes = ref<string[]>([])
const authMethod = ref('client_secret_post')
const scope = ref('')
const submitting = ref(false)
const error = ref<string | null>(null)

function resetForm(): void {
  clientName.value = ''
  redirectUris.value = ['']
  grantTypes.value = []
  authMethod.value = 'client_secret_post'
  scope.value = ''
  submitting.value = false
  error.value = null
}

function addRedirectUri(): void {
  redirectUris.value.push('')
}

function removeRedirectUri(index: number): void {
  if (redirectUris.value.length > 1) {
    redirectUris.value.splice(index, 1)
  }
}

function updateRedirectUri(index: number, value: string): void {
  redirectUris.value[index] = value
}

function toggleGrantType(grantType: string): void {
  const idx = grantTypes.value.indexOf(grantType)
  if (idx === -1) {
    grantTypes.value.push(grantType)
  } else {
    grantTypes.value.splice(idx, 1)
  }
}

function validate(): boolean {
  if (clientName.value.trim() === '') {
    error.value = 'Client name is required'
    return false
  }
  if (grantTypes.value.length === 0) {
    error.value = 'At least one grant type is required'
    return false
  }
  error.value = null
  return true
}

async function handleSubmit(): Promise<void> {
  if (!validate()) {
    return
  }

  submitting.value = true
  error.value = null

  const filteredUris = redirectUris.value.map((uri) => uri.trim()).filter((uri) => uri !== '')

  const body = JSON.stringify({
    client_name: clientName.value.trim(),
    redirect_uris: filteredUris,
    grant_types: grantTypes.value,
    token_endpoint_auth_method: authMethod.value,
    scope: scope.value.trim(),
  })

  try {
    const res = await fetch('/api/admin/oauth-clients', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
    })
    if (!res.ok) {
      const responseBody = await res.json().catch(() => ({ message: res.statusText }))
      throw new Error(responseBody.message || res.statusText)
    }
    const response = (await res.json()) as CreateResponse
    toast.success(`Client "${response.data.client_name}" created`)
    emit('created', {
      clientId: response.data.client_id,
      clientSecret: response.data.client_secret,
    })
    resetForm()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'An unexpected error occurred'
  } finally {
    submitting.value = false
  }
}

function handleClose(): void {
  resetForm()
  emit('close')
}
</script>

<template>
  <Dialog :open="open" class="relative z-50" @close="handleClose">
    <div class="fixed inset-0 bg-overlay backdrop-blur-sm transition-opacity" aria-hidden="true" />

    <div class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        <DialogPanel
          class="relative transform overflow-hidden rounded-lg bg-bg-surface px-4 pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6"
        >
          <DialogTitle as="h3" class="text-base font-semibold text-text-primary mb-4">
            Create OAuth Client
          </DialogTitle>

          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <!-- Client Name -->
              <div>
                <label for="client-name" class="block text-sm font-medium text-text-secondary">
                  Client Name
                </label>
                <input
                  id="client-name"
                  v-model="clientName"
                  type="text"
                  required
                  :aria-describedby="error ? 'oauth-client-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
              </div>

              <!-- Redirect URIs -->
              <div>
                <label class="block text-sm font-medium text-text-secondary mb-1">
                  Redirect URIs
                </label>
                <div class="space-y-2">
                  <div
                    v-for="(uri, index) in redirectUris"
                    :key="index"
                    class="flex items-center gap-2"
                  >
                    <input
                      :value="uri"
                      type="url"
                      placeholder="https://example.com/callback"
                      class="block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                      @input="updateRedirectUri(index, ($event.target as HTMLInputElement).value)"
                    />
                    <button
                      v-if="redirectUris.length > 1"
                      type="button"
                      class="shrink-0 text-text-disabled hover:text-red-600 dark:hover:text-red-400"
                      aria-label="Remove redirect URI"
                      @click="removeRedirectUri(index)"
                    >
                      <XMarkIcon class="h-5 w-5" />
                    </button>
                  </div>
                </div>
                <button
                  type="button"
                  class="mt-2 inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400 dark:hover:text-indigo-300"
                  @click="addRedirectUri"
                >
                  <PlusIcon class="h-4 w-4" />
                  Add URI
                </button>
              </div>

              <!-- Grant Types -->
              <fieldset>
                <legend class="block text-sm font-medium text-text-secondary mb-1">
                  Grant Types
                </legend>
                <div class="space-y-2">
                  <label
                    v-for="option in GRANT_TYPE_OPTIONS"
                    :key="option.value"
                    class="flex items-center gap-2"
                  >
                    <input
                      type="checkbox"
                      :checked="grantTypes.includes(option.value)"
                      class="h-4 w-4 rounded border-border-input text-indigo-600 focus:ring-indigo-600 dark:focus:ring-indigo-400 dark:bg-bg-surface-alt"
                      @change="toggleGrantType(option.value)"
                    />
                    <span class="text-sm text-text-secondary">{{ option.label }}</span>
                  </label>
                </div>
              </fieldset>

              <!-- Token Endpoint Auth Method -->
              <div>
                <label for="auth-method" class="block text-sm font-medium text-text-secondary">
                  Token Endpoint Auth Method
                </label>
                <select
                  id="auth-method"
                  v-model="authMethod"
                  :aria-describedby="error ? 'oauth-client-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                >
                  <option
                    v-for="option in AUTH_METHOD_OPTIONS"
                    :key="option.value"
                    :value="option.value"
                  >
                    {{ option.label }}
                  </option>
                </select>
              </div>

              <!-- Scope -->
              <div>
                <label for="client-scope" class="block text-sm font-medium text-text-secondary">
                  Scope
                </label>
                <input
                  id="client-scope"
                  v-model="scope"
                  type="text"
                  placeholder="read write"
                  :aria-describedby="error ? 'oauth-client-form-error' : undefined"
                  class="mt-1 block w-full rounded-md border-border-input bg-bg-surface text-text-primary shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:focus:border-indigo-400 dark:focus:ring-indigo-400 sm:text-sm"
                />
                <p class="mt-1 text-xs text-text-muted">
                  Space-separated list of scopes (optional)
                </p>
              </div>
            </div>

            <p
              v-if="error"
              id="oauth-client-form-error"
              role="alert"
              class="mt-3 text-sm text-red-600 dark:text-red-400"
            >
              {{ error }}
            </p>

            <div class="mt-5 flex justify-end gap-3">
              <button
                type="button"
                class="inline-flex justify-center rounded-md bg-bg-surface px-3 py-2 text-sm font-semibold text-text-primary shadow-sm ring-1 ring-inset ring-border-input hover:bg-bg-hover"
                @click="handleClose"
              >
                Cancel
              </button>
              <button
                type="submit"
                :disabled="submitting"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <template v-if="submitting">Creating...</template>
                <template v-else>Create</template>
              </button>
            </div>
          </form>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>

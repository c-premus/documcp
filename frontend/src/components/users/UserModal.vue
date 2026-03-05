<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import {
  Dialog,
  DialogPanel,
  DialogTitle,
  Switch,
} from '@headlessui/vue'
import { toast } from 'vue-sonner'

interface User {
  readonly id: number
  readonly name: string
  readonly email: string
  readonly oidc_sub: string
  readonly oidc_provider: string
  readonly is_admin: boolean
  readonly created_at: string
  readonly updated_at: string
}

interface SingleResponse {
  readonly data: User
}

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

const props = defineProps<{
  readonly open: boolean
  readonly user?: User | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const name = ref('')
const email = ref('')
const isAdmin = ref(false)
const submitting = ref(false)
const error = ref<string | null>(null)

const isEditMode = computed(() => props.user !== null && props.user !== undefined)
const dialogTitle = computed(() => isEditMode.value ? 'Edit User' : 'Create User')
const submitLabel = computed(() => isEditMode.value ? 'Save' : 'Create')

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    if (props.user !== null && props.user !== undefined) {
      name.value = props.user.name
      email.value = props.user.email
      isAdmin.value = props.user.is_admin
    } else {
      name.value = ''
      email.value = ''
      isAdmin.value = false
    }
    submitting.value = false
    error.value = null
  }
})

function validate(): boolean {
  if (name.value.trim() === '') {
    error.value = 'Name is required'
    return false
  }
  if (email.value.trim() === '') {
    error.value = 'Email is required'
    return false
  }
  if (!EMAIL_REGEX.test(email.value.trim())) {
    error.value = 'Please enter a valid email address'
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

  const body = JSON.stringify({
    name: name.value.trim(),
    email: email.value.trim(),
    is_admin: isAdmin.value,
  })

  const headers = { 'Content-Type': 'application/json' }

  try {
    if (isEditMode.value && props.user !== null && props.user !== undefined) {
      await apiFetch<SingleResponse>(`/api/admin/users/${props.user.id}`, {
        method: 'PUT',
        headers,
        body,
      })
      toast.success(`User "${name.value.trim()}" updated`)
    } else {
      await apiFetch<SingleResponse>('/api/admin/users', {
        method: 'POST',
        headers,
        body,
      })
      toast.success(`User "${name.value.trim()}" created`)
    }
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'An unexpected error occurred'
  } finally {
    submitting.value = false
  }
}

async function apiFetch<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)
  if (!res.ok) {
    const responseBody = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(responseBody.message || res.statusText)
  }
  return res.json() as Promise<T>
}
</script>

<template>
  <Dialog :open="open" class="relative z-50" @close="emit('close')">
    <div class="fixed inset-0 bg-gray-500/75 backdrop-blur-sm transition-opacity" aria-hidden="true" />

    <div class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        <DialogPanel
          class="relative transform overflow-hidden rounded-lg bg-white px-4 pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6"
        >
          <DialogTitle as="h3" class="text-base font-semibold text-gray-900 mb-4">
            {{ dialogTitle }}
          </DialogTitle>

          <form @submit.prevent="handleSubmit">
            <div class="space-y-4">
              <div>
                <label for="user-name" class="block text-sm font-medium text-gray-700">Name</label>
                <input
                  id="user-name"
                  v-model="name"
                  type="text"
                  required
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                />
              </div>

              <div>
                <label for="user-email" class="block text-sm font-medium text-gray-700">Email</label>
                <input
                  id="user-email"
                  v-model="email"
                  type="email"
                  required
                  class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                />
              </div>

              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700">Admin</span>
                <Switch
                  v-model="isAdmin"
                  :class="isAdmin ? 'bg-indigo-600' : 'bg-gray-200'"
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-600 focus-visible:ring-offset-2"
                >
                  <span
                    :class="isAdmin ? 'translate-x-5' : 'translate-x-0'"
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  />
                </Switch>
              </div>
            </div>

            <p v-if="error" class="mt-3 text-sm text-red-600">{{ error }}</p>

            <div class="mt-5 flex justify-end gap-3">
              <button
                type="button"
                class="inline-flex justify-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50"
                @click="emit('close')"
              >
                Cancel
              </button>
              <button
                type="submit"
                :disabled="submitting"
                class="inline-flex justify-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <template v-if="submitting">Saving...</template>
                <template v-else>{{ submitLabel }}</template>
              </button>
            </div>
          </form>
        </DialogPanel>
      </div>
    </div>
  </Dialog>
</template>

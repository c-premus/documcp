import { useSSE } from '@/composables/useSSE'
import { useDocumentsStore } from '@/stores/documents'
import { useNotificationsStore } from '@/stores/notifications'
import { toast } from 'vue-sonner'

export function useDocumentEvents() {
  const { connect, on, connected } = useSSE()
  const documents = useDocumentsStore()
  const notifications = useNotificationsStore()

  function start() {
    connect()

    on('job.completed', (event) => {
      notifications.addEvent(event)

      if (event.job_kind === 'document_extract') {
        toast.success('Document extracted successfully')
        documents.refreshCurrent()
      }
      if (event.job_kind === 'document_index') {
        toast.success('Document indexed successfully')
        documents.refreshCurrent()
      }
    })

    on('job.failed', (event) => {
      notifications.addEvent(event)

      if (event.job_kind.startsWith('document_')) {
        toast.error(`Document processing failed: ${event.error ?? 'Unknown error'}`)
      }
    })

    on('job.snoozed', (event) => {
      notifications.addEvent(event)
    })
  }

  return { start, connected }
}

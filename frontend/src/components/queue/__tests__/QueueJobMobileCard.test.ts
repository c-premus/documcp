import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import QueueJobMobileCard from '@/components/queue/QueueJobMobileCard.vue'
import type { FailedJob } from '@/stores/queue'

const LONG_ERROR =
  'connection refused at tcp://example.internal:5432 after 3 retries; pgxpool exhausted; ' +
  'last successful ping was 14 minutes ago'

const JOB: FailedJob = {
  id: 1234,
  kind: 'ProcessDocument',
  queue: 'default',
  state: 'discarded',
  attempt: 3,
  max_attempts: 5,
  created_at: '2026-04-25T00:00:00Z',
  errors: [{ at: '2026-04-25T00:00:01Z', error: LONG_ERROR }],
}

function mountCard(overrides: Partial<FailedJob> = {}) {
  return mount(QueueJobMobileCard, { props: { job: { ...JOB, ...overrides } } })
}

describe('QueueJobMobileCard', () => {
  it('renders the job kind and id', () => {
    const wrapper = mountCard()
    expect(wrapper.get('h3').text()).toBe('ProcessDocument')
    expect(wrapper.text()).toContain('#1234')
  })

  it('shows queue, attempts, and the state badge', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('queue: default')
    expect(wrapper.text()).toContain('3/5 attempts')
    const statuses = wrapper.findAll('[role="status"]').map((s) => s.text())
    expect(statuses).toContain('discarded')
  })

  it('renders the full error message in a role="alert" region (not truncated)', () => {
    const wrapper = mountCard()
    const alert = wrapper.get('[role="alert"]')
    expect(alert.text()).toBe(LONG_ERROR)
  })

  it('omits the alert region when there are no errors', () => {
    const wrapper = mountCard({ errors: [] })
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('exposes retry + delete buttons labeled with kind and id', () => {
    const wrapper = mountCard()
    const labels = wrapper.findAll('button').map((b) => b.attributes('aria-label'))
    expect(labels).toContain('Retry job ProcessDocument 1234')
    expect(labels).toContain('Delete job ProcessDocument 1234')
  })

  it('emits retry / delete with the job payload', async () => {
    const wrapper = mountCard()
    await wrapper.get('[aria-label="Retry job ProcessDocument 1234"]').trigger('click')
    await wrapper.get('[aria-label="Delete job ProcessDocument 1234"]').trigger('click')

    expect(wrapper.emitted('retry')![0]).toEqual([JOB])
    expect(wrapper.emitted('delete')![0]).toEqual([JOB])
  })
})

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import QueueJobRowActions from '@/components/queue/QueueJobRowActions.vue'
import type { FailedJob } from '@/stores/queue'

const JOB: FailedJob = {
  id: 123,
  kind: 'document.extract',
  queue: 'default',
  state: 'discarded',
  attempt: 3,
  max_attempts: 3,
  args: {},
  errors: [],
  created_at: '2026-04-01T00:00:00Z',
  attempted_at: null,
  finalized_at: null,
  metadata: null,
}

describe('QueueJobRowActions', () => {
  it('retry button aria-label identifies the job', () => {
    const wrapper = mount(QueueJobRowActions, { props: { job: JOB } })
    const retry = wrapper.find('[aria-label="Retry job document.extract 123"]')
    expect(retry.exists()).toBe(true)
  })

  it('delete button aria-label identifies the job', () => {
    const wrapper = mount(QueueJobRowActions, { props: { job: JOB } })
    const del = wrapper.find('[aria-label="Delete job document.extract 123"]')
    expect(del.exists()).toBe(true)
  })

  it('emits retry with the job when retry is clicked', async () => {
    const wrapper = mount(QueueJobRowActions, { props: { job: JOB } })
    await wrapper.find('[aria-label="Retry job document.extract 123"]').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    expect(wrapper.emitted('retry')![0]).toEqual([JOB])
  })

  it('emits delete with the job when delete is clicked', async () => {
    const wrapper = mount(QueueJobRowActions, { props: { job: JOB } })
    await wrapper.find('[aria-label="Delete job document.extract 123"]').trigger('click')
    expect(wrapper.emitted('delete')).toHaveLength(1)
    expect(wrapper.emitted('delete')![0]).toEqual([JOB])
  })

  it('stops click propagation so row click handlers do not fire', async () => {
    let rowClicked = false
    const wrapper = mount(
      {
        components: { QueueJobRowActions },
        props: ['job'],
        template:
          '<div @click="onRow"><QueueJobRowActions :job="job" @retry="() => {}" @delete="() => {}"/></div>',
        methods: {
          onRow() {
            rowClicked = true
          },
        },
      },
      { props: { job: JOB } },
    )
    await wrapper.find('[aria-label="Retry job document.extract 123"]').trigger('click')
    await wrapper.find('[aria-label="Delete job document.extract 123"]').trigger('click')
    expect(rowClicked).toBe(false)
  })
})

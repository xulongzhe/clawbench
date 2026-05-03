import { describe, expect, it, vi, beforeEach } from 'vitest'

// ────────────────────────────────────────────────────────────
// Pending message queue logic (extracted from ChatPanel.vue)
// The actual state lives inside the component. We replicate the
// queue state machine logic here for isolated testing.
//
// Key rules:
// - enqueueMessage: adds message to pendingMessages while loading
// - consumeQueue: called after onStreamEnd('done'), sends next message
// - onStreamEnd('cancelled'): clears the queue
// - onStreamEnd('error'): preserves the queue (user decides)
// - cleanupActiveStream (session switch/delete): clears the queue
// ────────────────────────────────────────────────────────────

interface PendingMessage {
  text: string
  filePaths: string[]
  files: string[]
  createdAt: string
}

function createQueueMachine() {
  const queue: PendingMessage[] = []
  const sentMessages: { text: string; filePaths: string[]; files: string[] }[] = []
  let loading = false

  function setLoading(val: boolean) { loading = val }

  function enqueueMessage(text: string, filePaths: string[] = [], files: string[] = []) {
    queue.push({ text, filePaths, files, createdAt: new Date().toISOString() })
  }

  function consumeQueue() {
    if (queue.length === 0) return
    if (loading) return // safety
    const next = queue.shift()!
    sentMessages.push({ text: next.text, filePaths: next.filePaths, files: next.files })
    loading = true // sending a message starts loading
  }

  function onStreamEnd(reason: 'done' | 'cancelled' | 'error') {
    loading = false
    if (reason === 'done') {
      consumeQueue()
    } else if (reason === 'cancelled') {
      queue.length = 0
    }
    // 'error': don't touch the queue
  }

  function cleanupActiveStream() {
    loading = false
    queue.length = 0
  }

  function getQueue() { return queue }
  function getSentMessages() { return sentMessages }
  function getLoading() { return loading }
  function getQueueLength() { return queue.length }

  return { setLoading, enqueueMessage, consumeQueue, onStreamEnd, cleanupActiveStream, getQueue, getSentMessages, getLoading, getQueueLength }
}

describe('pending-message-queue', () => {
  let machine: ReturnType<typeof createQueueMachine>

  beforeEach(() => {
    machine = createQueueMachine()
  })

  // ── Enqueue while loading ──
  it('enqueues message while loading', () => {
    machine.setLoading(true)
    machine.enqueueMessage('hello')
    expect(machine.getQueueLength()).toBe(1)
    expect(machine.getQueue()[0].text).toBe('hello')
  })

  it('enqueues multiple messages in order', () => {
    machine.setLoading(true)
    machine.enqueueMessage('first')
    machine.enqueueMessage('second')
    machine.enqueueMessage('third')
    expect(machine.getQueueLength()).toBe(3)
    expect(machine.getQueue().map(m => m.text)).toEqual(['first', 'second', 'third'])
  })

  // ── Consume queue on stream done ──
  it('consumes first message on stream done', () => {
    machine.setLoading(true)
    machine.enqueueMessage('hello')
    machine.onStreamEnd('done')
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getSentMessages()).toEqual([{ text: 'hello', filePaths: [], files: [] }])
  })

  it('consumes messages sequentially on multiple done events', () => {
    machine.setLoading(true)
    machine.enqueueMessage('first')
    machine.enqueueMessage('second')

    // First done → consume 'first'
    machine.onStreamEnd('done')
    expect(machine.getQueueLength()).toBe(1)
    expect(machine.getSentMessages().map(m => m.text)).toEqual(['first'])

    // Second done → consume 'second'
    machine.onStreamEnd('done')
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getSentMessages().map(m => m.text)).toEqual(['first', 'second'])
  })

  it('does nothing on done when queue is empty', () => {
    machine.setLoading(true)
    machine.onStreamEnd('done')
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getSentMessages()).toEqual([])
  })

  // ── Cancel clears queue ──
  it('cancelled clears the entire queue', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.enqueueMessage('msg2')
    machine.onStreamEnd('cancelled')
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getSentMessages()).toEqual([])
  })

  it('cancelled then done does not send anything', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.onStreamEnd('cancelled') // clears queue
    machine.onStreamEnd('done')      // nothing to consume
    expect(machine.getSentMessages()).toEqual([])
  })

  // ── Error preserves queue ──
  it('error preserves the queue', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.enqueueMessage('msg2')
    machine.onStreamEnd('error')
    expect(machine.getQueueLength()).toBe(2)
    expect(machine.getSentMessages()).toEqual([])
  })

  it('after error, done consumes the queue', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.onStreamEnd('error')  // queue preserved
    machine.onStreamEnd('done')   // now consume
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getSentMessages().map(m => m.text)).toEqual(['msg1'])
  })

  // ── Session switch clears queue ──
  it('cleanupActiveStream clears the queue', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.enqueueMessage('msg2')
    machine.cleanupActiveStream()
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getLoading()).toBe(false)
  })

  // ── Queue with file attachments ──
  it('enqueues messages with file paths', () => {
    machine.setLoading(true)
    machine.enqueueMessage('look at this', ['/tmp/file.go'], ['/tmp/file.go'])
    const msg = machine.getQueue()[0]
    expect(msg.text).toBe('look at this')
    expect(msg.filePaths).toEqual(['/tmp/file.go'])
    expect(msg.files).toEqual(['/tmp/file.go'])
  })

  it('file attachments are preserved through consume', () => {
    machine.setLoading(true)
    machine.enqueueMessage('check file', ['/src/main.go'], ['/src/main.go'])
    machine.onStreamEnd('done')
    expect(machine.getSentMessages()[0].filePaths).toEqual(['/src/main.go'])
    expect(machine.getSentMessages()[0].files).toEqual(['/src/main.go'])
  })

  // ── Safety: consumeQueue while loading is no-op ──
  it('consumeQueue is no-op while loading', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.setLoading(true) // simulate: still loading
    machine.consumeQueue()   // should be no-op
    expect(machine.getQueueLength()).toBe(1)
    expect(machine.getSentMessages()).toEqual([])
  })

  // ── Full lifecycle ──
  it('full lifecycle: enqueue → done → send → done → empty', () => {
    machine.setLoading(true)
    machine.enqueueMessage('question 1')
    machine.enqueueMessage('question 2')

    // AI finishes first response
    machine.onStreamEnd('done')
    expect(machine.getSentMessages().length).toBe(1)
    expect(machine.getSentMessages()[0].text).toBe('question 1')
    expect(machine.getLoading()).toBe(true) // sending next message

    // AI finishes second response
    machine.onStreamEnd('done')
    expect(machine.getSentMessages().length).toBe(2)
    expect(machine.getSentMessages()[1].text).toBe('question 2')

    // Queue is now empty, next done is no-op
    machine.onStreamEnd('done')
    expect(machine.getSentMessages().length).toBe(2) // no extra send
  })

  // ── Cancel mid-queue ──
  it('cancel mid-queue clears remaining messages', () => {
    machine.setLoading(true)
    machine.enqueueMessage('msg1')
    machine.enqueueMessage('msg2')
    machine.enqueueMessage('msg3')

    // First done → send msg1
    machine.onStreamEnd('done')
    expect(machine.getSentMessages().map(m => m.text)).toEqual(['msg1'])

    // User cancels while msg2 and msg3 are still queued
    machine.onStreamEnd('cancelled')
    expect(machine.getQueueLength()).toBe(0)
    expect(machine.getSentMessages().length).toBe(1) // only msg1 was sent
  })

  // ── Error then user retries then done ──
  it('error → user sends new message → done consumes both', () => {
    machine.setLoading(true)
    machine.enqueueMessage('original msg')
    machine.onStreamEnd('error')  // queue preserved
    expect(machine.getQueueLength()).toBe(1)

    // User sends a new message while in error state (loading=false)
    machine.enqueueMessage('retry msg')
    expect(machine.getQueueLength()).toBe(2)

    // Done consumes first
    machine.onStreamEnd('done')
    expect(machine.getQueueLength()).toBe(1)
    expect(machine.getSentMessages()[0].text).toBe('original msg')
  })
})

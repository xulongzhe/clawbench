/**
 * Pure functions extracted from useChatSession composable.
 * These have no Vue reactivity dependencies and can be tested in isolation.
 */

/**
 * Build a lightweight fingerprint of messages for change detection.
 * Used by polling to skip UI refresh when data is unchanged.
 */
export function buildMessageSnapshot(rawMsgs: any[]): string {
  return rawMsgs.map(m =>
    `${m.id ?? ''}:${m.role}:${(m.content || '').length}:${m.createdAt || ''}:${m.streaming ? 1 : 0}`
  ).join('|')
}

/**
 * Parse raw message objects from API into the format used by the UI.
 * Adds blocks, metadata, cancelled, fromDB fields as needed.
 */
export function parseMessages(
  rawMsgs: any[],
  onParseAssistantContent: (content: string) => any
): any[] {
  return rawMsgs.map(msg => {
    if (msg.role === 'assistant') {
      const { blocks, metadata, cancelled } = onParseAssistantContent(msg.content)
      msg.blocks = blocks
      if (metadata) msg.metadata = metadata
      if (cancelled) msg.cancelled = cancelled
      if (msg.streaming) { msg.streaming = true; msg.fromDB = true }
      // Auto-show summary for messages that have a non-empty summary
      if (msg.summary != null && msg.summary !== '') {
        msg.showingSummary = true
      } else {
        msg.showingSummary = false
      }
    } else if (msg.role === 'user' && !msg.blocks) {
      // User messages also use ContentBlocks for unified rendering & auto-collapse
      msg.blocks = msg.content ? [{ type: 'text', text: msg.content }] : []
    }
    return msg
  })
}

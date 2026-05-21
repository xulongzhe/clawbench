import { ref } from 'vue'
import { escapeHtml } from '@/utils/html.ts'
import { useAppMode } from '@/composables/useAppMode.ts'
import { usePortForward } from '@/composables/usePortForward.ts'
import { useToast } from '@/composables/useToast.ts'
import { gt } from '@/composables/useLocale'

/**
 * Localhost URL annotation composable.
 *
 * Detects localhost URLs (http://localhost:PORT, https://localhost:PORT,
 * http://127.0.0.1:PORT, https://127.0.0.1:PORT) in rendered chat HTML
 * and appends a clickable EthernetPort icon button after them.
 *
 * The <a> tag itself is preserved intact; a small button is appended after it,
 * exactly like the file-path open button pattern in useFilePathAnnotation.ts.
 * Clicking either the <a> link or the icon button triggers the same
 * port-forward-and-open-WebView flow.
 */

/** Regex to match bare localhost URLs in text (not inside <a> tags) */
const LOCALHOST_URL_RE = /https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)(\/[^\s<>"')\]]*)?/gi

/**
 * Check if an href points to a localhost address.
 */
export function isLocalhostUrl(href: string): boolean {
    return /^https?:\/\/(?:localhost|127\.0\.0\.1):\d+/i.test(href)
}

/**
 * Parse a localhost URL into its components.
 * Returns null if not a localhost URL.
 */
export function parseLocalhostUrl(url: string): { port: number; protocol: string; fullUrl: string } | null {
    const match = url.match(/^((https?):\/\/(?:localhost|127\.0\.0\.1):(\d+))/i)
    if (!match) return null
    return {
        port: parseInt(match[3]),
        protocol: match[2].toLowerCase(),
        fullUrl: match[1],
    }
}

/**
 * SVG icon markup for the localhost open button (EthernetPort icon from lucide).
 * Same pattern as FILE_OPEN_ICON_SVG in useFilePathAnnotation.ts.
 */
export const LOCALHOST_OPEN_ICON_SVG = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><path d="m15 20 3-3h2a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h2l3 3z"/><path d="M6 8v1"/><path d="M10 8v1"/><path d="M14 8v1"/><path d="M18 8v1"/></svg>'

/**
 * Generate HTML for the localhost open button (EthernetPort icon).
 * Same pattern as fileOpenButtonHtml() in useFilePathAnnotation.ts.
 */
export function localhostOpenButtonHtml(port: number, protocol: string, url: string): string {
    return `<button class="chat-url-open-btn" data-url="${escapeHtml(url)}" data-port="${port}" data-protocol="${escapeHtml(protocol)}" title="Open in WebView">${LOCALHOST_OPEN_ICON_SVG}</button>`
}

/**
 * Detect localhost URLs in rendered HTML and append clickable icon buttons.
 *
 * Three cases:
 * 1. <a> tags with localhost hrefs → keep <a> intact, append icon button after
 * 2. Bare localhost URLs in text → wrap in <a>, append icon button after
 * 3. <code> tags containing a localhost URL → wrap <code> in <a>, append icon button after
 *    (unlike file paths, localhost URLs in backticks are meant to be clickable)
 *
 * Returns the annotated HTML.
 */
export function annotateLocalhostUrls(html: string): string {
    if (!html) return html

    // Only annotate in App mode — web mode can access localhost directly,
    // no port forwarding or built-in WebView needed
    const { isAppMode } = useAppMode()
    if (!isAppMode.value) return html

    // Skip annotation when SSH is disabled (no port forwarding available)
    const { sshInfo } = usePortForward()
    if (sshInfo.value?.enabled === false) return html

    // Protect <pre> blocks from annotation (code blocks should not get buttons)
    const preBlocks: string[] = []
    html = html.replace(/<pre[^>]*>[\s\S]*?<\/pre>/gi, (match) => {
        preBlocks.push(match)
        return `<!--PREBLOCK_LOCALHOST${preBlocks.length - 1}-->`
    })

    // Protect <a> blocks from the bare-URL regex (same pattern as useFilePathAnnotation)
    // so that URLs inside <a> tags are NOT matched by the bare-URL step
    const aBlocks: string[] = []
    html = html.replace(/<a\s+[^>]*href="[^"]*"[^>]*>[\s\S]*?<\/a>/gi, (match) => {
        aBlocks.push(match)
        return `<!--ABLOCK_LOCALHOST${aBlocks.length - 1}-->`
    })

    // Protect <code> blocks from the bare-URL regex, then annotate localhost URLs
    // inside them during restore (unlike file paths, localhost URLs in inline code
    // are typically meant to be clickable — e.g. `http://localhost:20003`)
    const codeBlocks: string[] = []
    html = html.replace(/<code[^>]*>[\s\S]*?<\/code>/gi, (match) => {
        codeBlocks.push(match)
        return `<!--CODEBLOCK_LOCALHOST${codeBlocks.length - 1}-->`
    })

    // Step 1: For bare localhost URLs in text (not inside <a> or <code> tags),
    // wrap them in <a> and append icon button
    html = html.replace(LOCALHOST_URL_RE, (url, portStr) => {
        const port = parseInt(portStr)
        if (port <= 0 || port > 65535) return url
        const protocol = url.startsWith('https') ? 'https' : 'http'
        const linkHtml = `<a href="${escapeHtml(url)}" target="_blank" rel="noopener">${escapeHtml(url)}</a>`
        return `${linkHtml}${localhostOpenButtonHtml(port, protocol, url)}`
    })

    // Restore <code> blocks — annotate localhost URLs inside inline code
    // (unlike file paths, localhost URLs in backticks are meant to be clickable)
    html = html.replace(/<!--CODEBLOCK_LOCALHOST(\d+)-->/g, (_, idx) => {
        let match = codeBlocks[parseInt(idx)]
        // Check if the <code> content is a localhost URL
        const codeContent = match.replace(/<code[^>]*>/i, '').replace(/<\/code>/i, '')
        const parsed = parseLocalhostUrl(codeContent.trim())
        if (parsed) {
            // Replace the <code>localhost-url</code> with <a> + button
            // Keep the <code> styling for visual consistency but wrap in <a>
            match = `<a href="${escapeHtml(parsed.fullUrl)}" target="_blank" rel="noopener">${match}</a>${localhostOpenButtonHtml(parsed.port, parsed.protocol, parsed.fullUrl)}`
        }
        return match
    })

    // Restore <a> blocks and append icon button to localhost <a> tags
    html = html.replace(/<!--ABLOCK_LOCALHOST(\d+)-->/g, (_, idx) => {
        const match = aBlocks[parseInt(idx)]
        // Extract href from the <a> tag
        const hrefMatch = match.match(/href="([^"]*)"/)
        if (!hrefMatch) return match
        const href = hrefMatch[1]
        const parsed = parseLocalhostUrl(href)
        if (!parsed) return match
        // Keep the <a> tag as-is, append the icon button after
        return `${match}${localhostOpenButtonHtml(parsed.port, parsed.protocol, href)}`
    })

    // Restore <pre> blocks
    html = html.replace(/<!--PREBLOCK_LOCALHOST(\d+)-->/g, (_, idx) => preBlocks[parseInt(idx)])

    return html
}

/**
 * Composable for handling localhost URL click events in rendered chat HTML.
 *
 * Provides two reusable functions that encapsulate the shared click-handling logic
 * used by ChatMessageList, ToolDetailOverlay, and TaskExecDetail:
 *
 * - handleLocalhostUrlClick(event): inspects a click event for .chat-url-open-btn
 *   or <a href="localhost:..."> targets, intercepts the default navigation, and
 *   opens via SSH tunnel + WebView instead. Returns true if the event was handled.
 *
 * - openLocalhostUrl(element, port, protocol): low-level async function that
 *   ensures a port is registered for forwarding and opens it in the WebView.
 *
 * Usage in a component:
 *   const { handleLocalhostUrlClick } = useLocalhostUrlClickHandler()
 *   // In your @click handler:
 *   if (handleLocalhostUrlClick(event)) return
 */
export function useLocalhostUrlClickHandler() {
    const { isAppMode } = useAppMode()
    const { ensurePortRegistered, openPort, sshInfo } = usePortForward()
    const toast = useToast()

    // Module-level guard to prevent double-clicks
    const urlOpening = ref(false)

    /**
     * Open a localhost URL: ensure port forwarding is set up, then open in WebView.
     */
    async function openLocalhostUrl(element: Element, port: number, protocol: string) {
        if (urlOpening.value) return
        if (sshInfo.value?.enabled === false) return
        urlOpening.value = true
        element.classList.add('loading')

        try {
            await ensurePortRegistered(port, protocol)
            openPort(port, protocol)
        } catch (err) {
            toast.show(gt('chat.localhost.openFailed'), { type: 'error' })
        } finally {
            urlOpening.value = false
            element.classList.remove('loading')
        }
    }

    /**
     * Handle a click event that may target a localhost URL element.
     *
     * Checks for two targets (App mode only):
     * 1. .chat-url-open-btn — the icon button appended after localhost URLs
     * 2. <a href="localhost:..."> — direct link clicks on localhost URLs
     *
     * If either is found, prevents default navigation, ensures the port is
     * registered for forwarding, and opens via SSH tunnel + WebView.
     *
     * Returns true if the click was handled (caller should stop processing).
     */
    function handleLocalhostUrlClick(event: MouseEvent): boolean {
        if (!isAppMode.value) return false

        const urlBtn = (event.target as Element).closest('.chat-url-open-btn')
        if (urlBtn) {
            event.preventDefault()
            event.stopPropagation()
            const port = parseInt(urlBtn.getAttribute('data-port') || '0')
            const protocol = urlBtn.getAttribute('data-protocol') || 'http'
            if (port > 0) {
                openLocalhostUrl(urlBtn, port, protocol)
            }
            return true
        }

        const anchor = (event.target as Element).closest('a[href]')
        if (anchor) {
            const href = anchor.getAttribute('href') || ''
            if (isLocalhostUrl(href)) {
                event.preventDefault()
                event.stopPropagation()
                const parsed = parseLocalhostUrl(href)
                if (parsed) {
                    openLocalhostUrl(anchor, parsed.port, parsed.protocol)
                }
                return true
            }
        }

        return false
    }

    return { handleLocalhostUrlClick, openLocalhostUrl }
}

/**
 * Composable for localhost URL annotation in rendered HTML (v-html content).
 */
export function useLocalhostAnnotation() {
    return {
        annotateLocalhostUrls,
        isLocalhostUrl,
        parseLocalhostUrl,
        localhostOpenButtonHtml,
    }
}

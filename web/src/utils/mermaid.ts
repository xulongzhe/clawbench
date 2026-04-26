// Mermaid diagram utilities
import { mermaid } from './globals.ts'

// Initialize Mermaid
export function initMermaid(): void {
    mermaid.initialize({
        startOnLoad: false,
        theme: document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'default',
        securityLevel: 'loose',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    })
}

// Re-render all rendered mermaid diagrams on the page (called after theme switch)
export function reRenderMermaid(): void {
    document.querySelectorAll<HTMLDivElement>('div.mermaid[data-mermaid]').forEach(container => {
        const source = container.dataset.mermaid
        if (!source) return
        const id = container.id || `mermaid-${Date.now()}`
        container.removeAttribute('id')
        mermaid.render(id, source).then(result => {
            container.innerHTML = result.svg
            container.id = id
        }).catch(() => {})
    })
}

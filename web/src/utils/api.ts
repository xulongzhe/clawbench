// API utility functions

export async function apiGet<T = unknown>(url: string): Promise<T> {
    const resp = await fetch(url)
    if (!resp.ok) throw new Error(await resp.text())
    return resp.json()
}

export async function apiPost<T = unknown>(url: string, body: unknown): Promise<T> {
    const resp = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    })
    const data = await resp.json().catch(() => ({})) as Record<string, unknown>
    if (!resp.ok) throw new Error(data.error ? String(data.error) : resp.statusText)
    return data as T
}

export async function apiDelete<T = unknown>(url: string): Promise<T> {
    const resp = await fetch(url, { method: 'DELETE' })
    if (!resp.ok) throw new Error(resp.statusText)
    return resp.json()
}

export async function cancelChat(sessionId: string): Promise<void> {
    const resp = await fetch(`/api/ai/chat/cancel?session_id=${encodeURIComponent(sessionId)}`, {
        method: 'POST',
    })
    if (!resp.ok) throw new Error(resp.statusText)
}

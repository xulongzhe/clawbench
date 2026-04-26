// Time and task formatting utilities

/** Format a date as relative time (Chinese locale) */
export function formatRelativeTime(date: string | Date): string {
    if (!date) return ''
    const d = new Date(date)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    const minutes = Math.floor(diff / 60000)
    const hours = Math.floor(diff / 3600000)
    const days = Math.floor(diff / 86400000)

    if (minutes < 1) return '刚刚'
    if (minutes < 60) return `${minutes}分钟前`
    if (hours < 24) return `${hours}小时前`
    if (days < 7) return `${days}天前`
    return d.toLocaleDateString('zh-CN')
}

/** Format a date as a localized datetime string (zh-CN) */
export function formatDateTime(date: string | Date): string {
    if (!date) return ''
    const d = new Date(date)
    return d.toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
    })
}

/** Humanize a cron expression into Chinese description */
export function humanizeCron(expr: string): string {
    const parts = expr.split(' ')
    if (parts.length !== 5) return expr
    const [min, hour, day, month, weekday] = parts
    if (min.startsWith('*/') && hour === '*') return `每 ${min.slice(2)} 分钟`
    if (hour.startsWith('*/') && min === '0') return `每 ${hour.slice(2)} 小时`
    if (min === '0' && !hour.includes('/') && day === '*' && month === '*' && weekday === '*') return `每天 ${hour}:00`
    if (min === '0' && weekday === '1-5') return `工作日 ${hour}:00`
    return expr
}

/** Get a Chinese label for task repeat mode */
export function repeatLabel(mode: string, maxRuns: number): string {
    if (mode === 'once') return '单次'
    if (mode === 'limited') return `${maxRuns}次`
    return '不限'
}

/** Get a Chinese label for task status */
export function statusLabel(status: string): string {
    if (status === 'active') return '运行中'
    if (status === 'paused') return '已暂停'
    if (status === 'completed') return '已完成'
    return status
}

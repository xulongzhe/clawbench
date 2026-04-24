import { inject } from 'vue'
import { copyText } from '@/utils/helpers.ts'

const BLOCK_SELECTORS = 'p, h1, h2, h3, h4, h5, h6, li, pre, blockquote, table, .mermaid'

interface ToastShow {
    show: (msg: string, opts?: { icon?: string; duration?: number }) => void
}

export interface LinkHandler {
    (path: string): void
}

/**
 * 双击复制块级元素的文本
 * 使用 click 事件手动判断双击，确保两次点击的是同一个元素
 */
export function useDoubleClickCopy() {
    const toast = inject<ToastShow | null>('toast', null)
    let lastTarget: EventTarget | null = null
    let lastTime = 0
    const DBLCLICK_THRESHOLD = 300 // ms，与浏览器默认双击间隔一致

    /**
     * 执行复制操作
     */
    function doCopy(target: EventTarget | null): boolean {
        const block = (target as HTMLElement | null)?.closest<HTMLElement>(BLOCK_SELECTORS)
        if (!block) return false

        const text = block.textContent?.trim()
        if (!text) return false

        copyText(text, () => {
            // 触发闪烁动画
            block.classList.add('copy-flash')
            block.addEventListener('animationend', () => {
                block.classList.remove('copy-flash')
            }, { once: true })

            // 显示 toast 提示
            if (toast) {
                toast.show('已复制', { icon: '📋', duration: 1500 })
            }
        })

        return true
    }

    /**
     * 处理锚点链接点击
     */
    function handleAnchorClick(event: MouseEvent, onOpenFile?: LinkHandler): boolean {
        const target = event.target as HTMLElement
        const anchor = target.closest<HTMLAnchorElement>('a[href]')
        
        if (!anchor) return false

        const href = anchor.getAttribute('href')
        if (!href) return false

        // 处理锚点链接 (#xxx)
        if (href.startsWith('#')) {
            return handleHashLink(event, href, anchor)
        }

        // 处理相对路径链接 (非 http/https 链接)
        if (!/^(https?:|\/\/|mailto:|tel:)/i.test(href) && onOpenFile) {
            event.preventDefault()
            onOpenFile(href)
            return true
        }

        return false
    }

    /**
     * 处理锚点链接 (#xxx)
     */
    function handleHashLink(event: MouseEvent, href: string, anchor: HTMLAnchorElement): boolean {
        // 解码 URL 编码的 href
        const targetId = decodeURIComponent(href.substring(1))
        const linkText = anchor.textContent?.trim() || ''
        
        // 先尝试直接查找 ID
        let targetElement = document.querySelector(`[id="${CSS.escape(targetId)}"]`)
        
        // 如果找不到,尝试用 slugify 转换后查找
        if (!targetElement) {
            const slugifiedId = targetId
                .toLowerCase()
                .replace(/[^\w\u4e00-\u9fa5]+/g, '-')
                .replace(/^-+|-+$/g, '')
            targetElement = document.querySelector(`[id="${CSS.escape(slugifiedId)}"]`)
        }
        
        // 如果还是找不到,尝试通过链接文本匹配标题
        if (!targetElement && linkText) {
            const allHeadings = document.querySelectorAll('.markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4, .markdown-body h5, .markdown-body h6')
            for (const heading of allHeadings) {
                const headingText = heading.textContent?.trim() || ''
                // 精确匹配
                if (headingText === linkText) {
                    targetElement = heading
                    break
                }
            }
            
            // 如果精确匹配失败,尝试去除序号后的匹配
            if (!targetElement) {
                // 去除开头的数字和标点,如 "5. 第四部分" -> "第四部分"
                const cleanLinkText = linkText.replace(/^[\d\s.、:：]+/, '').trim()
                for (const heading of allHeadings) {
                    const headingText = heading.textContent?.trim() || ''
                    if (headingText === cleanLinkText || headingText.includes(cleanLinkText)) {
                        targetElement = heading
                        break
                    }
                }
            }
        }
        
        if (targetElement) {
            // 阻止默认行为
            event.preventDefault()
            
            // 找到滚动容器
            const scrollContainer = anchor.closest('.file-viewer-content') || 
                                   anchor.closest('.markdown-body')?.parentElement ||
                                   document.documentElement
            
            // 计算目标位置
            const containerRect = scrollContainer.getBoundingClientRect()
            const targetRect = targetElement.getBoundingClientRect()
            const scrollTop = (scrollContainer as HTMLElement).scrollTop || 0
            const targetTop = scrollTop + targetRect.top - containerRect.top - 20 // 20px offset
            
            // 平滑滚动
            scrollContainer.scrollTo({
                top: targetTop,
                behavior: 'smooth'
            })
            
            return true
        }

        return false
    }

    /**
     * 处理原生 click 事件，手动判断双击
     * 只有在短时间内点击同一个元素时才触发双击复制
     */
    function handleDblClick(event: MouseEvent, onOpenFile?: LinkHandler): void {
        // 首先检查是否点击了链接
        if (handleAnchorClick(event, onOpenFile)) {
            return
        }

        const now = Date.now()
        const target = event.target
        const timeDiff = now - lastTime

        // 判断是否是双击：短时间内点击同一个元素
        if (lastTarget === target && timeDiff < DBLCLICK_THRESHOLD) {
            // 清除状态，防止连续触发
            lastTarget = null
            lastTime = 0

            // 执行复制
            doCopy(target)
        } else {
            // 记录这次点击
            lastTarget = target
            lastTime = now
        }
    }

    return {
        handleDblClick,
    }
}

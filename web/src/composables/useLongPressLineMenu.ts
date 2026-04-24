import { ref, watch, onUnmounted, type Ref } from 'vue'
import { copyText } from '@/utils/helpers.ts'

interface PressData {
    target: EventTarget
    x: number
    y: number
}

export function useLongPressLineMenu(
    codeRef: Ref<HTMLElement | null>,
    filePath: string | (() => string),
    getFileContent: () => string,
    setFileContent: (content: string) => void,
    editable = true,
    onCopied: ((text: string) => void) | null = null
) {
    const showContextMenu = ref(false)
    const menuPos = ref({ x: 0, y: 0 })
    const selectedLineNum = ref<number | null>(null)
    const highlightedLine = ref<number | null>(null)
    const editingLine = ref<number | null>(null)
    const editContent = ref('')
    const copiedLine = ref<number | null>(null)

    // Insert mode: null = not inserting, 'above' = inserting above selectedLine, 'below' = inserting below
    const insertMode = ref<'above' | 'below' | null>(null)

    let pressStartTime = 0
    let pressData: PressData | null = null
    let pressMoved = false

    function getLineNum(target: EventTarget | null): number | null {
        return parseInt((target as HTMLElement | null)?.closest?.('.code-line')?.getAttribute('data-line') || '0') || null
    }

    function tryShowMenu(): void {
        if (!pressData || pressMoved) { pressData = null; return }
        if (Date.now() - pressStartTime < 450) { pressData = null; return }
        const lineNum = getLineNum(pressData.target)
        if (lineNum) {
            selectedLineNum.value = lineNum
            highlightedLine.value = lineNum
            menuPos.value = {
                x: Math.min(pressData.x, window.innerWidth - 140),
                y: Math.min(pressData.y + 10, window.innerHeight - 180),
            }
            showContextMenu.value = true
        }
        pressData = null
    }

    function onTouchStart(e: TouchEvent): void {
        const touch = e.touches[0]
        pressStartTime = Date.now()
        pressData = { target: touch.target!, x: touch.clientX, y: touch.clientY }
        pressMoved = false
    }

    function onTouchMove(): void { pressMoved = true }
    function onTouchEnd(): void { tryShowMenu() }

    function onContextMenu(e: MouseEvent): void {
        e.preventDefault()
        const lineNum = getLineNum(e.target)
        if (!lineNum) return
        selectedLineNum.value = lineNum
        highlightedLine.value = lineNum
        menuPos.value = {
            x: Math.min(e.clientX, window.innerWidth - 140),
            y: Math.min(e.clientY, window.innerHeight - 180),
        }
        showContextMenu.value = true
    }

    function getPath(): string {
        return typeof filePath === 'function' ? filePath() : filePath
    }

    function handleEditLine(): void {
        showContextMenu.value = false
        highlightedLine.value = null
        if (!editable || selectedLineNum.value === null) return
        const lines = getFileContent().split('\n')
        editingLine.value = selectedLineNum.value - 1
        editContent.value = lines[editingLine.value] ?? ''
    }

    async function handleSaveEdit(): Promise<void> {
        if (editingLine.value === null) return
        try {
            const resp = await fetch('/api/file/edit-line', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: getPath(), lineNum: editingLine.value + 1, content: editContent.value }),
            })
            if (resp.ok) {
                const lines = getFileContent().split('\n')
                lines[editingLine.value] = editContent.value
                setFileContent(lines.join('\n'))
                editingLine.value = null
            } else {
                alert('保存失败')
            }
        } catch (err) {
            alert(`错误: ${(err as Error).message}`)
        }
    }

    function handleDeleteLine(): void {
        showContextMenu.value = false
        highlightedLine.value = null
        if (!editable || selectedLineNum.value === null || !confirm(`确定删除第 ${selectedLineNum.value} 行？`)) return
        const ln = selectedLineNum.value
        const lines = getFileContent().split('\n')
        lines.splice(ln - 1, 1)
        fetch('/api/file/edit-line', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: getPath(), lineNum: ln, delete: true }),
        }).then(r => {
            if (r.ok) setFileContent(lines.join('\n'))
            else alert('删除失败')
        })
    }

    function handleCopyLine(): void {
        showContextMenu.value = false
        highlightedLine.value = null
        if (selectedLineNum.value === null) return
        const text = getFileContent().split('\n')[selectedLineNum.value - 1] ?? ''
        copyText(text, () => {
            copiedLine.value = selectedLineNum.value
            setTimeout(() => { copiedLine.value = null }, 800)
            if (onCopied) onCopied(text)
        })
    }

    function handleInsertAbove(): void {
        showContextMenu.value = false
        highlightedLine.value = null
        if (!editable || selectedLineNum.value === null) return
        editingLine.value = selectedLineNum.value - 1  // reuse editingLine as the "new line" target
        insertMode.value = 'above'
        editContent.value = ''
    }

    function handleInsertBelow(): void {
        showContextMenu.value = false
        highlightedLine.value = null
        if (!editable || selectedLineNum.value === null) return
        editingLine.value = selectedLineNum.value  // will insert at this position + 1 (0-indexed = lineNum)
        insertMode.value = 'below'
        editContent.value = ''
    }

    async function handleSaveEditOrInsert(): Promise<void> {
        if (editingLine.value === null) return
        const ln = editingLine.value + 1  // 1-based line number
        const content = editContent.value

        try {
            let resp: Response
            if (insertMode.value === 'above') {
                // Insert empty line above, then edit that new line
                resp = await fetch('/api/file/edit-line', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: getPath(), lineNum: ln, insertAbove: true }),
                })
                if (!resp.ok) { alert('插入失败'); return }
                // The inserted empty line is now at position ln, old ln becomes ln+1
                resp = await fetch('/api/file/edit-line', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: getPath(), lineNum: ln, content }),
                })
            } else if (insertMode.value === 'below') {
                // Insert empty line below, then edit that new line (ln+1)
                resp = await fetch('/api/file/edit-line', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: getPath(), lineNum: ln, insertBelow: true }),
                })
                if (!resp.ok) { alert('插入失败'); return }
                // The inserted empty line is now at position ln+1
                resp = await fetch('/api/file/edit-line', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: getPath(), lineNum: ln + 1, content }),
                })
            } else {
                // Normal edit
                resp = await fetch('/api/file/edit-line', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: getPath(), lineNum: ln, content }),
                })
            }
            if (resp.ok) {
                const lines = getFileContent().split('\n')
                if (insertMode.value === 'above') {
                    lines.splice(ln - 1, 0, content)
                } else if (insertMode.value === 'below') {
                    lines.splice(ln, 0, content)
                } else {
                    lines[editingLine.value] = content
                }
                setFileContent(lines.join('\n'))
                editingLine.value = null
                insertMode.value = null
            } else {
                alert('保存失败')
            }
        } catch (err) {
            alert(`错误: ${(err as Error).message}`)
        }
    }

    function attachListeners(el: HTMLElement): void {
        el.addEventListener('touchstart', onTouchStart, { passive: true })
        el.addEventListener('touchmove', onTouchMove, { passive: true })
        el.addEventListener('touchend', onTouchEnd, { passive: true })
        el.addEventListener('touchcancel', onTouchEnd, { passive: true })
        el.addEventListener('contextmenu', onContextMenu)
    }

    function detachListeners(el: HTMLElement): void {
        el.removeEventListener('touchstart', onTouchStart)
        el.removeEventListener('touchmove', onTouchMove)
        el.removeEventListener('touchend', onTouchEnd)
        el.removeEventListener('touchcancel', onTouchEnd)
        el.removeEventListener('contextmenu', onContextMenu)
    }

    watch(codeRef, (el, oldEl) => {
        if (oldEl) detachListeners(oldEl)
        if (el) attachListeners(el)
    }, { flush: 'post' })

    onUnmounted(() => {
        if (codeRef.value) detachListeners(codeRef.value)
    })

    return {
        showContextMenu, menuPos, selectedLineNum, highlightedLine,
        editingLine, editContent, insertMode,
        handleEditLine, handleSaveEditOrInsert, handleDeleteLine, handleCopyLine,
        handleInsertAbove, handleInsertBelow,
        copiedLine,
    }
}

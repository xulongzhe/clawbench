import { ref } from 'vue'

/**
 * 滑动导航 composable
 * 用于检测元素的左右滑动手势，并提供实时偏移量用于视觉反馈
 * @param onSwipeLeft - 左滑回调
 * @param onSwipeRight - 右滑回调
 * @param threshold - 滑动阈值（像素）
 */
export function useSwipeNavigation(
    onSwipeLeft: (() => void) | null,
    onSwipeRight: (() => void) | null,
    threshold = 50
) {
    const touchStartX = ref(0)
    const touchStartY = ref(0)
    const swipeOffset = ref(0)
    const isSwiping = ref(false)
    const settling = ref(false)

    function handleTouchStart(event: TouchEvent): void {
        const touch = event.touches[0]
        touchStartX.value = touch.clientX
        touchStartY.value = touch.clientY
        swipeOffset.value = 0
        isSwiping.value = false
        settling.value = false
    }

    function handleTouchMove(event: TouchEvent): void {
        const touch = event.touches[0]
        const deltaX = touch.clientX - touchStartX.value
        const deltaY = touch.clientY - touchStartY.value
        const absX = Math.abs(deltaX)
        const absY = Math.abs(deltaY)

        // 忽略垂直滑动
        if (!isSwiping.value && absY > absX) return

        // 超过最小距离后开始跟踪
        if (absX > 8) {
            isSwiping.value = true
        }

        if (!isSwiping.value) return

        // 用阻尼函数限制偏移量，越远越难拉
        const maxOffset = 120
        const sign = deltaX > 0 ? 1 : -1
        const rawAbs = Math.min(absX, maxOffset)
        // 阻尼：前 60px 1:1，之后逐渐衰减
        const damped = rawAbs <= 60
            ? rawAbs
            : 60 + (rawAbs - 60) * (1 - (rawAbs - 60) / (2 * (maxOffset - 60)))
        swipeOffset.value = sign * damped
    }

    function handleTouchEnd(_event: TouchEvent): void {
        if (!isSwiping.value) {
            swipeOffset.value = 0
            return
        }

        const offset = swipeOffset.value
        isSwiping.value = false
        settling.value = true

        if (Math.abs(offset) >= threshold) {
            if (offset > 0 && onSwipeRight) {
                // 右滑 -> 上一个文件
                onSwipeRight()
            } else if (offset < 0 && onSwipeLeft) {
                // 左滑 -> 下一个文件
                onSwipeLeft()
            }
        }

        // 归零偏移（CSS transition 会处理动画）
        swipeOffset.value = 0

        // 等过渡动画结束后清除 settling 状态
        setTimeout(() => {
            settling.value = false
        }, 300)
    }

    return {
        handleTouchStart,
        handleTouchMove,
        handleTouchEnd,
        swipeOffset,
        settling,
    }
}

import { ref } from 'vue'

/**
 * 滑动导航 composable
 * 用于检测元素的左右滑动手势
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

    function handleTouchStart(event: TouchEvent): void {
        const touch = event.touches[0]
        touchStartX.value = touch.clientX
        touchStartY.value = touch.clientY
    }

    function handleTouchEnd(event: TouchEvent): void {
        const touch = event.changedTouches[0]
        const deltaX = touch.clientX - touchStartX.value
        const deltaY = touch.clientY - touchStartY.value
        const absX = Math.abs(deltaX)
        const absY = Math.abs(deltaY)

        // 确保是水平滑动（水平距离大于垂直距离）
        if (absY > absX) return

        // 检查滑动距离是否超过阈值
        if (absX < threshold) return

        if (deltaX > 0 && onSwipeRight) {
            // 右滑 -> 上一个文件
            onSwipeRight()
        } else if (deltaX < 0 && onSwipeLeft) {
            // 左滑 -> 下一个文件
            onSwipeLeft()
        }
    }

    return {
        handleTouchStart,
        handleTouchEnd
    }
}

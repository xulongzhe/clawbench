/**
 * Global back navigation handler for drill-down pages.
 *
 * When the user swipes from the right edge (or presses Android back),
 * each feature can register a "can go back" check and a "go back" action.
 * The handler iterates registered features in reverse order (most recently
 * registered first) and calls the first one that can handle back navigation.
 *
 * If no feature handles back, the system back gesture proceeds normally
 * (which on Android would exit the app — but we prevent that by always
 * having a default handler that does nothing when there's nothing to go back to).
 */

export type BackHandler = {
    /** Unique ID for the handler (e.g., 'tasks', 'git', 'settings', 'browse') */
    id: string
    /** Returns true if this feature can navigate back */
    canGoBack: () => boolean
    /** Perform the back navigation */
    goBack: () => void
}

const handlers: BackHandler[] = []

/**
 * Register a back navigation handler for a feature.
 * Returns an unregister function.
 */
export function registerBackHandler(handler: BackHandler): () => void {
    handlers.push(handler)
    return () => {
        const idx = handlers.indexOf(handler)
        if (idx !== -1) handlers.splice(idx, 1)
    }
}

/**
 * Attempt to handle a back navigation event.
 * Returns true if a handler consumed the event (navigated back).
 */
export function handleBackNavigation(): boolean {
    // Iterate in reverse so the most recently registered handler gets priority
    for (let i = handlers.length - 1; i >= 0; i--) {
        const h = handlers[i]
        if (h.canGoBack()) {
            h.goBack()
            return true
        }
    }
    return false
}

/**
 * Check if any registered handler can navigate back.
 */
export function canNavigateBack(): boolean {
    return handlers.some(h => h.canGoBack())
}

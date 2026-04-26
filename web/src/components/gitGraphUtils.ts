// ─── Git Graph computation utilities ────────────────────────────────────────

const LANE_WIDTH = 20
const GRAPH_LEFT_PADDING = 10
const LANE_COLORS = [
  '#4a90d9', // blue
  '#e67e22', // orange
  '#2ecc71', // green
  '#9b59b6', // purple
  '#e74c3c', // red
  '#1abc9c', // teal
  '#f39c12', // yellow
  '#34495e', // dark gray
]

export { LANE_WIDTH, LANE_COLORS }

function laneCx(lane) {
  return lane * LANE_WIDTH + LANE_WIDTH / 2 + GRAPH_LEFT_PADDING
}

/**
 * Compute full graph data for a list of commits.
 * Returns per-row node info, connection lines, and overall dimensions.
 */
export function computeGraphData(commits, rowHeight) {
  if (!commits || !commits.length) {
    return { nodes: [], lines: [], laneCount: 0, graphWidth: 40 }
  }

  const laneColor = (lane) => LANE_COLORS[lane % LANE_COLORS.length]

  const shaToLane = new Map()
  // Build a reverse index: sha → list of row indices that reference it as a parent
  // This avoids O(n²) scanning for lane cleanup
  const parentRefRows = new Map()
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) continue
    for (const p of (c.parents || [])) {
      if (!parentRefRows.has(p)) parentRefRows.set(p, [])
      parentRefRows.get(p).push(row)
    }
  }

  const freeLanes = []
  let nextLane = 0
  function allocLane() {
    if (freeLanes.length > 0) return freeLanes.shift()
    return nextLane++
  }

  let maxLane = 0
  const nodes = []
  const lines = []

  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    const cy = row * rowHeight + rowHeight / 2

    if (c.isWT) {
      nodes.push({
        cx: laneCx(0),
        cy,
        color: '#f59e0b',
        isWT: true,
        refs: [],
        lane: 0,
        row,
      })
      continue
    }

    const sha = c.sha
    let lane
    if (shaToLane.has(sha)) {
      lane = shaToLane.get(sha)
    } else {
      lane = allocLane()
      shaToLane.set(sha, lane)
    }
    if (lane > maxLane) maxLane = lane

    const cx = laneCx(lane)
    const color = laneColor(lane)

    nodes.push({
      cx,
      cy,
      color,
      isWT: false,
      refs: c.refs || [],
      lane,
      row,
    })

    const parents = c.parents || []
    for (let pi = 0; pi < parents.length; pi++) {
      const parentSha = parents[pi]

      let parentLane
      if (shaToLane.has(parentSha)) {
        parentLane = shaToLane.get(parentSha)
      } else {
        // First parent inherits current lane, others get new lanes
        parentLane = pi === 0 ? lane : allocLane()
        if (parentLane > maxLane) maxLane = parentLane
        shaToLane.set(parentSha, parentLane)
      }

      if (parentLane !== lane) {
        // Curve from current commit to parent lane
        const parentCx = laneCx(parentLane)
        const midY = cy + rowHeight * 0.5
        lines.push({
          path: `M${cx},${cy + 4} C${cx},${midY} ${parentCx},${midY} ${parentCx},${cy + rowHeight}`,
          color: laneColor(parentLane),
          startRow: row,
        })
      } else if (pi === 0) {
        // Same lane vertical continuation
        lines.push({
          path: `M${cx},${cy + 4} L${cx},${cy + rowHeight}`,
          color,
          startRow: row,
        })
      }

      // Free parent lane if no future commits reference it
      // Use the pre-built reverse index for O(1) check
      if (parentLane !== lane) {
        const refRows = parentRefRows.get(parentSha) || []
        const hasFutureRefs = refRows.some(r => r > row)
        if (!hasFutureRefs) {
          shaToLane.delete(parentSha)
          freeLanes.push(parentLane)
          freeLanes.sort((a, b) => a - b)
        }
      }
    }

    // Root commit: free its lane if no future commit references it
    if (parents.length === 0) {
      const refRows = parentRefRows.get(sha) || []
      const hasFutureRefs = refRows.some(r => r > row)
      if (!hasFutureRefs) {
        shaToLane.delete(sha)
        freeLanes.push(lane)
        freeLanes.sort((a, b) => a - b)
      }
    }
  }

  const graphWidth = Math.max(40, (maxLane + 1) * LANE_WIDTH + GRAPH_LEFT_PADDING * 2)
  return { nodes, lines, laneCount: maxLane + 1, graphWidth }
}

// ─── Ref label helpers ─────────────────────────────────────────────────────

export function refLabelWidth(ref) {
  const text = ref.startsWith('tag: ') ? ref.slice(5) : ref
  return text.length * 6 + 8
}

export function refLabelText(ref) {
  if (ref.startsWith('tag: ')) return ref.slice(5)
  return ref
}

export function refLabelBg(ref) {
  if (ref === 'HEAD') return '#1a1a2e'
  if (ref.startsWith('tag: ')) return '#555'
  return '#4a90d9'
}

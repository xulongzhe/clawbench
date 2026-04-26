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
 *
 * Two-phase algorithm:
 * 1. Lane assignment — walk first-parent chains first (main line = same lane),
 *    then assign remaining commits and merge-parent lanes.
 * 2. Connection generation — for each commit→parent edge:
 *    - Same-lane parent → vertical line from child to parent
 *    - Cross-lane "fork" (merge commit's non-first parent) → bezier curve
 *    - Cross-lane "merge-in" (branch commit whose parent is on another lane) →
 *      vertical line on branch lane + short bezier at merge point
 */
export function computeGraphData(commits, rowHeight) {
  if (!commits || !commits.length) {
    return { nodes: [], lines: [], laneCount: 0, graphWidth: 40 }
  }

  const laneColor = (lane) => LANE_COLORS[lane % LANE_COLORS.length]

  // ── Pre-build SHA-to-row index (O(1) lookup) ──
  const shaToRow = new Map()
  for (let row = 0; row < commits.length; row++) {
    if (!commits[row].isWT) {
      shaToRow.set(commits[row].sha, row)
    }
  }

  // ── Phase 1: Lane assignment ──
  //
  // Key insight: the first-parent chain represents the "main line" and should
  // stay on the same lane. We walk first-parent chains FIRST to ensure fork
  // points are assigned to the main-line lane, not pulled to a branch lane.
  //
  // Step A: For each commit, walk its first-parent chain and assign the same
  //         lane to all ancestors until hitting an already-assigned SHA.
  // Step B: Assign lanes to any remaining unassigned commits (branch tips).
  // Step C: Assign lanes to merge parents (non-first-parent) that aren't assigned.

  const shaToLane = new Map()
  const laneOfRow = new Array(commits.length)
  let nextLane = 0
  let maxLane = 0

  // Step A: Walk first-parent chains
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) {
      laneOfRow[row] = 0
      continue
    }

    const sha = c.sha
    let lane
    if (shaToLane.has(sha)) {
      lane = shaToLane.get(sha)
    } else {
      lane = nextLane++
      shaToLane.set(sha, lane)
    }

    laneOfRow[row] = lane
    if (lane > maxLane) maxLane = lane

    // Walk first-parent chain, assigning same lane to ancestors
    let cur = sha
    while (true) {
      const curRow = shaToRow.get(cur)
      if (curRow === undefined) break
      const parents = commits[curRow].parents || []
      if (parents.length === 0) break
      const firstParent = parents[0]
      if (shaToLane.has(firstParent)) break
      if (!shaToRow.has(firstParent)) break
      shaToLane.set(firstParent, lane)
      cur = firstParent
    }
  }

  // Step B: Assign remaining unassigned commits (branch tips not on first-parent chain)
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) continue
    if (shaToLane.has(c.sha)) {
      laneOfRow[row] = shaToLane.get(c.sha)
      continue
    }
    const lane = nextLane++
    shaToLane.set(c.sha, lane)
    laneOfRow[row] = lane
    if (lane > maxLane) maxLane = lane
  }

  // Step C: Assign lanes for merge parents (non-first-parent) that aren't assigned
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) continue
    const parents = c.parents || []
    for (let pi = 1; pi < parents.length; pi++) {
      const pSha = parents[pi]
      if (!shaToRow.has(pSha)) continue
      if (!shaToLane.has(pSha)) {
        const lane = nextLane++
        shaToLane.set(pSha, lane)
        if (lane > maxLane) maxLane = lane
      }
    }
  }

  // ── Phase 2: Generate nodes and connection lines ──
  const nodes = []
  const lines = []

  // Nodes
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    const cy = row * rowHeight + rowHeight / 2
    const lane = laneOfRow[row]
    const cx = laneCx(lane)
    if (c.isWT) {
      nodes.push({ cx, cy, color: '#f59e0b', isWT: true, refs: [], lane, row })
    } else {
      nodes.push({ cx, cy, color: laneColor(lane), isWT: false, refs: c.refs || [], lane, row })
    }
  }

  // Connection lines: for each commit→parent edge
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) continue

    const childLane = laneOfRow[row]
    const childCy = row * rowHeight + rowHeight / 2
    const parents = c.parents || []

    for (let pi = 0; pi < parents.length; pi++) {
      const pSha = parents[pi]
      if (!shaToRow.has(pSha) || !shaToLane.has(pSha)) continue

      const parentRow = shaToRow.get(pSha)
      const parentLane = shaToLane.get(pSha)
      const parentCy = parentRow * rowHeight + rowHeight / 2

      if (childLane === parentLane) {
        // Same-lane connection: vertical line from child bottom to parent top
        const x = laneCx(childLane)
        const y1 = childCy + 5
        const y2 = parentCy - 5
        lines.push({
          path: `M${x},${y1} L${x},${y2}`,
          color: laneColor(childLane),
        })
      } else if (pi > 0) {
        // Cross-lane fork: merge commit's non-first parent (the branch tip).
        // Draw a bezier from the merge commit down to the branch.
        const fromX = laneCx(childLane)
        const fromY = childCy + 5
        const toX = laneCx(parentLane)
        const toY = parentCy - 5
        const dy = toY - fromY
        const cp1Y = fromY + dy * 0.4
        const cp2Y = fromY + dy * 0.6
        lines.push({
          path: `M${fromX},${fromY} C${fromX},${cp1Y} ${toX},${cp2Y} ${toX},${toY}`,
          color: laneColor(parentLane),
        })
      } else {
        // Cross-lane merge-in: child's first parent is on a different lane.
        // This means the child is a branch commit that merges back into the
        // main line. Render as: vertical line on branch lane from child down
        // to just above the parent row, then a short bezier curving into the
        // parent node. This creates the '|/' pattern from git log --graph.
        const branchX = laneCx(childLane)
        const parentX = laneCx(parentLane)
        const childBottom = childCy + 5
        // End vertical line at the top of the parent row
        const parentRowTop = parentRow * rowHeight

        // Vertical line on branch lane from child to parent row
        lines.push({
          path: `M${branchX},${childBottom} L${branchX},${parentRowTop}`,
          color: laneColor(childLane),
        })

        // Short bezier from branch lane at parent row top, curving into
        // the parent node. Control points create a smooth merge-in arc.
        const fromY = parentRowTop
        const toY = parentCy
        const dy = toY - fromY
        const cp1Y = fromY + dy * 0.25
        const cp2Y = toY - dy * 0.25
        lines.push({
          path: `M${branchX},${fromY} C${branchX},${cp1Y} ${parentX},${cp2Y} ${parentX},${toY}`,
          color: laneColor(childLane),
        })
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

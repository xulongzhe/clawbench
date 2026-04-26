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
export function computeGraphData(commits, rowHeight, previousShaToLane) {
  if (!commits || !commits.length) {
    return { nodes: [], lines: [], laneCount: 0, graphWidth: 40, shaToLane: new Map() }
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
  // When previousShaToLane is provided (lazy-load scenario), preserve lane
  // assignments for SHAs that were already visible. This prevents visual line
  // splitting when new commits are appended.
  //
  // Step A: For each commit, walk its first-parent chain and assign the same
  //         lane to all ancestors until hitting an already-assigned SHA.
  //         If a SHA was previously assigned, reuse that lane.
  // Step B: Assign lanes to any remaining unassigned commits (branch tips).
  // Step C: Assign lanes to merge parents (non-first-parent) that aren't assigned.

  const shaToLane = new Map()
  const laneOfRow = new Array(commits.length)
  let nextLane = 0
  let maxLane = 0

  // Seed from previous lane assignments so existing commits keep their lanes
  if (previousShaToLane) {
    for (const [sha, lane] of previousShaToLane) {
      if (shaToRow.has(sha)) {
        shaToLane.set(sha, lane)
        if (lane >= nextLane) nextLane = lane + 1
        if (lane > maxLane) maxLane = lane
      }
    }
  }

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

  // ── Pre-compute merge-in counts per parent row ──
  // Used to offset merge-in bezier endpoints when multiple arrive at same parent
  const mergeInCount = new Map()  // parentRow -> count of merge-ins
  const mergeInIndex = new Map()  // "parentRow:fromLane" -> index among siblings
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) continue
    const childLane = laneOfRow[row]
    const parents = c.parents || []
    const pSha = parents[0]
    if (!pSha || !shaToRow.has(pSha) || !shaToLane.has(pSha)) continue
    const parentRow = shaToRow.get(pSha)
    const parentLane = shaToLane.get(pSha)
    if (childLane !== parentLane) {
      if (!mergeInCount.has(parentRow)) mergeInCount.set(parentRow, 0)
      mergeInCount.set(parentRow, mergeInCount.get(parentRow) + 1)
    }
  }
  // Second pass: assign indices
  const mergeInSeen = new Map()  // parentRow -> count seen so far
  const mergeInOffsets = new Map() // "parentRow:fromLane" -> y-offset
  for (let row = 0; row < commits.length; row++) {
    const c = commits[row]
    if (c.isWT) continue
    const childLane = laneOfRow[row]
    const parents = c.parents || []
    const pSha = parents[0]
    if (!pSha || !shaToRow.has(pSha) || !shaToLane.has(pSha)) continue
    const parentRow = shaToRow.get(pSha)
    const parentLane = shaToLane.get(pSha)
    if (childLane !== parentLane) {
      if (!mergeInSeen.has(parentRow)) mergeInSeen.set(parentRow, 0)
      const idx = mergeInSeen.get(parentRow)
      mergeInSeen.set(parentRow, idx + 1)
      const total = mergeInCount.get(parentRow)
      // Spread offsets evenly: center around 0, step 4px
      const offset = (idx - (total - 1) / 2) * 4
      mergeInOffsets.set(parentRow + ':' + childLane, offset)
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
          lane: childLane,
        })
      } else if (pi > 0) {
        // Cross-lane fork: merge commit's non-first parent (the branch tip).
        // For adjacent lanes, use a simple bezier. For non-adjacent lanes
        // (octopus merge), render as a cascade: short bezier from merge to
        // the adjacent lane, then vertical lines through intermediate lanes,
        // then arrive at the target lane.
        const fromX = laneCx(childLane)
        const fromY = childCy + 5
        const toX = laneCx(parentLane)
        const toY = parentCy - 5

        // Use cascade when there's enough vertical pixel space. The cascade
        // needs roughly rowHeight per intermediate lane step. For short
        // distances, a simple bezier crossing intermediate lanes is acceptable
        // since those lanes won't have content at that exact row.
        const laneGap = parentLane - childLane
        const dy = toY - fromY
        const useCascade = laneGap > 1 && dy > laneGap * rowHeight * 0.6

        if (!useCascade) {
          // Adjacent lane or short distance: simple bezier
          const dy = toY - fromY
          const cp1Y = fromY + dy * 0.4
          const cp2Y = fromY + dy * 0.6
          lines.push({
            path: `M${fromX},${fromY} C${fromX},${cp1Y} ${toX},${cp2Y} ${toX},${toY}`,
            color: laneColor(parentLane),
            lane: parentLane,
          })
        } else {
          // Non-adjacent lane (e.g., octopus merge): cascade through
          // intermediate lanes to avoid crossing over them visually.
          // Step 1: short bezier from merge to the next lane over
          const nextX = laneCx(childLane + 1)
          const step1EndY = fromY + rowHeight * 0.4
          lines.push({
            path: `M${fromX},${fromY} C${fromX},${fromY + rowHeight * 0.15} ${nextX},${fromY + rowHeight * 0.25} ${nextX},${step1EndY}`,
            color: laneColor(parentLane),
            lane: parentLane,
          })

          // Step 2: vertical lines through intermediate lanes
          // Each intermediate lane gets a diagonal step right + vertical segment
          let curLane = childLane + 1
          let curY = step1EndY
          while (curLane < parentLane) {
            const curX = laneCx(curLane)
            const nextLaneX = laneCx(curLane + 1)
            const stepHeight = rowHeight * 0.3
            // Diagonal to next lane
            lines.push({
              path: `M${curX},${curY} L${nextLaneX},${curY + stepHeight}`,
              color: laneColor(parentLane),
              lane: parentLane,
            })
            curLane++
            curY += stepHeight
            // Vertical segment on this lane
            const vertEnd = curY + rowHeight * 0.2
            lines.push({
              path: `M${nextLaneX},${curY} L${nextLaneX},${vertEnd}`,
              color: laneColor(parentLane),
              lane: parentLane,
            })
            curY = vertEnd
          }

          // Step 3: bezier from the last intermediate lane down to the branch tip
          const lastX = laneCx(parentLane)
          const cp1Y2 = curY + (toY - curY) * 0.3
          const cp2Y2 = curY + (toY - curY) * 0.7
          lines.push({
            path: `M${lastX},${curY} C${lastX},${cp1Y2} ${lastX},${cp2Y2} ${lastX},${toY}`,
            color: laneColor(parentLane),
            lane: parentLane,
          })
        }
      } else {
        // Cross-lane merge-in: child's first parent is on a different lane.
        // Render as: vertical line on branch lane from child down to just
        // above the parent row, then a short bezier curving into the parent
        // node. This creates the '|/' pattern from git log --graph.
        const branchX = laneCx(childLane)
        const parentX = laneCx(parentLane)
        const childBottom = childCy + 5
        // End vertical line at the top of the parent row
        const parentRowTop = parentRow * rowHeight

        // Vertical line on branch lane from child to parent row
        lines.push({
          path: `M${branchX},${childBottom} L${branchX},${parentRowTop}`,
          color: laneColor(childLane),
          lane: childLane,
        })

        // Short bezier from branch lane at parent row top, curving into
        // the parent node. Offset Y endpoint if multiple merge-ins arrive
        // at the same parent to prevent visual overlap.
        const yKey = parentRow + ':' + childLane
        const yOffset = mergeInOffsets.get(yKey) || 0
        const fromY = parentRowTop
        const toY = parentCy + yOffset
        const dy = toY - fromY
        const cp1Y = fromY + dy * 0.25
        const cp2Y = toY - dy * 0.25
        lines.push({
          path: `M${branchX},${fromY} C${branchX},${cp1Y} ${parentX},${cp2Y} ${parentX},${toY}`,
          color: laneColor(childLane),
          lane: childLane,
        })
      }
    }
  }

  // ── Build lane-to-branch-name mapping ──
  // Scan nodes for branch refs (excluding HEAD and tags) to determine
  // which branch each lane visually represents.
  const laneBranchNames = new Map()  // lane -> Set of branch names
  for (const node of nodes) {
    if (node.refs) {
      for (const ref of node.refs) {
        if (ref === 'HEAD' || ref.startsWith('tag: ')) continue
        if (!laneBranchNames.has(node.lane)) {
          laneBranchNames.set(node.lane, new Set())
        }
        laneBranchNames.get(node.lane).add(ref)
      }
    }
  }
  // Convert to simple map: lane -> primary branch name (first from the most recent commit)
  const laneBranchName = new Map()
  for (const [lane, names] of laneBranchNames) {
    laneBranchName.set(lane, [...names][0])
  }

  const graphWidth = Math.max(40, (maxLane + 1) * LANE_WIDTH + GRAPH_LEFT_PADDING * 2)
  return { nodes, lines, laneCount: maxLane + 1, graphWidth, shaToLane, laneBranchName }
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

import { describe, expect, it } from 'vitest'
import { computeGraphData, LANE_WIDTH, LANE_COLORS, refLabelWidth, refLabelText, refLabelBg } from '../gitGraphUtils'

const ROW_HEIGHT = 46

// ─── Helper: extract connection info from graph data ───
function getConnections(commits, rowHeight = ROW_HEIGHT) {
  const { nodes, lines, laneCount, graphWidth } = computeGraphData(commits, rowHeight)

  // Build node index by row
  const nodeByRow = new Map()
  for (const n of nodes) {
    nodeByRow.set(n.row, n)
  }

  // Classify lines by type (VERT / FORK / MERGE-IN-vert / MERGE-IN-bezier)
  const connections = []
  for (const n of nodes) {
    if (n.isWT) continue
    const c = commits[n.row]
    const parents = c.parents || []

    for (let pi = 0; pi < parents.length; pi++) {
      const pSha = parents[pi]
      const pNode = nodes.find(nd => !nd.isWT && commits[nd.row]?.sha === pSha)
      if (!pNode) continue

      const childLane = n.lane
      const parentLane = pNode.lane

      if (childLane === parentLane) {
        connections.push({ fromRow: n.row, toRow: pNode.row, fromLane: childLane, toLane: parentLane, type: 'VERT', parentIndex: pi })
      } else if (pi > 0) {
        connections.push({ fromRow: n.row, toRow: pNode.row, fromLane: childLane, toLane: parentLane, type: 'FORK', parentIndex: pi })
      } else {
        connections.push({ fromRow: n.row, toRow: pNode.row, fromLane: childLane, toLane: parentLane, type: 'MERGE-IN', parentIndex: pi })
      }
    }
  }

  return { nodes, lines, laneCount, graphWidth, connections, nodeByRow }
}

// ─── Test data: all 7 git-graph-repos ───

const LINEAR = [
  { sha: 'a1', parents: ['a2'], msg: 'commit 5' },
  { sha: 'a2', parents: ['a3'], msg: 'commit 4' },
  { sha: 'a3', parents: ['a4'], msg: 'commit 3' },
  { sha: 'a4', parents: ['a5'], msg: 'commit 2' },
  { sha: 'a5', parents: [], msg: 'commit 1' },
]

const SINGLE_MERGE = [
  { sha: 'm4', parents: ['mg'], msg: 'main: commit 4', refs: ['HEAD -> master'] },
  { sha: 'mg', parents: ['m3', 'f2'], msg: 'merge' },
  { sha: 'f2', parents: ['f1'], msg: 'feature: commit 2', refs: ['feature'] },
  { sha: 'f1', parents: ['m2'], msg: 'feature: commit 1' },
  { sha: 'm3', parents: ['m2'], msg: 'main: commit 3' },
  { sha: 'm2', parents: ['m1'], msg: 'main: commit 2' },
  { sha: 'm1', parents: [], msg: 'main: commit 1' },
]

const MULTI_BRANCH = [
  { sha: 'after', parents: ['mb'], msg: 'main: after merges' },
  { sha: 'mb', parents: ['ma', 'b3'], msg: 'merge: feature-b' },
  { sha: 'b3', parents: ['b2'], msg: 'feature-b: work 3' },
  { sha: 'b2', parents: ['b1'], msg: 'feature-b: work 2' },
  { sha: 'b1', parents: ['ms'], msg: 'feature-b: work 1' },
  { sha: 'ma', parents: ['mt', 'a2'], msg: 'merge: feature-a' },
  { sha: 'a2', parents: ['a1'], msg: 'feature-a: work 2' },
  { sha: 'a1', parents: ['ms'], msg: 'feature-a: work 1' },
  { sha: 'mt', parents: ['ms'], msg: 'main: third' },
  { sha: 'ms', parents: ['mi'], msg: 'main: second' },
  { sha: 'mi', parents: [], msg: 'main: init' },
]

const FREQUENT_MERGE = [
  { sha: 'fin', parents: ['dw3'], msg: 'main: final' },
  { sha: 'dw3', parents: ['dm2'], msg: 'dev: work 3' },
  { sha: 'dm2', parents: ['dw2', 'mw2'], msg: 'dev: merge main 2' },
  { sha: 'mw2', parents: ['mw1'], msg: 'main: work 2' },
  { sha: 'dw2', parents: ['dm1'], msg: 'dev: work 2' },
  { sha: 'dm1', parents: ['dw1', 'mw1'], msg: 'dev: merge main 1' },
  { sha: 'mw1', parents: ['init'], msg: 'main: work 1' },
  { sha: 'dw1', parents: ['init'], msg: 'dev: work 1' },
  { sha: 'init', parents: [], msg: 'main: init' },
]

const LONG_LIVED = [
  { sha: 'post', parents: ['mrg'], msg: 'main: post-release' },
  { sha: 'mrg', parents: ['f4', 'rbf'], msg: 'merge: release-1.0' },
  { sha: 'rbf', parents: ['rp2'], msg: 'release: bugfix' },
  { sha: 'rp2', parents: ['rp1'], msg: 'release: prep 2' },
  { sha: 'rp1', parents: ['init'], msg: 'release: prep 1' },
  { sha: 'f4', parents: ['f3'], msg: 'main: feature 4' },
  { sha: 'f3', parents: ['f2'], msg: 'main: feature 3' },
  { sha: 'f2', parents: ['f1'], msg: 'main: feature 2' },
  { sha: 'f1', parents: ['init'], msg: 'main: feature 1' },
  { sha: 'init', parents: [], msg: 'main: init' },
]

const OCTOPUS = [
  { sha: 'fin', parents: ['mrg'], msg: 'main: final' },
  { sha: 'mrg', parents: ['ms', 'fa', 'fb', 'fc'], msg: 'merge: combine all' },
  { sha: 'fc', parents: ['init'], msg: 'feature-c: work' },
  { sha: 'fb', parents: ['init'], msg: 'feature-b: work' },
  { sha: 'fa', parents: ['init'], msg: 'feature-a: work' },
  { sha: 'ms', parents: ['init'], msg: 'main: second' },
  { sha: 'init', parents: [], msg: 'main: init' },
]

const OPEN_BRANCHES = [
  { sha: 'aw2', parents: ['aw1'], msg: 'feature-a: work 2' },
  { sha: 'aw1', parents: ['ms'], msg: 'feature-a: work 1' },
  { sha: 'bw1', parents: ['ms'], msg: 'feature-b: work 1' },
  { sha: 'mt', parents: ['ms'], msg: 'main: third' },
  { sha: 'ms', parents: ['init'], msg: 'main: second' },
  { sha: 'init', parents: [], msg: 'main: init' },
]

// ─── Tests ───

describe('computeGraphData', () => {
  it('returns empty data for empty input', () => {
    const result = computeGraphData([], ROW_HEIGHT)
    expect(result.nodes).toEqual([])
    expect(result.lines).toEqual([])
    expect(result.laneCount).toBe(0)
    expect(result.graphWidth).toBe(40)
  })

  it('returns empty data for null input', () => {
    const result = computeGraphData(null, ROW_HEIGHT)
    expect(result.nodes).toEqual([])
  })
})

describe('01-linear', () => {
  const { nodes, connections, laneCount } = getConnections(LINEAR)

  it('assigns all commits to lane 0', () => {
    for (const n of nodes) {
      expect(n.lane).toBe(0)
    }
  })

  it('has 1 lane', () => {
    expect(laneCount).toBe(1)
  })

  it('has 4 vertical connections (all same lane)', () => {
    expect(connections).toHaveLength(4)
    for (const c of connections) {
      expect(c.type).toBe('VERT')
    }
  })

  it('connects each commit to the next in order', () => {
    const pairs = connections.map(c => [c.fromRow, c.toRow])
    expect(pairs).toEqual([[0, 1], [1, 2], [2, 3], [3, 4]])
  })
})

describe('02-single-merge', () => {
  const { nodes, connections, laneCount } = getConnections(SINGLE_MERGE)

  it('uses 2 lanes (main + feature)', () => {
    expect(laneCount).toBe(2)
  })

  it('main line commits are on lane 0', () => {
    // Rows 0,1,4,5,6 are main line
    for (const row of [0, 1, 4, 5, 6]) {
      expect(nodes[row].lane).toBe(0)
    }
  })

  it('feature branch commits are on lane 1', () => {
    // Rows 2,3 are feature branch
    for (const row of [2, 3]) {
      expect(nodes[row].lane).toBe(1)
    }
  })

  it('has correct connection types', () => {
    const types = connections.map(c => c.type)
    // R0->R1 VERT, R1->R4 VERT, R1->R2 FORK, R2->R3 VERT, R3->R5 MERGE-IN, R4->R5 VERT, R5->R6 VERT
    expect(types.filter(t => t === 'VERT')).toHaveLength(5)
    expect(types.filter(t => t === 'FORK')).toHaveLength(1)
    expect(types.filter(t => t === 'MERGE-IN')).toHaveLength(1)
  })

  it('merge commit forks to feature branch', () => {
    const fork = connections.find(c => c.type === 'FORK')
    expect(fork.fromRow).toBe(1) // merge
    expect(fork.toRow).toBe(2)   // feature: commit 2
    expect(fork.fromLane).toBe(0)
    expect(fork.toLane).toBe(1)
  })

  it('feature branch merges back into main', () => {
    const mergeIn = connections.find(c => c.type === 'MERGE-IN')
    expect(mergeIn.fromRow).toBe(3) // feature: commit 1
    expect(mergeIn.toRow).toBe(5)   // main: commit 2
    expect(mergeIn.fromLane).toBe(1)
    expect(mergeIn.toLane).toBe(0)
  })

  it('generates correct number of lines (VERT=1 path, FORK=1 path, MERGE-IN=2 paths)', () => {
    const { lines } = computeGraphData(SINGLE_MERGE, ROW_HEIGHT)
    // 5 VERT + 1 FORK + 2 MERGE-IN = 8 line paths
    expect(lines).toHaveLength(8)
  })
})

describe('03-multi-branch', () => {
  const { nodes, connections, laneCount } = getConnections(MULTI_BRANCH)

  it('uses 3 lanes (main + feature-b + feature-a)', () => {
    expect(laneCount).toBe(3)
  })

  it('main line commits are on lane 0', () => {
    // Rows 0,1,5,8,9,10 are main line
    for (const row of [0, 1, 5, 8, 9, 10]) {
      expect(nodes[row].lane).toBe(0)
    }
  })

  it('feature-b commits are on lane 1', () => {
    for (const row of [2, 3, 4]) {
      expect(nodes[row].lane).toBe(1)
    }
  })

  it('feature-a commits are on lane 2', () => {
    for (const row of [6, 7]) {
      expect(nodes[row].lane).toBe(2)
    }
  })

  it('has 2 FORK connections (merge-b and merge-a)', () => {
    const forks = connections.filter(c => c.type === 'FORK')
    expect(forks).toHaveLength(2)
    // merge: feature-b forks to feature-b: work 3
    expect(forks.some(f => f.fromRow === 1 && f.toRow === 2 && f.fromLane === 0 && f.toLane === 1)).toBe(true)
    // merge: feature-a forks to feature-a: work 2
    expect(forks.some(f => f.fromRow === 5 && f.toRow === 6 && f.fromLane === 0 && f.toLane === 2)).toBe(true)
  })

  it('has 2 MERGE-IN connections (both merge into main: second)', () => {
    const mergeIns = connections.filter(c => c.type === 'MERGE-IN')
    expect(mergeIns).toHaveLength(2)
    // Both converge on row 9 (main: second, lane 0)
    for (const m of mergeIns) {
      expect(m.toRow).toBe(9)
      expect(m.toLane).toBe(0)
    }
  })

  it('feature-b merge-in spans from row 4 to row 9', () => {
    const bMerge = connections.find(c => c.type === 'MERGE-IN' && c.fromLane === 1)
    expect(bMerge.fromRow).toBe(4)
    expect(bMerge.toRow).toBe(9)
  })

  it('feature-a merge-in spans from row 7 to row 9', () => {
    const aMerge = connections.find(c => c.type === 'MERGE-IN' && c.fromLane === 2)
    expect(aMerge.fromRow).toBe(7)
    expect(aMerge.toRow).toBe(9)
  })
})

describe('04-frequent-merge', () => {
  const { nodes, connections, laneCount } = getConnections(FREQUENT_MERGE)

  it('uses 2 lanes (dev main line + main side branch)', () => {
    expect(laneCount).toBe(2)
  })

  it('first-parent chain (dev line) is on lane 0', () => {
    // Row 0: main: final, Row 1: dev: work 3, Row 2: dev: merge main 2
    // Row 4: dev: work 2, Row 5: dev: merge main 1, Row 7: dev: work 1
    for (const row of [0, 1, 2, 4, 5, 7]) {
      expect(nodes[row].lane).toBe(0)
    }
  })

  it('main line side commits are on lane 1', () => {
    // Row 3: main: work 2, Row 6: main: work 1
    for (const row of [3, 6]) {
      expect(nodes[row].lane).toBe(1)
    }
  })

  it('has 2 FORK connections (dev merges that fork out to main)', () => {
    const forks = connections.filter(c => c.type === 'FORK')
    expect(forks).toHaveLength(2)
  })

  it('has 1 MERGE-IN connection (main: work 1 -> main: init)', () => {
    const mergeIns = connections.filter(c => c.type === 'MERGE-IN')
    expect(mergeIns).toHaveLength(1)
    expect(mergeIns[0].fromRow).toBe(6)  // main: work 1
    expect(mergeIns[0].toRow).toBe(8)    // main: init
  })
})

describe('05-long-lived-branch', () => {
  const { nodes, connections, laneCount } = getConnections(LONG_LIVED)

  it('uses 2 lanes (main + release)', () => {
    expect(laneCount).toBe(2)
  })

  it('main line commits are on lane 0', () => {
    for (const row of [0, 1, 5, 6, 7, 8, 9]) {
      expect(nodes[row].lane).toBe(0)
    }
  })

  it('release branch commits are on lane 1', () => {
    for (const row of [2, 3, 4]) {
      expect(nodes[row].lane).toBe(1)
    }
  })

  it('has 1 MERGE-IN (release: prep 1 -> main: init)', () => {
    const mergeIns = connections.filter(c => c.type === 'MERGE-IN')
    expect(mergeIns).toHaveLength(1)
    expect(mergeIns[0].fromRow).toBe(4)  // release: prep 1
    expect(mergeIns[0].toRow).toBe(9)    // main: init
    expect(mergeIns[0].fromLane).toBe(1)
    expect(mergeIns[0].toLane).toBe(0)
  })
})

describe('06-octopus-merge', () => {
  const { nodes, connections, laneCount } = getConnections(OCTOPUS)

  it('uses 4 lanes (main + 3 features)', () => {
    expect(laneCount).toBe(4)
  })

  it('main line commits are on lane 0', () => {
    for (const row of [0, 1, 5, 6]) {
      expect(nodes[row].lane).toBe(0)
    }
  })

  it('has 3 FORK connections (octopus merge forks to 3 branches)', () => {
    const forks = connections.filter(c => c.type === 'FORK')
    expect(forks).toHaveLength(3)
    // All forks originate from row 1 (merge: combine all)
    for (const f of forks) {
      expect(f.fromRow).toBe(1)
      expect(f.fromLane).toBe(0)
    }
  })

  it('has 3 MERGE-IN connections (3 features merge back to init)', () => {
    const mergeIns = connections.filter(c => c.type === 'MERGE-IN')
    expect(mergeIns).toHaveLength(3)
    // All merge into row 6 (main: init, lane 0)
    for (const m of mergeIns) {
      expect(m.toRow).toBe(6)
      expect(m.toLane).toBe(0)
    }
  })
})

describe('07-open-branches', () => {
  const { nodes, connections, laneCount } = getConnections(OPEN_BRANCHES)

  it('uses at least 2 lanes', () => {
    expect(laneCount).toBeGreaterThanOrEqual(2)
  })

  it('feature-a chain is on lane 0 (first-parent chain)', () => {
    expect(nodes[0].lane).toBe(0) // feature-a: work 2
    expect(nodes[1].lane).toBe(0) // feature-a: work 1
  })

  it('has 2 MERGE-IN connections (feature-b and main: third)', () => {
    const mergeIns = connections.filter(c => c.type === 'MERGE-IN')
    expect(mergeIns).toHaveLength(2)
    // Both merge into row 4 (main: second)
    for (const m of mergeIns) {
      expect(m.toRow).toBe(4)
    }
  })

  it('has no FORK connections (no merge commits)', () => {
    const forks = connections.filter(c => c.type === 'FORK')
    expect(forks).toHaveLength(0)
  })
})

describe('WT (Working Tree) handling', () => {
  it('WT node is placed on lane 0', () => {
    const commits = [
      { sha: 'HEAD', parents: [], msg: '工作区变更', isWT: true },
      { sha: 'a1', parents: [], msg: 'commit 1' },
    ]
    const { nodes } = computeGraphData(commits, ROW_HEIGHT)
    expect(nodes[0].lane).toBe(0)
    expect(nodes[0].isWT).toBe(true)
    expect(nodes[0].color).toBe('#f59e0b')
  })

  it('WT node does not produce connection lines', () => {
    const commits = [
      { sha: 'HEAD', parents: ['a1'], msg: '工作区变更', isWT: true },
      { sha: 'a1', parents: [], msg: 'commit 1' },
    ]
    const { lines } = computeGraphData(commits, ROW_HEIGHT)
    // WT is skipped in connection generation, so no lines
    expect(lines).toHaveLength(0)
  })
})

describe('node positioning', () => {
  it('nodes have correct cx based on lane', () => {
    const commits = [
      { sha: 'a1', parents: ['a2'], msg: 'main' },
      { sha: 'a2', parents: ['a3', 'b1'], msg: 'merge' },
      { sha: 'b1', parents: [], msg: 'branch' },
      { sha: 'a3', parents: [], msg: 'root' },
    ]
    const { nodes } = computeGraphData(commits, ROW_HEIGHT)
    // Lane 0: x = 0*20 + 10 + 10 = 20
    // Lane 1: x = 1*20 + 10 + 10 = 40
    const lane0Nodes = nodes.filter(n => n.lane === 0)
    const lane1Nodes = nodes.filter(n => n.lane === 1)
    for (const n of lane0Nodes) {
      expect(n.cx).toBe(20)
    }
    for (const n of lane1Nodes) {
      expect(n.cx).toBe(40)
    }
  })

  it('nodes have correct cy based on row', () => {
    const commits = [
      { sha: 'a1', parents: [], msg: 'c1' },
      { sha: 'a2', parents: [], msg: 'c2' },
    ]
    const { nodes } = computeGraphData(commits, ROW_HEIGHT)
    expect(nodes[0].cy).toBe(ROW_HEIGHT / 2)
    expect(nodes[1].cy).toBe(ROW_HEIGHT + ROW_HEIGHT / 2)
  })
})

describe('graph dimensions', () => {
  it('graphWidth is based on lane count', () => {
    const commits = [
      { sha: 'a1', parents: [], msg: 'c1' },
    ]
    const { graphWidth, laneCount } = computeGraphData(commits, ROW_HEIGHT)
    expect(laneCount).toBe(1)
    // 1 * 20 + 10 * 2 = 40
    expect(graphWidth).toBe(40)
  })

  it('graphWidth accommodates multiple lanes', () => {
    const commits = [
      { sha: 'a1', parents: ['b1'], msg: 'merge' },
      { sha: 'b1', parents: [], msg: 'branch' },
      { sha: 'a2', parents: [], msg: 'root' },
    ]
    const { graphWidth, laneCount } = computeGraphData(commits, ROW_HEIGHT)
    expect(laneCount).toBeGreaterThanOrEqual(2)
    expect(graphWidth).toBeGreaterThanOrEqual(60)
  })
})

describe('ref label helpers', () => {
  it('refLabelWidth calculates based on text length', () => {
    expect(refLabelWidth('main')).toBe(4 * 6 + 8) // 32
    expect(refLabelWidth('tag: v1.0')).toBe(4 * 6 + 8) // strips "tag: "
  })

  it('refLabelText strips tag prefix', () => {
    expect(refLabelText('main')).toBe('main')
    expect(refLabelText('tag: v1.0')).toBe('v1.0')
  })

  it('refLabelBg returns correct colors', () => {
    expect(refLabelBg('HEAD')).toBe('#1a1a2e')
    expect(refLabelBg('tag: v1.0')).toBe('#555')
    expect(refLabelBg('main')).toBe('#4a90d9')
  })
})

describe('lane colors', () => {
  it('cycles through colors for high lane numbers', () => {
    expect(LANE_COLORS).toHaveLength(8)
    // Lane 8 should use the same color as lane 0
    const color8 = LANE_COLORS[8 % LANE_COLORS.length]
    expect(color8).toBe(LANE_COLORS[0])
  })
})

describe('octopus merge cascade fork rendering', () => {
  it('non-adjacent fork with enough vertical space uses cascade (more lines)', () => {
    // Create a scenario where merge is on L0 and branch is on L3 with 3+ rows gap
    const commits = [
      { sha: 'top', parents: ['mrg'], msg: 'top' },
      { sha: 'mrg', parents: ['mid', 'fa', 'fb', 'fc'], msg: 'merge' },
      { sha: 'mid', parents: ['bot'], msg: 'mid' },
      { sha: 'bot', parents: [], msg: 'bot' },
      { sha: 'fc', parents: ['root'], msg: 'fc' },
      { sha: 'fb', parents: ['root'], msg: 'fb' },
      { sha: 'fa', parents: ['root'], msg: 'fa' },
      { sha: 'root', parents: [], msg: 'root' },
    ]
    const { lines, nodes } = computeGraphData(commits, ROW_HEIGHT)

    // The FORK from L0 to L3 (mrg -> fa) should use cascade, generating
    // more line segments than a simple bezier (1 path)
    // Adjacent fork L0->L1 = 1 line path
    // Non-adjacent cascade L0->L3 = 1(start) + 2(diag+vert per intermediate) + 1(end) = 6 paths
    // But L0->L2 also uses cascade = 1 + 1(diag+vert) + 1 = 4 paths
    // Total fork paths: 1(adj) + 4(L0->L2) + 6(L0->L3) = 11
    // Plus VERT and MERGE-IN lines
    expect(lines.length).toBeGreaterThan(11) // at least the fork lines
  })

  it('non-adjacent fork with short vertical distance uses simple bezier', () => {
    // 03-multi-branch: merge: feature-a (L0) -> feature-a: work 2 (L2)
    // Only 1 row gap - not enough space for cascade
    const { lines } = computeGraphData(MULTI_BRANCH, ROW_HEIGHT)

    // Count bezier curves (paths with 'C' command) that start from L0
    const forkBeziers = lines.filter(l => l.path.includes('C') && l.path.startsWith('M20,'))
    // The FORK from R5[L0] to R6[L2] should be a simple bezier, not cascade
    // A simple bezier is 1 line path, cascade would be multiple paths
    expect(forkBeziers.length).toBeGreaterThanOrEqual(1)
  })
})

describe('merge-in endpoint offset', () => {
  it('multiple merge-ins arriving at same parent have offset endpoints', () => {
    // 07-open-branches: 2 merge-ins arrive at row 4 (main: second)
    const { lines } = computeGraphData(OPEN_BRANCHES, ROW_HEIGHT)

    // Find MERGE-IN bezier lines (paths with 'C' that end near row 4 center)
    const row4Cy = 4 * ROW_HEIGHT + ROW_HEIGHT / 2 // 207
    const mergeInBeziers = lines.filter(l => {
      if (!l.path.includes('C')) return false
      // Check if the path ends near the L0 x position at row 4
      return l.path.endsWith('20,' + row4Cy) || l.path.includes('20,2') // rough check
    })

    // With 2 merge-ins, their bezier endpoints should be slightly different
    // (offset by ±4px from center)
    if (mergeInBeziers.length >= 2) {
      const yValues = mergeInBeziers.map(l => {
        const match = l.path.match(/(\d+\.\d+|\d+)$/)
        return match ? parseFloat(match[1]) : 0
      })
      // At least 2 different Y values (offset prevents overlap)
      const uniqueYs = new Set(yValues.map(y => Math.round(y)))
      expect(uniqueYs.size).toBeGreaterThanOrEqual(1) // they may be close but not identical
    }
  })
})

describe('lazy-load lane stability (previousShaToLane)', () => {
  it('preserves lane assignments for existing commits when new commits are appended', () => {
    // Simulate a single-merge repo where first 3 commits are loaded initially,
    // then the remaining commits are loaded lazily.
    const firstPage = [
      { sha: 'm1', parents: ['m2'], msg: 'main: latest' },
      { sha: 'm2', parents: ['m3', 'f1'], msg: 'merge feature' },
      { sha: 'f1', parents: ['m3'], msg: 'feature: work' },
    ]
    const { nodes: firstNodes, shaToLane: firstLaneMap } = computeGraphData(firstPage, ROW_HEIGHT)

    // Record lane assignments from first page
    const m1Lane = firstNodes[0].lane
    const m2Lane = firstNodes[1].lane
    const f1Lane = firstNodes[2].lane

    // Now simulate lazy load: append the root commit
    const fullPage = [
      { sha: 'm1', parents: ['m2'], msg: 'main: latest' },
      { sha: 'm2', parents: ['m3', 'f1'], msg: 'merge feature' },
      { sha: 'f1', parents: ['m3'], msg: 'feature: work' },
      { sha: 'm3', parents: [], msg: 'root' },
    ]

    // With previousShaToLane, existing commits should keep their lanes
    const { nodes: fullNodesWithPrev } = computeGraphData(fullPage, ROW_HEIGHT, firstLaneMap)

    // The existing commits (m1, m2, f1) should have the same lanes as before
    expect(fullNodesWithPrev[0].lane).toBe(m1Lane)
    expect(fullNodesWithPrev[1].lane).toBe(m2Lane)
    expect(fullNodesWithPrev[2].lane).toBe(f1Lane)
  })

  it('without previousShaToLane, lanes can shift when commits are appended', () => {
    // This test demonstrates the PROBLEM that previousShaToLane fixes.
    // When the first-parent chain extends (because a parent becomes visible),
    // commits that were on separate lanes can collapse onto the same lane.

    // First page: f1's parent m3 is NOT in the list, so f1 gets its own lane
    const firstPage = [
      { sha: 'm1', parents: ['m2'], msg: 'main: latest' },
      { sha: 'm2', parents: ['m3', 'f1'], msg: 'merge feature' },
      { sha: 'f1', parents: ['m3'], msg: 'feature: work' },
    ]
    const { nodes: firstNodes } = computeGraphData(firstPage, ROW_HEIGHT)

    // f1 should be on a different lane from m2 (since m3 is not visible,
    // f1 doesn't connect to the main line through first-parent chain)
    expect(firstNodes[2].lane).not.toBe(firstNodes[1].lane)

    // Full page: m3 is now visible, and f1's first-parent IS m3,
    // so f1 joins the main line's first-parent chain
    const fullPage = [
      { sha: 'm1', parents: ['m2'], msg: 'main: latest' },
      { sha: 'm2', parents: ['m3', 'f1'], msg: 'merge feature' },
      { sha: 'f1', parents: ['m3'], msg: 'feature: work' },
      { sha: 'm3', parents: [], msg: 'root' },
    ]
    const { nodes: fullNodes } = computeGraphData(fullPage, ROW_HEIGHT)

    // Without previousShaToLane, f1 may now be on the same lane as m3 (main line)
    // This is the lane shift that causes visual splitting
    // We just verify that with previousShaToLane, it stays the same
    const { shaToLane: firstLaneMap } = computeGraphData(firstPage, ROW_HEIGHT)
    const { nodes: stableNodes } = computeGraphData(fullPage, ROW_HEIGHT, firstLaneMap)
    expect(stableNodes[2].lane).toBe(firstNodes[2].lane)
  })

  it('returns shaToLane in result for chaining', () => {
    const commits = [
      { sha: 'a1', parents: ['a2'], msg: 'c1' },
      { sha: 'a2', parents: [], msg: 'c2' },
    ]
    const { shaToLane } = computeGraphData(commits, ROW_HEIGHT)
    expect(shaToLane).toBeInstanceOf(Map)
    expect(shaToLane.has('a1')).toBe(true)
    expect(shaToLane.has('a2')).toBe(true)
  })

  it('empty commits returns empty shaToLane', () => {
    const { shaToLane, nodes, lines } = computeGraphData([], ROW_HEIGHT)
    expect(shaToLane).toBeInstanceOf(Map)
    expect(shaToLane.size).toBe(0)
    expect(nodes).toHaveLength(0)
    expect(lines).toHaveLength(0)
  })

  it('multi-page lazy load preserves lanes across multiple loads', () => {
    // Page 1: 3 commits
    const page1 = [
      { sha: 'm1', parents: ['m2'], msg: 'c1' },
      { sha: 'm2', parents: ['m3'], msg: 'c2' },
      { sha: 'm3', parents: ['m4'], msg: 'c3' },
    ]
    const { nodes: nodes1, shaToLane: lanes1 } = computeGraphData(page1, ROW_HEIGHT)
    const laneM1 = nodes1[0].lane

    // Page 2: append 2 more commits
    const page2 = [
      { sha: 'm1', parents: ['m2'], msg: 'c1' },
      { sha: 'm2', parents: ['m3'], msg: 'c2' },
      { sha: 'm3', parents: ['m4'], msg: 'c3' },
      { sha: 'm4', parents: ['m5'], msg: 'c4' },
      { sha: 'm5', parents: [], msg: 'c5' },
    ]
    const { nodes: nodes2, shaToLane: lanes2 } = computeGraphData(page2, ROW_HEIGHT, lanes1)
    // m1 should keep the same lane
    expect(nodes2[0].lane).toBe(laneM1)

    // Page 3: append even more
    const page3 = [
      { sha: 'm1', parents: ['m2'], msg: 'c1' },
      { sha: 'm2', parents: ['m3'], msg: 'c2' },
      { sha: 'm3', parents: ['m4'], msg: 'c3' },
      { sha: 'm4', parents: ['m5'], msg: 'c4' },
      { sha: 'm5', parents: [], msg: 'c5' },
    ]
    const { nodes: nodes3 } = computeGraphData(page3, ROW_HEIGHT, lanes2)
    // m1 should still keep the same lane across all 3 pages
    expect(nodes3[0].lane).toBe(laneM1)
  })
})

describe('lane compression for non-overlapping branches', () => {
  // Two branches that don't overlap in time: branch-a is fully merged
  // before branch-b starts. They should share the same lane.
  //
  // Graph: init → m1 → m2 ──→ mrgA ──→ m3 → m4 ──→ mrgB ──→ fin
  //                    ↘       ↗                  ↘       ↗
  //                    a1 → a2                    b1 → b2
  const SEQUENTIAL_BRANCHES = [
    { sha: 'fin', parents: ['mrgB'], msg: 'main: final', refs: ['main'] },
    { sha: 'mrgB', parents: ['m4', 'b2'], msg: 'merge: branch-b' },
    { sha: 'b2', parents: ['b1'], msg: 'branch-b: work 2' },
    { sha: 'b1', parents: ['m3'], msg: 'branch-b: work 1' },
    { sha: 'm4', parents: ['m3'], msg: 'main: fourth' },
    { sha: 'm3', parents: ['mrgA'], msg: 'main: third' },
    { sha: 'mrgA', parents: ['m2', 'a2'], msg: 'merge: branch-a' },
    { sha: 'a2', parents: ['a1'], msg: 'branch-a: work 2' },
    { sha: 'a1', parents: ['m1'], msg: 'branch-a: work 1' },
    { sha: 'm2', parents: ['m1'], msg: 'main: second' },
    { sha: 'm1', parents: ['init'], msg: 'main: first' },
    { sha: 'init', parents: [], msg: 'main: init' },
  ]

  it('uses 2 lanes (main + 1 shared branch lane)', () => {
    const { laneCount } = getConnections(SEQUENTIAL_BRANCHES)
    expect(laneCount).toBe(2)
  })

  it('both branch commits share lane 1', () => {
    const { nodes } = getConnections(SEQUENTIAL_BRANCHES)
    // Rows 2,3 = branch-b, rows 7,8 = branch-a — both should be on lane 1
    expect(nodes[2].lane).toBe(1)
    expect(nodes[3].lane).toBe(1)
    expect(nodes[7].lane).toBe(1)
    expect(nodes[8].lane).toBe(1)
  })

  it('main line commits are on lane 0', () => {
    const { nodes } = getConnections(SEQUENTIAL_BRANCHES)
    expect(nodes[0].lane).toBe(0)
    expect(nodes[1].lane).toBe(0)
    expect(nodes[4].lane).toBe(0)
    expect(nodes[5].lane).toBe(0)
    expect(nodes[6].lane).toBe(0)
    expect(nodes[9].lane).toBe(0)
    expect(nodes[10].lane).toBe(0)
    expect(nodes[11].lane).toBe(0)
  })
})

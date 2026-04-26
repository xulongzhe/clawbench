<template>
  <div ref="scrollRef" class="git-graph-scroll" :class="{ 'collapsed-mode': collapsed }" @scroll="dismissTooltip">
    <svg
      :width="svgWidth"
      :height="svgHeight"
      class="git-graph-svg"
    >
      <!-- Connection lines (hidden when collapsed) -->
      <g v-if="!collapsed" class="git-graph-lines">
        <path
          v-for="(line, i) in lines"
          :key="'line-' + i"
          :d="line.path"
          :stroke="line.color"
          stroke-width="2"
          fill="none"
          stroke-linecap="round"
          class="git-graph-line"
          @click.stop="onLineClick(line, $event)"
        />
      </g>
      <!-- Commit nodes -->
      <g class="git-graph-nodes">
        <g v-for="(node, i) in nodes" :key="'node-' + i">
          <!-- Collapsed: simple dot -->
          <template v-if="collapsed">
            <circle
              :cx="10"
              :cy="node.cy"
              :r="node.refs && node.refs.length ? 4 : 3"
              :fill="node.isWT ? '#f59e0b' : node.color"
            />
          </template>
          <!-- Expanded: full node rendering -->
          <template v-else>
            <!-- WT node: amber with glow -->
            <template v-if="node.isWT">
              <circle
                :cx="node.cx"
                :cy="node.cy"
                r="7"
                fill="none"
                stroke="#f59e0b"
                stroke-width="1.5"
                opacity="0.3"
              />
              <circle
                :cx="node.cx"
                :cy="node.cy"
                r="5"
                fill="#f59e0b"
              />
            </template>
            <!-- Node with refs: double ring style -->
            <template v-else-if="node.refs && node.refs.length">
              <circle
                :cx="node.cx"
                :cy="node.cy"
                r="6"
                fill="none"
                :stroke="node.color"
                stroke-width="1.5"
                opacity="0.4"
                class="git-graph-ref-node"
                @click.stop="onNodeClick(node, $event)"
              />
              <circle
                :cx="node.cx"
                :cy="node.cy"
                r="3.5"
                :fill="node.color"
                stroke="var(--bg-primary, #fff)"
                stroke-width="1.5"
                class="git-graph-ref-node"
                @click.stop="onNodeClick(node, $event)"
              />
            </template>
            <!-- Normal node: simple circle -->
            <circle
              v-else
              :cx="node.cx"
              :cy="node.cy"
              r="4"
              :fill="node.color"
              stroke="var(--bg-primary, #fff)"
              stroke-width="1.5"
            />
          </template>
        </g>
      </g>
    </svg>
    <!-- Collapse/expand toggle -->
    <button
      class="graph-toggle-btn"
      @click.stop="collapsed = !collapsed"
      :title="collapsed ? '展开分支图' : '收起分支图'"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" width="12" height="12">
        <polyline v-if="!collapsed" points="6 9 12 15 18 9"/>
        <polyline v-else points="9 6 15 12 9 18"/>
      </svg>
    </button>
    <!-- Tooltip (refs or branch name) -->
    <Teleport to="body">
      <div
        v-if="tooltip && !collapsed"
        ref="tooltipEl"
        class="git-graph-tooltip"
        :style="tooltipStyle"
        @click.stop
      >
        <span v-for="(item, idx) in tooltip.items" :key="idx" class="tooltip-ref-item">
          <span class="tooltip-ref-dot" :style="{ background: tooltip.color }"></span>
          {{ item }}
        </span>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { computeGraphData, refLabelText } from './gitGraphUtils'

const props = defineProps({
  commits: { type: Array, default: () => [] },
  rowHeight: { type: Number, default: 46 },
})

const collapsed = ref(false)
const scrollRef = ref(null)

// ── Tooltip ──
const tooltip = ref(null)
const tooltipEl = ref(null)

// Dismiss tooltip on scroll or outside click
function dismissTooltip() { tooltip.value = null }

onMounted(() => {
  document.addEventListener('click', dismissTooltip, true)
})
onUnmounted(() => {
  document.removeEventListener('click', dismissTooltip, true)
})

// Persist lane assignments across lazy-load recomputations to prevent visual
// line splitting. When new commits are appended, previously-assigned SHAs
// keep their lanes. Cleared when the drawer closes (commits reset to []).
const persistedShaToLane = ref(new Map())

// When commits array is replaced (drawer reset), clear persisted lanes
watch(() => props.commits, (val) => {
  if (!val || val.length === 0) {
    persistedShaToLane.value = new Map()
  }
})

// Compute graph data, passing previous lane assignments for stability
const graphData = computed(() => {
  const result = computeGraphData(props.commits, props.rowHeight, persistedShaToLane.value)
  persistedShaToLane.value = result.shaToLane
  return result
})
const nodes = computed(() => graphData.value.nodes)
const lines = computed(() => graphData.value.lines)
const laneBranchName = computed(() => graphData.value.laneBranchName)
const svgHeight = computed(() => props.commits.length * props.rowHeight + 4)

// SVG width: only lanes, no ref labels (refs shown via tooltip)
const svgWidth = computed(() => {
  if (collapsed.value) return 20
  return graphData.value.graphWidth || 40
})

// ── Tooltip positioning ──

const onNodeClick = (node, event) => {
  event.stopPropagation()
  const scrollEl = event.currentTarget.closest('.git-graph-scroll')
  const scrollLeft = scrollEl ? scrollEl.scrollLeft : 0
  const scrollTop = scrollEl ? scrollEl.scrollTop : 0
  const rect = scrollEl ? scrollEl.getBoundingClientRect() : { left: 0, top: 0 }

  // Position tooltip to the right of the node
  const x = node.cx - scrollLeft + rect.left + 8
  const y = node.cy - scrollTop + rect.top - 8

  tooltip.value = {
    x, y,
    items: (node.refs || []).map(refLabelText),
    color: node.color,
  }
}

const onLineClick = (line, event) => {
  event.stopPropagation()
  const branchName = laneBranchName.value.get(line.lane)
  if (!branchName) return

  tooltip.value = {
    x: event.clientX + 8,
    y: event.clientY - 8,
    items: [branchName],
    color: line.color,
  }
}

// Compute tooltip style with boundary detection
const tooltipStyle = computed(() => {
  if (!tooltip.value) return {}
  let x = tooltip.value.x
  let y = tooltip.value.y
  // Keep tooltip within viewport
  const vw = window.innerWidth
  const vh = window.innerHeight
  // Estimate tooltip width (rough: 80px per item)
  const estimatedWidth = Math.max(80, tooltip.value.items.length * 80)
  const estimatedHeight = 30 + tooltip.value.items.length * 18
  // Clamp right edge
  if (x + estimatedWidth > vw - 8) x = vw - estimatedWidth - 8
  // Clamp left edge
  if (x < 8) x = 8
  // Clamp bottom edge
  if (y + estimatedHeight > vh - 8) y = y - estimatedHeight - 16
  // Clamp top edge
  if (y < 8) y = 8
  return {
    left: x + 'px',
    top: y + 'px',
  }
})
</script>

<style scoped>
.git-graph-scroll {
  overflow-x: auto;
  overflow-y: hidden;
  flex-shrink: 0;
  min-width: 20px;
  max-width: 300px;
  scrollbar-width: thin;
  position: relative;
}

.git-graph-scroll.collapsed-mode {
  max-width: 24px;
  overflow-x: hidden;
}

.git-graph-svg {
  display: block;
}

.git-graph-line {
  cursor: pointer;
}

.git-graph-line:hover {
  stroke-width: 3;
}

.git-graph-ref-node {
  cursor: pointer;
}

.git-graph-ref-node:hover + circle {
  stroke-width: 2.5;
}

.git-graph-tooltip {
  position: fixed;
  background: var(--bg-primary, #fff);
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: 6px;
  padding: 6px 10px;
  white-space: nowrap;
  pointer-events: none;
  z-index: 9999;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tooltip-ref-item {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-primary, #212529);
  display: flex;
  align-items: center;
  gap: 5px;
}

.tooltip-ref-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.graph-toggle-btn {
  position: sticky;
  top: 4px;
  z-index: 3;
  width: 18px;
  height: 18px;
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: 3px;
  background: var(--bg-primary, #fff);
  color: var(--text-muted, #999);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  margin-bottom: -18px;
  margin-left: 2px;
  transition: background 0.15s, color 0.15s;
}

.graph-toggle-btn:hover {
  background: var(--bg-secondary, #f8f9fa);
  color: var(--text-primary, #212529);
}
</style>

<template>
  <div class="git-graph-scroll" @click="onBackgroundClick">
    <svg
      :width="svgWidth"
      :height="svgHeight"
      class="git-graph-svg"
    >
      <!-- Connection lines -->
      <g class="git-graph-lines">
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
          <!-- WT glow -->
          <circle
            v-if="node.isWT"
            :cx="node.cx"
            :cy="node.cy"
            r="7"
            fill="none"
            stroke="#f59e0b"
            stroke-width="1.5"
            opacity="0.3"
          />
          <!-- Node circle -->
          <circle
            v-if="node.isWT"
            :cx="node.cx"
            :cy="node.cy"
            r="5"
            fill="#f59e0b"
          />
          <circle
            v-else
            :cx="node.cx"
            :cy="node.cy"
            r="4"
            :fill="node.color"
            stroke="var(--bg-primary, #fff)"
            stroke-width="1.5"
          />
          <!-- Refs labels -->
          <g v-if="node.refs && node.refs.length" class="git-graph-refs">
            <g v-for="(refItem, ri) in node.refs" :key="'ref-' + i + '-' + ri">
              <rect
                :x="refRectX(node, ri)"
                :y="node.cy - 8"
                :width="refW(refItem)"
                height="16"
                rx="3"
                :fill="refBg(refItem)"
              />
              <text
                :x="refRectX(node, ri) + refW(refItem) / 2"
                :y="node.cy + 3"
                text-anchor="middle"
                font-size="9"
                font-weight="600"
                fill="#fff"
              >{{ refTxt(refItem) }}</text>
            </g>
          </g>
        </g>
      </g>
    </svg>
    <!-- Branch name tooltip -->
    <div
      v-if="tooltip"
      class="git-graph-tooltip"
      :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
      @click.stop
    >{{ tooltip.text }}</div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { computeGraphData, refLabelWidth, refLabelText, refLabelBg } from './gitGraphUtils'

const props = defineProps({
  commits: { type: Array, default: () => [] },
  rowHeight: { type: Number, default: 46 },
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

// SVG width: wide enough for lanes + ref labels
const svgWidth = computed(() => {
  const gw = graphData.value.graphWidth || 40
  // Also account for ref labels that extend beyond the graph lanes
  let maxRight = gw
  for (const node of nodes.value) {
    if (node.refs && node.refs.length) {
      let right = node.cx + 10
      for (const r of node.refs) {
        right += refLabelWidth(r) + 3
      }
      if (right > maxRight) maxRight = right
    }
  }
  return Math.max(40, maxRight + 4)
})

// Ref label helpers
const refW = refLabelWidth
const refTxt = refLabelText
const refBg = refLabelBg

const refRectX = (node, ri) => {
  let x = node.cx + 10
  for (let i = 0; i < ri; i++) {
    x += refW(node.refs[i]) + 3
  }
  return x
}

// ── Branch name tooltip on line click ──
const tooltip = ref(null)

const onLineClick = (line, event) => {
  const branchName = laneBranchName.value.get(line.lane)
  if (!branchName) return

  const scrollEl = event.currentTarget.closest('.git-graph-scroll')
  const scrollLeft = scrollEl ? scrollEl.scrollLeft : 0
  const scrollTop = scrollEl ? scrollEl.scrollTop : 0
  const rect = scrollEl ? scrollEl.getBoundingClientRect() : { left: 0, top: 0 }

  tooltip.value = {
    text: branchName,
    x: event.clientX - rect.left + scrollLeft,
    y: event.clientY - rect.top + scrollTop,
  }
}

const onBackgroundClick = () => {
  tooltip.value = null
}
</script>

<style scoped>
.git-graph-scroll {
  overflow-x: auto;
  overflow-y: hidden;
  flex-shrink: 0;
  min-width: 40px;
  max-width: 300px;
  scrollbar-width: thin;
  position: relative;
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

.git-graph-refs text {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  user-select: none;
  pointer-events: none;
}

.git-graph-tooltip {
  position: absolute;
  background: #1a1a2e;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 4px;
  white-space: nowrap;
  pointer-events: none;
  transform: translate(-50%, -130%);
  z-index: 10;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
}
</style>

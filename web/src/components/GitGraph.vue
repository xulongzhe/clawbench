<template>
  <div class="git-graph-scroll">
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
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { computeGraphData, refLabelWidth, refLabelText, refLabelBg } from './gitGraphUtils'

const props = defineProps({
  commits: { type: Array, default: () => [] },
  rowHeight: { type: Number, default: 46 },
})

// Compute graph data
const graphData = computed(() => computeGraphData(props.commits, props.rowHeight))
const nodes = computed(() => graphData.value.nodes)
const lines = computed(() => graphData.value.lines)
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
</script>

<style scoped>
.git-graph-scroll {
  overflow-x: auto;
  overflow-y: hidden;
  flex-shrink: 0;
  min-width: 40px;
  max-width: 300px;
  scrollbar-width: thin;
}

.git-graph-svg {
  display: block;
}

.git-graph-refs text {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  user-select: none;
  pointer-events: none;
}
</style>

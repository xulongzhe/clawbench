<template>
  <div class="git-manage-content">
    <GitWorktreeList
      :worktrees="worktrees"
      :loading="worktreesLoading"
      :error="worktreesError"
      :initial-collapsed="worktreesCollapsed"
      @switch-worktree="onSwitchWorktree"
      @retry="loadWorktrees"
    />
    <GitBranchList
      :branches="branches"
      :stash-count="stashCount"
      :loading="branchesLoading"
      :error="branchesError"
      :checkout-in-progress="checkoutInProgress"
      @switch-branch="onSwitchBranch"
      @retry="loadBranches"
    />

    <!-- Checkout options BottomSheet (dirty worktree) -->
    <BottomSheet :open="showCheckoutSheet" compact @close="showCheckoutSheet = false" :title="t('git.manage.switchBranch')">
      <div class="checkout-sheet-body">
        <p>{{ t('git.manage.dirty', { count: dirtyCount }) }}</p>
        <button class="checkout-option-btn checkout-stash-btn" @click="doCheckout('stash')">{{ t('git.manage.stashSwitch') }}</button>
        <button class="checkout-option-btn checkout-force-btn" @click="confirmForceCheckout">{{ t('git.manage.forceSwitch') }}</button>
        <button class="checkout-option-btn checkout-cancel-btn" @click="showCheckoutSheet = false">{{ t('common.cancel') }}</button>
      </div>
    </BottomSheet>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { store } from '@/stores/app.ts'
import { apiGet, apiPost } from '@/utils/api'
import { useDialog } from '@/composables/useDialog.ts'
import BottomSheet from '@/components/common/BottomSheet.vue'
import GitWorktreeList from './GitWorktreeList.vue'
import GitBranchList from './GitBranchList.vue'

const { t } = useI18n()
const dialog = useDialog()
const hotSwitchProject = inject('hotSwitchProject', null) as ((path: string, pendingSessionId?: string) => Promise<void>) | null

const worktrees = ref<any[]>([])
const branches = ref<any[]>([])
const stashCount = ref(0)

const worktreesLoading = ref(false)
const worktreesError = ref(false)
const branchesLoading = ref(false)
const branchesError = ref(false)

const checkoutInProgress = ref(false)
const showCheckoutSheet = ref(false)
const pendingBranch = ref<any>(null)
const dirtyCount = ref(0)
const worktreesCollapsed = ref(false)

async function loadWorktrees() {
  worktreesLoading.value = true
  worktreesError.value = false
  try {
    const data = await apiGet<{ isGit: boolean; worktrees: any[] }>('/api/git/worktrees')
    worktrees.value = data.worktrees || []
    if (worktrees.value.length <= 1) {
      worktreesCollapsed.value = true
    }
  } catch {
    worktreesError.value = true
  } finally {
    worktreesLoading.value = false
  }
}

async function loadBranches() {
  branchesLoading.value = true
  branchesError.value = false
  try {
    const data = await apiGet<{ isGit: boolean; branches: any[]; stashCount?: number }>('/api/git/branches')
    branches.value = data.branches || []
    stashCount.value = data.stashCount || 0
  } catch {
    branchesError.value = true
  } finally {
    branchesLoading.value = false
  }
}

onMounted(() => {
  Promise.all([loadWorktrees(), loadBranches()])
})

async function onSwitchWorktree(wt: any) {
  // Warn if AI session is running
  if (store.state.chatRunning) {
    const ok = await dialog.confirm(t('git.manage.switchWhileRunning'), { dangerous: true })
    if (!ok) return
  }
  const confirmed = await dialog.confirm(
    t('git.manage.switchWorktreeConfirm', { name: wt.branch || wt.displayPath }),
    { title: t('git.manage.switchWorktree') },
  )
  if (!confirmed) return
  if (hotSwitchProject) {
    await hotSwitchProject(wt.path)
  } else {
    await store.setProject(wt.path)
  }
}

async function onSwitchBranch(branch: any) {
  checkoutInProgress.value = true
  try {
    const result = await apiPost<{ success: boolean; error?: string; untrackedCount?: number; errorDetail?: string }>('/api/git/checkout', { branch: branch.name })
    if (result.success) {
      await store.loadGitBranch()
      await Promise.all([loadBranches(), loadWorktrees()])
    } else if (result.error === 'dirty_worktree') {
      pendingBranch.value = branch
      dirtyCount.value = result.untrackedCount || 0
      showCheckoutSheet.value = true
    } else if (result.error) {
      const errorMessages: Record<string, string> = {
        checkout_conflict: t('git.manage.checkoutConflict'),
        hook_rejected: t('git.manage.hookRejected'),
        branch_not_found: t('git.manage.branchNotFound'),
        checkout_in_progress: t('git.manage.checkoutInProgress'),
        checkout_failed: result.errorDetail || t('git.manage.checkoutFailed'),
      }
      await dialog.alert(errorMessages[result.error] || t('git.manage.checkoutFailed'))
      if (result.error === 'branch_not_found') {
        await loadBranches()
      }
    }
  } finally {
    checkoutInProgress.value = false
  }
}

async function doCheckout(mode: 'stash' | 'force') {
  showCheckoutSheet.value = false
  checkoutInProgress.value = true
  try {
    await apiPost('/api/git/checkout', {
      branch: pendingBranch.value.name,
      stash: mode === 'stash',
      force: mode === 'force',
    })
    await store.loadGitBranch()
    await Promise.all([loadBranches(), loadWorktrees()])
  } finally {
    checkoutInProgress.value = false
    pendingBranch.value = null
  }
}

async function confirmForceCheckout() {
  const confirmed = await dialog.confirm(t('git.manage.forceSwitchConfirm'), { dangerous: true })
  if (confirmed) {
    await doCheckout('force')
  }
}
</script>

<style scoped>
.git-manage-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.checkout-sheet-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 0;
}

.checkout-sheet-body p {
  font-size: 13px;
  color: var(--text-secondary, #666);
  margin: 0;
}

.checkout-option-btn {
  width: 100%;
  padding: 10px;
  border-radius: 6px;
  border: 1px solid;
  font-size: 14px;
  cursor: pointer;
  text-align: center;
  background: transparent;
}

.checkout-stash-btn {
  border-color: var(--accent-color, #4a90d9);
  color: var(--accent-color, #4a90d9);
}

.checkout-force-btn {
  border-color: var(--danger-color, #dc3545);
  color: var(--danger-color, #dc3545);
}

.checkout-cancel-btn {
  border-color: var(--border-color, #dee2e6);
  color: var(--text-secondary, #666);
}
</style>

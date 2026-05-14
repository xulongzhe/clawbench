<template>
  <div class="login-page">
    <!-- Decorative background elements -->
    <div class="login-bg-gradient"></div>
    <div class="login-bg-grid"></div>

    <div class="login-content">
      <!-- Brand section -->
      <div class="login-brand">
        <div class="login-logo-wrapper">
          <img class="login-logo" src="/logo.png" alt="ClawBench">
          <div class="login-logo-ring"></div>
        </div>
        <h1 class="login-title">ClawBench</h1>
        <p class="login-slogan">{{ t('login.slogan') }}</p>
        <p class="login-subtitle">{{ t('login.subtitle') }}</p>
      </div>

      <!-- Form section -->
      <div class="login-form-card">
        <form @submit.prevent="handleLogin">
          <div class="input-group">
            <svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
              <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
            </svg>
            <input
              type="password"
              v-model="password"
              :placeholder="t('login.passwordPlaceholder')"
              autocomplete="current-password"
              :disabled="loading"
            />
          </div>
          <button type="submit" :disabled="loading" class="login-btn">
            <span v-if="loading" class="btn-spinner"></span>
            <span>{{ loading ? t('login.verifying') : t('login.submit') }}</span>
          </button>
          <div v-if="error" class="error">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
              <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
            </svg>
            {{ error }}
            <button v-if="isAppMode && networkError" class="reconfigure-link" @click="handleReconfigure">{{ t('appHeader.reconfigureServer') }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppMode } from '@/composables/useAppMode'

const { t } = useI18n()
const { isAppMode } = useAppMode()
const emit = defineEmits(['loginSuccess'])

const password = ref('')
const loading = ref(false)
const error = ref('')
const networkError = ref(false)

async function handleLogin() {
    if (!password.value) return
    loading.value = true
    error.value = ''
    networkError.value = false
    try {
        const res = await fetch('/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password: password.value })
        })
        if (res.ok) {
            // Save password to Android native layer for SSH tunnel authentication
            if (window.AndroidNative?.isNativeApp?.()) {
                window.AndroidNative.setSSHPassword(password.value)
            }
            emit('loginSuccess')
        } else if (res.status >= 500) {
            error.value = t('login.serverError')
        } else {
            error.value = t('login.wrongPassword')
        }
    } catch (_) {
        error.value = t('login.networkError')
        networkError.value = true
    } finally {
        loading.value = false
    }
}

function handleReconfigure() {
    if (window.AndroidNative?.showServerDialog) {
        window.AndroidNative.showServerDialog()
    }
}
</script>

<style scoped>
.login-page {
    min-height: 100vh;
    min-height: 100dvh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-primary);
    position: relative;
    overflow: hidden;
}

/* Decorative background */
.login-bg-gradient {
    position: absolute;
    inset: 0;
    background:
        radial-gradient(ellipse 60% 50% at 20% 20%, color-mix(in srgb, var(--accent-color) 8%, transparent), transparent),
        radial-gradient(ellipse 50% 60% at 80% 80%, color-mix(in srgb, var(--accent-color) 6%, transparent), transparent);
    pointer-events: none;
}

.login-bg-grid {
    position: absolute;
    inset: 0;
    background-image:
        linear-gradient(color-mix(in srgb, var(--border-color) 30%, transparent) 1px, transparent 1px),
        linear-gradient(90deg, color-mix(in srgb, var(--border-color) 30%, transparent) 1px, transparent 1px);
    background-size: 48px 48px;
    mask-image: radial-gradient(ellipse 70% 70% at center, black, transparent);
    -webkit-mask-image: radial-gradient(ellipse 70% 70% at center, black, transparent);
    opacity: 0.4;
    pointer-events: none;
}

/* Content layout */
.login-content {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 100%;
    max-width: 380px;
    padding: 0 24px;
    gap: 36px;
}

/* Brand section */
.login-brand {
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
}

.login-logo-wrapper {
    position: relative;
    width: 96px;
    height: 96px;
    margin-bottom: 20px;
}

.login-logo {
    width: 96px;
    height: 96px;
    border-radius: 50%;
    display: block;
    position: relative;
    z-index: 1;
    box-shadow: var(--shadow-md);
}

.login-logo-ring {
    position: absolute;
    inset: -4px;
    border-radius: 50%;
    border: 2px solid color-mix(in srgb, var(--accent-color) 30%, transparent);
    animation: ring-pulse 3s ease-in-out infinite;
}

@keyframes ring-pulse {
    0%, 100% { opacity: 0.4; transform: scale(1); }
    50% { opacity: 0.8; transform: scale(1.04); }
}

.login-title {
    font-size: 26px;
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: -0.02em;
    margin: 0 0 8px;
}

.login-slogan {
    font-size: 18px;
    font-weight: 500;
    color: var(--accent-color);
    margin: 0 0 4px;
    letter-spacing: 0.08em;
}

.login-subtitle {
    font-size: 13px;
    color: var(--text-muted);
    margin: 0;
}

/* Form card */
.login-form-card {
    width: 100%;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 28px 24px;
    box-shadow: var(--shadow-sm);
}

.input-group {
    position: relative;
    display: flex;
    align-items: center;
}

.input-icon {
    position: absolute;
    left: 14px;
    width: 18px;
    height: 18px;
    color: var(--text-muted);
    pointer-events: none;
    flex-shrink: 0;
}

input[type="password"] {
    width: 100%;
    padding: 13px 14px 13px 42px;
    border: 1.5px solid var(--border-color);
    border-radius: 10px;
    font-size: 15px;
    outline: none;
    background: var(--bg-primary);
    color: var(--text-primary);
    transition: border-color 0.2s, box-shadow 0.2s;
    box-sizing: border-box;
}

input[type="password"]:focus {
    border-color: var(--accent-color);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-color) 12%, transparent);
}

.login-btn {
    width: 100%;
    padding: 13px;
    margin-top: 16px;
    border: none;
    border-radius: 10px;
    background: var(--accent-color);
    color: #fff;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s, transform 0.1s, box-shadow 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
}

.login-btn:hover:not(:disabled) {
    background: var(--accent-hover);
    box-shadow: 0 4px 14px color-mix(in srgb, var(--accent-color) 30%, transparent);
}

.login-btn:active:not(:disabled) {
    transform: scale(0.98);
}

.login-btn:disabled {
    opacity: 0.6;
    cursor: default;
}

.btn-spinner {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}

.error {
    margin-top: 14px;
    padding: 10px 14px;
    border-radius: 8px;
    background: color-mix(in srgb, var(--color-red, #dc2626) 8%, var(--bg-primary));
    border: 1px solid color-mix(in srgb, var(--color-red, #dc2626) 20%, var(--border-color));
    color: var(--color-red, #dc2626);
    font-size: 13px;
    display: flex;
    align-items: center;
    gap: 8px;
}

.error svg {
    flex-shrink: 0;
}

.reconfigure-link {
    margin-left: auto;
    padding: 2px 8px;
    border: 1px solid color-mix(in srgb, var(--color-red, #dc2626) 40%, transparent);
    border-radius: 6px;
    background: color-mix(in srgb, var(--color-red, #dc2626) 10%, transparent);
    color: var(--color-red, #dc2626);
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    transition: background 0.15s;
}

.reconfigure-link:hover {
    background: color-mix(in srgb, var(--color-red, #dc2626) 20%, transparent);
}
</style>

<template>
  <div class="login-page">
    <div class="login-box">
      <img class="login-logo" src="/logo.png" alt="ClawBench">
      <h1>ClawBench</h1>
      <p>请输入密码以继续访问。</p>
      <form @submit.prevent="handleLogin">
        <input
          type="password"
          v-model="password"
          placeholder="请输入密码"
          autocomplete="current-password"
          :disabled="loading"
        />
        <button type="submit" :disabled="loading">
          {{ loading ? '验证中...' : '登 录' }}
        </button>
        <div v-if="error" class="error">{{ error }}</div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const emit = defineEmits(['loginSuccess'])

const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
    if (!password.value) return
    loading.value = true
    error.value = ''
    try {
        const res = await fetch('/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password: password.value })
        })
        if (res.ok) {
            emit('loginSuccess')
        } else if (res.status >= 500) {
            error.value = '服务器错误，请稍后重试。'
        } else {
            error.value = '密码错误，请重试。'
        }
    } catch (_) {
        error.value = '网络错误，请检查后端服务是否启动。'
    } finally {
        loading.value = false
    }
}
</script>

<style scoped>
.login-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-primary);
}

.login-box {
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    box-shadow: var(--shadow-md);
    padding: 40px 32px;
    width: 100%;
    max-width: 380px;
    margin: 0 16px;
}

h1 {
    font-size: 22px;
    margin-bottom: 8px;
    color: var(--text-primary);
    text-align: center;
}

p {
    color: var(--text-secondary);
    font-size: 14px;
    margin-bottom: 28px;
    text-align: center;
}

input[type="password"] {
    width: 100%;
    padding: 12px 14px;
    border: 1.5px solid var(--border-color);
    border-radius: 8px;
    font-size: 15px;
    outline: none;
    background: var(--bg-primary);
    color: var(--text-primary);
    transition: border-color 0.2s;
    box-sizing: border-box;
}

input[type="password"]:focus {
    border-color: var(--accent-color);
}

button {
    width: 100%;
    padding: 13px;
    margin-top: 20px;
    border: none;
    border-radius: 8px;
    background: var(--accent-color);
    color: #fff;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s;
}

button:hover:not(:disabled) {
    background: var(--accent-hover);
}

button:disabled {
    opacity: 0.6;
    cursor: default;
}

.error {
    margin-top: 12px;
    padding: 10px 14px;
    border-radius: 6px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    color: var(--text-primary);
    font-size: 13px;
}

.login-logo {
    width: 120px;
    height: 120px;
    border-radius: 50%;
    display: block;
    margin: 0 auto 24px;
}
</style>

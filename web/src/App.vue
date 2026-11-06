<script setup>
import { reactive, ref, onMounted, computed } from 'vue'
import { get, post, connectWs } from './ws.js'
import Overview from './views/Overview.vue'
import Modules from './views/Modules.vue'
import Rules from './views/Rules.vue'
import Gifts from './views/Gifts.vue'
import Scripts from './views/Scripts.vue'
import Products from './views/Products.vue'
import Fans from './views/Fans.vue'
import Debug from './views/Debug.vue'
import Settings from './views/Settings.vue'

const tab = ref('overview')
const connected = ref(false)
const data = reactive({
  config: {}, rules: [], modules: [], categories: [], stats: {}, fans: [], sfx: [], voices: [], gifts: [], info: {},
  scripts: [], scriptCategories: [], products: [], modes: [], mode: 'fun'
})
const events = ref([])
const traces = ref([])
const llmLines = ref([])

const navs = computed(() => [
  { id: 'overview', name: '总览' },
  { id: 'modules', name: '功能中心' },
  { id: 'rules', name: '规则' },
  { id: 'gifts', name: '礼物库' },
  { id: 'scripts', name: '话术库' },
  { id: 'products', name: '产品库' },
  { id: 'fans', name: '观众画像' },
  { id: 'debug', name: '调试台' },
  { id: 'settings', name: '设置' }
])

const audioCache = new Map()

async function pickMode(m) {
  if (data.mode === m.id) return
  const res = await post('/api/mode', { mode: m.id })
  if (res.ok) await load()
}

function playSound(payload) {
  const url = payload.file
  if (!url) return
  try {
    let audio = audioCache.get(url)
    if (!audio) {
      audio = new Audio(url)
      audioCache.set(url, audio)
    }
    audio.volume = Math.max(0, Math.min(1, payload.volume ?? 1))
    audio.currentTime = 0
    audio.play().catch(() => {})
  } catch {
    /* ignore */
  }
}

function speak(text) {
  if (!('speechSynthesis' in window)) return
  const u = new SpeechSynthesisUtterance(text)
  u.lang = 'zh-CN'
  speechSynthesis.speak(u)
}

async function load() {
  const d = await get('/api/bootstrap')
  Object.assign(data, d)
  events.value = d.events || []
  if (d.traces && d.traces.length) traces.value = d.traces.map((t) => ({ key: t.ts + Math.random(), ...t }))
  if (d.info) data.info = d.info
}

function pushEvent(ev) {
  events.value = [ev, ...events.value].slice(0, 120)
}

function pushTrace(t) {
  traces.value = [{ key: Date.now() + Math.random(), ...t }, ...traces.value].slice(0, 120)
}

onMounted(() => {
  load()
  connectWs('console', (msg) => {
    switch (msg.kind) {
      case '__open':
        connected.value = true
        break
      case '__close':
        connected.value = false
        break
      case 'event':
        pushEvent(msg.data)
        break
      case 'stats':
        data.stats = msg.data
        break
      case 'trace':
        pushTrace(msg.data)
        break
      case 'llm':
        llmLines.value = [{ text: msg.data.text, ok: msg.data.ok !== false }, ...llmLines.value].slice(0, 30)
        break
      case 'sound':
        playSound(msg.data)
        break
      case 'tts':
        speak(msg.data.text)
        break
    }
  })
  // 控制台自身也读音效：需要一次用户手势解锁
  window.addEventListener('click', () => {
    try {
      const a = new Audio()
      a.volume = 0
      a.play().catch(() => {})
    } catch {
      /* ignore */
    }
  }, { once: true })
})
</script>

<template>
  <div class="shell">
    <aside class="side">
      <div class="brand">DYweb<span> · 场控</span></div>
      <div v-for="n in navs" :key="n.id" class="nav" :class="{ on: tab === n.id }" @click="tab = n.id">
        {{ n.name }}
      </div>
      <div class="side-bottom">
        <div>端口 {{ data.config.port }}</div>
        <div>客户端 {{ data.info.clients ?? 0 }}</div>
        <div class="small">v{{ data.info.version }} · 本地运行</div>
      </div>
    </aside>

    <main class="main">
      <div class="topbar">
        <span class="dot" :class="{ on: connected }"></span>
        <span class="muted small">{{ connected ? '本机已连接' : '连接中断，正在重试' }}</span>
        <span class="muted small">事件/分 {{ Math.round(data.stats.perMinute || 0) }}</span>
        <span class="spacer"></span>
        <span class="small muted">直播定位</span>
        <span class="modes" style="display: flex; gap: 6px">
          <button
            v-for="m in data.modes"
            :key="m.id"
            class="btn sm"
            :class="data.mode === m.id ? 'primary' : 'ghost'"
            :title="m.name + ' — ' + m.desc"
            @click="pickMode(m)"
          >
            {{ m.short || m.name }}
          </button>
        </span>
        <a class="btn ghost sm" href="/panel" target="_blank">手机面板</a>
        <a class="btn ghost sm" href="/overlay" target="_blank">打开舞台层</a>
        <button class="btn sm" @click="load">刷新数据</button>
      </div>

      <div class="content">
        <Overview v-if="tab === 'overview'" :data="data" :events="events" :traces="traces" />
        <Modules v-else-if="tab === 'modules'" :data="data" @reload="load" />
        <Rules v-else-if="tab === 'rules'" :data="data" @reload="load" />
        <Gifts v-else-if="tab === 'gifts'" :data="data" @reload="load" />
        <Scripts v-else-if="tab === 'scripts'" :data="data" @reload="load" />
        <Products v-else-if="tab === 'products'" :data="data" @reload="load" />
        <Fans v-else-if="tab === 'fans'" :data="data" @reload="load" />
        <Debug v-else-if="tab === 'debug'" :data="data" :traces="traces" :llm="llmLines" />
        <Settings v-else-if="tab === 'settings'" :data="data" @reload="load" />
      </div>
    </main>
  </div>
</template>

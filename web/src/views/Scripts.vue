<script setup>
import { ref, computed, watch } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])

const local = computed(() => props.data.scripts || [])
const cats = computed(() => props.data.scriptCategories || [])

// 场景：娱乐的话术不该出现在卖货场，反之亦然；"" = 通用（哪个场景都能用）
const scenes = [
  { id: 'all', name: '全部' },
  { id: '', name: '通用' },
  { id: 'fun', name: '娱乐' },
  { id: 'ecom', name: '卖货' },
  { id: 'auto', name: '无人' }
]
const activeScene = ref('all')
const activeCat = ref('all')
const q = ref('')
const sayMsg = ref('')
const askQuery = ref('')
const askResult = ref('')
const askRefs = ref([])
const busy = ref(false)

const sceneName = (id) => scenes.find((s) => s.id === id)?.name || '通用'
const sceneOf = (s) => s.scene || ''
// 当前定位决定默认场景：娱乐只看「娱乐 + 通用」，卖货只看「卖货 + 通用」
function sceneForMode(m) {
  return m === 'fun' || m === 'ecom' || m === 'auto' ? m : 'all'
}
const sceneCount = (id) =>
  local.value.filter((s) => (id === 'all' ? true : id === '' ? !s.scene : !s.scene || s.scene === id)).length

// 切到某个直播定位时，默认停在它最常用的那类话术 + 对应场景
watch(
  () => props.data.mode,
  (m) => {
    const c = (props.data.modes || []).find((x) => x.id === m)
    if (c?.script) activeCat.value = c.script
    activeScene.value = sceneForMode(m)
  },
  { immediate: true }
)

const shown = computed(() => {
  let list = local.value
  if (activeScene.value !== 'all') {
    list = list.filter((s) => (activeScene.value === '' ? !s.scene : !s.scene || s.scene === activeScene.value))
  }
  if (activeCat.value !== 'all') list = list.filter((s) => s.category === activeCat.value)
  if (q.value.trim()) {
    const k = q.value.trim()
    list = list.filter((s) => (s.title + s.text).includes(k))
  }
  return list
})

const catName = (id) => cats.value.find((c) => c.id === id)?.name || id

async function say(s) {
  const res = await post('/api/scripts/say', { text: s.text, user: '新来的朋友' })
  sayMsg.value = res.ok ? '念了：' + res.text : '失败：' + (res.error || '')
  setTimeout(() => (sayMsg.value = ''), 4000)
}

async function ask() {
  busy.value = true
  askResult.value = 'AI 正在翻话术库…'
  const res = await post('/api/scripts/ask', {
    query: askQuery.value || '现在有点冷场，帮我说一句留人的话',
    scene: activeScene.value === 'all' ? '' : activeScene.value
  })
  busy.value = false
  if (res.ok) {
    askResult.value = res.text
    askRefs.value = res.refs || []
  } else {
    askResult.value = '没成功：' + (res.error || '')
    askRefs.value = []
  }
}

async function sayAsk() {
  if (!askResult.value) return
  await post('/api/scripts/say', { text: askResult.value, user: '新来的朋友' })
}

// 自定义话术
const showAdd = ref(false)
const form = ref({ title: '', category: 'keep', scene: '', text: '' })
function add() {
  if (!form.value.text.trim()) return
  const list = local.value.filter((s) => !s.builtin).concat([{ ...form.value, builtin: false, id: '' }])
  save(list)
  form.value = { title: '', category: 'keep', scene: '', text: '' }
  showAdd.value = false
}
async function save(list) {
  const custom = list.filter((s) => !s.builtin)
  const res = await post('/api/scripts', { scripts: custom })
  sayMsg.value = res.ok ? '已保存' : '保存失败：' + (res.error || '')
  if (res.ok) emit('reload')
}
function removeCustom(s) {
  const custom = local.value.filter((x) => !x.builtin && x.id !== s.id)
  save(custom)
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">先选场景，再选分类：娱乐场不会给你混卖货话术。点「念一遍」直接从直播间音箱念出来，不用自己背。</div>
    <span style="flex: 1"></span>
    <span v-if="sayMsg" class="tag warn">{{ sayMsg }}</span>
    <button class="btn" @click="showAdd = !showAdd">加一条我的话术</button>
  </div>

  <div class="card" style="margin-bottom: 14px">
    <div class="row" style="gap: 8px; flex-wrap: wrap; margin-bottom: 8px">
      <span class="small muted" style="min-width: 44px">场景</span>
      <button
        v-for="s in scenes"
        :key="'sc' + s.id"
        class="btn sm"
        :class="activeScene === s.id ? 'primary' : 'ghost'"
        @click="activeScene = s.id"
      >
        {{ s.name }} {{ sceneCount(s.id) }}
      </button>
    </div>
    <div class="row" style="gap: 8px; flex-wrap: wrap; margin-bottom: 10px">
      <span class="small muted" style="min-width: 44px">分类</span>
      <button class="btn sm" :class="activeCat === 'all' ? 'primary' : 'ghost'" @click="activeCat = 'all'">全部</button>
      <button
        v-for="c in cats"
        :key="c.id"
        class="btn sm"
        :class="activeCat === c.id ? 'primary' : 'ghost'"
        @click="activeCat = c.id"
      >
        {{ c.name }}
      </button>
      <span style="flex: 1"></span>
      <input v-model="q" placeholder="搜关键词，比如 算账 / 冷场" style="max-width: 220px" />
    </div>

    <div class="small muted" style="margin-bottom: 10px">
      场景：{{ activeScene === 'all' ? '全部' : activeScene === '' ? '通用（哪个场景都能用）' : sceneName(activeScene) + '（含通用）' }} ·
      {{ activeCat === 'all' ? '全部分类' : catName(activeCat) }}：{{ cats.find((c) => c.id === activeCat)?.desc || '主播随手加的话术' }}
    </div>

    <div class="col" style="gap: 10px">
      <div v-for="s in shown" :key="s.id" style="border: 1px solid var(--line); border-radius: 10px; padding: 12px">
        <div class="row" style="margin-bottom: 6px">
          <span class="tag" :class="s.builtin ? '' : 'warn'">{{ catName(s.category) }}</span>
          <span class="tag" :class="s.scene ? 'ok' : ''">{{ sceneName(s.scene) }}</span>
          <b class="small">{{ s.title || '我加的话术' }}</b>
          <span v-if="!s.builtin" class="small muted">自己加的</span>
          <span style="flex: 1"></span>
          <button class="btn sm" @click="say(s)">念一遍</button>
          <button v-if="!s.builtin" class="btn sm ghost" @click="removeCustom(s)">删除</button>
        </div>
        <div style="line-height: 1.7">{{ s.text }}</div>
      </div>
      <div v-if="!shown.length" class="muted small">这个场景 + 分类下还没有话术，换个筛选或自己加一条。</div>
    </div>
  </div>

  <div v-if="showAdd" class="card" style="margin-bottom: 14px">
    <h3>加一条我自己的话术</h3>
    <div class="kv">
      <label>标题</label>
      <input v-model="form.title" placeholder="比如：我家专属开场白" />
      <label>场景</label>
      <select v-model="form.scene">
        <option value="">通用（哪个场景都能用）</option>
        <option value="fun">娱乐</option>
        <option value="ecom">卖货</option>
        <option value="auto">无人</option>
      </select>
      <label>分类</label>
      <select v-model="form.category">
        <option v-for="c in cats" :key="c.id" :value="c.id">{{ c.name }}</option>
      </select>
    </div>
    <textarea v-model="form.text" rows="4" placeholder="写成你平时说话的样子，越像你自己越好。可用 {user}" style="margin-top: 10px"></textarea>
    <div class="row" style="margin-top: 10px; justify-content: flex-end">
      <button class="btn ghost" @click="showAdd = false">取消</button>
      <button class="btn primary" @click="add">保存这条</button>
    </div>
  </div>

  <div class="card">
    <h3>让 AI 翻着库给你来一句</h3>
    <div class="hint" style="margin-bottom: 10px">
      AI 会先从话术库里找最贴的几条，再按你描述的场景改写。需要先在设置里配好 AI。
    </div>
    <div class="row">
      <input v-model="askQuery" placeholder="比如：刚上人但没人说话，帮我热个场" style="flex: 1" />
      <button class="btn primary" :disabled="busy" @click="ask">AI 帮我想一句</button>
    </div>
    <div v-if="askResult" style="margin-top: 12px">
      <div style="line-height: 1.8">{{ askResult }}</div>
      <div v-if="askRefs.length" class="small muted" style="margin-top: 6px">
        参考了：{{ askRefs.map((r) => r.title).join('、') }}
      </div>
      <div class="row" style="margin-top: 10px">
        <button class="btn sm" @click="sayAsk">念这句</button>
      </div>
    </div>
  </div>
</template>

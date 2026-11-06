<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { get, post, connectWs } from '../ws.js'

const game = ref(null)
const theme = ref('')
const busy = ref('')
const connected = ref(false)
const toast = ref('')
let ws = null

const hosts = computed(() => location.host)

const themes = ['尺度大一点', '职场吐槽', '土味情话', '狠一点', '宠粉']

// 转盘词条：主播可以自己改，一行一个
const words = ref('')
const speed = ref(4200)
const speeds = [
  { label: '慢', ms: 6200 },
  { label: '标准', ms: 4200 },
  { label: '快', ms: 2600 }
]

function loadWheel() {
  try {
    words.value = localStorage.getItem('dyw_wheel_words') || ''
    const s = Number(localStorage.getItem('dyw_wheel_speed'))
    if (s) speed.value = s
  } catch {
    /* ignore */
  }
}
function saveWheel() {
  try {
    localStorage.setItem('dyw_wheel_words', words.value)
    localStorage.setItem('dyw_wheel_speed', String(speed.value))
  } catch {
    /* ignore */
  }
}
loadWheel()

const wordList = computed(() =>
  words.value
    .split(/[\n,，、]+/)
    .map((s) => s.trim())
    .filter(Boolean)
    .slice(0, 12)
)

async function aiWords() {
  if (busy.value) return
  busy.value = 'AI 想词'
  toast.value = 'AI 正在想词…'
  try {
    const r = await post('/api/game/words', { theme: theme.value })
    if (r.items && r.items.length) {
      words.value = r.items.join('\n')
      saveWheel()
      toast.value = '词想好了，可以改，再点开始'
    } else {
      toast.value = 'AI 没想出来，直接点开始也能用'
    }
  } catch (e) {
    toast.value = '连不上服务：' + e.message
  } finally {
    busy.value = ''
    setTimeout(() => (toast.value = ''), 2500)
  }
}

function clearWords() {
  words.value = ''
  saveWheel()
}

onMounted(async () => {
  try {
    const d = await get('/api/game')
    if (d && d.game && d.game.type) game.value = d.game
  } catch {
    /* ignore */
  }
  ws = connectWs('console', (msg) => {
    if (msg.kind === '__open') connected.value = true
    else if (msg.kind === '__close') connected.value = false
    else if (msg.kind === 'game') {
      const d = msg.data
      game.value = !d || d.type === 'clear' || d.type === '' ? null : d
    }
  })
})
onUnmounted(() => ws && ws.close())

async function trigger(label, path, body) {
  if (busy.value) return
  busy.value = label
  toast.value = label + '…'
  try {
    const r = await post(path, body || {})
    if (r.ok === false) {
      toast.value = r.error || r.msg || '失败了'
      return
    }
    if (r.game) game.value = r.game
    toast.value = label + ' 已推到大屏'
  } catch (e) {
    toast.value = '连不上服务：' + e.message
  } finally {
    busy.value = ''
    setTimeout(() => (toast.value = ''), 2500)
  }
}

const spin = () => {
  saveWheel()
  return trigger('大转盘', '/api/game/wheel', {
    theme: theme.value,
    items: wordList.value,
    speed: speed.value
  })
}
const guess = () => {
  saveGuess()
  return trigger('猜数字', '/api/game/guess', { lo: Number(gLo.value) || 1, hi: Number(gHi.value) || 1000 })
}
const traitor = () => trigger('抓叛徒', '/api/game/traitor')
const clear = () => trigger('清屏', '/api/game/clear')

// 猜数字范围，主播自己定
const gLo = ref(1)
const gHi = ref(1000)
const guessPresets = [
  { label: '1~100', lo: 1, hi: 100 },
  { label: '1~500', lo: 1, hi: 500 },
  { label: '1~1000', lo: 1, hi: 1000 }
]
function saveGuess() {
  try {
    localStorage.setItem('dyw_guess_lo', String(gLo.value))
    localStorage.setItem('dyw_guess_hi', String(gHi.value))
  } catch {
    /* ignore */
  }
}
function loadGuess() {
  try {
    const lo = Number(localStorage.getItem('dyw_guess_lo'))
    const hi = Number(localStorage.getItem('dyw_guess_hi'))
    if (lo) gLo.value = lo
    if (hi) gHi.value = hi
  } catch {
    /* ignore */
  }
}
loadGuess()

// 名字抽奖：默认从观众名单里抽，也能自己贴名单
const nameCount = ref(1)
const namesText = ref('')
const audienceCount = ref(0)
const audiences = [
  { n: 1, label: '抽 1 个' },
  { n: 3, label: '抽 3 个' },
  { n: 5, label: '抽 5 个' },
  { n: 10, label: '抽 10 个' }
]

async function loadAudience() {
  try {
    const d = await get('/api/game/audience')
    audienceCount.value = d.count || 0
  } catch {
    /* ignore */
  }
}
loadAudience()

const manualNames = computed(() =>
  namesText.value
    .split(/[\n,，、]+/)
    .map((s) => s.trim())
    .filter(Boolean)
)
const drawNames = () =>
  trigger('名字抽奖', '/api/game/names', {
    count: nameCount.value,
    names: manualNames.value
  })

const statusText = computed(() => {
  const g = game.value
  if (!g) return '现在大屏上没有游戏'
  if (g.type === 'wheel') return '转盘：' + g.title
  if (g.type === 'guess') {
    if (g.won) return '已开奖：' + g.user
    const fb = g.guess ? ` ｜ ${g.user} 猜 ${g.guess}：${g.dir === 'high' ? '猜大了' : '猜小了'}` : ''
    return `猜数字 ${g.lo}~${g.hi}${fb}`
  }
  if (g.type === 'traitor') return '通缉：' + (g.user || '没人可抓')
  if (g.type === 'names') return g.wins && g.wins.length ? '中奖：' + g.wins.join('、') : '名字抽奖：' + (g.tip || '')
  return ''
})
</script>

<template>
  <div class="wrap">
    <header>
      <h1>主播中控台</h1>
      <span class="dot" :class="connected ? 'on' : 'off'"></span>
    </header>

    <p class="hint">点一下，观众大屏立刻出效果。手机 / iPad 打开 <b>{{ hosts }}/panel</b> 就能用。</p>

    <div class="status" :class="{ live: !!game }">
      {{ statusText }}
      <span v-if="game && game.tip" class="tip">{{ game.tip }}</span>
    </div>

    <div class="grid">
      <button class="big amber" :disabled="!!busy" @click="spin">
        <span class="ico">🎡</span>
        <span class="lab">一键大转盘</span>
        <span class="sub">{{ wordList.length ? `用你填的 ${wordList.length} 个词` : '没填词就让 AI 现编 6 个' }}</span>
      </button>
      <button class="big blue" :disabled="!!busy" @click="guess">
        <span class="ico">🔢</span>
        <span class="lab">一键猜数字</span>
        <span class="sub">{{ gLo }}~{{ gHi }}，猜大了小了会回显</span>
      </button>
      <button class="big violet" :disabled="!!busy" @click="drawNames">
        <span class="ico">🎟️</span>
        <span class="lab">名字抽奖</span>
        <span class="sub">{{ manualNames.length ? `用你贴的 ${manualNames.length} 人` : `从观众名单抽，现 ${audienceCount} 人` }}</span>
      </button>
      <button class="big red" :disabled="!!busy" @click="traitor">
        <span class="ico">🚨</span>
        <span class="lab">AI 抓叛徒</span>
        <span class="sub">抓最近 10 秒最带节奏的</span>
      </button>
      <button class="big ghost" :disabled="!!busy" @click="clear">
        <span class="ico">🧹</span>
        <span class="lab">清屏</span>
        <span class="sub">把游戏从大屏收掉</span>
      </button>
    </div>

    <div class="theme">
      <label>转盘词条（一行一个，2~12 个；不填就 AI 现编）</label>
      <textarea
        v-model="words"
        rows="4"
        placeholder="比如：&#10;做10个深蹲&#10;学猫叫三声&#10;用方言夸主播"
        @change="saveWheel"
      ></textarea>
      <div class="row">
        <button class="mini" :disabled="!!busy" @click="aiWords">AI 帮我想词</button>
        <button class="mini ghost" @click="clearWords">清空</button>
        <span class="count">{{ wordList.length }}/12</span>
      </div>

      <label class="mt">转盘速度</label>
      <div class="chips">
        <button
          v-for="s in speeds"
          :key="s.label"
          class="chip"
          :class="{ on: speed === s.ms }"
          @click="speed = s.ms; saveWheel()"
        >{{ s.label }}</button>
      </div>

      <label class="mt">AI 想词的主题（选了 AI 帮我想词才用）</label>
      <input v-model="theme" placeholder="比如：职场吐槽 / 尺度大一点" />
      <div class="chips">
        <button v-for="t in themes" :key="t" class="chip" @click="theme = t">{{ t }}</button>
      </div>
    </div>

    <div class="theme">
      <label>猜数字 · 范围（主播自己定，越小越快猜中）</label>
      <div class="row two">
        <input v-model.number="gLo" type="number" min="0" placeholder="最小" @change="saveGuess" />
        <span class="dash">~</span>
        <input v-model.number="gHi" type="number" min="1" placeholder="最大" @change="saveGuess" />
      </div>
      <div class="chips">
        <button
          v-for="p in guessPresets"
          :key="p.label"
          class="chip"
          :class="{ on: gLo === p.lo && gHi === p.hi }"
          @click="gLo = p.lo; gHi = p.hi; saveGuess()"
        >{{ p.label }}</button>
      </div>
    </div>

    <div class="theme">
      <label>名字抽奖 · 抽几个（从观众名单里抽，不重复）</label>
      <div class="chips">
        <button
          v-for="a in audiences"
          :key="a.n"
          class="chip"
          :class="{ on: nameCount === a.n }"
          @click="nameCount = a.n"
        >{{ a.label }}</button>
      </div>
      <div class="row">
        <button class="mini ghost" @click="loadAudience">刷新名单人数</button>
        <span class="count">观众名单 {{ audienceCount }} 人</span>
      </div>
      <label class="mt">不想用观众名单？贴自己的（一行一个，留空就用观众名单）</label>
      <textarea v-model="namesText" rows="3" placeholder="比如：&#10;张三&#10;李四"></textarea>
    </div>

    <footer>
      <a href="/" target="_blank">打开控制台</a>
      <a href="/overlay?debug=1" target="_blank">测试大屏</a>
      <span v-if="toast" class="toast">{{ toast }}</span>
    </footer>
  </div>
</template>

<style scoped>
.wrap {
  min-height: 100%;
  box-sizing: border-box;
  padding: 18px 16px 28px;
  color: #e9ecf4;
  font-family: "Microsoft YaHei", system-ui, sans-serif;
  background: radial-gradient(circle at 50% -10%, #1a2133 0%, #0e1016 60%);
}
header { display: flex; align-items: center; gap: 10px; }
h1 { font-size: 22px; margin: 0; letter-spacing: 1px; }
.dot { width: 10px; height: 10px; border-radius: 50%; }
.dot.on { background: #37d67a; box-shadow: 0 0 10px #37d67a; }
.dot.off { background: #ff5d5d; }
.hint { color: #8b93a7; font-size: 13px; margin: 10px 0 14px; line-height: 1.5; }
.hint b { color: #ffd24a; }

.status {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid #262c3a;
  border-radius: 12px;
  padding: 12px 14px;
  font-size: 15px;
  color: #aab3c7;
  margin-bottom: 14px;
}
.status.live { border-color: #ffd24a; color: #ffe9b3; }
.status .tip { display: block; margin-top: 4px; font-size: 12px; color: #8b93a7; }

.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.big {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 16px 18px;
  border-radius: 16px;
  border: 1px solid transparent;
  cursor: pointer;
  color: #16181f;
  min-height: 96px;
  transition: transform 0.12s ease;
}
.big:active { transform: scale(0.97); }
.big:disabled { opacity: 0.55; }
.ico { font-size: 22px; }
.lab { font-size: 20px; font-weight: 800; }
.sub { font-size: 12px; opacity: 0.75; text-align: left; }
.amber { background: linear-gradient(160deg, #ffd24a, #f0a93b); }
.blue { background: linear-gradient(160deg, #7ed0ff, #4a9bf0); }
.red { background: linear-gradient(160deg, #ff7a86, #e8394d); color: #fff; }
.violet { background: linear-gradient(160deg, #b18cff, #6a4bff); color: #fff; }
.ghost { background: rgba(255, 255, 255, 0.06); border-color: #2b3242; color: #dfe4ee; }

.theme { margin-top: 18px; }
.theme label { display: block; font-size: 13px; color: #8b93a7; margin-bottom: 8px; }
.theme input,
.theme textarea {
  width: 100%;
  box-sizing: border-box;
  background: #12151d;
  border: 1px solid #262c3a;
  color: #e9ecf4;
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 15px;
  outline: none;
  font-family: inherit;
  resize: vertical;
}
.theme label.mt { margin-top: 16px; }
.row { display: flex; align-items: center; gap: 10px; margin-top: 10px; }
.row.two input { flex: 1; }
.row.two .dash { color: #8b93a7; }
.mini {
  background: rgba(255, 210, 74, 0.14);
  border: 1px solid #6b5717;
  color: #ffd24a;
  border-radius: 999px;
  padding: 8px 16px;
  font-size: 14px;
  cursor: pointer;
}
.mini.ghost {
  background: rgba(255, 255, 255, 0.06);
  border-color: #2b3242;
  color: #cbd2e0;
}
.count { margin-left: auto; font-size: 13px; color: #8b93a7; }
.chips { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.chip {
  background: rgba(255, 255, 255, 0.06);
  color: #cbd2e0;
  border: 1px solid #2b3242;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 13px;
  cursor: pointer;
}
.chip.on {
  background: #ffd24a;
  border-color: #ffd24a;
  color: #2a1d00;
  font-weight: 700;
}

footer {
  margin-top: 22px;
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 13px;
}
footer a { color: #7ed0ff; text-decoration: none; }
.toast { margin-left: auto; color: #ffd24a; }

@media (max-width: 380px) {
  .grid { grid-template-columns: 1fr; }
}
</style>

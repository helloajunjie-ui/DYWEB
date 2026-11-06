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

const spin = () => trigger('大转盘', '/api/game/wheel', { theme: theme.value })
const guess = () => trigger('猜数字', '/api/game/guess')
const traitor = () => trigger('抓叛徒', '/api/game/traitor')
const clear = () => trigger('清屏', '/api/game/clear')

const statusText = computed(() => {
  const g = game.value
  if (!g) return '现在大屏上没有游戏'
  if (g.type === 'wheel') return '转盘：' + g.title
  if (g.type === 'guess') return g.won ? '已开奖：' + g.user : `猜数字进行中 ${g.lo}~${g.hi}`
  if (g.type === 'traitor') return '通缉：' + (g.user || '没人可抓')
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
        <span class="sub">AI 现编 6 个惩罚词</span>
      </button>
      <button class="big blue" :disabled="!!busy" @click="guess">
        <span class="ico">🔢</span>
        <span class="lab">一键猜数字</span>
        <span class="sub">1~1000，弹幕猜，AI 毒舌</span>
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
      <label>转盘主题（可选）</label>
      <input v-model="theme" placeholder="比如：职场吐槽 / 尺度大一点" />
      <div class="chips">
        <button v-for="t in themes" :key="t" class="chip" @click="theme = t">{{ t }}</button>
      </div>
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
.ghost { background: rgba(255, 255, 255, 0.06); border-color: #2b3242; color: #dfe4ee; }

.theme { margin-top: 18px; }
.theme label { display: block; font-size: 13px; color: #8b93a7; margin-bottom: 8px; }
.theme input {
  width: 100%;
  box-sizing: border-box;
  background: #12151d;
  border: 1px solid #262c3a;
  color: #e9ecf4;
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 15px;
  outline: none;
}
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

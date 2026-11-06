<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { connectWs, get, post } from '../ws.js'
import { LuckyWheel } from '@lucky-canvas/vue'

const canvasRef = ref(null)
const stageRef = ref(null)
const wraps = ref([])
const audioReady = ref(false)
const connected = ref(false)
const fps = ref(60)
const debug = ref(false)

let ctx = null
let W = 0
let H = 0
let particles = []
let raf = 0
let lastFrame = 0
let slowFrames = 0
let lastMsg = Date.now()
let bannerSeq = 0
const audioMap = new Map()
let stopWs = null

const shakeUntil = ref(0)
const spotUntil = ref(0)
const pcount = ref(0)
const maxParticles = ref(800)

// 中控面板触发的游戏状态
const game = ref(null)
const guessSeq = ref(0)
const luckyRef = ref(null)

const wheelPalette = [
  ['#ff4d6d', '#ffffff'],
  ['#ffd166', '#5a3200'],
  ['#06d6a0', '#043f2f'],
  ['#4cc9f0', '#04283a'],
  ['#b5179e', '#ffffff'],
  ['#f77f00', '#3a1c00'],
  ['#8338ec', '#ffffff'],
  ['#2a9d8f', '#ffffff']
]

const prizes = ref([])
const blocks = [
  { padding: '8px', background: 'linear-gradient(0deg, #7a0d12, #d81e26)', borderRadius: '50%' },
  { padding: '8px', background: '#ffe08a', borderRadius: '50%' },
  { padding: '8px', background: '#7a0d12', borderRadius: '50%' }
]
const buttons = ref([])
const luckyConfig = reactive({ speed: 18, accelerationTime: 1600, decelerationTime: 4200, offsetDegree: 0 })
const luckyStyle = { fontSize: '21px', fontColor: '#fff', fontStyle: '"Microsoft YaHei", sans-serif', fontWeight: '900', wordWrap: true, lengthLimit: '100%' }

function setWheelItems(items) {
  prizes.value = items.map((text, i) => {
    const [bg, fg] = wheelPalette[i % wheelPalette.length]
    return {
      name: text,
      background: bg,
      fonts: [{ text, fontColor: fg, fontSize: items.length > 6 ? '20px' : '24px', fontWeight: '900' }]
    }
  })
  buttons.value = [
    {
      radius: '17%',
      background: '#ffcf5c',
      pointer: true,
      fonts: [{ text: '开转', fontColor: '#5a2b00', fontSize: '20px', fontWeight: '900' }]
    }
  ]
}

function onWheelStart() {
  const r = luckyRef.value
  if (!r) return
  r.play()
  const idx = Math.floor(Math.random() * prizes.value.length)
  // 先加速转一会儿再定落点，看着像真转盘
  setTimeout(() => r.stop(idx), 1400)
}

function onWheelEnd(prize) {
  const text = prize && prize.name ? prize.name : ''
  if (text) post('/api/game/result', { text }).catch(() => {})
}

function onGame(d) {
  if (!d || d.type === 'clear' || d.type === '') {
    game.value = null
    return
  }
  if (d.type === 'wheel' && (!d.items || !d.items.length)) return
  game.value = d
  if (d.type === 'wheel') {
    setWheelItems(d.items)
    // 主播在面板上选的速度（慢/标准/快）决定转多久
    const decel = Number(d.spinMs) || 4200
    luckyConfig.decelerationTime = decel
    luckyConfig.accelerationTime = Math.round(Math.min(1600, decel * 0.35))
  }
  if (d.type === 'guess') {
    guessSeq.value++
  }
}

const params = new URLSearchParams(location.search)
debug.value = params.get('debug') === '1'

onMounted(() => {
  setupCanvas()
  const ws = connectWs('overlay', handleMsg)
  stopWs = () => ws.close()
  // 刷新后把正在玩的游戏接回来，不用主播重开
  get('/api/game').then((d) => {
    if (d && d.game && d.game.type) onGame(d.game)
  }).catch(() => {})
  window.addEventListener('resize', setupCanvas)

  // 看门狗：30 秒没有任何消息就自愈刷新（后端每秒会推 stats，正常不会触发）
  setInterval(() => {
    if (Date.now() - lastMsg > 30000) location.reload()
  }, 5000)
  // 每 6 小时软刷新一次，防止长时间运行累积问题
  setInterval(() => location.reload(), 6 * 60 * 60 * 1000)
})

onUnmounted(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', setupCanvas)
  stopWs && stopWs()
})

function setupCanvas() {
  const c = canvasRef.value
  if (!c) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  W = window.innerWidth
  H = window.innerHeight
  c.width = Math.floor(W * dpr)
  c.height = Math.floor(H * dpr)
  c.style.width = W + 'px'
  c.style.height = H + 'px'
  ctx = c.getContext('2d')
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  if (!raf) loop(0)
}

/* ---------------- 消息分发 ---------------- */

function handleMsg(msg) {
  lastMsg = Date.now()
  if (msg.kind === '__open') {
    connected.value = true
    return
  }
  if (msg.kind === '__close') {
    connected.value = false
    return
  }
  if (msg.kind === 'yl') return

  switch (msg.kind) {
    case 'effect':
      playEffect(msg.data)
      break
    case 'sound':
      playSound(msg.data)
      break
    case 'tts':
      speak(msg.data.text)
      break
    case 'stats':
      break
    case 'game':
      onGame(msg.data)
      break
    default:
      break
  }
}

function playEffect(cmd) {
  const preset = cmd.preset || 'hearts'
  const strength = Math.max(1, Math.min(4, cmd.strength || 1))
  switch (preset) {
    case 'fireworks':
      burstShells(2 + strength * 2, 60 + strength * 40)
      break
    case 'hearts':
      heartRain(18 * strength + 20, cmd.count || 1)
      break
    case 'meteor':
      meteors(3 * strength)
      break
    case 'confetti':
      confettiRain(30 * strength)
      break
    case 'shake':
      shakeUntil.value = Date.now() + (cmd.durationMs || 400)
      break
    case 'spotlight':
      spotUntil.value = Date.now() + (cmd.durationMs || 2000)
      break
    case 'banner':
      pushWrap(cmd)
      break
    default:
      heartRain(20, 1)
  }
}

/* ---------------- 粒子 ---------------- */

function budget() {
  return Math.min(maxParticles.value, 900)
}

function add(p) {
  if (particles.length >= budget()) {
    particles.splice(0, Math.ceil(particles.length / 8))
  }
  particles.push(p)
}

function burstShells(shells, sparksPer) {
  for (let s = 0; s < shells; s++) {
    const x = W * (0.15 + Math.random() * 0.7)
    const y = H * (0.12 + Math.random() * 0.45)
    const hue = Math.floor(Math.random() * 360)
    const n = Math.min(sparksPer, 90)
    for (let i = 0; i < n; i++) {
      const ang = (Math.PI * 2 * i) / n + Math.random() * 0.1
      const sp = 2.5 + Math.random() * 4
      add({
        kind: 'spark',
        x,
        y,
        vx: Math.cos(ang) * sp,
        vy: Math.sin(ang) * sp,
        life: 1,
        decay: 0.012 + Math.random() * 0.012,
        size: 1.6 + Math.random() * 1.6,
        color: `hsl(${hue + Math.random() * 40}, 95%, 65%)`
      })
    }
  }
}

function heartRain(count, giftCount) {
  const n = Math.min(count, 400)
  for (let i = 0; i < n; i++) {
    add({
      kind: 'heart',
      x: Math.random() * W,
      y: -40 - Math.random() * H * 0.6,
      vy: 1.6 + Math.random() * 2.2,
      sway: Math.random() * Math.PI * 2,
      swaySpd: 0.02 + Math.random() * 0.03,
      amp: 18 + Math.random() * 26,
      size: 14 + Math.random() * 16 + giftCount * 0.6,
      rot: (Math.random() - 0.5) * 0.5,
      life: 1
    })
  }
}

function meteors(count) {
  for (let i = 0; i < count; i++) {
    add({
      kind: 'streak',
      x: -100 + Math.random() * W * 0.6,
      y: -60 + Math.random() * H * 0.3,
      vx: 9 + Math.random() * 6,
      vy: 4 + Math.random() * 3,
      len: 60 + Math.random() * 80,
      life: 1,
      decay: 0.02,
      color: Math.random() > 0.5 ? '#ffe6a3' : '#9fe8ff'
    })
  }
}

function confettiRain(count) {
  const n = Math.min(count, 300)
  for (let i = 0; i < n; i++) {
    add({
      kind: 'confetti',
      x: Math.random() * W,
      y: -20 - Math.random() * 200,
      vx: (Math.random() - 0.5) * 2,
      vy: 2 + Math.random() * 3,
      rot: Math.random() * Math.PI,
      vrot: (Math.random() - 0.5) * 0.2,
      w: 6 + Math.random() * 8,
      h: 10 + Math.random() * 14,
      color: `hsl(${Math.floor(Math.random() * 360)}, 90%, 62%)`,
      life: 1,
      decay: 0
    })
  }
}

function loop(ts) {
  const dt = lastFrame ? Math.min(32, ts - lastFrame) : 16
  lastFrame = ts
  if (dt > 33) slowFrames++
  else slowFrames = Math.max(0, slowFrames - 1)
  if (slowFrames > 90) {
    // 连续掉帧：自动降粒子上限，宁少勿卡
    maxParticles.value = Math.max(150, Math.floor(maxParticles.value * 0.8))
    slowFrames = 0
  }
  if (ts % 30 < 2) fps.value = Math.round(1000 / Math.max(1, dt))
  pcount.value = particles.length

  if (!ctx) {
    raf = requestAnimationFrame(loop)
    return
  }
  ctx.clearRect(0, 0, W, H)

  for (let i = particles.length - 1; i >= 0; i--) {
    const p = particles[i]
    if (p.kind === 'heart') {
      p.y += p.vy
      p.sway += p.swaySpd
      p.x += Math.sin(p.sway) * p.amp * 0.05
      p.rot += Math.sin(p.sway * 0.5) * 0.02
      drawHeart(p.x, p.y, p.size, p.rot, 'rgba(255, 96, 140, .9)')
      if (p.y > H + 60) particles.splice(i, 1)
    } else if (p.kind === 'spark') {
      p.x += p.vx
      p.y += p.vy
      p.vx *= 0.985
      p.vy = p.vy * 0.985 + 0.02
      p.life -= p.decay
      if (p.life <= 0) {
        particles.splice(i, 1)
        continue
      }
      ctx.globalAlpha = Math.max(0, p.life)
      ctx.fillStyle = p.color
      ctx.beginPath()
      ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
      ctx.fill()
      ctx.globalAlpha = 1
    } else if (p.kind === 'streak') {
      p.x += p.vx
      p.y += p.vy
      p.life -= p.decay
      if (p.life <= 0 || p.x > W + 100) {
        particles.splice(i, 1)
        continue
      }
      ctx.strokeStyle = p.color
      ctx.globalAlpha = Math.max(0, p.life)
      ctx.lineWidth = 3
      ctx.beginPath()
      ctx.moveTo(p.x, p.y)
      ctx.lineTo(p.x - p.len, p.y - p.len * 0.5)
      ctx.stroke()
      ctx.globalAlpha = 1
    } else if (p.kind === 'confetti') {
      p.x += p.vx
      p.y += p.vy
      p.rot += p.vrot
      if (p.y > H + 30) {
        particles.splice(i, 1)
        continue
      }
      ctx.save()
      ctx.translate(p.x, p.y)
      ctx.rotate(p.rot)
      ctx.fillStyle = p.color
      ctx.fillRect(-p.w / 2, -p.h / 2, p.w, p.h)
      ctx.restore()
    }
  }

  raf = requestAnimationFrame(loop)
}

function drawHeart(x, y, size, rot, color) {
  ctx.save()
  ctx.translate(x, y)
  ctx.rotate(rot)
  ctx.scale(size / 16, size / 16)
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.moveTo(0, 5)
  ctx.bezierCurveTo(0, 2, -8, -2, -8, -8)
  ctx.bezierCurveTo(-8, -13, -3, -14, 0, -9)
  ctx.bezierCurveTo(3, -14, 8, -13, 8, -8)
  ctx.bezierCurveTo(8, -2, 0, 2, 0, 5)
  ctx.fill()
  ctx.restore()
}

/* ---------------- 横幅 ---------------- */

function pushWrap(cmd) {
  const item = {
    id: ++bannerSeq,
    text: cmd.text || '',
    user: cmd.user || '',
    color: cmd.color || '#FFD24A',
    duration: cmd.durationMs || 5000,
    show: true
  }
  wraps.value = [...wraps.value, item]
  setTimeout(() => {
    wraps.value = wraps.value.filter((w) => w.id !== item.id)
  }, item.duration)
}

/* ---------------- 声音 ---------------- */

function playSound(payload) {
  if (!audioReady.value || !payload.file) return
  let a = audioMap.get(payload.file)
  if (!a) {
    a = new Audio(payload.file)
    audioMap.set(payload.file, a)
  }
  a.volume = Math.max(0, Math.min(1, payload.volume ?? 1))
  a.currentTime = 0
  a.play().catch(() => {})
}

function speak(text) {
  if (!text || !('speechSynthesis' in window)) return
  const u = new SpeechSynthesisUtterance(text)
  u.lang = 'zh-CN'
  speechSynthesis.speak(u)
}

function enableAudio() {
  audioReady.value = true
  try {
    const a = new Audio('data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA=')
    a.play().catch(() => {})
  } catch {
    /* ignore */
  }
}
</script>

<template>
  <div class="stage">
    <div
      ref="stageRef"
      class="fx"
      :style="{ animation: shakeUntil > Date.now() ? 'shake .18s linear 4' : 'none' }"
    >
      <canvas ref="canvasRef" class="cv"></canvas>
    </div>

    <div v-if="spotUntil > Date.now()" class="spot"></div>

    <!-- 大转盘 -->
    <div v-if="game && game.type === 'wheel'" class="wheel-wrap" :key="'wheel-' + game.started">
      <div class="wheel-title">{{ game.title }}</div>
      <div class="wheel-box">
        <div class="wheel-glow"></div>
        <LuckyWheel
          ref="luckyRef"
          width="520px"
          height="520px"
          :prizes="prizes"
          :blocks="blocks"
          :buttons="buttons"
          :defaultConfig="luckyConfig"
          :defaultStyle="luckyStyle"
          @start="onWheelStart"
          @end="onWheelEnd"
        />
      </div>
    </div>

    <!-- 猜数字 -->
    <div v-if="game && game.type === 'guess'" class="guess-wrap" :key="guessSeq">
      <div class="guess-title">弹幕猜数字</div>
      <div v-if="game.won" class="guess-win">
        <div class="gw-name">{{ game.user }}</div>
        <div class="gw-text">猜中了！</div>
      </div>
      <div v-else class="guess-range" :key="game.lo + '-' + game.hi">
        <span>{{ game.lo }}</span>
        <i>~</i>
        <span>{{ game.hi }}</span>
      </div>
    </div>

    <!-- 抓叛徒 -->
    <div v-if="game && game.type === 'traitor'" class="traitor-wrap">
      <div class="traitor-tag">全城通缉</div>
      <div class="traitor-name">{{ game.user || '——' }}</div>
      <div class="traitor-reason">{{ game.reason }}</div>
    </div>

    <div class="banners">
      <div v-for="w in wraps" :key="w.id" class="banner"
        :style="{ borderColor: w.color, '--fade-delay': Math.max(600, w.duration - 600) + 'ms' }">
        <div class="btxt" :style="{ color: w.color }">{{ w.text }}</div>
        <div v-if="w.user" class="buser">来自 {{ w.user }}</div>
      </div>
    </div>

    <button v-if="!audioReady" class="audio-btn" @click="enableAudio">点击启用声音</button>

    <div v-if="debug" class="dbg">
      WS {{ connected ? 'OK' : 'DOWN' }} · FPS {{ fps }} · 粒子 {{ pcount }} / 上限 {{ maxParticles }}
      <div>
        <button @click="burstShells(3, 70)">烟花</button>
        <button @click="heartRain(60, 1)">爱心雨</button>
        <button @click="meteors(3)">流星</button>
        <button @click="confettiRain(60)">彩带</button>
        <button @click="pushWrap({ text: '感谢 测试观众 的 小心心 ×10', user: '测试观众' })">横幅</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.stage {
  position: fixed;
  inset: 0;
  overflow: hidden;
  background: transparent;
  font-family: "Microsoft YaHei", system-ui, sans-serif;
}
.fx { position: absolute; inset: 0; }
.cv { display: block; }

.spot {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 40%, rgba(255, 226, 150, 0.25), transparent 60%);
  animation: sweep 1.6s ease-in-out infinite alternate;
  pointer-events: none;
}
@keyframes sweep {
  from { transform: translateX(-22%); }
  to { transform: translateX(22%); }
}
@keyframes shake {
  0% { transform: translate(0, 0); }
  25% { transform: translate(-9px, 5px); }
  50% { transform: translate(7px, -6px); }
  75% { transform: translate(-5px, -3px); }
  100% { transform: translate(0, 0); }
}

.banners {
  position: absolute;
  left: 50%;
  bottom: 12%;
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  pointer-events: none;
}
.banner {
  background: rgba(10, 12, 18, 0.72);
  border: 2px solid #ffd24a;
  border-left-width: 6px;
  border-radius: 14px;
  padding: 12px 26px;
  text-align: center;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.45);
  animation: pop 0.35s cubic-bezier(0.2, 1.4, 0.4, 1) both,
    fade 0.45s ease var(--fade-delay, 4.4s) forwards;
  backdrop-filter: blur(4px);
}
.btxt { font-size: 30px; font-weight: 800; letter-spacing: 0.5px; }
.buser { margin-top: 4px; font-size: 14px; color: rgba(255, 255, 255, 0.72); }
@keyframes pop {
  from { transform: scale(0.7) translateY(12px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}
@keyframes fade {
  to { opacity: 0; transform: translateY(-10px); }
}

.audio-btn {
  position: absolute;
  top: 14px;
  right: 14px;
  background: rgba(245, 181, 68, 0.92);
  color: #16181f;
  border: 0;
  border-radius: 999px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.dbg {
  position: absolute;
  left: 14px;
  bottom: 14px;
  color: #cfe;
  background: rgba(0, 0, 0, 0.6);
  border-radius: 10px;
  padding: 10px 14px;
  font-size: 12px;
  font-family: Consolas, monospace;
}

/* ---------- 中控玩法 ---------- */
.wheel-wrap {
  position: absolute;
  left: 50%;
  top: 46%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  pointer-events: none;
}
.wheel-title {
  font-size: 38px;
  font-weight: 900;
  letter-spacing: 3px;
  color: #ffe9a8;
  text-shadow: 0 0 18px rgba(255, 190, 60, 0.85), 0 4px 16px rgba(0, 0, 0, 0.8);
  margin-bottom: 18px;
}
.wheel-box {
  position: relative;
  width: 520px;
  height: 520px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.wheel-glow {
  position: absolute;
  inset: -10px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(255, 205, 90, 0.34) 0%, rgba(255, 150, 30, 0.16) 54%, rgba(0, 0, 0, 0) 72%);
  filter: blur(16px);
  animation: wheelGlow 2.2s ease-in-out infinite;
}
@keyframes wheelGlow {
  0%, 100% { opacity: 0.7; }
  50% { opacity: 1; }
}
.wheel-result {
  margin-top: 12px;
  background: rgba(10, 12, 18, 0.78);
  border: 2px solid #ffd24a;
  border-radius: 999px;
  padding: 8px 26px;
  color: #ffe9b3;
  font-size: 22px;
}

.guess-wrap {
  position: absolute;
  left: 50%;
  top: 38%;
  transform: translate(-50%, -50%);
  text-align: center;
  pointer-events: none;
}
.guess-title {
  font-size: 28px;
  font-weight: 800;
  color: #ffd24a;
  text-shadow: 0 4px 18px rgba(0, 0, 0, 0.7);
  margin-bottom: 8px;
}
.guess-range {
  display: flex;
  align-items: center;
  gap: 22px;
  font-size: 88px;
  font-weight: 900;
  color: #fff;
  text-shadow: 0 0 24px rgba(255, 107, 138, 0.9), 0 6px 24px rgba(0, 0, 0, 0.8);
  animation: guesspulse 0.6s ease both;
}
.guess-range i { font-style: normal; font-size: 46px; color: #ffd24a; }
@keyframes guesspulse {
  0% { transform: scale(1.16); opacity: 0.4; }
  100% { transform: scale(1); opacity: 1; }
}
.guess-win { font-size: 40px; font-weight: 900; color: #fff; }
.gw-name { color: #ffd24a; font-size: 48px; }
.gw-text { margin-top: 6px; }

.traitor-wrap {
  position: absolute;
  left: 50%;
  top: 44%;
  transform: translate(-50%, -50%);
  text-align: center;
  pointer-events: none;
  background: rgba(28, 6, 8, 0.84);
  border: 3px solid #ff4d5e;
  border-radius: 20px;
  padding: 26px 54px;
  box-shadow: 0 0 60px rgba(255, 60, 80, 0.55);
}
.traitor-tag {
  font-size: 20px;
  letter-spacing: 8px;
  color: #ff8a95;
  font-weight: 700;
}
.traitor-name {
  font-size: 64px;
  font-weight: 900;
  color: #fff;
  margin: 6px 0;
  text-shadow: 0 0 26px rgba(255, 80, 100, 0.9);
}
.traitor-reason {
  font-size: 20px;
  color: #ffd24a;
  max-width: 620px;
}
.dbg button {
  margin: 6px 6px 0 0;
  background: #1b1f2a;
  color: #e7eaf2;
  border: 1px solid #333a4a;
  border-radius: 6px;
  padding: 4px 8px;
  cursor: pointer;
}
</style>

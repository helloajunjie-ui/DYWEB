<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { connectWs, get, post } from '../ws.js'

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

// 转盘：自绘 SVG 盘面（深红金浮雕风），转动用 CSS 缓动
const wheelRot = ref(0)
const wheelDur = ref(4200)
const wheelSpin = ref(false)
const wheelWinner = ref('')
const wheelWinIdx = ref(-1)
const wheelSlices = ref([])

// 彩色玻璃：宝石色玻璃片 + 深色铅条分隔，不发光
const sliceThemes = [
  { a: '#235ca6', b: '#3a7bd5', glow: '#0b1119' }, // 蓝宝石
  { a: '#177a5e', b: '#22a074', glow: '#0b1119' }, // 祖母绿
  { a: '#a3264a', b: '#c73460', glow: '#0b1119' }, // 红宝石
  { a: '#9a6414', b: '#bd8220', glow: '#0b1119' }, // 琥珀
  { a: '#6a3899', b: '#8850bd', glow: '#0b1119' }, // 紫水晶
  { a: '#237a9e', b: '#2f9dc4', glow: '#0b1119' }, // 青金石
  { a: '#8a3661', b: '#a84577', glow: '#0b1119' }, // 石榴石
  { a: '#5c7a28', b: '#78992f', glow: '#0b1119' } // 橄榄石
]

function buildSlices(items) {
  const n = items.length
  const seg = 360 / n
  // 块越多字越小，避免字压到中心轴
  const fontSize = n >= 11 ? 16 : n >= 9 ? 18 : n >= 7 ? 21 : 25
  return items.map((text, i) => {
    const t = sliceThemes[i % sliceThemes.length]
    const mid = -90 + seg * (i + 0.5)
    // 落在下半圈的扇区把文字翻 180°，避免“倒着念”
    const m = ((mid % 360) + 360) % 360
    const flip = m > 90 && m < 270
    return {
      text,
      i,
      a0: -90 + seg * i + 0.7,
      a1: -90 + seg * (i + 1) - 0.7,
      mid,
      seg,
      from: t.a,
      to: t.b,
      glow: t.glow,
      labelRot: flip ? mid + 180 : mid,
      labelX: flip ? -(R_TEXT - 6) : R_TEXT - 6,
      labelAnchor: flip ? 'start' : 'end',
      fontSize,
      lines: splitLabel(text, n)
    }
  })
}

function splitLabel(text, n) {
  const s = String(text || '')
  // 块多的时候一行放不下，截断加省略号，别压到中轴
  if (n >= 10) return [s.length > 6 ? s.slice(0, 6) + '…' : s]
  if (s.length <= 5) return [s]
  const mid = Math.ceil(s.length / 2)
  return [s.slice(0, mid), s.slice(mid)]
}

const R_OUT = 252
const R_TEXT = 206

function pt(r, deg) {
  const a = (deg * Math.PI) / 180
  return [260 + r * Math.cos(a), 260 + r * Math.sin(a)]
}

function slicePath(sl) {
  const [x0, y0] = pt(R_OUT, sl.a0)
  const [x1, y1] = pt(R_OUT, sl.a1)
  return `M 260 260 L ${x0.toFixed(2)} ${y0.toFixed(2)} A ${R_OUT} ${R_OUT} 0 0 1 ${x1.toFixed(2)} ${y1.toFixed(2)} Z`
}

function startWheel(items, ms) {
  wheelItems.value = items.slice()
  nextTick(drawWheel)
  wheelWinner.value = ''
  wheelWinIdx.value = -1
  wheelDur.value = Number(ms) || 4200
  wheelSpin.value = false
  wheelRot.value = 0

  const n = items.length
  const idx = Math.floor(Math.random() * n)
  const seg = 360 / n
  const jitter = (Math.random() - 0.5) * seg * 0.45
  // 指针在正上方，让 idx 扇区中心停在 12 点，外加 6 圈
  const target = 360 * 6 - seg * (idx + 0.5) + jitter
  wheelRot.value = 0
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      wheelSpin.value = true
      wheelRot.value = target
    })
  })
  setTimeout(() => {
    wheelSpin.value = false
    wheelWinIdx.value = idx
    wheelWinner.value = items[idx]
    post('/api/game/result', { text: items[idx] }).catch(() => {})
  }, wheelDur.value + 150)
}

const wheelCanvas = ref(null)
const wheelItems = ref([])

const WHEEL_COLORS = ['#ff2d55', '#ff9f0a', '#ffd60a', '#2ee06a', '#00d4d4', '#2f7bff', '#7b5cff', '#ff49c8']

function hex2rgb(h) {
  const s = h.replace('#', '')
  return [parseInt(s.slice(0, 2), 16), parseInt(s.slice(2, 4), 16), parseInt(s.slice(4, 6), 16)]
}
function mixHex(c1, c2, t) {
  const a = hex2rgb(c1)
  const b = hex2rgb(c2)
  const m = (i) => Math.round(a[i] + (b[i] - a[i]) * t)
  return 'rgb(' + m(0) + ',' + m(1) + ',' + m(2) + ')'
}

// 彩色玻璃盘面：宝石色扇区 + 铅条 + 斜向反光 + 金属环，全部画在 canvas 上
function drawWheel() {
  const cv = wheelCanvas.value
  if (!cv) return
  const ctx = cv.getContext('2d')
  if (!ctx) return
  const SIZE = 640
  const CX = 320
  const CY = 320
  const R = 286
  const RING_R = 296
  const RING_W = 18
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  if (cv.width !== SIZE * dpr) {
    cv.width = SIZE * dpr
    cv.height = SIZE * dpr
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, SIZE, SIZE)

  const items = wheelItems.value
  const n = items.length
  if (!n) return
  const step = (Math.PI * 2) / n
  const start = -Math.PI / 2 - step / 2
  const win = wheelWinIdx.value

  ctx.beginPath()
  ctx.arc(CX, CY, R + RING_W + 6, 0, Math.PI * 2)
  ctx.fillStyle = '#05070d'
  ctx.fill()

  for (let i = 0; i < n; i++) {
    const a0 = start + i * step
    const a1 = a0 + step
    const col = WHEEL_COLORS[i % WHEEL_COLORS.length]
    ctx.save()
    ctx.beginPath()
    ctx.moveTo(CX, CY)
    ctx.arc(CX, CY, R, a0, a1)
    ctx.closePath()
    ctx.clip()

    const g = ctx.createRadialGradient(CX, CY, R * 0.04, CX, CY, R)
    g.addColorStop(0, mixHex(col, '#ffffff', 0.62))
    g.addColorStop(0.4, mixHex(col, '#ffffff', 0.12))
    g.addColorStop(0.78, col)
    g.addColorStop(1, mixHex(col, '#000000', 0.25))
    ctx.fillStyle = g
    ctx.fillRect(0, 0, SIZE, SIZE)

    ctx.globalCompositeOperation = 'overlay'
    const lg = ctx.createLinearGradient(CX - R, CY - R, CX + R, CY + R)
    lg.addColorStop(0, 'rgba(255,255,255,0.62)')
    lg.addColorStop(0.28, 'rgba(255,255,255,0.05)')
    lg.addColorStop(0.52, 'rgba(0,0,0,0.22)')
    lg.addColorStop(0.78, 'rgba(255,255,255,0.08)')
    lg.addColorStop(1, 'rgba(255,255,255,0.28)')
    ctx.fillStyle = lg
    ctx.fillRect(0, 0, SIZE, SIZE)

    ctx.globalCompositeOperation = 'source-over'
    // 镜面高光带：让每块玻璃有厚度
    ctx.globalCompositeOperation = 'lighter'
    const midA = a0 + step / 2
    const hw = Math.sin(step / 2) * R * 0.62
    ctx.save()
    ctx.translate(CX, CY)
    ctx.rotate(midA)
    const hg = ctx.createLinearGradient(0, -hw, 0, hw)
    hg.addColorStop(0, 'rgba(255,255,255,0)')
    hg.addColorStop(0.5, 'rgba(255,255,255,0.22)')
    hg.addColorStop(1, 'rgba(255,255,255,0)')
    ctx.fillStyle = hg
    ctx.beginPath()
    ctx.moveTo(R * 0.06, 0)
    ctx.lineTo(R * 0.99, -hw * 1.15)
    ctx.lineTo(R * 0.99, hw * 1.15)
    ctx.closePath()
    ctx.fill()
    ctx.restore()
    ctx.globalCompositeOperation = 'source-over'
    ctx.strokeStyle = 'rgba(255,255,255,0.14)'
    ctx.lineWidth = 1.4
    for (let k = 0; k < 3; k++) {
      const s = i * 31 + k * 17 + 3
      const ra = a0 + step * (0.18 + ((Math.sin(s * 12.9898) * 43758.5453) % 1 + 1) % 1 * 0.64)
      ctx.beginPath()
      ctx.moveTo(CX + Math.cos(ra) * R * 0.14, CY + Math.sin(ra) * R * 0.14)
      ctx.lineTo(CX + Math.cos(ra + 0.08) * R * 0.6, CY + Math.sin(ra + 0.08) * R * 0.6)
      ctx.lineTo(CX + Math.cos(ra + 0.16) * R * 0.94, CY + Math.sin(ra + 0.16) * R * 0.94)
      ctx.stroke()
    }

    // 中奖块：其余压暗后再统一描白边
    if (win >= 0 && win !== i) {
      ctx.fillStyle = 'rgba(0,0,0,0.3)'
      ctx.fillRect(0, 0, SIZE, SIZE)
    }
    ctx.restore()
  }

  // 铅条
  for (let i = 0; i < n; i++) {
    const a = start + i * step
    const cos = Math.cos(a)
    const sin = Math.sin(a)
    ctx.beginPath()
    ctx.moveTo(CX + cos * 26, CY + sin * 26)
    ctx.lineTo(CX + cos * (R - 2), CY + sin * (R - 2))
    ctx.lineWidth = 10
    ctx.strokeStyle = 'rgba(6,9,15,0.94)'
    ctx.stroke()
    ctx.beginPath()
    ctx.moveTo(CX + cos * 26 - sin * 2.4, CY + sin * 26 + cos * 2.4)
    ctx.lineTo(CX + cos * (R - 2) - sin * 2.4, CY + sin * (R - 2) + cos * 2.4)
    ctx.lineWidth = 1.6
    ctx.strokeStyle = 'rgba(205,222,246,0.18)'
    ctx.stroke()
  }

  // 文字：白字 + 黑边，下半圈翻正
  const fs = n <= 6 ? 34 : n <= 8 ? 30 : n <= 10 ? 25 : n <= 12 ? 21 : 18
  ctx.font = '800 ' + fs + 'px "Microsoft YaHei", sans-serif'
  ctx.textBaseline = 'middle'
  for (let i = 0; i < n; i++) {
    const mid = start + i * step + step / 2
    const deg = ((((mid * 180) / Math.PI) % 360) + 360) % 360
    const flip = deg > 90 && deg < 270
    let label = String(items[i] || '')
    if (label.length > 5 && n >= 10) label = label.slice(0, 5) + '…'
    ctx.save()
    ctx.translate(CX, CY)
    ctx.rotate(flip ? mid + Math.PI : mid)
    ctx.textAlign = flip ? 'left' : 'right'
    const x = flip ? -(R - 26) : R - 26
    ctx.lineWidth = 6
    ctx.lineJoin = 'round'
    ctx.strokeStyle = '#000000'
    ctx.strokeText(label, x, 0)
    ctx.fillStyle = '#ffffff'
    ctx.fillText(label, x, 0)
    ctx.restore()
  }

  // 金属外环
  ctx.beginPath()
  ctx.arc(CX, CY, RING_R, 0, Math.PI * 2)
  ctx.lineWidth = RING_W + 8
  ctx.strokeStyle = '#05070d'
  ctx.stroke()
  const mg = ctx.createLinearGradient(CX - 300, CY - 300, CX + 300, CY + 300)
  mg.addColorStop(0, '#fff3c4')
  mg.addColorStop(0.18, '#d8ae52')
  mg.addColorStop(0.38, '#8a6520')
  mg.addColorStop(0.55, '#ffeaa8')
  mg.addColorStop(0.72, '#a97f2e')
  mg.addColorStop(0.88, '#f0d78e')
  mg.addColorStop(1, '#6d4c12')
  ctx.beginPath()
  ctx.arc(CX, CY, RING_R, 0, Math.PI * 2)
  ctx.lineWidth = RING_W - 5
  ctx.strokeStyle = mg
  ctx.stroke()

  // 环上宝石
  for (let i = 0; i < n; i++) {
    const a = start + i * step
    const x = CX + Math.cos(a) * RING_R
    const y = CY + Math.sin(a) * RING_R
    const col = WHEEL_COLORS[i % WHEEL_COLORS.length]
    const gg = ctx.createRadialGradient(x - 2.6, y - 2.6, 0.5, x, y, 8.5)
    gg.addColorStop(0, '#ffffff')
    gg.addColorStop(0.38, col)
    gg.addColorStop(1, mixHex(col, '#000000', 0.62))
    ctx.beginPath()
    ctx.arc(x, y, 7.5, 0, Math.PI * 2)
    ctx.fillStyle = gg
    ctx.fill()
    ctx.lineWidth = 1.6
    ctx.strokeStyle = 'rgba(0,0,0,0.6)'
    ctx.stroke()
  }

  // 整盘玻璃高光
  ctx.save()
  ctx.beginPath()
  ctx.arc(CX, CY, R + RING_W / 2 + 2, 0, Math.PI * 2)
  ctx.clip()
  ctx.globalCompositeOperation = 'lighter'
  const gl = ctx.createRadialGradient(CX - R * 0.45, CY - R * 0.52, 0, CX - R * 0.45, CY - R * 0.52, R * 1.5)
  gl.addColorStop(0, 'rgba(255,255,255,0.40)')
  gl.addColorStop(0.28, 'rgba(255,255,255,0.12)')
  gl.addColorStop(0.6, 'rgba(255,255,255,0.02)')
  gl.addColorStop(1, 'rgba(255,255,255,0)')
  ctx.fillStyle = gl
  ctx.fillRect(0, 0, SIZE, SIZE)
  ctx.globalCompositeOperation = 'source-over'
  const vg = ctx.createRadialGradient(CX, CY, R * 0.5, CX, CY, R + 10)
  vg.addColorStop(0, 'rgba(0,0,0,0)')
  vg.addColorStop(0.75, 'rgba(0,0,0,0.08)')
  vg.addColorStop(1, 'rgba(0,0,0,0.32)')
  ctx.fillStyle = vg
  ctx.fillRect(0, 0, SIZE, SIZE)
  ctx.restore()

  // 中奖块描白边
  if (win >= 0) {
    const a0 = start + win * step
    const a1 = a0 + step
    ctx.save()
    ctx.beginPath()
    ctx.moveTo(CX, CY)
    ctx.arc(CX, CY, R, a0, a1)
    ctx.closePath()
    ctx.lineWidth = 6
    ctx.strokeStyle = 'rgba(255,255,255,0.95)'
    ctx.stroke()
    ctx.restore()
  }
}

watch([wheelItems, wheelWinIdx], () => {
  nextTick(drawWheel)
})
// 名字抽奖：从观众名单里抽 1~10 个
const namePool = ref([])
const nameWins = ref([])
const nameRoll = ref('')
const nameDone = ref(false)
let nameTimer = null

function startNames(d) {
  if (nameTimer) {
    clearInterval(nameTimer)
    nameTimer = null
  }
  namePool.value = d.items && d.items.length ? d.items : []
  nameWins.value = d.wins || []
  nameDone.value = false
  nameRoll.value = ''
  const ms = Number(d.spinMs) || 2400
  if (!namePool.value.length) {
    nameRoll.value = '名单空的'
    return
  }
  const t0 = Date.now()
  nameTimer = setInterval(() => {
    nameRoll.value = namePool.value[Math.floor(Math.random() * namePool.value.length)]
    if (Date.now() - t0 >= ms) {
      clearInterval(nameTimer)
      nameTimer = null
      nameRoll.value = ''
      nameDone.value = true
    }
  }, 70)
}

function onGame(d) {
  if (!d || d.type === 'clear' || d.type === '') {
    game.value = null
    if (nameTimer) {
      clearInterval(nameTimer)
      nameTimer = null
    }
    return
  }
  if (d.type === 'wheel' && (!d.items || !d.items.length)) return
  game.value = d
  if (d.type === 'wheel') {
    startWheel(d.items, d.spinMs)
  }
  if (d.type === 'names') {
    startNames(d)
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
      <div class="wheel-title">大转盘</div>
      <div class="wheel-box">
          <div
            class="wheel-rotor"
            :style="{
              transform: 'rotate(' + wheelRot + 'deg)',
              transitionDuration: wheelDur + 'ms',
              transitionTimingFunction: 'cubic-bezier(.12,.72,.16,1)'
            }"
          >
            <canvas ref="wheelCanvas" class="wheel-canvas" width="640" height="640"></canvas>
          </div>
          <div class="wheel-hub">开转</div>
          <div class="wheel-pointer"></div>      </div>
      <div v-if="wheelWinner" class="wheel-win">🎉 {{ wheelWinner }}</div>
    </div>

    <!-- 名字抽奖 -->
    <div v-if="game && game.type === 'names'" class="names-wrap" :key="'names-' + game.started">
      <div class="names-title">名字抽奖</div>
      <div v-if="!nameDone" class="names-roll-box">
        <div class="names-roll">{{ nameRoll || '准备…' }}</div>
        <div class="names-sub">从 {{ game.pool || namePool.length }} 位观众里抽 {{ game.count || nameWins.length }} 个</div>
      </div>
      <div v-else class="names-wins">
        <div
          v-for="(w, i) in nameWins"
          :key="i"
          class="name-card"
          :style="{ animationDelay: i * 0.14 + 's' }"
        >
          <span class="name-no">{{ i + 1 }}</span>
          <span class="name-text">{{ w }}</span>
        </div>
      </div>
    </div>

    <!-- 猜数字 -->
    <div v-if="game && game.type === 'guess'" class="guess-wrap" :key="guessSeq">
      <div class="guess-title">弹幕猜数字</div>
      <div v-if="game.won" class="guess-win">
        <div class="gw-name">{{ game.user }}</div>
        <div class="gw-text">猜中了！数字就是 {{ game.guess }}</div>
      </div>
      <template v-else>
        <div class="guess-range" :key="game.lo + '-' + game.hi">
          <span>{{ game.lo }}</span>
          <i>~</i>
          <span>{{ game.hi }}</span>
        </div>
        <div class="guess-hint">范围 {{ game.min }} ~ {{ game.max }}（已经缩到这里）</div>
        <div v-if="game.guess" class="guess-fb" :class="game.dir" :key="'fb-' + game.started">
          <span class="fb-user">{{ game.user }}</span>
          <span class="fb-num">猜了 {{ game.guess }}</span>
          <span class="fb-word">{{ game.dir === 'high' ? '猜大了' : game.dir === 'low' ? '猜小了' : '猜中' }}</span>
        </div>
      </template>
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
  color: #ffffff;
  text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000, 0 4px 18px rgba(0, 0, 0, 0.9);
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
  display: none;
  position: absolute;
  inset: -14px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(255, 190, 80, 0.30) 0%, rgba(190, 40, 20, 0.18) 52%, rgba(0, 0, 0, 0) 72%);
  filter: none;
  animation: wheelGlow 2.2s ease-in-out infinite;
}
.wheel-rim {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: 50%;
  pointer-events: none;
}
.bulb {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 9px;
  height: 9px;
  margin: -4.5px 0 0 -4.5px;
  border-radius: 50%;
  background: #bff6ff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.6);
  animation: twinkle 1.05s ease-in-out infinite alternate;
}
.wheel-svg {
  width: 520px;
  height: 520px;
  display: block;
  filter: none;
}
.wheel-canvas {
  width: 520px;
  height: 520px;
  display: block;
  border-radius: 50%;
}
.wheel-hub {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 96px;
  height: 96px;
  margin: -48px 0 0 -48px;
  border-radius: 50%;
  background: #05070e;
  border: 2px solid #4a5763;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #dfe8f2;
  font-family: 'Microsoft YaHei', sans-serif;
  font-weight: 800;
  font-size: 22px;
  z-index: 3;
}
.wheel-rotor {
  transform-origin: 50% 50%;
  transition-property: transform;
  will-change: transform;
}
.wheel-label {
  fill: #ffffff;
  font-family: "Microsoft YaHei", sans-serif;
  font-weight: 800;
  paint-order: stroke;
  stroke: #000000;
  stroke-width: 4px;
  stroke-linejoin: round;
  letter-spacing: 1px;
}
.hub-text {
  fill: #9ef0ff;
  font-family: "Microsoft YaHei", sans-serif;
  font-size: 20px;
  font-weight: 900;
  letter-spacing: 3px;
}
.wheel-pointer {
  position: absolute;
  left: 50%;
  top: -10px;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 16px solid transparent;
  border-right: 16px solid transparent;
  border-top: 42px solid #dfe8f2;
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.7));
  z-index: 3;
  animation: pointerPulse 1.5s ease-in-out infinite;
}
@keyframes pointerPulse {
  0%, 100% { transform: translateX(-50%) translateY(0); }
  50% { transform: translateX(-50%) translateY(-4px); }
}
.slice {
  transition: opacity 0.35s ease;
}
.slice.dim {
  opacity: 0.5;
}
.wheel-scan {
  display: none;
  position: absolute;
  inset: 6px;
  border-radius: 50%;
  pointer-events: none;
  background: repeating-linear-gradient(180deg, rgba(120, 220, 255, 0.14) 0px, rgba(120, 220, 255, 0.14) 1px, transparent 1px, transparent 4px);
  mix-blend-mode: screen;
  opacity: 0.3;
  animation: scanMove 2.4s linear infinite;
}
@keyframes scanMove {
  from { background-position: 0 0; }
  to { background-position: 0 40px; }
}
.win-flash {
  fill: rgba(255, 255, 255, 0.03);
  stroke-width: 4;
  pointer-events: none;
  animation: winFlash 0.55s ease-in-out infinite alternate;
}
@keyframes winFlash {
  from { opacity: 0.25; }
  to { opacity: 1; }
}
.wheel-label.lit {
  fill: #ffffff;
  font-weight: 900;
}
/* ---------- 名字抽奖 ---------- */
.names-wrap {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 860px;
  max-width: 92vw;
  pointer-events: none;
}
.names-title {
  font-size: 40px;
  font-weight: 900;
  letter-spacing: 4px;
  color: #ffffff;
  text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000;
  margin-bottom: 26px;
}
.names-roll-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 34px 60px 26px;
  border-radius: 24px;
  background: radial-gradient(circle at 50% 30%, rgba(24, 60, 96, 0.92), rgba(5, 9, 16, 0.95));
  border: 2px solid #4ac7ff;
  box-shadow: 0 8px 26px rgba(0, 0, 0, 0.7);
}
.names-roll {
  font-size: 62px;
  font-weight: 900;
  letter-spacing: 3px;
  color: #ffffff;
  text-shadow: -3px -3px 0 #000, 3px -3px 0 #000, -3px 3px 0 #000, 3px 3px 0 #000, 0 5px 14px rgba(0, 0, 0, 0.9);
  min-width: 360px;
  text-align: center;
  transition: transform 0.08s ease;
}
.names-sub {
  margin-top: 14px;
  font-size: 17px;
  color: #9fd9f5;
  letter-spacing: 1px;
}
.names-wins {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 16px;
  max-width: 860px;
}
.name-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 28px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(18, 46, 78, 0.95), rgba(8, 14, 24, 0.95));
  border: 2px solid #7de9ff;
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.65);
  animation: namePop 0.5s cubic-bezier(0.2, 1.5, 0.4, 1) both;
}
.name-no {
  width: 34px;
  height: 34px;
  line-height: 34px;
  text-align: center;
  border-radius: 50%;
  background: #7de9ff;
  color: #05121e;
  font-weight: 900;
  font-size: 18px;
}
.name-text {
  font-size: 30px;
  font-weight: 900;
  letter-spacing: 1px;
  color: #ffffff;
  text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000;
}
@keyframes namePop {
  from { transform: scale(0.6) translateY(16px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}
.wheel-win {
  margin-top: 14px;
  padding: 10px 30px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(8, 20, 36, 0.92), rgba(4, 8, 14, 0.92));
  border: 2px solid #7de9ff;
  color: #bff6ff;
  font-size: 24px;
  font-weight: 800;
  letter-spacing: 1px;
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.6);
  animation: pop 0.4s cubic-bezier(0.2, 1.4, 0.4, 1) both;
}
@keyframes twinkle {
  from { opacity: 0.3; }
  to { opacity: 1; }
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
  text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000, 0 6px 18px rgba(0, 0, 0, 0.85);
  animation: guesspulse 0.6s ease both;
}
.guess-range i { font-style: normal; font-size: 46px; color: #ffd24a; }
@keyframes guesspulse {
  0% { transform: scale(1.16); opacity: 0.4; }
  100% { transform: scale(1); opacity: 1; }
}
.guess-win { font-size: 40px; font-weight: 900; color: #fff; }
.gw-name { color: #ffd24a; font-size: 48px; }
.gw-text { margin-top: 6px; text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000, 0 4px 12px rgba(0, 0, 0, 0.85); }

.guess-hint {
  margin-top: 10px;
  font-size: 18px;
  color: #cfeeff;
  letter-spacing: 1px;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.8);
}
.guess-fb {
  margin-top: 16px;
  display: inline-flex;
  align-items: center;
  gap: 14px;
  padding: 12px 28px;
  border-radius: 999px;
  background: rgba(5, 9, 16, 0.85);
  border: 2px solid #7de9ff;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.7);
  animation: fbIn 0.45s cubic-bezier(0.2, 1.4, 0.4, 1) both;
}
.guess-fb.high { border-color: #ff6b8a; }
.guess-fb.low { border-color: #5fd8ff; }
.fb-user { font-size: 20px; color: #cfd8e6; }
.fb-num { font-size: 24px; color: #ffffff; font-weight: 800; }
.fb-word {
  font-size: 34px;
  font-weight: 900;
  letter-spacing: 2px;
  color: #ffffff;
  text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000, 0 3px 10px rgba(0, 0, 0, 0.9);
}
.guess-fb.high .fb-word { color: #ffffff; }
.guess-fb.low .fb-word { color: #ffffff; }
@keyframes fbIn {
  from { transform: scale(0.7) translateY(12px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}

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
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.6);
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
  text-shadow: -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000;
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

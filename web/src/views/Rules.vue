<script setup>
import { ref, computed } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])

const local = ref(JSON.parse(JSON.stringify(props.data.rules || [])))
const editing = ref(null) // 索引
const message = ref('')
const advanced = ref(false)
const editText = ref('')
const welcome = ref({ enabled: false, minLevel: 0, minFans: 0, minValue: 0 })

async function loadWelcome() {
  try {
    const c = await (await fetch('/api/config')).json()
    if (c.welcome) welcome.value = c.welcome
  } catch (e) {}
}

async function saveWelcome() {
  try {
    const c = await (await fetch('/api/config')).json()
    c.welcome = welcome.value
    const res = await post('/api/config', c)
    message.value = res.ok ? '欢迎门槛已保存' : res.msg || '保存失败'
  } catch (e) {
    message.value = '保存失败：' + e.message
  }
}

loadWelcome()

const triggers = [
  { id: 'gift', name: '有人送礼物' },
  { id: 'danmaku', name: '有人发弹幕' },
  { id: 'enter', name: '有人进直播间' },
  { id: 'follow', name: '有人关注' },
  { id: 'fansclub', name: '有人加入粉丝团' },
  { id: 'guard', name: '有人开星守护' },
  { id: 'like', name: '有人点赞' }
]

const presets = [
  { id: 'fireworks', name: '烟花' },
  { id: 'hearts', name: '爱心雨' },
  { id: 'meteor', name: '流星' },
  { id: 'confetti', name: '彩带' },
  { id: 'shake', name: '画面抖动' },
  { id: 'spotlight', name: '聚光灯' }
]

const sfxOptions = computed(() => props.data.sfx || [])

// 表单模型（小白只填这些）
function blankForm() {
  return {
    id: '',
    name: '',
    enabled: true,
    trigger: 'gift',
    keyword: '',
    giftNames: '',
    minValue: 0,
    cooldownSec: 3,
    doTts: true,
    ttsText: '感谢{user}送的{gift}×{count}，谢谢老板！',
    doEffect: false,
    effectPreset: 'hearts',
    effectMs: 4000,
    doBanner: false,
    bannerText: '感谢 {user} 的 {gift} × {count}',
    bannerColor: '#FFD24A',
    doSound: false,
    soundFile: '',
    doHotkey: false,
    hotkeyKeys: '',
    doWebhook: false,
    webhookUrl: '',
    doLlm: false,
    llmPrompt: '观众说：{text}。用一句有梗的话回应。'
  }
}
const form = ref(blankForm())

function toForm(r) {
  const m = r.match || {}
  const f = blankForm()
  f.id = r.id
  f.name = r.name
  f.enabled = r.enabled !== false
  f.trigger = (m.types && m.types[0]) || 'gift'
  f.keyword = m.keyword || ''
  f.giftNames = (m.giftNames || []).join('、')
  f.minValue = m.minValue || 0
  f.cooldownSec = Math.round((r.cooldownMs || 0) / 1000)
  f.ttsText = '感谢{user}送的{gift}×{count}，谢谢老板！'
  for (const a of r.actions || []) {
    if (a.type === 'tts') { f.doTts = true; f.ttsText = a.template || f.ttsText }
    if (a.type === 'effect') { f.doEffect = true; f.effectPreset = a.preset || 'hearts'; f.effectMs = a.durationMs || 4000 }
    if (a.type === 'banner') { f.doBanner = true; f.bannerText = a.template || f.bannerText; f.bannerColor = a.color || '#FFD24A' }
    if (a.type === 'sound') { f.doSound = true; f.soundFile = a.file || '' }
    if (a.type === 'hotkey') { f.doHotkey = true; f.hotkeyKeys = a.keys || '' }
    if (a.type === 'webhook') { f.doWebhook = true; f.webhookUrl = a.url || '' }
    if (a.type === 'llm') { f.doLlm = true; f.llmPrompt = a.llmPrompt || f.llmPrompt }
  }
  return f
}

function toRule(f) {
  const match = { types: [f.trigger], keyword: '', keywordMode: 'contains', giftNames: [], minValue: 0, minCount: 0, minLevel: 0, comboWithinMs: 0, onlyRegular: false }
  if (f.trigger === 'danmaku' && f.keyword) match.keyword = f.keyword
  if (f.trigger === 'gift') {
    match.minValue = Number(f.minValue) || 0
    match.giftNames = (f.giftNames || '').split(/[、,，\s]+/).map((s) => s.trim()).filter(Boolean)
  }
  const actions = []
  if (f.doTts) actions.push({ type: 'tts', template: f.ttsText })
  if (f.doEffect) actions.push({ type: 'effect', preset: f.effectPreset, durationMs: Number(f.effectMs) || 4000, merge: true })
  if (f.doBanner) actions.push({ type: 'banner', template: f.bannerText, durationMs: Number(f.effectMs) || 4000, color: f.bannerColor })
  if (f.doSound && f.soundFile) actions.push({ type: 'sound', file: f.soundFile, volume: 0.85 })
  if (f.doHotkey && f.hotkeyKeys) actions.push({ type: 'hotkey', keys: f.hotkeyKeys })
  if (f.doWebhook && f.webhookUrl) actions.push({ type: 'webhook', url: f.webhookUrl, method: 'POST', template: '{"user":"{user}","text":"{text}","gift":"{gift}","count":"{count}"}' })
  if (f.doLlm) actions.push({ type: 'llm', llmPrompt: f.llmPrompt })
  return {
    id: f.id || 'r_' + Math.random().toString(36).slice(2, 7),
    name: f.name || '未命名规则',
    enabled: f.enabled !== false,
    priority: 10,
    cooldownMs: (Number(f.cooldownSec) || 0) * 1000,
    match,
    actions
  }
}

function openNew() {
  form.value = blankForm()
  advanced.value = false
  editing.value = local.value.length
  local.value.push(toRule(form.value))
  editText.value = JSON.stringify(local.value[editing.value], null, 2)
}

function openEdit(i) {
  form.value = toForm(local.value[i])
  editing.value = i
  advanced.value = false
  editText.value = JSON.stringify(local.value[i], null, 2)
}

function pickScript(e) {
  const v = e.target.value
  if (v) form.value.ttsText = v
  e.target.value = ''
}

function apply() {  const rule = toRule(form.value)
  local.value[editing.value] = rule
  editing.value = null
}

function applyJson() {
  try {
    local.value[editing.value] = JSON.parse(editText.value)
    editing.value = null
  } catch (e) {
    message.value = 'JSON 有错：' + e.message
  }
}

function remove(i) {
  local.value.splice(i, 1)
  editing.value = null
}

async function save() {
  const res = await post('/api/rules', { rules: local.value })
  message.value = res.ok ? `已保存 ${res.count} 条规则` : '保存失败：' + (res.msg || '')
  if (res.ok) {
    emit('reload')
    setTimeout(() => (local.value = JSON.parse(JSON.stringify(props.data.rules || []))), 300)
  }
}

async function toggle(r) {
  r.enabled = !r.enabled
  await save()
}

const triggerName = (t) => triggers.find((x) => x.id === t)?.name || t
const actionsLabel = (r) =>
  (r.actions || [])
    .map((a) => ({ tts: '播报', sound: '音效', effect: '特效', banner: '横幅', webhook: '触发网址', hotkey: '按键', llm: 'AI' }[a.type] || a.type))
    .join(' + ')
const matchLabel = (r) => {
  const m = r.match || {}
  const bits = []
  if (m.types?.length) bits.push(m.types.map(triggerName).join('/'))
  if (m.keyword) bits.push(`说「${m.keyword}」`)
  if (m.giftNames?.length) bits.push('礼物 ' + m.giftNames.join('/'))
  if (m.minValue) bits.push(`≥${m.minValue}币`)
  if (m.minLevel) bits.push(`Lv${m.minLevel}+`)
  if (m.onlyRegular) bits.push('仅老粉')
  return bits.join(' · ') || '任何事件'
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">一条规则 = 什么时候触发 + 触发后干什么。点「新建规则」按格子填就行。</div>
    <span style="flex: 1"></span>
    <span v-if="message" class="tag warn">{{ message }}</span>
    <button class="btn" @click="openNew">新建规则</button>
    <button class="btn primary" @click="save">保存全部</button>
  </div>

  <div class="card" style="margin-bottom: 12px">
    <div class="row">
      <h3 style="margin: 0">进场欢迎：只欢迎值得欢迎的人</h3>
      <span style="flex: 1"></span>
      <label class="switch"><input type="checkbox" v-model="welcome.enabled" /><i></i></label>
    </div>
    <div class="small muted" style="margin: 6px 0 0">
      开着 → 只欢迎下面任意一项达标的人；关掉 → 谁进来都欢迎（默认关）。
    </div>
    <div v-if="welcome.enabled" class="wrap" style="margin-top: 12px">
      <span>账号等级 ≥</span>
      <input v-model.number="welcome.minLevel" type="number" min="0" style="width: 80px" />
      <span>粉丝团等级 ≥</span>
      <input v-model.number="welcome.minFans" type="number" min="0" style="width: 80px" />
      <span>累计消费 ≥</span>
      <input v-model.number="welcome.minValue" type="number" min="0" style="width: 100px" />
      <span class="small muted">抖币</span>
    </div>
    <div v-if="welcome.enabled" class="small muted" style="margin-top: 6px">填 0 = 这一项不限；三项满足任意一项就算「值得欢迎」。</div>
    <div class="row" style="margin-top: 10px">
      <button class="btn primary" @click="saveWelcome">保存欢迎门槛</button>
    </div>
  </div>

  <div class="card" style="padding: 0">
    <table>
      <thead>
        <tr>
          <th style="width: 60px">启用</th>
          <th style="width: 170px">规则名</th>
          <th>什么时候触发</th>
          <th>触发后做什么</th>
          <th style="width: 70px">命中</th>
          <th style="width: 140px"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(r, i) in local" :key="r.id + i">
          <td>
            <label class="switch">
              <input type="checkbox" :checked="r.enabled" @change="toggle(r)" />
              <i></i>
            </label>
          </td>
          <td>{{ r.name }}<div class="small muted">{{ r.id }}</div></td>
          <td class="small">{{ matchLabel(r) }}</td>
          <td class="small">{{ actionsLabel(r) }}</td>
          <td class="mono">{{ r.hitCount || 0 }}</td>
          <td>
            <button class="btn sm ghost" @click="openEdit(i)">编辑</button>
            <button class="btn sm ghost" @click="remove(i)">删除</button>
          </td>
        </tr>
        <tr v-if="!local.length"><td colspan="6" class="muted">还没有规则，点右上角「新建规则」</td></tr>
      </tbody>
    </table>
  </div>

  <div v-if="editing !== null" class="modal-mask" @click.self="editing = null">
    <div class="modal" style="width: min(720px, 94vw)">
      <div class="row" style="margin-bottom: 10px">
        <h3 style="margin: 0">规则设置</h3>
        <span style="flex: 1"></span>
        <button class="btn sm ghost" @click="advanced = !advanced">{{ advanced ? '收起高级' : '高级（直接改 JSON）' }}</button>
      </div>

      <div v-if="!advanced" class="col">
        <div class="kv">
          <label>规则名</label>
          <input v-model="form.name" placeholder="例如：大礼物放烟花" />

          <label>什么时候触发</label>
          <select v-model="form.trigger">
            <option v-for="t in triggers" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>

          <template v-if="form.trigger === 'danmaku'">
            <label>说的什么词</label>
            <input v-model="form.keyword" placeholder="例如：点什么（弹幕里包含这两个字就触发）" />
          </template>

          <template v-if="form.trigger === 'gift'">
            <label>指定礼物</label>
            <input v-model="form.giftNames" placeholder="留空 = 任何礼物；多个用、隔开，例如：火箭、嘉年华" />
            <label>礼物价值门槛</label>
            <input v-model.number="form.minValue" type="number" min="0" />
          </template>

          <label>两次触发最短间隔（秒）</label>
          <input v-model.number="form.cooldownSec" type="number" min="0" />
        </div>

        <div class="small muted" style="margin: 14px 0 8px">触发后做什么（可多选，按顺序执行）</div>

        <div class="col">
          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doTts" /><i></i></label>
            <div style="flex: 1">
              <div class="small">语音播报</div>
              <input v-if="form.doTts" v-model="form.ttsText" placeholder="要念的话" />
              <select v-if="form.doTts" style="margin-top: 6px" @change="pickScript($event)">
                <option value="">想不出词？从话术库挑一句</option>
                <option v-for="s in (data.scripts || []).filter((x) => x.category === 'thanks' || x.category === 'keep')" :key="s.id" :value="s.text">
                  {{ s.title }}
                </option>
              </select>
            </div>
          </div>

          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doEffect" /><i></i></label>
            <div style="flex: 1">
              <div class="small">画面特效</div>
              <div v-if="form.doEffect" class="row">
                <select v-model="form.effectPreset" style="max-width: 180px">
                  <option v-for="p in presets" :key="p.id" :value="p.id">{{ p.name }}</option>
                </select>
                <input v-model.number="form.effectMs" type="number" placeholder="持续毫秒" style="max-width: 140px" />
              </div>
            </div>
          </div>

          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doBanner" /><i></i></label>
            <div style="flex: 1">
              <div class="small">大字横幅</div>
              <input v-if="form.doBanner" v-model="form.bannerText" placeholder="横幅文字，可用 {user} {gift} {count}" />
            </div>
          </div>

          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doSound" /><i></i></label>
            <div style="flex: 1">
              <div class="small">音效</div>
              <select v-if="form.doSound" v-model="form.soundFile">
                <option value="">选择音效</option>
                <option v-for="f in sfxOptions" :key="f" :value="f">{{ f }}</option>
              </select>
            </div>
          </div>

          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doLlm" /><i></i></label>
            <div style="flex: 1">
              <div class="small">AI 生成台词（需要先在设置里配好 AI）</div>
              <input v-if="form.doLlm" v-model="form.llmPrompt" placeholder="给 AI 的指令，可用 {text} {user}" />
            </div>
          </div>

          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doHotkey" /><i></i></label>
            <div style="flex: 1">
              <div class="small">模拟按键（整蛊游戏用）</div>
              <input v-if="form.doHotkey" v-model="form.hotkeyKeys" placeholder="例如：space / ctrl+space / f5" />
            </div>
          </div>

          <div class="row">
            <label class="switch"><input type="checkbox" v-model="form.doWebhook" /><i></i></label>
            <div style="flex: 1">
              <div class="small">调用一个网址（控灯、机械臂等硬件）</div>
              <input v-if="form.doWebhook" v-model="form.webhookUrl" placeholder="http://192.168.1.50/flash" />
            </div>
          </div>
        </div>
      </div>

      <div v-else>
        <textarea v-model="editText" rows="20" spellcheck="false" style="font-family: Consolas, monospace"></textarea>
        <div class="hint" style="margin-top: 8px">
          变量：{user} {text} {gift} {count} {value} {total} {level} {guard} {combo} {fanNote} {time}
        </div>
      </div>

      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button class="btn ghost" @click="editing = null">取消</button>
        <button v-if="!advanced" class="btn primary" @click="apply">确定</button>
        <button v-else class="btn primary" @click="applyJson">应用 JSON</button>
      </div>
    </div>
  </div>
</template>

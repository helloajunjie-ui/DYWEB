<script setup>
import { ref, computed } from 'vue'
import { post } from '../ws.js'

const props = defineProps({
  data: { type: Object, required: true },
  traces: { type: Array, default: () => [] },
  llm: { type: Array, default: () => [] }
})

const msg = ref('')
const form = ref({ type: 'danmaku', user: '测试观众', text: '主播好呀', giftName: '小心心', giftCount: 1, giftValue: 1, level: 0, guardLevel: 1 })
const rawJson = ref('')
const ttsText = ref('测试一下，声音正常吗')
const sfxFile = ref('')
const hotkey = ref('ctrl+space')
const llmText = ref('观众问：今天有什么好听的歌？')

const presets = [
  { id: 'fireworks', name: '烟花' },
  { id: 'hearts', name: '爱心雨' },
  { id: 'meteor', name: '流星' },
  { id: 'confetti', name: '彩带' },
  { id: 'shake', name: '抖动' },
  { id: 'spotlight', name: '聚光' },
  { id: 'banner', name: '横幅' }
]

const sfxOptions = computed(() => props.data.sfx || [])

async function simulate() {
  const body = { ...form.value }
  const res = await post('/api/simulate', body)
  msg.value = res.ok ? `已推送：${res.event?.type} / ${res.event?.user}` : res.msg || '失败'
}

async function simulateRaw() {
  try {
    const raw = JSON.parse(rawJson.value)
    const res = await post('/api/simulate', { raw })
    msg.value = res.ok ? `已解析：${res.event?.type} / ${res.event?.user} / ${res.event?.text || res.event?.giftName }` : res.msg || '失败'
  } catch (e) {
    msg.value = 'JSON 解析失败：' + e.message
  }
}

async function effect(preset) {
  const res = await post('/api/test/effect', { preset, text: '测试横幅：感谢测试观众', durationMs: 4000 })
  msg.value = res.ok ? `已触发 ${preset}` : '失败'
}

async function testTts() {
  const res = await post('/api/test/tts', { text: ttsText.value })
  msg.value = res.ok ? `已试念（模式：${res.mode}）` : '失败'
}

async function testSfx() {
  const res = await post('/api/test/sound', { file: sfxFile.value, volume: 1 })
  msg.value = res.ok ? `已播放 ${sfxFile.value}` : res.msg || '失败'
}

async function testHotkey() {
  const res = await post('/api/test/hotkey', { keys: hotkey.value })
  msg.value = res.ok ? res.msg : '失败：' + res.msg
}

async function testLlm() {
  const res = await post('/api/test/llm', { text: llmText.value })
  msg.value = res.ok ? res.msg : res.msg || '失败'
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">不写规则也能验：这里发的事件会走完整链路（归一化 → 规则 → 动作）。</div>
    <span style="flex: 1"></span>
    <span v-if="msg" class="tag warn">{{ msg }}</span>
  </div>

  <div class="grid c2">
    <div class="card">
      <h3>模拟观众事件</h3>
      <div class="kv">
        <label>类型</label>
        <select v-model="form.type">
          <option value="danmaku">弹幕</option>
          <option value="gift">礼物</option>
          <option value="enter">进场</option>
          <option value="follow">关注</option>
          <option value="fansclub">加入粉丝团</option>
          <option value="guard">星守护</option>
          <option value="like">点赞</option>
        </select>
        <label>观众</label>
        <input v-model="form.user" />
        <label v-if="form.type === 'danmaku'">弹幕内容</label>
        <input v-if="form.type === 'danmaku'" v-model="form.text" />
        <template v-if="form.type === 'gift'">
          <label>礼物名</label>
          <input v-model="form.giftName" />
          <label>数量</label>
          <input v-model.number="form.giftCount" type="number" min="1" />
          <label>单价（抖币）</label>
          <input v-model.number="form.giftValue" type="number" min="0" />
        </template>
        <template v-if="form.type === 'guard'">
          <label>守护等级</label>
          <input v-model.number="form.guardLevel" type="number" min="1" max="3" />
        </template>
        <label>观众等级</label>
        <input v-model.number="form.level" type="number" min="0" />
      </div>
      <div class="row" style="margin-top: 12px">
        <button class="btn primary" @click="simulate">发送事件</button>
      </div>
    </div>

    <div class="card">
      <h3>贴原始 JSON（验字段映射）</h3>
      <textarea v-model="rawJson" rows="8" spellcheck="false" style="font-family: Consolas, monospace"
        placeholder='例如：{"Type":2,"Data":{"User":{"NickName":"大哥","Id":"1001"},"Gift":{"Name":"小心心","RepeatCount":3,"DiamondCount":1}}}'></textarea>
      <div class="row" style="margin-top: 10px">
        <button class="btn" @click="simulateRaw">解析并推送</button>
        <a class="btn ghost sm" href="/#settings">字段映射在「设置」里改</a>
      </div>
    </div>
  </div>

  <div class="card" style="margin-top: 14px">
    <h3>单项测试</h3>
    <div class="wrap" style="margin-bottom: 12px">
      <button v-for="p in presets" :key="p.id" class="btn sm" @click="effect(p.id)">{{ p.name }}</button>
      <a class="btn sm ghost" href="/overlay" target="_blank">在舞台层看</a>
    </div>

    <div class="grid c2">
      <div class="col">
        <div class="row">
          <input v-model="ttsText" placeholder="TTS 试念文本" />
          <button class="btn" style="flex: 0 0 auto" @click="testTts">念一下</button>
        </div>
        <div class="row">
          <select v-model="sfxFile">
            <option value="">选择音效</option>
            <option v-for="f in sfxOptions" :key="f" :value="f">{{ f }}</option>
          </select>
          <button class="btn" style="flex: 0 0 auto" @click="testSfx" :disabled="!sfxFile">播放</button>
        </div>
        <div class="row">
          <input v-model="hotkey" placeholder="ctrl+space / f5 / ctrl+shift+a" />
          <button class="btn" style="flex: 0 0 auto" @click="testHotkey">按键</button>
        </div>
      </div>
      <div class="col">
        <div class="row">
          <input v-model="llmText" placeholder="给 AI 的一句话" />
          <button class="btn" style="flex: 0 0 auto" @click="testLlm">问 AI</button>
        </div>
        <div class="log" style="max-height: 180px">
          <div v-if="!llm.length" class="muted">AI 回复会显示在这里（需在设置里开启 LLM）</div>
          <div v-for="(l, i) in llm" :key="i" :class="l.ok ? 'ok' : 'bad'">{{ l.text }}</div>
        </div>
      </div>
    </div>
  </div>

  <div class="card" style="margin-top: 14px">
    <h3>动作日志（实时）</h3>
    <div class="log">
      <div v-if="!traces.length" class="muted">还没有命中记录</div>
      <div v-for="t in traces" :key="t.key" :class="t.ok ? 'ok' : 'bad'">
        [{{ t.rule }}] {{ t.action }} → {{ t.msg }}
      </div>
    </div>
  </div>
</template>

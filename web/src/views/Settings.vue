<script setup>
import { ref, onMounted, watch } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])

const cfg = ref(JSON.parse(JSON.stringify(props.data.config || {})))
const mappingText = ref(JSON.stringify(props.data.config?.mapping || {}, null, 2))
const aliasText = ref(JSON.stringify(props.data.config?.typeAliases || {}, null, 2))
const msg = ref('')
const modelList = ref([])
const aiMsg = ref('')
const busy = ref(false)
const showAdvanced = ref(false)
const showSystem = ref(false)

// 一次只显示一件事：先选要调什么，再出对应配置
const tab = ref('ai')

const providers = [
  { id: 'deepseek', name: 'DeepSeek', base: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
  { id: 'ark', name: '火山方舟', base: 'https://ark.cn-beijing.volces.com/api/v3', model: '' },
  { id: 'dashscope', name: '阿里通义', base: 'https://dashscope.aliyuncs.com/compatible-mode/v1', model: 'qwen-plus' },
  { id: 'ollama', name: '本机 Ollama', base: 'http://127.0.0.1:11434/v1', model: 'qwen2.5' }
]
const activeProvider = ref('')
const providerName = () => (providers.find((p) => p.id === activeProvider.value) || {}).name || ''

const modelLabels = {
  'deepseek-chat': 'DeepSeek 通用对话',
  'deepseek-reasoner': 'DeepSeek 深度推理',
  'qwen-plus': '通义千问 Plus',
  'qwen-turbo': '通义千问 Turbo',
  'qwen-max': '通义千问 Max',
  'glm-4-flash': '智谱 GLM-4 Flash',
  'glm-4': '智谱 GLM-4',
  'moonshot-v1-8k': 'Kimi 8K'
}
const modelLabel = (id) => modelLabels[id] || id

function useProvider(p) {
  activeProvider.value = p.id
  cfg.value.llm.baseUrl = p.base
  if (p.model) cfg.value.llm.model = p.model
  aiMsg.value = '已填好 ' + p.name + ' 的地址，粘贴你的 Key 后点「自动获取模型」'
  modelList.value = []
}

async function fetchModels() {
  busy.value = true
  aiMsg.value = '正在向服务商要模型列表…'
  const res = await post('/api/llm/models', {
    baseUrl: cfg.value.llm.baseUrl,
    apiKey: cfg.value.llm.apiKey
  })
  busy.value = false
  if (res.ok) {
    modelList.value = res.models || []
    if (!cfg.value.llm.model || !modelList.value.includes(cfg.value.llm.model)) {
      cfg.value.llm.model = modelList.value[0] || ''
    }
    aiMsg.value = `拿到了 ${modelList.value.length} 个模型，已自动选好第一个，点「测试连接」验一下`
  } else {
    modelList.value = []
    aiMsg.value = '没拿到列表：' + (res.error || '未知原因') + (res.suggestions?.length ? `｜可手填：${res.suggestions.join(' / ')}` : '')
  }
}

async function probe() {
  busy.value = true
  aiMsg.value = '正在试发一句话…'
  const res = await post('/api/llm/probe', {
    baseUrl: cfg.value.llm.baseUrl,
    apiKey: cfg.value.llm.apiKey,
    model: cfg.value.llm.model
  })
  busy.value = false
  aiMsg.value = res.ok ? `通了！模型回你：${res.reply}` : '不通：' + (res.error || '未知原因')
}

onMounted(() => {
  if (!cfg.value.llm) cfg.value.llm = {}
  if (!cfg.value.tts) cfg.value.tts = {}
  if (!cfg.value.tts.rate) cfg.value.tts.rate = 0
  if (!cfg.value.tts.volume) cfg.value.tts.volume = 90
  if (cfg.value.llm.baseUrl && cfg.value.llm.apiKey) {
    tab.value = 'ai'
    fetchModels()
  } else if (cfg.value.tts && cfg.value.tts.enabled) {
    tab.value = 'tts'
  }
})

let saveTimer = null
async function save() {
  try {
    cfg.value.mapping = JSON.parse(mappingText.value)
    cfg.value.typeAliases = JSON.parse(aliasText.value)
  } catch (e) {
    msg.value = '字段映射 JSON 还没写完整，先不同步：' + e.message
    return
  }
  const res = await post('/api/config', cfg.value)
  msg.value = res.ok ? '已自动保存' : '保存失败：' + (res.msg || '')
  setTimeout(() => {
    if (msg.value === '已自动保存') msg.value = ''
  }, 1600)
}

watch(
  [cfg, mappingText, aliasText],
  () => {
    clearTimeout(saveTimer)
    saveTimer = setTimeout(save, 1200)
  },
  { deep: true }
)

function setTtsMode(m) {
  cfg.value.tts.mode = m
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">先选你要改的那一件事，下面的配置就只显示这一块。改完自动保存，不用点按钮。</div>
    <span style="flex: 1"></span>
    <span v-if="msg" class="tag warn">{{ msg }}</span>
  </div>

  <div class="tabs">
    <button class="tab" :class="{ on: tab === 'ai' }" @click="tab = 'ai'">🤖 让 AI 写台词</button>
    <button class="tab" :class="{ on: tab === 'tts' }" @click="tab = 'tts'">🔊 语音播报</button>
    <button class="tab" :class="{ on: tab === 'basic' }" @click="tab = 'basic'">🧰 基础与对接</button>
  </div>

  <!-- AI 话术 -->
  <div v-if="tab === 'ai'" class="card">
    <h3>让 AI 帮你写台词</h3>
    <p class="small muted" style="margin: 4px 0 16px">
      三步：选服务商 → 粘 Key → 点自动获取模型。模型名不用自己打。
    </p>

    <div class="step">
      <b>1. 你用的是哪家</b>
      <div class="chips">
        <button
          v-for="p in providers"
          :key="p.id"
          class="chip"
          :class="{ on: activeProvider === p.id }"
          @click="useProvider(p)"
        >{{ p.name }}</button>
      </div>
    </div>

    <div class="step">
      <b>2. 把 Key 粘进来</b>
      <input v-model="cfg.llm.apiKey" placeholder="sk-...（粘一次就存在本机，下次不用再粘）" />
      <div class="hint" style="margin-top: 6px">Ollama 是本机跑的，这格可以留空。</div>
    </div>

    <div class="step">
      <b>3. 让系统自己认模型</b>
      <div class="row" style="margin: 8px 0">
        <button class="btn" :disabled="busy" @click="fetchModels">自动获取模型</button>
        <button class="btn" :disabled="busy" @click="probe">测试连接</button>
        <select v-if="modelList.length" v-model="cfg.llm.model" style="max-width: 260px">
          <option v-for="m in modelList" :key="m" :value="m">{{ modelLabel(m) }}</option>
        </select>
        <span v-else class="small muted">{{ cfg.llm.model || '还没有模型，先点左边那个按钮' }}</span>
      </div>
      <div class="small" :class="aiMsg.startsWith('通了') ? 'ok' : 'muted'">{{ aiMsg }}</div>
    </div>

    <div class="step row" style="align-items: center">
      <b style="flex: 1">启用 AI 话术（规则里的 AI 动作才会跑）</b>
      <label class="switch"><input type="checkbox" v-model="cfg.llm.enabled" /><i></i></label>
    </div>

    <div style="margin-top: 14px">
      <button class="btn sm ghost" @click="showSystem = !showSystem">
        {{ showSystem ? '收起人设' : 'AI 的人设与红线（新手不用改）' }}
      </button>
      <div v-if="showSystem" style="margin-top: 8px">
        <textarea v-model="cfg.llm.system" rows="5"></textarea>
      </div>
    </div>
  </div>

  <!-- 语音播报 -->
  <div v-if="tab === 'tts'" class="card">
    <div class="row" style="align-items: center">
      <div style="flex: 1">
        <h3 style="margin: 0">语音播报</h3>
        <p class="small muted" style="margin: 4px 0 0">AI 负责写词，这里负责嗓子，两者不冲突；关掉后 AI 只出字不出声。</p>
      </div>
      <label class="switch big"><input type="checkbox" v-model="cfg.tts.enabled" /><i></i></label>
    </div>

    <div v-if="cfg.tts.enabled" style="margin-top: 18px">
      <div class="step">
        <b>用哪个嗓子</b>
        <div class="chips">
          <button class="chip" :class="{ on: cfg.tts.mode === 'local' }" @click="setTtsMode('local')">本机声音（离线可用）</button>
          <button class="chip" :class="{ on: cfg.tts.mode === 'browser' }" @click="setTtsMode('browser')">网页声音（跟随浏览器）</button>
          <button class="chip" :class="{ on: cfg.tts.mode === 'both' }" @click="setTtsMode('both')">两个都放</button>
        </div>
      </div>

      <div v-if="cfg.tts.mode !== 'browser'" class="step">
        <b>音色</b>
        <select v-model="cfg.tts.voice" style="max-width: 320px">
          <option value="">系统默认</option>
          <option v-for="v in data.voices || []" :key="v" :value="v">{{ v }}</option>
        </select>
      </div>

      <div class="step">
        <b>语速：{{ cfg.tts.rate > 0 ? '偏快' : cfg.tts.rate < 0 ? '偏慢' : '正常' }}</b>
        <input v-model.number="cfg.tts.rate" type="range" min="-10" max="10" step="1" />
      </div>

      <div class="step">
        <b>音量：{{ cfg.tts.volume }}</b>
        <input v-model.number="cfg.tts.volume" type="range" min="0" max="100" step="5" />
      </div>
    </div>
  </div>

  <!-- 基础与对接 -->
  <div v-if="tab === 'basic'" class="card">
    <h3>基础与对接</h3>
    <p class="small muted" style="margin: 4px 0 16px">这一页平时不用动，改端口或对接第三方工具时才看。</p>

    <div class="step">
      <b>监听端口</b>
      <input v-model.number="cfg.port" type="number" style="max-width: 180px" />
      <div class="hint" style="margin-top: 6px">默认 9101，改了要重启 exe 才生效；改之前先确认没被别的软件占用。</div>
    </div>

    <div class="step">
      <b>给别的软件推事件用这两个地址</b>
      <div class="hint">
        推送：<code>http://127.0.0.1:{{ cfg.port }}/push</code><br />
        长连接：<code>ws://127.0.0.1:{{ cfg.port }}/ws/ingest</code>
      </div>
    </div>

    <div class="step">
      <b>音效目录</b>
      <input v-model="cfg.sfxDir" />
      <div class="hint" style="margin-top: 6px">放手写音效的文件夹，默认就是程序旁边的 sfx。</div>
    </div>

    <div class="step">
      <b>横幅默认显示时长（毫秒）</b>
      <input v-model.number="cfg.padDurationMs" type="number" style="max-width: 180px" />
      <div class="hint" style="margin-top: 6px">4000 = 4 秒。</div>
    </div>

    <div style="margin-top: 16px">
      <button class="btn sm ghost" @click="showAdvanced = !showAdvanced">
        {{ showAdvanced ? '收起字段映射' : '对接第三方工具时，字段对不上才动这里' }}
      </button>
      <div v-if="showAdvanced">
        <div class="hint" style="margin: 10px 0 8px">
          左边是系统认的字段，右边是候选路径（点号取嵌套，命中即用）。这是 JSON，改错会保存失败。
        </div>
        <div class="grid c2">
          <div>
            <div class="small muted" style="margin-bottom: 6px">mapping</div>
            <textarea v-model="mappingText" rows="14" spellcheck="false" style="font-family: Consolas, monospace"></textarea>
          </div>
          <div>
            <div class="small muted" style="margin-bottom: 6px">typeAliases（事件类型别名）</div>
            <textarea v-model="aliasText" rows="14" spellcheck="false" style="font-family: Consolas, monospace"></textarea>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.tab {
  border: 1px solid #262c3a;
  background: rgba(255, 255, 255, 0.03);
  color: #b7bfd2;
  border-radius: 14px;
  padding: 12px 18px;
  font-size: 15px;
  cursor: pointer;
  transition: all 0.16s;
}
.tab:hover {
  border-color: #3a465c;
  color: #fff;
}
.tab.on {
  background: rgba(255, 210, 74, 0.14);
  border-color: #ffd24a;
  color: #ffd24a;
  font-weight: 700;
}
.step {
  padding: 14px 0;
  border-top: 1px solid #202634;
}
.step:first-of-type {
  border-top: none;
}
.step b {
  display: block;
  margin-bottom: 8px;
  font-size: 15px;
  color: #e8ecf6;
}
.step input[type='range'] {
  width: 100%;
  max-width: 420px;
  accent-color: #ffd24a;
}
.chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.chip {
  border: 1px solid #2c3546;
  background: rgba(255, 255, 255, 0.04);
  color: #c7cfdf;
  border-radius: 999px;
  padding: 8px 16px;
  font-size: 14px;
  cursor: pointer;
}
.chip.on {
  border-color: #ffd24a;
  color: #ffd24a;
  background: rgba(255, 210, 74, 0.14);
  font-weight: 700;
}
.switch.big {
  transform: scale(1.25);
  transform-origin: right center;
}
</style>

<script setup>
import { ref, onMounted } from 'vue'
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

const providers = [
  { id: 'deepseek', name: 'DeepSeek', base: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
  { id: 'ark', name: '火山方舟', base: 'https://ark.cn-beijing.volces.com/api/v3', model: '' },
  { id: 'dashscope', name: '阿里通义', base: 'https://dashscope.aliyuncs.com/compatible-mode/v1', model: 'qwen-plus' },
  { id: 'ollama', name: '本机 Ollama', base: 'http://127.0.0.1:11434/v1', model: 'qwen2.5' }
]

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
    aiMsg.value = `拿到了 ${modelList.value.length} 个模型，已自动选好第一个，可以点「测试连接」验一下`
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
  // 已经存过 Key 的，进页面就自动拿一次模型列表
  if (cfg.value.llm.baseUrl && cfg.value.llm.apiKey) fetchModels()
})

async function save() {
  try {
    cfg.value.mapping = JSON.parse(mappingText.value)
    cfg.value.typeAliases = JSON.parse(aliasText.value)
  } catch (e) {
    msg.value = 'JSON 解析失败：' + e.message
    return
  }
  const res = await post('/api/config', cfg.value)
  msg.value = res.ok ? res.msg : '保存失败：' + (res.msg || '')
  if (res.ok) emit('reload')
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">改端口或字段映射需要重启 exe 才生效；TTS / LLM 保存后立即生效。</div>
    <span style="flex: 1"></span>
    <span v-if="msg" class="tag warn">{{ msg }}</span>
    <button class="btn primary" @click="save">保存设置</button>
  </div>

  <div class="grid c2">
    <div class="card">
      <h3>基础</h3>
      <div class="kv">
        <label>监听端口</label>
        <input v-model.number="cfg.port" type="number" />
        <label>横幅默认时长</label>
        <input v-model.number="cfg.padDurationMs" type="number" />
        <label>音效目录</label>
        <input v-model="cfg.sfxDir" />
      </div>
      <div class="hint" style="margin-top: 10px">
        推送地址：<code>http://127.0.0.1:{{ cfg.port }}/push</code>　WebSocket：<code>ws://127.0.0.1:{{ cfg.port }}/ws/ingest</code>
      </div>
    </div>

    <div class="card">
      <h3>TTS 播报</h3>
      <div class="kv">
        <label>启用</label>
        <label class="switch"><input type="checkbox" v-model="cfg.tts.enabled" /><i></i></label>
        <label>模式</label>
        <select v-model="cfg.tts.mode">
          <option value="local">本机 SAPI（离线）</option>
          <option value="browser">页面朗读</option>
          <option value="both">两者都念</option>
        </select>
        <label>音色</label>
        <select v-model="cfg.tts.voice">
          <option value="">系统默认</option>
          <option v-for="v in data.voices || []" :key="v" :value="v">{{ v }}</option>
        </select>
        <label>语速 (-10~10)</label>
        <input v-model.number="cfg.tts.rate" type="number" min="-10" max="10" />
        <label>音量 (0~100)</label>
        <input v-model.number="cfg.tts.volume" type="number" min="0" max="100" />
      </div>
    </div>
  </div>

  <div class="card" style="margin-top: 14px">
    <h3>AI 话术</h3>
    <div class="row" style="margin-bottom: 10px">
      <span class="small muted">先点服务商，再粘 Key，剩下的交给系统：</span>
    </div>
    <div class="wrap" style="margin-bottom: 14px">
      <button v-for="p in providers" :key="p.id" class="btn sm" @click="useProvider(p)">{{ p.name }}</button>
    </div>
    <div class="kv">
      <label>启用</label>
      <label class="switch"><input type="checkbox" v-model="cfg.llm.enabled" /><i></i></label>
      <label>API Key</label>
      <input v-model="cfg.llm.apiKey" placeholder="sk-...（粘贴到这里，Ollama 可留空）" />
      <label>地址</label>
      <input v-model="cfg.llm.baseUrl" placeholder="点上面的服务商就会自动填好" />
      <label>模型</label>
      <select v-if="modelList.length" v-model="cfg.llm.model">
        <option v-for="m in modelList" :key="m" :value="m">{{ modelLabel(m) }}</option>
      </select>
      <input v-else v-model="cfg.llm.model" placeholder="点「自动获取模型」后这里会变成下拉框" />
      <label>超时(ms)</label>
      <input v-model.number="cfg.llm.timeoutMs" type="number" />
    </div>
    <div class="row" style="margin-top: 12px">
      <button class="btn" :disabled="busy" @click="fetchModels">自动获取模型</button>
      <button class="btn" :disabled="busy" @click="probe">测试连接</button>
      <span class="small" :class="aiMsg.startsWith('通了') ? 'ok' : 'muted'">{{ aiMsg }}</span>
    </div>
    <div class="hint" style="margin-top: 10px">
      填好这三个之后，点右上角「保存设置」；之后规则里的 AI 动作就会用它来生成台词。
    </div>
    <div style="margin-top: 12px">
      <div class="small muted" style="margin-bottom: 6px">人设与红线（System Prompt，新手不用改）</div>
      <textarea v-model="cfg.llm.system" rows="4"></textarea>
    </div>
  </div>

  <div class="card" style="margin-top: 14px">
    <div class="row">
      <h3 style="margin: 0">字段映射 · 类型别名</h3>
      <span style="flex: 1"></span>
      <button class="btn sm ghost" @click="showAdvanced = !showAdvanced">
        {{ showAdvanced ? '收起' : '展开（第三方工具推的数据对不上时才动）' }}
      </button>
    </div>
    <div v-if="showAdvanced">
      <div class="hint" style="margin: 10px 0 8px">
        左边是我们认的字段，右边是候选路径（点号取嵌套，命中即用）。注意这是 JSON，改错会保存失败。
      </div>
      <div class="grid c2">
        <div>
          <div class="small muted" style="margin-bottom: 6px">mapping</div>
          <textarea v-model="mappingText" rows="16" spellcheck="false" style="font-family: Consolas, monospace"></textarea>
        </div>
        <div>
          <div class="small muted" style="margin-bottom: 6px">typeAliases（事件类型别名）</div>
          <textarea v-model="aliasText" rows="16" spellcheck="false" style="font-family: Consolas, monospace"></textarea>
        </div>
      </div>
    </div>
  </div>
</template>

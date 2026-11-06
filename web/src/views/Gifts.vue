<script setup>
import { ref, computed } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])

const local = ref(JSON.parse(JSON.stringify(props.data.gifts || [])))
const q = ref('')
const message = ref('')
const importing = ref(false)
const importText = ref('')

const filtered = computed(() => {
  const k = q.value.trim().toLowerCase()
  if (!k) return local.value
  return local.value.filter((g) => g.name.toLowerCase().includes(k) || String(g.value).includes(k))
})

const totalValue = computed(() => local.value.reduce((s, g) => s + (Number(g.value) || 0), 0))

function addRow() {
  local.value.unshift({ name: '', value: 0 })
}

function removeRow(i) {
  local.value.splice(i, 1)
}

async function save() {
  const clean = local.value.filter((g) => (g.name || '').trim())
  const res = await post('/api/gifts', { gifts: clean })
  message.value = res.ok ? `已保存 ${res.count} 个礼物价` : '保存失败：' + (res.msg || res.error || '')
  if (res.ok) {
    local.value = clean
    emit('reload')
  }
}

function parseImport() {
  let added = 0
  for (const line of importText.value.split(/\n/)) {
    const t = line.trim()
    if (!t) continue
    const m = t.match(/^(.+?)\s+(\d+(?:\.\d+)?)\s*(?:钻石|币)?$/)
    if (!m) continue
    const name = m[1].replace(/\s+/g, '')
    const value = Number(m[2])
    const hit = local.value.find((g) => g.name === name)
    if (hit) {
      hit.value = value
    } else {
      local.value.push({ name, value })
    }
    added++
  }
  local.value.sort((a, b) => a.value - b.value)
  message.value = `已解析 ${added} 行，记得点「保存全部」`
  importing.value = false
  importText.value = ''
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">
      礼物价目表：第三方工具只推「礼物名 + 数量」时，系统按这张表算出抖币价值，规则里的「≥300币」才准。
      单价口径为钻石（与抖币 1:1）。共 {{ local.length }} 个礼物。
    </div>
    <span style="flex: 1"></span>
    <span v-if="message" class="tag warn">{{ message }}</span>
    <button class="btn" @click="importing = true">批量粘贴导入</button>
    <button class="btn" @click="addRow">新增</button>
    <button class="btn primary" @click="save">保存全部</button>
  </div>

  <div class="row" style="margin-bottom: 10px">
    <input v-model="q" placeholder="搜索礼物名或单价，例如 嘉年华 / 99" style="max-width: 320px" />
    <span class="small muted">显示 {{ filtered.length }} 条 · 价目合计 {{ totalValue }}</span>
  </div>

  <div class="card" style="padding: 0; max-height: calc(100vh - 260px); overflow: auto">
    <table>
      <thead>
        <tr>
          <th style="width: 220px">礼物</th>
          <th style="width: 120px">单价（钻石）</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="g in filtered" :key="g.name + g.value">
          <td><input v-model="g.name" style="width: 100%" /></td>
          <td><input v-model.number="g.value" type="number" min="0" style="width: 100%" /></td>
          <td style="width: 80px">
            <button class="btn sm ghost" @click="removeRow(local.indexOf(g))">删除</button>
          </td>
        </tr>
        <tr v-if="!filtered.length"><td colspan="3" class="muted">没有匹配的礼物</td></tr>
      </tbody>
    </table>
  </div>

  <div v-if="importing" class="modal-mask" @click.self="importing = false">
    <div class="modal">
      <h3>批量粘贴导入</h3>
      <div class="hint" style="margin-bottom: 8px">
        一行一个：「礼物名 单价」，直接粘抖音的价目表也行（带「钻石」后缀会自动去掉）。
      </div>
      <textarea v-model="importText" rows="14" spellcheck="false" placeholder="嘉年华 30000&#10;跑车 1200&#10;小心心 1"></textarea>
      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button class="btn ghost" @click="importing = false">取消</button>
        <button class="btn primary" @click="parseImport">解析并加入</button>
      </div>
    </div>
  </div>
</template>

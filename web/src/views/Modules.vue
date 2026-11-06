<script setup>
import { ref, computed } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])
const toast = ref('')

const grouped = computed(() => {
  const cats = props.data.categories || []
  return cats.map((c) => ({
    ...c,
    items: (props.data.modules || []).filter((m) => m.category === c.id)
  })).filter((c) => c.items.length)
})

const recIds = computed(() => (props.data.modes || []).find((m) => m.id === props.data.mode)?.rec || [])
const isRec = (id) => recIds.value.includes(id)

async function toggle(m) {
  const res = await post('/api/modules/toggle', { id: m.id, enabled: !m.enabled })
  if (res.ok) {
    m.enabled = !m.enabled
    toast.value = `${m.name} 已${m.enabled ? '开启' : '关闭'}`
  } else {
    toast.value = res.msg || '操作失败'
  }
}

async function test(m) {
  toast.value = '测试中…'
  const res = await post('/api/modules/test', { id: m.id })
  toast.value = res.ok ? `${m.name}：${res.msg}` : `${m.name} 失败：${res.msg}`
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">按分类开关能力。带「本模式推荐」标签的，是当前直播定位最该开的。</div>
    <span style="flex: 1"></span>
    <span v-if="toast" class="tag warn">{{ toast }}</span>
  </div>

  <div class="grid c2">
    <div v-for="cat in grouped" :key="cat.id" class="card">
      <h3>{{ cat.name }}</h3>
      <div v-for="m in cat.items" :key="m.id" class="row" style="padding: 8px 0; border-bottom: 1px solid var(--line)">
        <label class="switch">
          <input type="checkbox" :checked="m.enabled" :disabled="!m.ready" @change="toggle(m)" />
          <i></i>
        </label>
        <div class="col" style="gap: 2px; flex: 1">
          <div class="row" style="gap: 8px; flex-wrap: wrap">
            <strong style="font-weight: 600">{{ m.name }}</strong>
            <span v-if="!m.ready" class="tag" style="white-space: nowrap; flex-shrink: 0">路线图</span>
            <span v-else class="tag ok" style="white-space: nowrap; flex-shrink: 0">可用</span>
            <span v-if="isRec(m.id)" class="tag warn" style="white-space: nowrap; flex-shrink: 0">本模式推荐</span>
          </div>
          <div class="small muted">{{ m.desc }}</div>
        </div>
        <button v-if="m.testable && m.enabled" class="btn sm ghost" @click="test(m)">试一下</button>
      </div>
    </div>
  </div>
</template>

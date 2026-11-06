<script setup>
import { ref, computed } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])
const toast = ref('')

const recIds = computed(() => (props.data.modes || []).find((m) => m.id === props.data.mode)?.rec || [])
const isRec = (id) => recIds.value.includes(id)

// 每个分类默认只显示"能用的"，路线图收起来，别占着版面吓人
const openRoad = ref({})

const grouped = computed(() => {
  const cats = props.data.categories || []
  return cats
    .map((c) => {
      const all = (props.data.modules || []).filter((m) => m.category === c.id)
      const ready = all
        .filter((m) => m.ready)
        .sort((a, b) => Number(isRec(b.id)) - Number(isRec(a.id)) || Number(b.enabled) - Number(a.enabled))
      const road = all.filter((m) => !m.ready)
      return { ...c, ready, road, total: all.length }
    })
    .filter((c) => c.total)
})

const onCount = computed(() => (props.data.modules || []).filter((m) => m.enabled).length)
const readyCount = computed(() => (props.data.modules || []).filter((m) => m.ready).length)

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
  <div class="row" style="margin-bottom: 10px; flex-wrap: wrap; gap: 10px">
    <div class="hint" style="margin: 0">
      直播间的功能开关，按「哪一步用得上」分成了 7 组；整组看不懂的可以先不管。
    </div>
    <span style="flex: 1"></span>
    <span class="tag ok">已开 {{ onCount }} 个</span>
    <span class="tag">可用共 {{ readyCount }} 个</span>
    <span v-if="toast" class="tag warn">{{ toast }}</span>
  </div>

  <div class="grid c2">
    <div v-for="cat in grouped" :key="cat.id" class="card">
      <h3 style="margin-bottom: 2px">{{ cat.name }}</h3>
      <div class="small muted" style="margin-bottom: 10px">{{ cat.desc }}</div>

      <div v-if="!cat.ready.length" class="small muted" style="padding: 6px 0 2px">
        这个分组暂时没有能开的开关。
      </div>

      <div
        v-for="m in cat.ready"
        :key="m.id"
        class="row mrow"
        :class="{ rec: isRec(m.id) }"
      >
        <label class="switch">
          <input type="checkbox" :checked="m.enabled" @change="toggle(m)" />
          <i></i>
        </label>
        <div class="col" style="gap: 2px; flex: 1">
          <div class="row" style="gap: 8px; flex-wrap: wrap">
            <strong style="font-weight: 600">{{ m.name }}</strong>
            <span v-if="isRec(m.id)" class="tag warn" style="white-space: nowrap">建议开</span>
          </div>
          <div class="small muted">{{ m.desc }}</div>
        </div>
        <button v-if="m.testable && m.enabled" class="btn sm ghost" @click="test(m)">试一下</button>
      </div>

      <div v-if="cat.road.length" style="margin-top: 8px">
        <button class="btn sm ghost" @click="openRoad[cat.id] = !openRoad[cat.id]">
          {{ openRoad[cat.id] ? '收起' : `还有 ${cat.road.length} 个没上线` }}
        </button>
        <div v-if="openRoad[cat.id]" style="margin-top: 6px">
          <div v-for="m in cat.road" :key="m.id" class="row mrow off">
            <label class="switch" style="opacity: 0.5">
              <input type="checkbox" disabled />
              <i></i>
            </label>
            <div class="col" style="gap: 2px; flex: 1">
              <div class="row" style="gap: 8px; flex-wrap: wrap">
                <strong style="font-weight: 600; color: #8b93a7">{{ m.name }}</strong>
                <span class="tag" style="white-space: nowrap">没上线</span>
              </div>
              <div class="small muted">{{ m.desc }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mrow {
  padding: 9px 0;
  border-bottom: 1px solid var(--line);
  align-items: flex-start;
}
.mrow:last-child {
  border-bottom: none;
}
.mrow.rec {
  background: linear-gradient(90deg, rgba(255, 210, 74, 0.07), transparent 70%);
  border-radius: 8px;
  padding-left: 6px;
  margin-left: -6px;
}
.mrow.off {
  opacity: 0.75;
}
</style>

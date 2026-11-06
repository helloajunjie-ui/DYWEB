<script setup>
import { ref, computed, watch } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])

const local = ref(JSON.parse(JSON.stringify(props.data.products || [])))
const editing = ref(null)
const msg = ref('')
const pitch = ref('')
const busy = ref(false)

const blank = () => ({
  id: '',
  name: '',
  price: 0,
  origin: 0,
  spec: '',
  points: '',
  suitable: '',
  avoid: '',
  faq: '',
  script: '',
  active: false
})
const form = ref(blank())

const activeName = computed(() => local.value.find((p) => p.active)?.name || '未设置')

function add() {
  form.value = blank()
  local.value.push({ ...form.value })
  editing.value = local.value.length - 1
}

function edit(i) {
  form.value = JSON.parse(JSON.stringify(local.value[i]))
  editing.value = i
}

function apply() {
  if (!form.value.name.trim()) {
    msg.value = '商品名不能空'
    return
  }
  const rec = { ...form.value }
  if (rec.active) local.value.forEach((p) => (p.active = false))
  local.value[editing.value] = rec
  editing.value = null
  msg.value = ''
}

function remove(i) {
  local.value.splice(i, 1)
  editing.value = null
}

let saveTimer = null
async function save() {
  const res = await post('/api/products', { products: local.value })
  msg.value = res.ok ? `已自动保存 ${res.count} 个商品` : '保存失败：' + (res.error || '')
}

watch(
  local,
  () => {
    clearTimeout(saveTimer)
    saveTimer = setTimeout(save, 800)
  },
  { deep: true }
)

async function makePitch(p) {
  busy.value = true
  pitch.value = 'AI 正在看商品资料…'
  const res = await post('/api/products/pitch', { id: p.id, style: '' })
  busy.value = false
  pitch.value = res.ok ? res.text : '没成功：' + (res.error || '')
}

async function sayPitch() {
  if (!pitch.value) return
  await post('/api/scripts/say', { text: pitch.value, user: '直播间的朋友' })
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">把商品资料录进来，AI 才知道怎么说、跟谁不能说。当前主推：{{ activeName }}</div>
    <span style="flex: 1"></span>
    <span v-if="msg" class="tag warn">{{ msg }}</span>
    <button class="btn" @click="add">新增商品</button>
  </div>

  <div class="card" style="padding: 0">
    <table>
      <thead>
        <tr>
          <th style="width: 60px">主推</th>
          <th>商品</th>
          <th style="width: 90px">现价</th>
          <th style="width: 90px">原价</th>
          <th>适合 / 不适合</th>
          <th style="width: 160px"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(p, i) in local" :key="p.id + i">
          <td>
            <label class="switch">
              <input type="checkbox" :checked="p.active" @change="((p.active = !p.active), p.active && local.forEach((x) => (x.id !== p.id ? (x.active = false) : null)))" />
              <i></i>
            </label>
          </td>
          <td>
            {{ p.name || '（没名字）' }}
            <div class="small muted">{{ p.spec }}</div>
          </td>
          <td>{{ p.price || '-' }}</td>
          <td class="muted">{{ p.origin || '-' }}</td>
          <td class="small">{{ p.suitable || '—' }}<div class="small muted">{{ p.avoid }}</div></td>
          <td>
            <button class="btn sm ghost" @click="edit(i)">编辑</button>
            <button class="btn sm" :disabled="busy" @click="makePitch(p)">AI 推荐话术</button>
            <button class="btn sm ghost" @click="remove(i)">删</button>
          </td>
        </tr>
        <tr v-if="!local.length"><td colspan="6" class="muted">还没有商品。点「新增商品」把名字、价格、卖点填进去。</td></tr>
      </tbody>
    </table>
  </div>

  <div v-if="pitch" class="card" style="margin-top: 14px">
    <h3>AI 生成的推荐话术</h3>
    <div style="line-height: 1.8">{{ pitch }}</div>
    <div class="row" style="margin-top: 10px">
      <button class="btn sm" @click="sayPitch">念这句</button>
    </div>
  </div>

  <div v-if="editing !== null" class="modal-mask" @click.self="editing = null">
    <div class="modal" style="width: min(760px, 94vw)">
      <h3 style="margin-top: 0">商品资料</h3>
      <div class="kv">
        <label>商品名</label>
        <input v-model="form.name" placeholder="比如：纯棉短袖 T 恤" />
        <label>现价（元）</label>
        <input v-model.number="form.price" type="number" min="0" />
        <label>原价（元）</label>
        <input v-model.number="form.origin" type="number" min="0" placeholder="没有可留 0" />
        <label>规格</label>
        <input v-model="form.spec" placeholder="均码 / 500ml / 2 件装" />
      </div>
      <div class="small muted" style="margin: 12px 0 6px">卖点（一行一条，AI 会挑着讲）</div>
      <textarea v-model="form.points" rows="4" placeholder="100% 纯棉不闷汗&#10;洗 50 次不起球&#10;今天买送运费险"></textarea>
      <div class="grid c2" style="margin-top: 12px">
        <div>
          <div class="small muted" style="margin-bottom: 6px">适合谁</div>
          <textarea v-model="form.suitable" rows="3" placeholder="上班族、怕闷汗的人"></textarea>
        </div>
        <div>
          <div class="small muted" style="margin-bottom: 6px">不适合谁（帮观众避坑，降退货）</div>
          <textarea v-model="form.avoid" rows="3" placeholder="要收腰版型的别拍，这款是宽松的"></textarea>
        </div>
      </div>
      <div class="small muted" style="margin: 12px 0 6px">常见问题（一行一条）</div>
      <textarea v-model="form.faq" rows="3" placeholder="会不会掉色？&#10;多久发货？"></textarea>
      <div class="small muted" style="margin: 12px 0 6px">我自己的话术（可选）</div>
      <textarea v-model="form.script" rows="3" placeholder="你自己顺口的那几句"></textarea>
      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button class="btn ghost" @click="editing = null">取消</button>
        <button class="btn primary" @click="apply">确定</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { post } from '../ws.js'

const props = defineProps({ data: { type: Object, required: true } })
const emit = defineEmits(['reload'])

const editing = ref(null)
const note = ref('')
const tags = ref('')
const message = ref('')

const fmt = (ts) => (ts ? new Date(ts).toLocaleString('zh-CN', { hour12: false }) : '—')

const q = ref('')
const onlyNote = ref(false)
const list = computed(() => {
  const arr = props.data.fans || []
  return arr.filter((f) => {
    if (onlyNote.value && !(f.note || (f.notes || []).length)) return false
    if (!q.value.trim()) return true
    const s = q.value.trim().toLowerCase()
    return (
      (f.name || '').toLowerCase().includes(s) ||
      (f.note || '').toLowerCase().includes(s) ||
      (f.tags || []).join(',').toLowerCase().includes(s)
    )
  })
})

function open(f) {
  editing.value = f
  note.value = ''
  tags.value = (f.tags || []).join(', ')
}

const historyNotes = (f) => (f.notes || []).slice().reverse()

async function save() {
  const res = await post('/api/fans/note', {
    key: editing.value.key,
    note: note.value,
    tags: tags.value.split(',').map((s) => s.trim()).filter(Boolean)
  })
  message.value = res.ok ? '备注已保存' : res.msg || '保存失败'
  if (res.ok) {
    editing.value = null
    emit('reload')
  }
}
</script>

<template>
  <div class="row" style="margin-bottom: 12px">
    <div class="hint">备注只给自己看，用来筛人（谁值得维护、谁是路人），不会念进播报。</div>
    <span style="flex: 1"></span>
    <span v-if="message" class="tag warn">{{ message }}</span>
  </div>

  <div class="row" style="margin-bottom: 10px">
    <input v-model="q" placeholder="搜名字 / 备注 / 标签" style="width: 220px" />
    <label class="switch"><input type="checkbox" v-model="onlyNote" /><i></i></label>
    <span class="small muted">只看有备注的</span>
    <span style="flex: 1"></span>
    <span class="small muted">共 {{ list.length }} 人</span>
  </div>

  <div class="card" style="padding: 0">
    <table>
      <thead>
        <tr>
          <th>观众</th><th>账号等级</th><th>粉丝团</th><th>互动</th><th>礼物</th><th>价值</th><th>最近</th><th>备注</th><th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="f in list" :key="f.key">
          <td>
            {{ f.name }}
            <span v-if="f.regular" class="tag bad">老粉</span>
            <span v-if="f.guardLevel" class="tag warn">守护{{ f.guardLevel }}</span>
            <span v-if="f.fansClub" class="tag ok">粉丝团</span>
            <span v-if="f.followed" class="tag ok">已关注</span>
          </td>
          <td class="mono">{{ f.level || '—' }}</td>
          <td class="mono">{{ f.fansClubLevel ? 'Lv' + f.fansClubLevel : (f.fansClub ? '已加团' : '—') }}</td>
          <td class="mono">{{ f.msgCount }}</td>
          <td class="mono">{{ f.giftCount }}</td>
          <td class="mono">{{ Math.round(f.giftValue) }}</td>
          <td class="small muted">{{ fmt(f.lastSeen).slice(5) }}</td>
          <td class="small">{{ f.note || '—' }}<span v-if="(f.notes || []).length > 1" class="tag" style="margin-left: 6px">{{ f.notes.length }} 条</span></td>
          <td><button class="btn sm ghost" @click="open(f)">备注</button></td>
        </tr>
        <tr v-if="!list.length"><td colspan="9" class="muted">{{ (props.data.fans || []).length ? '没有符合条件的观众' : '还没有观众数据，先推几条弹幕进来' }}</td></tr>
      </tbody>
    </table>
  </div>

  <div v-if="editing" class="modal-mask" @click.self="editing = null">
    <div class="modal" style="width: min(560px, 92vw)">
      <h3>{{ editing.name }} · 备注</h3>
      <div class="col">
        <div v-if="historyNotes(editing).length">
          <div class="small muted" style="margin-bottom: 6px">以前的备注（越新越上）</div>
          <div class="note-list">
            <div v-for="(n, i) in historyNotes(editing)" :key="i" class="note-item">
              <span class="small muted">{{ fmt(n.ts) }}</span>
              <span>{{ n.text }}</span>
            </div>
          </div>
        </div>
        <div>
          <div class="small muted" style="margin-bottom: 6px">追加一条备注（不会覆盖上面的，会念进播报里）</div>
          <textarea v-model="note" rows="3" placeholder="例如：上周过生日，爱听《晴天》，做设计的"></textarea>
        </div>
        <div>
          <div class="small muted" style="margin-bottom: 6px">标签（逗号分隔）</div>
          <input v-model="tags" placeholder="大哥, 夜猫子, 打工人" />
        </div>
      </div>
      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button class="btn ghost" @click="editing = null">取消</button>
        <button class="btn primary" @click="save">保存</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.note-list {
  max-height: 140px;
  overflow-y: auto;
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 8px 10px;
  margin-bottom: 14px;
  background: rgba(255, 255, 255, 0.03);
}
.note-item {
  display: flex;
  gap: 10px;
  padding: 5px 0;
  font-size: 13px;
  border-bottom: 1px dashed rgba(255, 255, 255, 0.06);
}
.note-item:last-child {
  border-bottom: none;
}
</style>

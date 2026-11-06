<script setup>
const props = defineProps({
  data: { type: Object, required: true },
  events: { type: Array, required: true },
  traces: { type: Array, required: true }
})

const fmtTime = (ts) => (ts ? new Date(ts).toLocaleTimeString('zh-CN', { hour12: false }) : '')
const typeName = (t) => ({ danmaku: '弹幕', gift: '礼物', like: '点赞', enter: '进场', follow: '关注', guard: '星守护', fansclub: '粉丝团' }[t] || t)
const typeClass = (t) => (t === 'gift' || t === 'guard' ? 'warn' : t === 'danmaku' ? '' : 'ok')
const eventText = (e) =>
  e.type === 'gift'
    ? `${e.giftName || '礼物'} ×${e.giftCount}${e.total ? ` · ${e.total}币` : ''}`
    : e.type === 'guard'
      ? `开通星守护${e.guardLevel ? ` · ${e.guardLevel}级` : ''}`
      : e.type === 'fansclub'
        ? '加入粉丝团'
        : e.type === 'follow'
          ? '关注了主播'
          : e.text || (e.type === 'enter' ? '进入直播间' : '')
const gifters = (d) => d.stats?.topGifters || []
const curMode = (d) => (d.modes || []).find((m) => m.id === d.mode) || null
</script>

<template>
  <div v-if="curMode(data)" class="card" style="margin-bottom: 14px">
    <div class="row" style="margin-bottom: 6px">
      <h3 style="margin: 0">当前定位：{{ curMode(data).name }}</h3>
      <span style="flex: 1"></span>
      <span class="small muted">右上角可切换</span>
    </div>
    <div class="small">{{ curMode(data).desc }}</div>
    <div class="small muted" style="margin: 6px 0 8px">重点：{{ curMode(data).focus }}</div>
    <div class="col" style="gap: 4px">
      <div v-for="(t, i) in curMode(data).tips" :key="i" class="small" style="opacity: .85">· {{ t }}</div>
    </div>
  </div>

  <div class="grid c4">
    <div class="stat">
      <div class="k">弹幕</div>
      <div class="v">{{ data.stats.danmaku || 0 }}<small>条</small></div>
    </div>
    <div class="stat">
      <div class="k">礼物</div>
      <div class="v">{{ data.stats.gifts || 0 }}<small>次</small></div>
    </div>
    <div class="stat">
      <div class="k">礼物价值</div>
      <div class="v">{{ Math.round(data.stats.giftValue || 0) }}<small>抖币</small></div>
    </div>
    <div class="stat">
      <div class="k">规则命中</div>
      <div class="v">{{ data.stats.ruleHits || 0 }}<small>次</small></div>
    </div>
  </div>

  <div class="grid c4" style="margin-top: 10px">
    <div class="stat">
      <div class="k">关注</div>
      <div class="v">{{ data.stats.follows || 0 }}<small>位</small></div>
    </div>
    <div class="stat">
      <div class="k">加入粉丝团</div>
      <div class="v">{{ data.stats.fansClubs || 0 }}<small>位</small></div>
    </div>
    <div class="stat">
      <div class="k">星守护</div>
      <div class="v">{{ data.stats.guards || 0 }}<small>位</small></div>
    </div>
    <div class="stat">
      <div class="k">事件总数</div>
      <div class="v">{{ data.stats.totalEvents || 0 }}<small>条</small></div>
    </div>
  </div>

  <div class="grid c2" style="margin-top: 14px">
    <div class="card">
      <h3>实时事件流</h3>
      <div class="feed">
        <div v-if="!events.length" class="hint">还没有事件。打开「调试台」发一条测试弹幕，或让第三方工具 POST 到 /push。</div>
        <div v-for="e in events" :key="e.id" class="item">
          <span class="t">{{ fmtTime(e.ts) }}</span>
          <span class="tag" :class="typeClass(e.type)">{{ typeName(e.type) }}</span>
          <span class="u">{{ e.user || '匿名' }}</span>
          <span class="c">{{ eventText(e) }}</span>
          <span v-if="e.isRegular" class="tag bad">老粉</span>
        </div>
      </div>
    </div>

    <div>
      <div class="card">
        <h3>送礼榜</h3>
        <table>
          <thead><tr><th>观众</th><th>价值</th><th>数量</th></tr></thead>
          <tbody>
            <tr v-if="!gifters(data).length"><td colspan="3" class="muted">暂无</td></tr>
            <tr v-for="g in gifters(data)" :key="g.user">
              <td>{{ g.user }}</td>
              <td class="mono">{{ Math.round(g.value) }}</td>
              <td class="mono">{{ g.count }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="card">
        <h3>动作日志</h3>
        <div class="log">
          <div v-if="!traces.length" class="muted">规则命中后会记录每一步动作的执行结果。</div>
          <div v-for="t in traces" :key="t.key" :class="t.ok ? 'ok' : 'bad'">
            [{{ t.rule }}] {{ t.action }} · {{ t.msg }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

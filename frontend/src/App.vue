<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { OpenDemo } from '../wailsjs/go/main/App'
import {
  buildPlayerStats,
  fileName,
  formatBytes,
  formatDuration,
  formatRecordedAt,
  initials,
  playerInKill,
  shortTeam,
  teamName,
  weaponName,
} from './demo'
import type { DemoData, Kill, PlayerStat } from './types'

type TabID = 'overview' | 'players' | 'kills' | 'rounds' | 'technical'
type KillFilter = 'all' | 'headshot' | 'pov'
type Theme = 'light' | 'dark'

const demo = ref<DemoData | null>(null)
const loading = ref(false)
const errorMessage = ref('')
const activeTab = ref<TabID>('overview')
const killSearch = ref('')
const killFilter = ref<KillFilter>('all')
const theme = ref<Theme>('light')

onMounted(() => {
  const saved = localStorage.getItem('demoscope-theme')
  if (saved === 'light' || saved === 'dark') theme.value = saved
})

const kills = computed(() => demo.value?.kills ?? [])
const rounds = computed(() => demo.value?.rounds ?? [])
const playerStats = computed(() => demo.value ? buildPlayerStats(demo.value) : [])
const isHLTV = computed(() => demo.value?.recording_type === 'hltv')

const finalScore = computed(() => {
  const updates = demo.value?.score_updates ?? []
  const last = updates[updates.length - 1]
  return { ct: last?.ct_score ?? 0, terrorist: last?.terrorist_score ?? 0 }
})

function rankPlayers(players: PlayerStat[]): PlayerStat[] {
  return [...players].sort((a, b) => b.kills - a.kills || a.deaths - b.deaths || a.name.localeCompare(b.name))
}

const ctPlayers = computed(() => rankPlayers(playerStats.value.filter((player) => player.team === 'CT')))
const terroristPlayers = computed(() => rankPlayers(playerStats.value.filter((player) => player.team === 'TERRORIST')))
const spectatorPlayers = computed(() => rankPlayers(playerStats.value.filter((player) => player.team !== 'CT' && player.team !== 'TERRORIST')))

const playerGroups = computed(() => [
  { team: 'TERRORIST', label: 'T', name: 'Terrorists', dotClass: 'bg-orange-500', players: terroristPlayers.value },
  { team: 'CT', label: 'CT', name: 'Counter-Terrorists', dotClass: 'bg-blue-500', players: ctPlayers.value },
  { team: 'SPECTATOR', label: 'SPEC', name: 'Spectators', dotClass: 'bg-slate-400', players: spectatorPlayers.value },
])

const povStat = computed(() => {
  const reference = demo.value?.pov_player
  return playerStats.value.find((player) => player.is_pov)
    ?? playerStats.value.find((player) => reference?.steam_id64 && player.steam_id64 === reference.steam_id64)
    ?? playerStats.value.find((player) => reference?.name && player.name.toLocaleLowerCase() === reference.name.toLocaleLowerCase())
})

const totalHeadshots = computed(() => kills.value.filter((kill) => kill.headshot).length)
const headshotRate = computed(() => kills.value.length ? Math.round(totalHeadshots.value / kills.value.length * 100) : 0)
const topPlayers = computed(() => playerStats.value.slice(0, 10))
const maxKills = computed(() => Math.max(...playerStats.value.map((player) => player.kills), 1))
const recentKills = computed(() => kills.value.slice(-7).reverse())

const tabs = computed(() => [
  { id: 'overview' as const, label: '总览', count: null },
  { id: 'players' as const, label: '玩家统计', count: playerStats.value.length },
  { id: 'kills' as const, label: '击杀记录', count: kills.value.length },
  { id: 'rounds' as const, label: '回合比分', count: rounds.value.length },
  { id: 'technical' as const, label: 'Demo 信息', count: null },
])

const filteredKills = computed(() => {
  const query = killSearch.value.trim().toLocaleLowerCase()
  return kills.value.filter((kill) => {
    if (killFilter.value === 'headshot' && !kill.headshot) return false
    if (killFilter.value === 'pov' && !playerInKill(kill, demo.value?.pov_player)) return false
    if (!query) return true
    return [kill.killer?.name, kill.victim.name, kill.weapon, weaponName(kill.weapon)]
      .filter(Boolean)
      .some((value) => value!.toLocaleLowerCase().includes(query))
  })
})

const matchLabel = computed(() => {
  if (!demo.value) return ''
  if (isHLTV.value) return demo.value.server?.name || demo.value.hltv_proxy?.name || 'HLTV Match Demo'
  return demo.value.pov_player?.name ? `${demo.value.pov_player.name} 的第一视角` : 'Player POV Demo'
})

async function openDemo() {
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await OpenDemo()
    if (result) {
      demo.value = result as unknown as DemoData
      activeTab.value = 'overview'
      killSearch.value = ''
      killFilter.value = 'all'
    }
  } catch (error) {
    errorMessage.value = String(error).replace(/^Error:\s*/, '')
  } finally {
    loading.value = false
  }
}

function toggleTheme() {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  localStorage.setItem('demoscope-theme', theme.value)
}

function teamBadgeClass(team?: string): string {
  if (team === 'CT') return 'team-ct'
  if (team === 'TERRORIST') return 'team-t'
  return 'team-neutral'
}

function teamDotClass(team?: string): string {
  if (team === 'CT') return 'bg-blue-500'
  if (team === 'TERRORIST') return 'bg-orange-500'
  return 'bg-slate-400'
}

function killDotClass(kill: Kill): string {
  return teamDotClass(kill.killer?.team)
}

function statWidth(player: PlayerStat): string {
  return `${Math.max(3, player.kills / maxKills.value * 100)}%`
}
</script>

<template>
  <div class="demo-app flex h-screen min-h-[680px] flex-col overflow-hidden" :data-theme="theme">
    <header class="titlebar-drag relative z-30 flex h-14 shrink-0 items-center border-b border-[var(--border)] bg-[var(--surface)] px-5">
      <div class="flex items-center gap-3">
        <div class="grid h-8 w-8 place-items-center rounded-lg bg-[#3564b6] text-sm font-bold text-white">D</div>
        <div class="flex items-baseline gap-2.5">
          <span class="text-sm font-semibold tracking-[-0.01em] text-[var(--text)]">DemoScope</span>
          <span class="text-[10px] text-[var(--text-faint)]">CS 1.6 Demo 查看器</span>
        </div>
      </div>

      <div v-if="demo" class="ml-8 hidden min-w-0 items-center gap-2 border-l border-[var(--border)] pl-6 lg:flex">
        <span class="h-2 w-2 shrink-0 rounded-full bg-emerald-500"></span>
        <span class="max-w-[420px] truncate text-xs text-[var(--text-muted)]">{{ fileName(demo.path) }}</span>
      </div>

      <div class="ml-auto flex items-center gap-2">
        <button
          type="button"
          class="flex h-9 items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 text-xs font-medium text-[var(--text-secondary)] transition hover:border-[var(--border-strong)] hover:bg-[var(--surface-subtle)]"
          :title="theme === 'light' ? '切换到深色主题' : '切换到浅色主题'"
          @click="toggleTheme"
        >
          <svg v-if="theme === 'light'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M20 15.2A8.5 8.5 0 0 1 8.8 4 8.5 8.5 0 1 0 20 15.2Z" /></svg>
          <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>
          {{ theme === 'light' ? '深色' : '浅色' }}
        </button>
        <button
          type="button"
          class="flex h-9 items-center gap-2 rounded-lg bg-[#3564b6] px-3.5 text-xs font-semibold text-white transition hover:bg-[#2f599f] disabled:cursor-wait disabled:opacity-60"
          :disabled="loading"
          @click="openDemo"
        >
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3.5 7.5h6l2-2h9v13h-17z" stroke-linejoin="round"/><path d="M3.5 9.5h17"/></svg>
          {{ demo ? '打开其他 Demo' : '打开 Demo' }}
        </button>
      </div>
    </header>

    <Transition name="fade">
      <div v-if="errorMessage" class="absolute left-1/2 top-[68px] z-50 flex w-[min(680px,calc(100%-48px))] -translate-x-1/2 items-start gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-red-800 shadow-lg">
        <svg class="mt-0.5 h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 4h.01"/></svg>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold">无法读取这个 Demo</p>
          <p class="mt-0.5 break-words text-xs text-red-700">{{ errorMessage }}</p>
        </div>
        <button class="text-red-500 hover:text-red-800" @click="errorMessage = ''">×</button>
      </div>
    </Transition>

    <main v-if="!demo" class="flex min-h-0 flex-1 items-center justify-center bg-[var(--canvas)] px-8">
      <section class="w-full max-w-[760px] rounded-2xl border border-[var(--border)] bg-[var(--surface)] px-14 py-12 shadow-[0_8px_30px_rgba(24,33,47,.06)]">
        <div class="mx-auto grid h-14 w-14 place-items-center rounded-xl bg-[var(--primary-soft)] text-[var(--primary)]">
          <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M3.5 7.5h6l2-2h9v13h-17z" stroke-linejoin="round"/><path d="M3.5 9.5h17"/><path d="m10 12 4 2.5-4 2.5z"/></svg>
        </div>
        <div class="mt-6 text-center">
          <h1 class="text-2xl font-semibold tracking-[-0.025em] text-[var(--text)]">打开一份 Counter-Strike 1.6 Demo</h1>
          <p class="mx-auto mt-3 max-w-xl text-sm leading-6 text-[var(--text-muted)]">读取 POV 或 HLTV `.dem` 文件，查看比赛信息、玩家统计、击杀记录与回合比分。文件只在本机解析。</p>
          <button class="mt-7 inline-flex h-10 items-center gap-2 rounded-lg bg-[#3564b6] px-5 text-sm font-semibold text-white transition hover:bg-[#2f599f]" @click="openDemo">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3.5 7.5h6l2-2h9v13h-17z" stroke-linejoin="round"/><path d="M3.5 9.5h17"/></svg>
            选择 .dem 文件
          </button>
        </div>
        <div class="mt-9 grid grid-cols-3 divide-x divide-[var(--border)] border-t border-[var(--border)] pt-6 text-center">
          <div><p class="text-xs font-medium text-[var(--text-secondary)]">POV 与 HLTV</p><p class="mt-1 text-[11px] text-[var(--text-faint)]">自动识别类型</p></div>
          <div><p class="text-xs font-medium text-[var(--text-secondary)]">统计与时间线</p><p class="mt-1 text-[11px] text-[var(--text-faint)]">玩家、击杀、回合</p></div>
          <div><p class="text-xs font-medium text-[var(--text-secondary)]">本地处理</p><p class="mt-1 text-[11px] text-[var(--text-faint)]">不会上传文件</p></div>
        </div>
      </section>
    </main>

    <div v-else class="flex min-h-0 flex-1">
      <aside class="flex w-[218px] shrink-0 flex-col border-r border-[var(--border)] bg-[var(--surface)] px-3 py-5">
        <p class="px-3 text-[11px] font-medium text-[var(--text-faint)]">查看内容</p>
        <nav class="mt-2 space-y-1">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            class="flex h-10 w-full items-center rounded-lg px-3 text-left text-sm transition"
            :class="activeTab === tab.id ? 'bg-[var(--surface-active)] font-semibold text-[var(--primary)]' : 'font-medium text-[var(--text-secondary)] hover:bg-[var(--surface-subtle)] hover:text-[var(--text)]'"
            @click="activeTab = tab.id"
          >
            <span class="mr-3 grid h-5 w-5 place-items-center text-[var(--text-muted)]">
              <svg v-if="tab.id === 'overview'" class="h-[17px] w-[17px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
              <svg v-else-if="tab.id === 'players'" class="h-[17px] w-[17px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="9" cy="8" r="3"/><path d="M3.5 19c.5-4 2.4-6 5.5-6s5 2 5.5 6M16 5.5a3 3 0 0 1 0 5.8M17 13c2.1.7 3.3 2.7 3.5 5.5"/></svg>
              <svg v-else-if="tab.id === 'kills'" class="h-[17px] w-[17px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="12" cy="12" r="7"/><circle cx="12" cy="12" r="2"/><path d="M12 2v3m0 14v3M2 12h3m14 0h3"/></svg>
              <svg v-else-if="tab.id === 'rounds'" class="h-[17px] w-[17px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M5 4h14v16H5z"/><path d="M8 8h8M8 12h8M8 16h5"/></svg>
              <svg v-else class="h-[17px] w-[17px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10h.01"/></svg>
            </span>
            <span>{{ tab.label }}</span>
            <span v-if="tab.count !== null" class="ml-auto rounded-md bg-[var(--surface-subtle)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--text-muted)]">{{ tab.count }}</span>
          </button>
        </nav>

        <div class="mt-auto border-t border-[var(--border)] px-3 pt-4">
          <div class="flex items-center gap-2">
            <span class="rounded-md border px-2 py-1 text-[10px] font-semibold" :class="isHLTV ? 'border-violet-200 bg-violet-50 text-violet-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'">{{ isHLTV ? 'HLTV' : 'POV' }}</span>
            <span class="truncate text-xs text-[var(--text-secondary)]">{{ demo.map }}</span>
          </div>
          <p class="mt-2 truncate text-[10px] text-[var(--text-faint)]" :title="demo.path">{{ fileName(demo.path) }}</p>
          <p class="mt-1 text-[10px] text-[var(--text-faint)]">{{ formatBytes(demo.file_size) }} · Protocol {{ demo.network_protocol }}</p>
        </div>
      </aside>

      <main class="min-w-0 flex-1 overflow-y-auto bg-[var(--canvas)]">
        <div class="mx-auto w-full max-w-[1480px] px-7 py-6">
          <section class="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)] shadow-[0_1px_2px_rgba(24,33,47,.03)]">
            <div class="flex min-h-36 items-stretch">
              <div class="flex min-w-0 flex-1 flex-col justify-center px-6 py-5">
                <div class="flex items-center gap-2">
                  <span class="rounded-md border px-2 py-1 text-[10px] font-semibold" :class="isHLTV ? 'border-violet-200 bg-violet-50 text-violet-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'">{{ isHLTV ? 'HLTV Demo' : 'POV Demo' }}</span>
                  <span class="text-xs text-[var(--text-faint)]">解析完成</span>
                </div>
                <h1 class="mt-3 text-2xl font-semibold tracking-[-0.025em] text-[var(--text)]">{{ demo.map }}</h1>
                <p class="mt-1 truncate text-sm text-[var(--text-muted)]">{{ matchLabel }}</p>
              </div>

              <div v-if="isHLTV" class="flex min-w-[470px] items-center justify-center border-l border-[var(--border)] px-8">
                <div class="flex items-center gap-7">
                  <div class="text-right"><p class="text-sm font-semibold text-[var(--text)]">Counter-Terrorists</p><p class="mt-1 text-xs text-blue-600">{{ ctPlayers.length }} 名玩家</p></div>
                  <span class="tabular text-4xl font-semibold text-blue-600">{{ finalScore.ct }}</span>
                  <span class="text-xl text-[var(--text-faint)]">—</span>
                  <span class="tabular text-4xl font-semibold text-orange-600">{{ finalScore.terrorist }}</span>
                  <div><p class="text-sm font-semibold text-[var(--text)]">Terrorists</p><p class="mt-1 text-xs text-orange-600">{{ terroristPlayers.length }} 名玩家</p></div>
                </div>
              </div>

              <div v-else class="flex min-w-[520px] items-center border-l border-[var(--border)] px-7">
                <div class="grid h-12 w-12 shrink-0 place-items-center rounded-full bg-[var(--primary-soft)] text-sm font-semibold text-[var(--primary)]">{{ initials(demo.pov_player?.name) }}</div>
                <div class="ml-3 min-w-0">
                  <p class="truncate text-sm font-semibold text-[var(--text)]">{{ demo.pov_player?.name || '未知玩家' }}</p>
                  <p class="mt-1 truncate text-[11px] text-[var(--text-faint)]">{{ demo.pov_player?.steam_id64 || 'SteamID 不可用' }}</p>
                </div>
                <div class="ml-auto flex items-center divide-x divide-[var(--border)]">
                  <div class="px-5 text-center"><p class="tabular text-xl font-semibold text-[var(--text)]">{{ povStat?.kills ?? 0 }}</p><p class="mt-1 text-[10px] text-[var(--text-faint)]">击杀</p></div>
                  <div class="px-5 text-center"><p class="tabular text-xl font-semibold text-[var(--text)]">{{ povStat?.deaths ?? 0 }}</p><p class="mt-1 text-[10px] text-[var(--text-faint)]">死亡</p></div>
                  <div class="pl-5 text-center"><p class="tabular text-xl font-semibold text-[var(--text)]">{{ (povStat?.kd ?? 0).toFixed(2) }}</p><p class="mt-1 text-[10px] text-[var(--text-faint)]">K / D</p></div>
                </div>
              </div>
            </div>

            <div class="grid grid-cols-5 divide-x divide-[var(--border)] border-t border-[var(--border)] bg-[var(--surface-subtle)] px-2 py-3.5">
              <div class="px-4"><p class="text-[10px] text-[var(--text-faint)]">时长</p><p class="tabular mt-1 text-xs font-medium text-[var(--text-secondary)]">{{ formatDuration(demo.duration_seconds) }}</p></div>
              <div class="px-4"><p class="text-[10px] text-[var(--text-faint)]">帧数</p><p class="tabular mt-1 text-xs font-medium text-[var(--text-secondary)]">{{ demo.frame_count.toLocaleString() }}</p></div>
              <div class="px-4"><p class="text-[10px] text-[var(--text-faint)]">录制时间</p><p class="tabular mt-1 text-xs font-medium text-[var(--text-secondary)]">{{ formatRecordedAt(demo.recorded_at_hint) }}</p></div>
              <div class="px-4"><p class="text-[10px] text-[var(--text-faint)]">Network protocol</p><p class="tabular mt-1 text-xs font-medium text-[var(--text-secondary)]">{{ demo.network_protocol }}</p></div>
              <div class="px-4"><p class="text-[10px] text-[var(--text-faint)]">文件大小</p><p class="tabular mt-1 text-xs font-medium text-[var(--text-secondary)]">{{ formatBytes(demo.file_size) }}</p></div>
            </div>
          </section>

          <section class="mt-5 grid grid-cols-4 divide-x divide-[var(--border)] rounded-xl border border-[var(--border)] bg-[var(--surface)] py-4">
            <div class="px-5"><p class="text-xs text-[var(--text-muted)]">玩家</p><p class="tabular mt-1 text-xl font-semibold text-[var(--text)]">{{ playerStats.length }}</p></div>
            <div class="px-5"><p class="text-xs text-[var(--text-muted)]">击杀事件</p><p class="tabular mt-1 text-xl font-semibold text-[var(--text)]">{{ kills.length }}</p></div>
            <div class="px-5"><p class="text-xs text-[var(--text-muted)]">爆头率</p><p class="tabular mt-1 text-xl font-semibold text-[var(--text)]">{{ headshotRate }}<span class="ml-0.5 text-sm font-medium text-[var(--text-muted)]">%</span></p></div>
            <div class="px-5"><p class="text-xs text-[var(--text-muted)]">回合</p><p class="tabular mt-1 text-xl font-semibold text-[var(--text)]">{{ rounds.length }}</p></div>
          </section>

          <section v-if="activeTab === 'overview'" class="mt-5 grid grid-cols-[minmax(0,1.45fr)_minmax(340px,.75fr)] gap-5">
            <div class="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
              <div class="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
                <div><h2 class="text-sm font-semibold text-[var(--text)]">玩家表现</h2><p class="mt-1 text-xs text-[var(--text-faint)]">按击杀数排序</p></div>
                <button class="text-xs font-medium text-[var(--primary)] hover:underline" @click="activeTab = 'players'">全部玩家</button>
              </div>
              <div class="grid grid-cols-[36px_minmax(190px,1fr)_100px_64px_64px_64px] gap-4 border-b border-[var(--border)] bg-[var(--surface-subtle)] px-5 py-2.5 text-[10px] font-medium text-[var(--text-faint)]">
                <span>#</span><span>玩家</span><span>表现</span><span class="text-right">队伍</span><span class="text-right">K</span><span class="text-right">D</span>
              </div>
              <div v-if="topPlayers.length" class="divide-y divide-[var(--border)]">
                <div v-for="(player, index) in topPlayers" :key="player.key" class="grid grid-cols-[36px_minmax(190px,1fr)_100px_64px_64px_64px] items-center gap-4 px-5 py-3 text-xs hover:bg-[var(--row-hover)]">
                  <span class="tabular text-[11px] text-[var(--text-faint)]">{{ index + 1 }}</span>
                  <div class="flex min-w-0 items-center gap-2.5">
                    <div class="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-[var(--surface-subtle)] text-[9px] font-semibold text-[var(--text-muted)]">{{ initials(player.name) }}</div>
                    <div class="min-w-0"><p class="truncate font-medium text-[var(--text)]">{{ player.name }}</p><p v-if="player.is_pov" class="mt-0.5 text-[9px] font-medium text-emerald-600">POV 玩家</p></div>
                  </div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-[var(--surface-subtle)]"><div class="h-full rounded-full bg-[#5680c7]" :style="{ width: statWidth(player) }"></div></div>
                  <span class="justify-self-end rounded border px-1.5 py-0.5 text-[9px] font-semibold" :class="teamBadgeClass(player.team)">{{ shortTeam(player.team) }}</span>
                  <span class="tabular text-right font-semibold text-[var(--text)]">{{ player.kills }}</span>
                  <span class="tabular text-right text-[var(--text-muted)]">{{ player.deaths }}</span>
                </div>
              </div>
              <div v-else class="py-14 text-center text-xs text-[var(--text-faint)]">没有可用的玩家数据</div>
            </div>

            <div class="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
              <div class="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
                <div><h2 class="text-sm font-semibold text-[var(--text)]">最近击杀</h2><p class="mt-1 text-xs text-[var(--text-faint)]">时间线末端事件</p></div>
                <button class="text-xs font-medium text-[var(--primary)] hover:underline" @click="activeTab = 'kills'">全部记录</button>
              </div>
              <div v-if="recentKills.length" class="divide-y divide-[var(--border)]">
                <div v-for="kill in recentKills" :key="`${kill.frame}-${kill.victim.slot_zero_based}`" class="grid grid-cols-[44px_7px_minmax(0,1fr)] items-center gap-2.5 px-5 py-3 text-[11px] hover:bg-[var(--row-hover)]">
                  <span class="tabular text-[10px] text-[var(--text-faint)]">{{ formatDuration(kill.time_seconds) }}</span>
                  <span class="h-2 w-2 rounded-full" :class="killDotClass(kill)"></span>
                  <p class="flex min-w-0 items-center gap-1.5"><span class="truncate font-medium text-[var(--text)]">{{ kill.killer?.name || 'WORLD' }}</span><span class="shrink-0 text-[9px] text-[var(--text-faint)]">{{ weaponName(kill.weapon) }}</span><span v-if="kill.headshot" class="shrink-0 text-orange-500">◆</span><span class="shrink-0 text-[var(--text-faint)]">→</span><span class="truncate text-[var(--text-muted)]">{{ kill.victim.name }}</span></p>
                </div>
              </div>
              <div v-else class="py-14 text-center text-xs text-[var(--text-faint)]">没有可用的击杀数据</div>
            </div>
          </section>

          <section v-else-if="activeTab === 'players'" class="mt-5 overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
            <div class="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
              <div><h2 class="text-sm font-semibold text-[var(--text)]">玩家统计</h2><p class="mt-1 text-xs text-[var(--text-faint)]">按 Demo 最后队伍状态分组，组内按真实击杀数排序</p></div>
              <div class="flex items-center gap-4 text-xs text-[var(--text-muted)]"><span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full bg-orange-500"></span>{{ terroristPlayers.length }} T</span><span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full bg-blue-500"></span>{{ ctPlayers.length }} CT</span><span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full bg-slate-400"></span>{{ spectatorPlayers.length }} SPEC</span></div>
            </div>
            <div class="grid grid-cols-[42px_minmax(220px,1.4fr)_90px_74px_74px_74px_90px_minmax(175px,1fr)] gap-4 border-b border-[var(--border)] bg-[var(--surface-subtle)] px-5 py-3 text-[10px] font-medium text-[var(--text-faint)]">
              <span>排名</span><span>玩家</span><span>队伍</span><span class="text-right">真实击杀</span><span class="text-right">死亡</span><span class="text-right">K/D</span><span class="text-right">爆头</span><span>SteamID64</span>
            </div>
            <div v-if="playerStats.length">
              <div v-for="group in playerGroups" :key="group.team" class="border-b border-[var(--border)] last:border-b-0">
                <div class="flex items-center justify-between bg-[var(--surface-subtle)] px-5 py-2.5">
                  <div class="flex items-center gap-2"><span class="h-2.5 w-2.5 rounded-full" :class="group.dotClass"></span><span class="text-xs font-semibold text-[var(--text)]">{{ group.label }}</span><span class="text-[10px] text-[var(--text-faint)]">{{ group.name }}</span></div>
                  <span class="tabular rounded-full border border-[var(--border)] bg-[var(--surface)] px-2 py-0.5 text-[10px] text-[var(--text-muted)]">{{ group.players.length }} 人</span>
                </div>
                <div v-if="group.players.length" class="divide-y divide-[var(--border)]">
                  <div v-for="(player, index) in group.players" :key="player.key" class="grid grid-cols-[42px_minmax(220px,1.4fr)_90px_74px_74px_74px_90px_minmax(175px,1fr)] items-center gap-4 px-5 py-3.5 text-xs hover:bg-[var(--row-hover)]">
                    <span class="tabular text-[var(--text-faint)]">{{ index + 1 }}</span>
                    <div class="flex min-w-0 items-center gap-3"><div class="relative grid h-8 w-8 shrink-0 place-items-center rounded-full bg-[var(--surface-subtle)] text-[9px] font-semibold text-[var(--text-muted)]">{{ initials(player.name) }}<span v-if="player.is_pov" class="absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full border-2 border-[var(--surface)] bg-emerald-500"></span></div><div class="min-w-0"><p class="truncate font-medium text-[var(--text)]">{{ player.name }}</p><p v-if="player.aliases?.length" class="mt-0.5 truncate text-[10px] text-[var(--text-faint)]">曾用名：{{ player.aliases.join('、') }}</p><p v-else-if="player.models?.length" class="mt-0.5 truncate text-[10px] text-[var(--text-faint)]">{{ player.models.join(' · ') }}</p></div></div>
                    <span class="w-fit rounded border px-2 py-0.5 text-[9px] font-semibold" :class="teamBadgeClass(player.team)">{{ shortTeam(player.team) }}</span>
                    <span class="tabular text-right font-semibold text-[var(--text)]">{{ player.kills }}</span>
                    <span class="tabular text-right text-[var(--text-muted)]">{{ player.deaths }}</span>
                    <span class="tabular text-right font-medium" :class="player.kd >= 1 ? 'text-emerald-600' : 'text-[var(--text-muted)]'">{{ player.kd.toFixed(2) }}</span>
                    <span class="tabular text-right text-[var(--text-secondary)]">{{ player.headshots }}</span>
                    <span class="truncate text-[11px] text-[var(--text-faint)]" :title="player.steam_id64">{{ player.steam_id64 || '—' }}</span>
                  </div>
                </div>
                <div v-else class="px-5 py-4 text-xs text-[var(--text-faint)]">该队伍没有玩家</div>
              </div>
            </div>
            <div v-else class="py-16 text-center text-xs text-[var(--text-faint)]">没有可识别的玩家记录</div>
          </section>

          <section v-else-if="activeTab === 'kills'" class="mt-5 overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
            <div class="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
              <div><h2 class="text-sm font-semibold text-[var(--text)]">击杀记录</h2><p class="mt-1 text-xs text-[var(--text-faint)]">时间从 Demo Playback 段开始计算</p></div>
              <div class="flex items-center gap-2">
                <div class="relative"><svg class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--text-faint)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="killSearch" type="text" placeholder="搜索玩家或武器" class="h-9 w-56 rounded-lg border border-[var(--border)] bg-[var(--surface)] pl-9 pr-3 text-xs text-[var(--text)] outline-none placeholder:text-[var(--text-faint)] focus:border-[#7195cf]"/></div>
                <select v-model="killFilter" class="h-9 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 text-xs text-[var(--text-secondary)] outline-none focus:border-[#7195cf]"><option value="all">全部事件</option><option value="headshot">仅爆头</option><option v-if="!isHLTV" value="pov">与 POV 玩家有关</option></select>
              </div>
            </div>
            <div class="grid grid-cols-[82px_8px_minmax(170px,1fr)_130px_42px_minmax(170px,1fr)_92px] items-center gap-4 border-b border-[var(--border)] bg-[var(--surface-subtle)] px-5 py-3 text-[10px] font-medium text-[var(--text-faint)]">
              <span>时间</span><span></span><span>击杀者</span><span class="text-center">武器</span><span></span><span>被击杀者</span><span class="text-right">帧</span>
            </div>
            <div v-if="filteredKills.length" class="divide-y divide-[var(--border)]">
              <div v-for="kill in filteredKills" :key="`${kill.frame}-${kill.killer?.slot_zero_based}-${kill.victim.slot_zero_based}`" class="grid grid-cols-[82px_8px_minmax(170px,1fr)_130px_42px_minmax(170px,1fr)_92px] items-center gap-4 px-5 py-3 text-xs hover:bg-[var(--row-hover)]">
                <span class="tabular text-[10px] text-[var(--text-faint)]">{{ formatDuration(kill.time_seconds, true) }}</span>
                <span class="h-5 w-1 rounded-full" :class="killDotClass(kill)"></span>
                <div class="min-w-0"><p class="truncate font-medium text-[var(--text)]">{{ kill.killer?.name || 'WORLD' }}</p><p class="mt-0.5 text-[9px] text-[var(--text-faint)]">{{ kill.killer ? shortTeam(kill.killer.team) : '环境' }}</p></div>
                <span class="justify-self-center rounded-md bg-[var(--surface-subtle)] px-2 py-1 text-[10px] font-medium text-[var(--text-secondary)]">{{ weaponName(kill.weapon) }}</span>
                <span class="text-center" :class="kill.headshot ? 'text-orange-500' : 'text-[var(--text-faint)]'">{{ kill.headshot ? '◆' : '→' }}</span>
                <div class="min-w-0"><p class="truncate text-[var(--text-secondary)]">{{ kill.victim.name || `Slot ${kill.victim.slot_zero_based + 1}` }}</p><p class="mt-0.5 text-[9px] text-[var(--text-faint)]">{{ shortTeam(kill.victim.team) }}</p></div>
                <span class="tabular text-right text-[10px] text-[var(--text-faint)]">{{ kill.frame.toLocaleString() }}</span>
              </div>
            </div>
            <div v-else class="py-16 text-center text-xs text-[var(--text-faint)]">没有符合条件的击杀事件</div>
          </section>

          <section v-else-if="activeTab === 'rounds'" class="mt-5 overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
            <div class="border-b border-[var(--border)] px-5 py-4"><h2 class="text-sm font-semibold text-[var(--text)]">回合比分</h2><p class="mt-1 text-xs text-[var(--text-faint)]">根据 TeamScore 更新推导</p></div>
            <div class="grid grid-cols-[80px_minmax(200px,1fr)_130px_160px_100px] gap-4 border-b border-[var(--border)] bg-[var(--surface-subtle)] px-5 py-3 text-[10px] font-medium text-[var(--text-faint)]"><span>回合</span><span>获胜方</span><span>比分</span><span>时间</span><span class="text-right">击杀数</span></div>
            <div v-if="rounds.length" class="divide-y divide-[var(--border)]">
              <div v-for="round in rounds" :key="round.number" class="grid grid-cols-[80px_minmax(200px,1fr)_130px_160px_100px] items-center gap-4 px-5 py-3.5 text-xs hover:bg-[var(--row-hover)]">
                <span class="tabular text-[var(--text-muted)]">{{ round.number }}</span>
                <div class="flex items-center gap-2"><span class="h-2 w-2 rounded-full" :class="teamDotClass(round.winner)"></span><span class="font-medium text-[var(--text)]">{{ teamName(round.winner) }}</span></div>
                <div class="tabular font-semibold"><span class="text-blue-600">{{ round.ct_score }}</span><span class="mx-2 text-[var(--text-faint)]">:</span><span class="text-orange-600">{{ round.terrorist_score }}</span></div>
                <span class="tabular text-[var(--text-muted)]">{{ formatDuration(round.start_seconds) }} — {{ formatDuration(round.end_seconds) }}</span>
                <span class="tabular text-right text-[var(--text-secondary)]">{{ round.kills?.length ?? 0 }}</span>
              </div>
            </div>
            <div v-else class="py-16 text-center text-xs text-[var(--text-faint)]">这个 Demo 没有可用的 TeamScore 回合数据</div>
          </section>

          <section v-else class="mt-5 grid grid-cols-[minmax(0,1fr)_minmax(360px,.65fr)] gap-5">
            <div class="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
              <div class="border-b border-[var(--border)] px-5 py-4"><h2 class="text-sm font-semibold text-[var(--text)]">Demo 元数据</h2><p class="mt-1 text-xs text-[var(--text-faint)]">来自 HLDEMO 文件头与服务器信息</p></div>
              <dl class="divide-y divide-[var(--border)]">
                <div v-for="item in [
                  ['文件名', fileName(demo.path)], ['绝对路径', demo.path], ['容器类型', `${demo.format.toUpperCase()} / ${demo.recording_type.toUpperCase()}`], ['游戏目录', demo.game_directory], ['Demo protocol', demo.demo_protocol], ['Network protocol', demo.network_protocol], ['Map CRC32', demo.map_crc32], ['Directory offset', demo.directory_offset.toLocaleString()], ['文件大小', formatBytes(demo.file_size)], ['帧总数', demo.frame_count.toLocaleString()], ['服务器', demo.server?.name || '—'], ['Max clients', demo.server?.max_clients ?? '—'],
                ]" :key="String(item[0])" class="grid grid-cols-[150px_minmax(0,1fr)] gap-6 px-5 py-3 text-xs"><dt class="text-[var(--text-muted)]">{{ item[0] }}</dt><dd class="break-words text-[var(--text-secondary)]">{{ item[1] }}</dd></div>
              </dl>
            </div>
            <div class="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
              <div class="border-b border-[var(--border)] px-5 py-4"><h2 class="text-sm font-semibold text-[var(--text)]">Directory entries</h2><p class="mt-1 text-xs text-[var(--text-faint)]">Demo 的加载与回放分段</p></div>
              <div class="divide-y divide-[var(--border)]">
                <div v-for="entry in demo.directory_entries" :key="entry.number" class="px-5 py-4">
                  <div class="flex items-center justify-between"><div><p class="text-xs font-medium text-[var(--text)]">{{ entry.title || 'Untitled segment' }}</p><p class="mt-1 text-[10px] text-[var(--text-faint)]">Entry {{ entry.number }} · offset {{ entry.offset.toLocaleString() }}</p></div><span class="tabular text-[11px] text-[var(--text-muted)]">{{ formatDuration(entry.time_seconds, true) }}</span></div>
                  <div class="mt-3 flex items-center gap-5 text-[10px] text-[var(--text-faint)]"><span>{{ entry.frames.toLocaleString() }} 帧</span><span>{{ formatBytes(entry.length) }}</span><span>play {{ entry.play }}</span></div>
                </div>
              </div>
            </div>
          </section>
        </div>
      </main>
    </div>

    <Transition name="fade">
      <div v-if="loading" class="absolute inset-0 z-[100] grid place-items-center bg-black/20 backdrop-blur-[2px]">
        <div class="flex items-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-5 py-4 shadow-xl">
          <div class="h-5 w-5 animate-spin rounded-full border-2 border-[var(--border)] border-t-[#3564b6]"></div>
          <div><p class="text-sm font-semibold text-[var(--text)]">正在解析 Demo</p><p class="mt-0.5 text-[10px] text-[var(--text-faint)]">读取 GoldSrc 网络帧</p></div>
        </div>
      </div>
    </Transition>
  </div>
</template>

import type { DemoData, Player, PlayerReference, PlayerStat } from './types'

const weapons: Record<string, string> = {
  ak47: 'AK-47', m4a1: 'M4A1', awp: 'AWP', deagle: 'Desert Eagle',
  usp: 'USP', glock18: 'Glock-18', mp5navy: 'MP5', hegrenade: 'HE Grenade',
  fiveseven: 'Five-SeveN', elite: 'Dual Elites', xm1014: 'XM1014',
  mac10: 'MAC-10', ump45: 'UMP-45', sg550: 'SG-550', galil: 'Galil',
  famas: 'FAMAS', m249: 'M249', m3: 'M3', tmp: 'TMP', g3sg1: 'G3SG1',
  sg552: 'SG-552', aug: 'AUG', scout: 'Scout', p90: 'P90', p228: 'P228',
  knife: 'Knife', grenade: 'Grenade', world: 'World', worldspawn: 'World',
  trigger_hurt: 'Trigger Hurt', vehicle: 'Vehicle', c4: 'C4',
}

export function weaponName(value: string): string {
  return weapons[value] ?? value.toUpperCase()
}

export function teamName(team?: string): string {
  if (team === 'CT') return 'Counter-Terrorists'
  if (team === 'TERRORIST') return 'Terrorists'
  if (team === 'SPECTATOR') return 'Spectator'
  return 'Unknown'
}

export function shortTeam(team?: string): string {
  if (team === 'CT') return 'CT'
  if (team === 'TERRORIST') return 'T'
  if (team === 'SPECTATOR') return 'SPEC'
  return '—'
}

export function formatDuration(seconds: number, milliseconds = false): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '00:00'
  const whole = Math.floor(seconds)
  const minutes = Math.floor(whole / 60)
  const remainder = whole % 60
  const base = `${String(minutes).padStart(2, '0')}:${String(remainder).padStart(2, '0')}`
  return milliseconds ? `${base}.${String(Math.floor((seconds - whole) * 1000)).padStart(3, '0')}` : base
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const tier = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, tier)).toFixed(tier === 0 ? 0 : 1)} ${units[tier]}`
}

export function fileName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path
}

export function formatRecordedAt(value?: string): string {
  if (!value) return '文件中未提供'
  return value.replace('T', ' ')
}

function identity(value: Player | PlayerReference): string {
  if (value.steam_id64) return `steam:${value.steam_id64}`
  if (value.name) return `name:${value.name.toLocaleLowerCase()}`
  const slots = 'slot_zero_based' in value ? [value.slot_zero_based] : value.slots_zero_based
  return `slot:${slots?.[0] ?? -1}`
}

function refMatchesPlayer(reference: PlayerReference, player: Player): boolean {
  if (reference.steam_id64 && player.steam_id64) return reference.steam_id64 === player.steam_id64
  if (reference.name && player.name) return reference.name.toLocaleLowerCase() === player.name.toLocaleLowerCase()
  return player.slots_zero_based?.includes(reference.slot_zero_based) ?? false
}

export function buildPlayerStats(demo: DemoData): PlayerStat[] {
  const stats: PlayerStat[] = (demo.players ?? []).map((player) => ({
    ...player,
    key: identity(player),
    team: player.team ?? '',
    kills: 0,
    deaths: 0,
    headshots: 0,
    kd: 0,
  }))

  const ensureReference = (reference?: PlayerReference): PlayerStat | undefined => {
    if (!reference || (!reference.name && !reference.steam_id64)) return undefined
    let stat = stats.find((player) => refMatchesPlayer(reference, player))
    if (!stat) {
      stat = {
        name: reference.name || `Slot ${reference.slot_zero_based + 1}`,
        steam_id64: reference.steam_id64,
        slots_zero_based: [reference.slot_zero_based],
        key: identity(reference),
        team: reference.team ?? '',
        kills: 0,
        deaths: 0,
        headshots: 0,
        kd: 0,
      }
      stats.push(stat)
    }
    if (reference.team && reference.team !== 'UNASSIGNED') stat.team = reference.team
    return stat
  }

  for (const change of demo.team_changes ?? []) {
    const stat = ensureReference(change.player)
    if (stat && change.team !== 'UNASSIGNED') stat.team = change.team
  }
  // These are real DeathMsg events, not the scoreboard frag/score value.
  // Objective bonuses such as planting, exploding, or defusing C4 are not
  // DeathMsg events and therefore must not change this kill count.
  for (const kill of demo.kills ?? []) {
    const killer = ensureReference(kill.killer)
    const victim = ensureReference(kill.victim)
    if (killer && !kill.world) {
      killer.kills++
      if (kill.headshot) killer.headshots++
    }
    if (victim) victim.deaths++
  }
  for (const stat of stats) {
    stat.kd = stat.deaths === 0 ? stat.kills : stat.kills / stat.deaths
  }
  return stats.sort((a, b) => b.kills - a.kills || a.deaths - b.deaths || a.name.localeCompare(b.name))
}

export function initials(name?: string): string {
  if (!name) return '—'
  const parts = name.trim().split(/\s+/)
  return (parts.length === 1 ? parts[0].slice(0, 2) : parts.slice(0, 2).map((part) => part[0]).join('')).toUpperCase()
}

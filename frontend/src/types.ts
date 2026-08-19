export interface ServerInfo {
  name?: string
  count: number
  crc32: string
  max_clients: number
  client_slot_zero_based: number
  map_file?: string
  map_checksum?: string
}

export interface PlayerReference {
  name: string
  steam_id64?: string
  slot_zero_based: number
  team?: string
}

export interface Player {
  name: string
  aliases?: string[]
  steam_id64?: string
  team?: string
  models?: string[]
  slots_zero_based?: number[]
  user_ids?: number[]
  is_pov?: boolean
}

export interface Kill {
  time_seconds: number
  frame: number
  killer?: PlayerReference
  victim: PlayerReference
  weapon: string
  headshot: boolean
  world?: boolean
}

export interface TeamChange {
  time_seconds: number
  frame: number
  player: PlayerReference
  team: string
}

export interface ScoreUpdate {
  time_seconds: number
  frame: number
  ct_score: number
  terrorist_score: number
}

export interface Round {
  number: number
  start_seconds: number
  end_seconds: number
  winner: string
  ct_score: number
  terrorist_score: number
  kills?: Kill[]
}

export interface DirectoryEntry {
  number: number
  title: string
  flags: number
  play: number
  time_seconds: number
  frames: number
  offset: number
  length: number
}

export interface HLTVProxy {
  name: string
  steam_id64?: string
  slot_zero_based: number
  delay_seconds?: number
  spectator_slots?: number
}

export interface DemoData {
  path: string
  file_size: number
  format: string
  demo_protocol: number
  network_protocol: number
  map: string
  game_directory: string
  map_crc32: string
  directory_offset: number
  duration_seconds: number
  frame_count: number
  recording_type: 'pov' | 'hltv'
  side_hint?: string
  recorded_at_hint?: string
  server?: ServerInfo
  hltv_proxy?: HLTVProxy
  pov_player?: PlayerReference
  players?: Player[]
  userinfo_updates: number
  kills?: Kill[]
  team_changes?: TeamChange[]
  score_updates?: ScoreUpdate[]
  rounds?: Round[]
  directory_entries: DirectoryEntry[]
}

export interface PlayerStat extends Player {
  key: string
  team: string
  kills: number
  deaths: number
  headshots: number
  kd: number
}

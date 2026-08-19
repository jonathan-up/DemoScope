export namespace goldsrc {
	
	export class DirectoryEntry {
	    number: number;
	    title: string;
	    flags: number;
	    play: number;
	    time_seconds: number;
	    frames: number;
	    offset: number;
	    length: number;
	
	    static createFrom(source: any = {}) {
	        return new DirectoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.flags = source["flags"];
	        this.play = source["play"];
	        this.time_seconds = source["time_seconds"];
	        this.frames = source["frames"];
	        this.offset = source["offset"];
	        this.length = source["length"];
	    }
	}
	export class Round {
	    number: number;
	    start_seconds: number;
	    end_seconds: number;
	    winner: string;
	    ct_score: number;
	    terrorist_score: number;
	    kills: Kill[];
	
	    static createFrom(source: any = {}) {
	        return new Round(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.start_seconds = source["start_seconds"];
	        this.end_seconds = source["end_seconds"];
	        this.winner = source["winner"];
	        this.ct_score = source["ct_score"];
	        this.terrorist_score = source["terrorist_score"];
	        this.kills = this.convertValues(source["kills"], Kill);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ScoreUpdate {
	    time_seconds: number;
	    frame: number;
	    ct_score: number;
	    terrorist_score: number;
	
	    static createFrom(source: any = {}) {
	        return new ScoreUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time_seconds = source["time_seconds"];
	        this.frame = source["frame"];
	        this.ct_score = source["ct_score"];
	        this.terrorist_score = source["terrorist_score"];
	    }
	}
	export class TeamChange {
	    time_seconds: number;
	    frame: number;
	    player: PlayerReference;
	    team: string;
	
	    static createFrom(source: any = {}) {
	        return new TeamChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time_seconds = source["time_seconds"];
	        this.frame = source["frame"];
	        this.player = this.convertValues(source["player"], PlayerReference);
	        this.team = source["team"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Kill {
	    time_seconds: number;
	    frame: number;
	    killer?: PlayerReference;
	    victim: PlayerReference;
	    weapon: string;
	    headshot: boolean;
	    world?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Kill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time_seconds = source["time_seconds"];
	        this.frame = source["frame"];
	        this.killer = this.convertValues(source["killer"], PlayerReference);
	        this.victim = this.convertValues(source["victim"], PlayerReference);
	        this.weapon = source["weapon"];
	        this.headshot = source["headshot"];
	        this.world = source["world"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Player {
	    name: string;
	    aliases?: string[];
	    steam_id64?: string;
	    team?: string;
	    models?: string[];
	    slots_zero_based?: number[];
	    user_ids?: number[];
	    is_pov?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Player(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.aliases = source["aliases"];
	        this.steam_id64 = source["steam_id64"];
	        this.team = source["team"];
	        this.models = source["models"];
	        this.slots_zero_based = source["slots_zero_based"];
	        this.user_ids = source["user_ids"];
	        this.is_pov = source["is_pov"];
	    }
	}
	export class PlayerReference {
	    name: string;
	    steam_id64?: string;
	    slot_zero_based: number;
	    team?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayerReference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.steam_id64 = source["steam_id64"];
	        this.slot_zero_based = source["slot_zero_based"];
	        this.team = source["team"];
	    }
	}
	export class HLTVProxy {
	    name: string;
	    steam_id64?: string;
	    slot_zero_based: number;
	    delay_seconds?: number;
	    spectator_slots?: number;
	
	    static createFrom(source: any = {}) {
	        return new HLTVProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.steam_id64 = source["steam_id64"];
	        this.slot_zero_based = source["slot_zero_based"];
	        this.delay_seconds = source["delay_seconds"];
	        this.spectator_slots = source["spectator_slots"];
	    }
	}
	export class Server {
	    name?: string;
	    count: number;
	    crc32: string;
	    max_clients: number;
	    client_slot_zero_based: number;
	    map_file?: string;
	    map_checksum?: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.count = source["count"];
	        this.crc32 = source["crc32"];
	        this.max_clients = source["max_clients"];
	        this.client_slot_zero_based = source["client_slot_zero_based"];
	        this.map_file = source["map_file"];
	        this.map_checksum = source["map_checksum"];
	    }
	}
	export class Demo {
	    path: string;
	    file_size: number;
	    format: string;
	    demo_protocol: number;
	    network_protocol: number;
	    map: string;
	    game_directory: string;
	    map_crc32: string;
	    directory_offset: number;
	    duration_seconds: number;
	    frame_count: number;
	    recording_type: string;
	    side_hint?: string;
	    recorded_at_hint?: string;
	    server?: Server;
	    hltv_proxy?: HLTVProxy;
	    pov_player?: PlayerReference;
	    players?: Player[];
	    userinfo_updates: number;
	    kills?: Kill[];
	    team_changes?: TeamChange[];
	    score_updates?: ScoreUpdate[];
	    rounds?: Round[];
	    directory_entries: DirectoryEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Demo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.file_size = source["file_size"];
	        this.format = source["format"];
	        this.demo_protocol = source["demo_protocol"];
	        this.network_protocol = source["network_protocol"];
	        this.map = source["map"];
	        this.game_directory = source["game_directory"];
	        this.map_crc32 = source["map_crc32"];
	        this.directory_offset = source["directory_offset"];
	        this.duration_seconds = source["duration_seconds"];
	        this.frame_count = source["frame_count"];
	        this.recording_type = source["recording_type"];
	        this.side_hint = source["side_hint"];
	        this.recorded_at_hint = source["recorded_at_hint"];
	        this.server = this.convertValues(source["server"], Server);
	        this.hltv_proxy = this.convertValues(source["hltv_proxy"], HLTVProxy);
	        this.pov_player = this.convertValues(source["pov_player"], PlayerReference);
	        this.players = this.convertValues(source["players"], Player);
	        this.userinfo_updates = source["userinfo_updates"];
	        this.kills = this.convertValues(source["kills"], Kill);
	        this.team_changes = this.convertValues(source["team_changes"], TeamChange);
	        this.score_updates = this.convertValues(source["score_updates"], ScoreUpdate);
	        this.rounds = this.convertValues(source["rounds"], Round);
	        this.directory_entries = this.convertValues(source["directory_entries"], DirectoryEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	

}


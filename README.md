# DemoScope

一个使用 Wails、Vue 3、TypeScript 和 Tailwind CSS 构建的 Counter-Strike 1.6 Demo 桌面查看器。应用只解析 `.dem` 文件中的信息，不播放回放视频，所有分析都在本机完成。

## 桌面端能力

- 使用系统文件选择器打开 GoldSrc `.dem` 文件
- 自动识别 POV Demo 与 HLTV Demo
- 展示地图、协议、服务器、时长、帧数、文件大小等元数据
- 提取玩家名、别名、SteamID64、模型、slot、队伍与 POV 玩家
- 展示击杀者、被击杀者、武器、爆头、时间点和帧号
- 从 `TeamScore` 推导比分和回合，并处理 HLTV 中的记分板重置
- HLTV 使用双队记分板、全场排行和完整击杀时间线
- POV 突出录制玩家，展示个人 K/D、爆头和相关事件
- 玩家、击杀、回合和底层 Demo Directory 分标签查看

## 技术结构

```text
Vue 3 + Tailwind UI
        │ Wails bindings
        ▼
Go desktop API
        │
        ▼
GoldSrc parser
  ├─ HLDEMO header / directory
  ├─ svc_serverinfo / svc_updateuserinfo
  ├─ DeathMsg / TeamInfo / TeamScore
  └─ POV / HLTV detection
```

## 开发

需要 Go、Node.js/npm 和 Wails v2。

```bash
npm --prefix frontend install
wails dev
```

生产构建：

```bash
wails build
```

macOS 构建产物位于：

```text
build/bin/DemoScope.app
```

运行测试与前端类型检查：

```bash
go test ./...
npm --prefix frontend run build
```

## 命令行工具

桌面端之外仍保留了原有 CLI。不传路径时递归读取 `demos/`：

```bash
go run ./cmd/demoinfo
go run ./cmd/demoinfo demos
go run ./cmd/demoinfo -format json demos
```

查询某位玩家的击杀时间线，可以使用完整名字、SteamID64 或能够唯一匹配的名字片段：

```bash
go run ./cmd/demoinfo -player "H0meV" demos
go run ./cmd/demoinfo -player 76561198975579577 -format json demos
```

击杀时间从 Demo `Playback` 段开始计算。玩家统计中的“真实击杀”逐条累计 `DeathMsg`，不是计分板的 frag/score，因此不包含拆除、引爆 C4 等目标奖励分。`side_hint` 和 `recorded_at_hint` 只来自文件名，录制时间提示没有时区；其余核心字段来自 Demo 二进制内容。

## 当前解析范围

当前覆盖 Counter-Strike 1.6 常见 protocol 42 和 protocol 45 及以上 Demo。聊天、经济、炸弹事件、逐帧玩家坐标与视角轨迹尚未解析；损坏或来自特殊 MOD 的非标准 Demo 可能无法读取。

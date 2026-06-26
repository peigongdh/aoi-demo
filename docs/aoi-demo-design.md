# AOI 可视化网游 Demo 设计文档

## 1. 项目目标

本项目用于实现一个可在局域网运行的简化 MMORPG AOI 同步实验 Demo。

核心目标不是制作完整游戏，而是构建一个可观察、可替换、可扩展的网络同步实验环境，用于研究不同 AOI（Area of Interest，兴趣区域）算法在多人在线场景中的表现。

Demo 需要具备以下能力：

1. 启动一个可运行的世界服务器。
2. 多个客户端可以通过不同 IP 在局域网中连接同一个服务器。
3. 客户端具有二维可视化界面，可以看到玩家、NPC、AOI 范围、地图分区、进入/离开 AOI 的事件。
4. 服务端具有基本地图与实体管理架构。
5. AOI 组件以插件方式接入，支持替换不同 AOI 算法。
6. 本期实现重点放在 AOI，不实现复杂战斗、任务、背包、装备等 MMORPG 系统。
7. 架构上预留后续实验不同网络同步算法的能力，例如状态同步、快照同步、插值、预测、回滚等，但本期不实现。

---

## 2. 推荐技术栈

### 2.1 服务端

推荐使用 Go 实现。

原因：

1. 网络服务开发简单。
2. goroutine 适合处理连接与世界 Tick。
3. interface 适合抽象 AOI 插件。
4. 编译部署方便，适合局域网测试。

服务端建议模块：

```text
server/
  cmd/worldserver/
  internal/world/
  internal/aoi/
  internal/network/
  internal/protocol/
  internal/entity/
  internal/sync/
  internal/config/
```

### 2.2 客户端

推荐使用 Web 技术实现。

可以选择：

```text
client/
  index.html
  src/
    main.ts
    network.ts
    renderer.ts
    world.ts
    debug-panel.ts
```

客户端技术建议：

1. TypeScript
2. WebSocket
3. Canvas 或 SVG
4. Vite 作为开发工具

原因：

1. 局域网中其他机器直接打开浏览器即可连接。
2. Canvas/SVG 适合二维地图可视化。
3. WebSocket 和服务端模型接近真实网游长连接。
4. 不依赖 Unity、Godot、Unreal 等大型游戏引擎，避免引擎耦合过深。

---

## 3. Demo 玩法设计

### 3.1 世界设定

使用一个二维平面地图，例如：

```text
地图大小：2000 x 2000
玩家半径：8
NPC 半径：8
AOI 半径：200
Grid 大小：200
Tower 大小：200
服务器 Tick：10 / 20 / 30 TPS 可配置
```

地图中包含：

1. 玩家 Player
2. NPC
3. 障碍物可暂时不实现
4. 地图网格可视化
5. AOI 圆形范围可视化
6. AOI 事件日志

### 3.2 玩家行为

玩家客户端可以执行：

1. WASD 移动
2. 鼠标点击移动，作为可选项
3. 选择观察某个本地玩家
4. 查看该玩家当前 AOI 内对象
5. 查看服务端发来的 enter / leave / update 事件

### 3.3 NPC 行为

NPC 作为 AOI 触发器，用于制造进入/离开视野的事件。

NPC 行为可选：

1. 静止 NPC
2. 随机游走 NPC
3. 按固定路径巡逻 NPC
4. 自动生成 NPC 群组

本期建议至少实现：

```text
静止 NPC + 随机游走 NPC
```

NPC 的作用：

1. 测试玩家移动时 NPC 进入/离开 AOI。
2. 测试 NPC 移动时进入/离开玩家 AOI。
3. 测试多个客户端看到的世界是否一致。
4. 测试不同 AOI 算法的触发结果是否一致。

---

## 4. 客户端可视化要求

客户端界面建议分为三块：

```text
+--------------------------------------------------+
|                  2D 世界视图                      |
|                                                  |
|  玩家 / NPC / AOI 圆 / Grid / Tower / 事件闪烁      |
|                                                  |
+----------------------+---------------------------+
| 当前实体状态          | AOI 事件日志               |
| PlayerId             | enter entity              |
| Position             | leave entity              |
| Visible entities     | update entity             |
+----------------------+---------------------------+
```

### 4.1 世界视图

需要显示：

1. 当前玩家。
2. 其他玩家。
3. NPC。
4. 当前玩家 AOI 半径。
5. 当前 AOI 内对象高亮。
6. 地图边界。
7. Grid 或 Tower 边界。
8. 对象进入 AOI 时短暂闪烁。
9. 对象离开 AOI 时在日志中记录。

### 4.2 颜色建议

可以使用类似规则：

```text
当前玩家：蓝色
其他玩家：绿色
NPC：橙色
AOI 内对象：高亮描边
AOI 外对象：不显示，或者调试模式下半透明显示
AOI 圆：半透明圆圈
Grid / Tower 边界：浅灰线
进入 AOI：绿色闪烁
离开 AOI：红色日志
```

### 4.3 调试模式

客户端应支持 Debug Mode。

Debug Mode 开启时：

1. 显示所有服务器对象。
2. 显示当前玩家实际 AOI 范围。
3. 显示 Grid / Tower。
4. 显示当前 AOI 算法名称。
5. 显示当前服务端 Tick。
6. 显示当前网络延迟估计。
7. 显示当前客户端收到的实体数量。

Debug Mode 关闭时：

1. 只显示当前玩家可见对象。
2. 模拟真实网游客户端视野。

---

## 5. 服务端总体架构

### 5.1 服务端核心结构

```text
WorldServer
  ├── NetworkServer
  ├── World
  │    ├── Map
  │    ├── EntityManager
  │    ├── PlayerManager
  │    ├── NPCManager
  │    ├── AOIManager
  │    └── SyncManager
  ├── TickLoop
  └── Config
```

### 5.2 Tick 模型

服务端使用固定 Tick 驱动世界更新。

示例：

```text
20 TPS = 每 50ms 更新一次
```

每个 Tick 执行：

```text
1. 读取客户端输入
2. 更新玩家位置
3. 更新 NPC AI
4. 更新实体位置
5. 调用 AOIManager 更新兴趣区域
6. 生成 enter / leave / update 事件
7. SyncManager 向客户端发送同步消息
```

伪代码：

```go
func (s *WorldServer) Tick(dt time.Duration) {
    s.InputSystem.ApplyInputs()
    s.NPCSystem.Update(dt)
    s.EntityManager.Update(dt)

    events := s.AOIManager.Update(s.World)

    s.SyncManager.Dispatch(events)
}
```

---

## 6. 实体模型设计

### 6.1 Entity 接口

```go
type EntityID string

type EntityType string

const (
    EntityPlayer EntityType = "player"
    EntityNPC    EntityType = "npc"
)

type Vec2 struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

type Entity interface {
    ID() EntityID
    Type() EntityType
    Position() Vec2
    Radius() float64
}
```

### 6.2 BaseEntity

```go
type BaseEntity struct {
    Id       EntityID
    Kind     EntityType
    Pos      Vec2
    Velocity Vec2
    RadiusV  float64
}
```

### 6.3 Player

```go
type Player struct {
    BaseEntity
    ConnID       string
    AOIRadius    float64
    VisibleSet   map[EntityID]bool
    LastInputSeq  uint64
}
```

### 6.4 NPC

```go
type NPC struct {
    BaseEntity
    Behavior NPCBehavior
}
```

---

## 7. AOI 插件架构

### 7.1 AOIManager 接口

所有 AOI 算法都实现同一个接口。

```go
type AOIManager interface {
    Name() string

    Init(world *World) error

    AddEntity(entity Entity) error
    RemoveEntity(entityID EntityID) error
    MoveEntity(entityID EntityID, oldPos Vec2, newPos Vec2) error

    Query(entityID EntityID, radius float64) ([]EntityID, error)

    Update(world *World) ([]AOIEvent, error)
}
```

### 7.2 AOIEvent

```go
type AOIEventType string

const (
    AOIEnter  AOIEventType = "enter"
    AOILeave  AOIEventType = "leave"
    AOIUpdate AOIEventType = "update"
)

type AOIEvent struct {
    ObserverID EntityID     `json:"observer_id"`
    TargetID   EntityID     `json:"target_id"`
    Type       AOIEventType `json:"type"`
}
```

含义：

```text
ObserverID：观察者，例如玩家 A
TargetID：被观察对象，例如 NPC 01 或玩家 B
Type：进入视野、离开视野、状态更新
```

### 7.3 AOI 插件选择

服务端配置文件中选择 AOI 算法：

```yaml
server:
  tick_rate: 20
  listen_addr: "0.0.0.0:8100"

world:
  width: 2000
  height: 2000

aoi:
  type: "grid"
  radius: 200
  grid_size: 200
```

支持：

```text
bruteforce
grid
tower
```

未来可扩展：

```text
quadtree
sweep_prune
kd_tree
```

但本期不实现这些复杂算法。

---

## 8. 本期 AOI 算法范围

### 8.1 BruteForceAOI

用途：

1. 作为最简单的正确性基准。
2. 用于测试 GridAOI 和 TowerAOI 的结果是否一致。
3. 适合小规模实体数量。

实现方式：

```text
每个玩家遍历所有实体
如果距离小于 AOI 半径，则进入可见集合
```

复杂度：

```text
O(P * N)
```

其中：

```text
P = 玩家数量
N = 总实体数量
```

伪代码：

```go
func (a *BruteForceAOI) Query(observer Entity, radius float64) []EntityID {
    result := []EntityID{}

    for _, target := range a.entities {
        if target.ID() == observer.ID() {
            continue
        }

        if Distance(observer.Position(), target.Position()) <= radius {
            result = append(result, target.ID())
        }
    }

    return result
}
```

---

### 8.2 GridAOI / 九宫格 AOI

用途：

1. 本期主力算法。
2. 适合解释 MMORPG 中常见地图分区。
3. 方便可视化。

地图按固定大小切分为 Grid。

例如：

```text
地图大小：2000 x 2000
Grid 大小：200 x 200
则地图被切成 10 x 10 个格子
```

每个 Grid 维护其中的实体列表。

```go
type GridCoord struct {
    X int
    Y int
}

type GridCell struct {
    Coord    GridCoord
    Entities map[EntityID]bool
}
```

查询时：

```text
1. 找到观察者所在 Grid
2. 查询附近若干 Grid
3. 收集候选实体
4. 再做精确距离过滤
```

注意：不要只固定查九宫格，而应该根据 AOI 半径和 Grid Size 计算覆盖范围。

```go
gridRange := int(math.Ceil(radius / gridSize))
```

然后查询：

```text
x - gridRange 到 x + gridRange
y - gridRange 到 y + gridRange
```

这样即使 AOI 半径大于 Grid Size，也不会漏对象。

伪代码：

```go
func (a *GridAOI) Query(observer Entity, radius float64) []EntityID {
    center := a.PosToGrid(observer.Position())
    r := int(math.Ceil(radius / a.GridSize))

    candidates := []EntityID{}

    for gx := center.X - r; gx <= center.X + r; gx++ {
        for gy := center.Y - r; gy <= center.Y + r; gy++ {
            cell := a.GetCell(gx, gy)
            if cell == nil {
                continue
            }

            for id := range cell.Entities {
                candidates = append(candidates, id)
            }
        }
    }

    return a.FilterByDistance(observer, candidates, radius)
}
```

---

### 8.3 TowerAOI / 订阅式 AOI

TowerAOI 可以理解为 GridAOI 的事件驱动版本。

每个 Tower 维护：

```text
1. 当前在这个 Tower 中的实体
2. 正在观察这个 Tower 的玩家
```

玩家进入某个 Tower 后，会订阅周围若干 Tower。

当某个实体进入 Tower 时，可以通知订阅该 Tower 的玩家。

```go
type Tower struct {
    Coord     TowerCoord
    Entities  map[EntityID]bool
    Watchers  map[EntityID]bool
}
```

玩家移动时需要处理：

```text
1. 玩家是否跨 Tower
2. 如果跨 Tower：
   - 取消订阅旧 Tower 范围
   - 订阅新 Tower 范围
   - 计算新增可见对象
   - 计算移除可见对象
```

TowerAOI 的意义：

1. 强调“观察者订阅区域”。
2. 适合事件驱动同步。
3. 更接近 MMO 服务端中常见的兴趣管理模型。

本期实现可以简化：

```text
Tower 大小 = Grid 大小
订阅范围 = 根据 AOI 半径计算周围 Tower
实体进入 Tower 时，不直接广播完整状态，而是生成 AOIEvent
```

---

### 8.4 PriorityAOIDecorator

PriorityAOIDecorator 不作为独立 AOI 空间算法，而是包装在其他 AOI 算法之上。

结构：

```text
GridAOI -> PriorityAOIDecorator -> SyncManager
```

作用：

1. AOI 算法负责找出“可见对象”。
2. PriorityAOIDecorator 负责排序和限流。
3. SyncManager 根据优先级决定同步频率。

优先级示例：

```text
当前玩家自己：最高
当前目标：最高
距离很近的对象：高
移动中的对象：中
静止 NPC：低
远距离玩家：低
```

本期可以先只预留接口，不强制实现完整优先级同步。

```go
type AOIPriorityEvaluator interface {
    Priority(observer EntityID, target EntityID, world *World) int
}
```

---

## 9. 暂不实现的算法

### 9.1 四叉树 / 八叉树

原因：

1. 动态插入、删除、移动维护成本较高。
2. 热点区域需要频繁拆分和合并。
3. 首期目标是可视化 AOI 和网络同步，不适合引入复杂空间树。

可以保留未来接口：

```text
QuadtreeAOI
```

但本期不实现。

### 9.2 KD-Tree / R-Tree

原因：

1. 更适合偏静态空间查询。
2. 动态更新成本较高。
3. 和实时移动同步不是最自然的组合。

### 9.3 十字链表 / Sweep and Prune

原因：

1. 适合碰撞检测和连续移动对象筛选。
2. 二维 AOI 下需要维护多轴排序。
3. 和 Grid/Tower 插件架构差异较大。
4. 首期实现会增加调试难度。

---

## 10. 网络模型

### 10.1 连接方式

客户端通过 WebSocket 连接服务端。

示例：

```text
ws://192.168.1.10:8100/ws
```

服务端监听：

```text
0.0.0.0:8100
```

这样局域网内其他机器可以通过服务端机器 IP 加入。

### 10.2 消息协议

本期使用 JSON。

通用消息结构：

```json
{
  "type": "message_type",
  "seq": 1,
  "payload": {}
}
```

### 10.3 客户端到服务端消息

#### JoinWorld

```json
{
  "type": "join_world",
  "payload": {
    "name": "player_001"
  }
}
```

#### Input

```json
{
  "type": "input",
  "seq": 1001,
  "payload": {
    "up": true,
    "down": false,
    "left": false,
    "right": true
  }
}
```

#### Ping

```json
{
  "type": "ping",
  "payload": {
    "client_time": 123456789
  }
}
```

### 10.4 服务端到客户端消息

#### Welcome

```json
{
  "type": "welcome",
  "payload": {
    "player_id": "player_abc",
    "map_width": 2000,
    "map_height": 2000,
    "aoi_type": "grid",
    "aoi_radius": 200,
    "grid_size": 200
  }
}
```

#### EntityEnter

```json
{
  "type": "entity_enter",
  "payload": {
    "entity": {
      "id": "npc_001",
      "type": "npc",
      "x": 500,
      "y": 300
    }
  }
}
```

#### EntityLeave

```json
{
  "type": "entity_leave",
  "payload": {
    "entity_id": "npc_001"
  }
}
```

#### EntityUpdate

```json
{
  "type": "entity_update",
  "payload": {
    "entities": [
      {
        "id": "npc_001",
        "type": "npc",
        "x": 510,
        "y": 305
      }
    ],
    "server_tick": 12345
  }
}
```

#### Pong

```json
{
  "type": "pong",
  "payload": {
    "client_time": 123456789,
    "server_time": 987654321
  }
}
```

---

## 11. 同步模型

### 11.1 本期同步策略

本期采用最简单的服务器权威状态同步。

客户端发送输入：

```text
客户端 -> 服务端：我正在按 W/D
```

服务端计算位置：

```text
服务端更新玩家坐标
```

服务端广播状态：

```text
服务端 -> AOI 内客户端：实体位置变化
```

客户端只负责表现。

### 11.2 客户端表现

客户端收到 EntityUpdate 后：

1. 更新本地实体状态。
2. 对远程实体做简单插值。
3. 当前玩家可以先不做预测，降低首期复杂度。
4. 后续可加入客户端预测。

---

## 12. 预留网络同步算法扩展

虽然本期不实现复杂同步算法，但架构应预留 SyncStrategy。

```go
type SyncStrategy interface {
    Name() string
    BuildMessages(player *Player, world *World, events []AOIEvent) []OutboundMessage
}
```

本期实现：

```text
SnapshotSyncStrategy
```

含义：

```text
服务端定期向客户端发送当前可见对象状态。
```

未来可以扩展：

### 12.1 DeltaSyncStrategy

只发送变化字段。

```text
上次同步：x=100, y=100
本次同步：x=110, y=100
只发送 x 变化
```

### 12.2 InterpolationSyncStrategy

服务端发送带时间戳的状态快照，客户端延迟播放并插值。

### 12.3 ClientPredictionStrategy

客户端本地预测自己移动，服务端回包后校正。

### 12.4 ReconciliationStrategy

客户端保存输入序列，服务端确认后进行误差修正。

### 12.5 InterestPrioritySyncStrategy

结合 AOI 优先级，重要对象高频同步，不重要对象低频同步。

本期只需要定义接口和目录结构，不需要实现复杂算法。

---

## 13. 地图架构设计

### 13.1 Map

```go
type WorldMap struct {
    Width  float64
    Height float64
}
```

### 13.2 World

```go
type World struct {
    Map      *WorldMap
    Entities *EntityManager
    AOI      AOIManager
    Sync     SyncStrategy
}
```

### 13.3 EntityManager

```go
type EntityManager struct {
    entities map[EntityID]Entity
}
```

需要提供：

```go
func (m *EntityManager) Add(entity Entity)
func (m *EntityManager) Remove(id EntityID)
func (m *EntityManager) Get(id EntityID) Entity
func (m *EntityManager) All() []Entity
func (m *EntityManager) Players() []*Player
func (m *EntityManager) NPCs() []*NPC
```

---

## 14. AOI 状态维护

每个玩家需要维护当前可见集合：

```go
type VisibleSet map[EntityID]bool
```

每次 AOI 查询后，对比旧集合和新集合。

```go
func DiffVisibleSet(oldSet, newSet map[EntityID]bool) []AOIEvent {
    events := []AOIEvent{}

    for id := range newSet {
        if !oldSet[id] {
            events = append(events, AOIEvent{
                TargetID: id,
                Type: AOIEnter,
            })
        }
    }

    for id := range oldSet {
        if !newSet[id] {
            events = append(events, AOIEvent{
                TargetID: id,
                Type: AOILeave,
            })
        }
    }

    return events
}
```

---

## 15. 服务端启动方式

### 15.1 命令行启动

```bash
go run ./cmd/worldserver --config ./config/dev.yaml
```

### 15.2 配置文件示例

```yaml
server:
  listen_addr: "0.0.0.0:8100"
  tick_rate: 20

world:
  width: 2000
  height: 2000
  npc_count: 100

player:
  speed: 180
  aoi_radius: 200

npc:
  speed: 80
  random_walk: true

aoi:
  type: "grid"
  grid_size: 200

sync:
  type: "snapshot"
  snapshot_rate: 10
```

---

## 16. 客户端启动方式

### 16.1 开发模式

```bash
cd client
npm install
npm run dev
```

### 16.2 连接服务端

页面中提供服务端地址输入框：

```text
ws://localhost:8100/ws
```

局域网连接示例：

```text
ws://192.168.1.10:8100/ws
```

### 16.3 客户端界面控件

需要提供：

1. Server URL 输入框
2. Connect 按钮
3. Disconnect 按钮
4. Debug Mode 开关
5. Show Grid 开关
6. Show AOI Radius 开关
7. Show All Entities 开关
8. 当前 AOI 算法显示
9. 当前 Ping 显示
10. 当前可见对象数量显示

---

## 17. 最小可运行版本范围

MVP 版本必须包含：

### 服务端

1. WebSocket 服务。
2. 玩家连接和断开。
3. 玩家移动。
4. NPC 生成。
5. NPC 随机移动。
6. BruteForceAOI。
7. GridAOI。
8. 服务端配置切换 AOI 算法。
9. EntityEnter / EntityLeave / EntityUpdate 消息。
10. 局域网访问能力。

### 客户端

1. WebSocket 连接。
2. WASD 移动。
3. Canvas/SVG 世界渲染。
4. 玩家和 NPC 显示。
5. AOI 范围显示。
6. Grid 显示。
7. AOI 事件日志。
8. Debug Mode。
9. 支持通过 IP 连接局域网服务器。

---

## 18. 第二阶段功能

第二阶段可以加入：

1. TowerAOI。
2. AOI 算法对比模式。
3. 同屏显示 BruteForceAOI 和 GridAOI 的结果差异。
4. NPC 数量压力测试。
5. 服务端统计面板。
6. 每 Tick AOI 查询耗时。
7. 每 Tick 消息数量。
8. 每客户端可见对象数量。
9. 简单的网络延迟模拟。
10. 丢包模拟。

---

## 19. 第三阶段功能

第三阶段可以实验网络同步算法：

1. Snapshot Sync
2. Delta Sync
3. Client Interpolation
4. Client Prediction
5. Server Reconciliation
6. Interest Priority Sync
7. 低频同步远处对象
8. 高频同步近处对象
9. 事件同步和状态同步混合

---

## 20. 推荐开发顺序

### Step 1：基础服务端

1. 创建 Go 项目。
2. 实现 World、EntityManager、Player、NPC。
3. 实现固定 TickLoop。
4. 实现 NPC 随机移动。

### Step 2：WebSocket 通信

1. 实现客户端连接。
2. 实现 JoinWorld。
3. 实现玩家输入。
4. 实现服务端回传 Welcome 和 EntityUpdate。

### Step 3：客户端可视化

1. 创建 Canvas。
2. 绘制地图。
3. 绘制玩家。
4. 绘制 NPC。
5. 绘制 AOI 半径。
6. 绘制 Grid。

### Step 4：BruteForceAOI

1. 实现暴力查询。
2. 维护玩家 VisibleSet。
3. 生成 enter / leave 事件。
4. 客户端显示事件日志。

### Step 5：GridAOI

1. 实现 Grid 切分。
2. 实现实体加入、移除、移动。
3. 实现基于 Grid 的候选查询。
4. 精确距离过滤。
5. 和 BruteForceAOI 对比结果。

### Step 6：插件化

1. 抽象 AOIManager 接口。
2. 根据配置加载不同 AOI。
3. 确保 BruteForceAOI 和 GridAOI 可以无缝替换。

### Step 7：局域网测试

1. 服务端监听 0.0.0.0。
2. 客户端允许输入服务端 IP。
3. 多台设备连接。
4. 验证玩家互相进入/离开 AOI。

### Step 8：预留同步策略

1. 抽象 SyncStrategy。
2. 当前实现 SnapshotSyncStrategy。
3. 保留 DeltaSyncStrategy、InterpolationSyncStrategy 的接口和 TODO。

---

## 21. 验收标准

### 21.1 基础运行

1. 服务端可以正常启动。
2. 客户端可以连接服务端。
3. 多个客户端可以从局域网不同机器加入。
4. 每个客户端可以控制自己的玩家移动。

### 21.2 AOI 可视化

1. 当前玩家有可见 AOI 半径。
2. NPC 或其他玩家进入 AOI 时，客户端显示 enter 事件。
3. NPC 或其他玩家离开 AOI 时，客户端显示 leave 事件。
4. Debug Mode 下可以看到 Grid。
5. Debug Mode 下可以看到所有实体。

### 21.3 AOI 插件

1. 配置为 bruteforce 时使用 BruteForceAOI。
2. 配置为 grid 时使用 GridAOI。
3. 两种算法在相同场景下返回的可见对象应基本一致。
4. AOIManager 接口清晰，后续可以添加 TowerAOI。

### 21.4 网络能力

1. 服务端监听局域网地址。
2. 客户端可以输入不同 IP 连接。
3. 多客户端移动时，彼此在 AOI 内可以看到对方。
4. 离开 AOI 后对象从客户端视图中消失。

### 21.5 架构扩展

1. AOI 算法可以替换。
2. 同步策略接口已预留。
3. 客户端渲染和网络逻辑分离。
4. 服务端 AOI、Entity、Network、Sync 模块分离。

---

## 22. Codex 实现提示

请优先实现一个可运行的最小版本，不要一次性实现所有高级功能。

优先级如下：

```text
P0:
- Go WebSocket 世界服务器
- Web Canvas 客户端
- 玩家移动
- NPC 随机移动
- BruteForceAOI
- GridAOI
- AOI enter / leave / update 可视化
- 局域网连接

P1:
- TowerAOI
- AOI 算法切换 UI
- 服务端统计信息
- AOI 查询耗时统计

P2:
- 同步算法扩展
- 延迟/丢包模拟
- 客户端预测
- 插值同步
```

代码实现时请保持模块边界清晰，不要把 AOI、网络通信、实体更新、客户端渲染混在一起。

建议先做出端到端闭环：

```text
服务端启动
-> 客户端连接
-> 玩家出现在地图
-> 玩家移动
-> NPC 进入 AOI
-> 客户端收到 entity_enter
-> NPC 离开 AOI
-> 客户端收到 entity_leave
```

只要这个闭环跑通，再逐步增加算法和调试面板。

---

## 23. 项目最终效果预期

最终 Demo 应该像一个简化的二维 MMO 沙盒：

1. 局域网中多名玩家打开浏览器加入同一世界。
2. 每个玩家只能看到自己 AOI 范围内的实体。
3. 玩家移动时，周围 NPC 和其他玩家会进入/离开视野。
4. 客户端能直观看到 AOI 圆、地图 Grid、触发事件。
5. 服务端可以通过配置切换不同 AOI 算法。
6. 后续可以基于同一架构继续实验状态同步、插值、预测、优先级同步等网络同步机制。

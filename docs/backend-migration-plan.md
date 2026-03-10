# 《我的放置仙途》前后端化改造完整方案（单机→网络游戏）

## 1. 目标与原则

### 1.1 改造目标
- 将当前“纯前端 + IndexedDB 本地存档”模式升级为“前端 + 服务端 + 数据库”的网络游戏架构。
- 保持当前核心玩法和数值体验不变：修炼、突破、探索、抽卡、背包、炼丹、秘境、成就。
- 新增三大在线系统：
  - 排行榜系统
  - 拍卖行系统
  - 聊天系统

### 1.2 设计原则
- **玩法不变，数据迁移**：核心公式尽量保留，仅迁移到服务端权威执行。
- **服务端权威**：所有资源变更、战斗结算、抽卡、交易由后端计算并落库。
- **渐进重构**：采用“兼容期 + 开关式迁移”避免一次性大爆炸改造。
- **可运营可扩展**：为未来跨服、赛季、活动、反作弊预留结构。

---

## 2. 现状映射（来自当前代码）

当前项目核心状态集中在 Pinia 的 `player` store，包括角色属性、资源、背包、装备、灵宠、成就、秘境统计等，后续应拆分为多个后端领域服务。可参考现有字段与行为定义。 

- 玩家状态定义与主要业务动作在 `src/stores/player.js`。
- 玩法配置和算法在 `src/plugins/*.js`（如境界、探索、战斗、丹药、装备、成就）。
- 当前本地存档使用 IndexedDB + AES 字符串加密（仅客户端可信）。

---

## 3. 目标技术架构（推荐）

## 3.1 架构分层

### 前端（保留 Vue 方案）
- 继续使用 Vue3 + Pinia + Naive UI。
- Pinia 从“业务计算中心”变为“状态展示缓存层 + API 调用层”。
- 新增 API SDK：`src/api/` 按模块封装请求。

### 网关层
- **API Gateway（Nginx / Kong / APISIX）**：
  - 统一鉴权（JWT）
  - 限流
  - 路由转发
  - 灰度发布

### 应用层（推荐 Golang）
- 鉴权与账号服务（Auth）
- 玩家核心服务（Player）
- 玩法服务（Game Logic）
- 社交服务（Chat）
- 交易服务（Auction）
- 排行榜服务（Rank）


### Golang 后端工程建议
- 推荐框架：`Gin` / `Fiber`（HTTP API）+ `gorilla/websocket` 或 `nhooyr/websocket`（实时聊天）。
- ORM 与数据库：`GORM` 或 `sqlc + pgx`（更强类型约束）。
- 配置与依赖：`Viper`（配置）+ `wire` / `fx`（依赖注入，可选）。
- 鉴权与安全：`golang-jwt/jwt`、`oauth2`、`go-playground/validator`。
- 可观测性：`OpenTelemetry` + `Prometheus` + `Zap` 结构化日志。
- 并发与任务：Goroutine + Channel，异步任务建议配合 `Redis Streams` / `Kafka`。
- 项目结构建议：
  - `cmd/api`（程序入口）
  - `internal/auth|player|game|rank|auction|chat`（领域模块）
  - `internal/repo`（数据访问）
  - `internal/service`（业务服务）
  - `internal/transport/http|ws`（接口层）
  - `pkg`（通用工具）

### 数据层
- **PostgreSQL**：强一致核心数据（账号、角色、库存、订单、拍卖）
- **Redis**：
  - 排行榜 ZSET
  - 会话缓存
  - 防重/分布式锁
  - 聊天频道消息缓存
- **对象存储（可选）**：头像、公告资源等。

### 异步层
- 消息队列（RabbitMQ / Kafka / Redis Streams）
  - 行为日志
  - 排行榜异步刷新
  - 聊天审核管线
  - 运营统计

---

## 4. 服务端领域拆分与职责

## 4.1 Auth 服务（仅 Linux.do OAuth2）
- **唯一登录方式**：仅允许 Linux.do OAuth2.0 登录，不提供用户名密码注册/登录、不提供游客模式。
- 首次登录自动注册：根据 Linux.do 返回的唯一用户标识（`sub` 或等价唯一字段）自动创建本地账号与角色。
- 使用 JWT Access + Refresh Token 维持会话；本地仅保存会话凭证，不保存可伪造的核心角色态。
- 账号绑定规则：`linux_do_user_id` 全局唯一，不允许一个 Linux.do 账号绑定多个本地账号。

## 4.2 Player 服务
- 玩家基础资料、资源资产、角色属性。
- 原先 `playerStore` 的字段映射到后端模型。
- 提供快照查询接口（登录后拉取完整角色态）。

## 4.3 Game Logic 服务（核心）
- 修炼、突破、探索、事件、掉落、抽卡、炼丹、秘境、成就判定全部服务端执行。
- 前端仅发“意图指令”（如 `startCultivate`, `explore(locationId)`, `performGacha(times)`），后端返回结果。
- 保留现有公式（从 `plugins` 迁移），确保体验一致。

## 4.4 Rank 服务
- 维护多榜单：
  - 境界榜（level）
  - 修为榜（cultivation_total）
  - 战力榜（综合战力）
  - 秘境榜（最高层）
  - 财富榜（灵石）
- Redis ZSET + 周期持久化到 PostgreSQL。
- 支持全服榜、好友榜、赛季榜。

## 4.5 Auction 服务
- 上架、下架、购买、竞价（可选）、手续费、邮件结算。
- 交易对象：装备、丹药、材料（灵草）、部分可交易灵宠（需限制）。
- 核心保障：
  - 资产冻结与解冻
  - 订单状态机
  - 幂等扣款
  - 事务一致性（库存/资金/订单）

## 4.6 Chat 服务
- 世界频道、宗门频道（未来）、私聊频道、系统频道。
- WebSocket 实时通信。
- 敏感词过滤、禁言、举报、消息撤回（可选）。
- 消息分片存储与最近消息回溯。

---

## 5. 数据库设计（核心表）

## 5.1 账号与角色
- `users`：账号主表（新增 `linux_do_user_id`、`linux_do_username`、`linux_do_avatar`、`last_login_at`）
- `oauth_accounts`：OAuth 绑定表（provider 固定为 `linux_do`，保存 `subject`、token 元数据、过期时间）
- `player_profiles`：角色基础资料（名称、境界、头像、创建时间）
- `player_resources`：灵力、灵石、强化石、洗练石等
- `player_attributes`：基础属性、战斗属性、抗性、特殊属性（建议 JSONB + version）

## 5.2 背包与养成
- `player_items`：背包物品实例（装备/丹药/灵宠）
- `player_equips`：已穿戴槽位映射
- `player_pets`：灵宠实例（等级、星级、战斗属性）
- `player_herbs`：灵草库存（可聚合存储）
- `player_recipes`：已解锁丹方
- `player_effects`：生效中的丹药BUFF

## 5.3 玩法进度
- `player_achievements`：成就完成与领取状态
- `player_dungeon_progress`：秘境最高层/击杀统计
- `player_exploration_stats`：探索次数/事件触发等
- `player_cultivation_stats`：修炼时长/突破次数

## 5.4 社交与交易
- `chat_messages`
- `chat_mutes`
- `auction_orders`
- `auction_bids`（若支持竞价）
- `mail_box`（交易结算与系统补偿）

## 5.5 审计与反作弊
- `economy_logs`（资产流水）
- `game_action_logs`（关键行为）
- `risk_events`（风控命中）
- `recharge_orders`（充值订单：外部积分订单号、状态机、到账灵石、幂等键）
- `recharge_callbacks`（第三方回调原文与签名验签结果，便于审计追溯）

---

## 6. API 设计（示例）

## 6.1 鉴权（Linux.do OAuth2 专用）
- `GET /auth/linux-do/authorize`：重定向到 Linux.do OAuth 授权页
- `GET /auth/linux-do/callback`：处理授权回调，首次自动注册，签发本地 JWT
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /auth/me`：获取当前会话绑定的 Linux.do 用户信息

## 6.2 玩家与主循环
- `GET /player/snapshot`：拉取完整玩家状态
- `POST /game/cultivation/once`
- `POST /game/cultivation/until-breakthrough`
- `POST /game/breakthrough`
- `POST /game/exploration/start`
- `POST /game/gacha/perform`
- `POST /game/alchemy/craft`
- `POST /game/dungeon/start`
- `POST /game/dungeon/next-turn`
- `POST /game/item/use`

## 6.3 排行榜
- `GET /rankings?type=realm&scope=global`
- `GET /rankings/self?type=power`
- `GET /rankings/friends?type=dungeon`

## 6.4 拍卖行
- `GET /auction/list`
- `POST /auction/create`
- `POST /auction/cancel`
- `POST /auction/buy`
- `POST /auction/bid`（可选）
- `GET /auction/my-orders`

## 6.5 聊天（WebSocket + REST）
- `WS /chat/connect`
- `WS event: chat.send`
- `WS event: chat.receive`
- `GET /chat/history?channel=world`
- `POST /chat/report`


## 6.6 充值（credit.linux.do 积分系统）
- `POST /recharge/create`：创建充值订单（指定兑换档位/灵石数量），服务端调用 credit.linux.do 下单
- `POST /recharge/callback`：接收 credit.linux.do 回调（验签 + 幂等）
- `GET /recharge/orders`：查询我的充值记录
- `GET /recharge/products`：查询可售充值档位（防前端硬编码）

---

## 7. 新增系统详细方案

## 7.1 排行榜系统

### 榜单类型（首期）
1. 境界榜（`level desc`）
2. 战力榜（基于战斗属性统一计算）
3. 秘境榜（最高层 + 通关时间）
4. 财富榜（灵石 + 资产折算）

### 刷新机制
- 关键行为触发异步更新（突破、装备变化、秘境结算）。
- Redis ZSET 维护实时排名：`rank:{type}:{season}`。
- 定时落库快照（每5~10分钟） + 每日结算归档。

### 防刷策略
- 仅服务端写榜。
- 战力计算统一在后端函数，避免前端自报。
- 异常增幅触发风控审计。

## 7.2 拍卖行系统

### 交易规则建议
- 可交易：装备、灵草、丹药；灵宠先限制仅“未绑定”可交易。
- 上架费：固定费用 + 成交手续费（例如 5%）。
- 有效期：6/12/24小时。
- 最低价与最高价限制，防止洗钱。

### 交易流程
1. 玩家上架：冻结物品 → 创建订单。
2. 买家购买：扣灵石 → 订单锁定。
3. 事务提交：
   - 卖家得款（扣手续费）
   - 买家得物品
   - 写资金流水
4. 失败回滚：解冻物品/退款。

### 并发一致性
- 订单行加锁（`SELECT ... FOR UPDATE`）。
- 幂等键（`X-Idempotency-Key`）防止重复提交。
- Redis 分布式锁辅助高并发抢单。

## 7.3 聊天系统

### 频道设计
- `world` 世界频道（全服）
- `system` 系统公告
- `private:{uid}` 私聊
- （未来）`guild:{id}` 宗门频道

### 关键能力
- 登录后下发 WS token，鉴权入会。
- 消息链路：
  - 客户端发送 -> 网关 -> Chat 服务
  - 敏感词过滤/频率限制 -> 广播
- 历史消息：每频道最近 N 条。

### 风控与治理
- 频控（如 3 秒 1 条，突发桶）。
- 违禁词库（可热更新）。
- 管理员禁言、举报审核。

---


## 7.4 充值系统（credit.linux.do → 灵石到账）

### 目标
- 仅通过 `https://credit.linux.do/docs/api` 提供的积分 API 完成充值，不接入其他支付渠道。
- 充值成功后按兑换比例发放游戏内“灵石”。

### 业务流程（推荐）
1. 前端选择充值档位（如 6/30/68/128 元等价档位，具体以后台配置为准）。
2. 前端调用 `POST /recharge/create`，后端创建本地订单（`PENDING`）并调用 credit.linux.do 创建外部订单。
3. 用户在 Linux.do 积分系统完成支付/扣积分。
4. credit.linux.do 回调 `POST /recharge/callback` 到后端。
5. 后端执行：
   - 验签（签名、时间戳、来源 IP 白名单可选）
   - 幂等检查（`external_order_id` + `status`）
   - 事务到账（增加 `player_resources.spirit_stones`）
   - 写入 `economy_logs` 与 `recharge_callbacks`
6. 返回成功，订单状态更新为 `SUCCESS`；失败则 `FAILED` 或 `MANUAL_REVIEW`。

### 到账与风控规则
- **到账唯一凭证**：仅以服务端收到并验签通过的回调为准，不信任前端“支付成功”状态。
- **幂等到账**：同一个 `external_order_id` 只能加一次灵石。
- **金额映射配置化**：`recharge_products` 表维护“积分金额 -> 灵石数”，支持活动倍率。
- **异常处理**：
  - 回调重复：直接返回成功并不重复到账
  - 金额不一致：进入人工复核队列
  - 验签失败：记录风控事件并拒绝

### 数据模型补充
- `recharge_products(id, code, credit_amount, spirit_stones, bonus_rate, enabled)`
- `recharge_orders(id, user_id, product_code, credit_amount, spirit_stones, external_order_id, status, idempotency_key, created_at, paid_at)`
- `recharge_callbacks(id, external_order_id, payload, signature_valid, received_at)`

### 前端改造点
- 设置页或商城页新增“灵石充值”入口。
- 创建订单后跳转到 Linux.do 积分支付页。
- 提供“订单轮询 + 最终以服务端订单状态为准”的到账提示。


## 8. 前端改造方案

## 8.1 Store 分层改造
- 新建 `stores/session.js`：账号态、token。
- `stores/player.js` 保留展示字段，但业务 action 改为调后端 API。
- 新建 `stores/ranking.js`、`stores/auction.js`、`stores/chat.js`。

## 8.2 网络层
- 新建 `src/api/http.ts`（axios/fetch 封装）：
  - 自动注入 token
  - 刷新 token
  - 统一错误处理
  - 幂等键支持
- 新建 `src/api/modules/*.ts`：按业务拆分。

## 8.3 页面与路由
- 新增页面：
  - `views/Ranking.vue`
  - `views/Auction.vue`
  - `views/Chat.vue`（可做底部常驻悬浮）
- 菜单增加入口，保持现有页面结构风格。

## 8.4 兼容阶段策略
- 增加 `USE_SERVER_MODE` 开关：
  - false：仍走本地（开发联调前）
  - true：走服务端
- 逐玩法切换，避免一次性重写。

---

## 9. 数值与玩法迁移策略

## 9.1 迁移顺序
1. 先迁移“资源变更类”玩法（修炼、突破、探索）
2. 再迁移“随机与经济”玩法（抽卡、炼丹、掉落）
3. 再迁移“战斗复杂逻辑”（秘境回合）
4. 最后开放“玩家交互”（排行榜、拍卖、聊天）

## 9.2 一致性校验
- 以当前前端公式作为基线，建立后端单元测试，保证同输入同输出。
- 建立“回放用例集”：同种子下比较旧前端与新后端结算结果。

---

## 10. 安全与反作弊设计

- 所有关键行为后端校验：
  - 资源足够性
  - 冷却时间
  - 状态合法性（例如装备必须在背包中）
- 请求签名（可选）+ 重放保护（nonce + timestamp）。
- 经济系统强审计：所有货币与高价值道具必须有流水。
- 关键接口限流：抽卡、拍卖购买、聊天发送、充值下单与回调。
- 异常检测：短时资源暴涨、异常高频行为、价格异常交易。

---

## 11. 测试与发布方案

## 11.1 测试层次
- 单元测试：玩法公式、状态机、订单流程。
- 集成测试：API + DB + Redis。
- 压测：
  - 聊天广播吞吐
  - 排行榜读写并发
  - 拍卖抢单并发
- 回归测试：保证原玩法体验不变。

## 11.2 发布流程
1. Dev：本地联调
2. Staging：小规模内测服
3. 灰度：10% 用户切换 server mode
4. 全量：关闭本地写路径

## 11.3 监控告警
- Prometheus + Grafana + Loki（或 ELK）
- 指标重点：
  - 接口错误率
  - 下单失败率
  - 聊天延迟
  - 排行榜刷新延迟
  - 经济系统异常波动

---

## 12. 分阶段排期（12~16 周参考）

### Phase 0（1~2 周）：基础设施
- Golang 服务端脚手架、数据库初始化、网关、CI/CD。
- 建立统一中间件（鉴权、限流、日志、trace、recover、幂等键）。
- 完成 Linux.do OAuth2 应用配置（client id/secret/callback）。

### Phase 1（2~3 周）：核心账号与玩家快照
- Linux.do OAuth 登录、首次自动注册、角色创建、玩家快照读写、资产流水。
- 打通 credit.linux.do 充值最小闭环（下单、回调、灵石到账）。

### Phase 2（3~4 周）：基础玩法迁移
- 修炼/突破/探索/事件/背包基础接口。

### Phase 3（2~3 周）：随机与养成迁移
- 抽卡、炼丹、装备强化洗练、灵宠升级升星。

### Phase 4（2~3 周）：秘境战斗迁移
- 回合战斗后端化、结算与掉落。

### Phase 5（2~3 周）：新增在线系统
- 排行榜 + 拍卖行 + 聊天。

### Phase 6（1~2 周）：灰度与收尾
- 数据迁移工具、风控调优、运营后台基础功能。

---

## 13. 团队分工建议

- 前端 2 人：界面改造、API 接入、状态管理改造。
- 后端 3~4 人：Auth/Player、Game Logic、Auction/Chat/Rank。
- 测试 1~2 人：自动化、压测、回归。
- 运维 1 人：部署、监控、告警、容量规划。

---

## 14. 关键风险与规避

1. **玩法迁移偏差**：建立公式一致性测试和回放比对。
2. **经济系统漏洞**：先做流水与审计，再开拍卖。
3. **并发导致的资产不一致**：订单事务 + 幂等 + 锁。
4. **聊天被滥用**：先上频控与敏感词，再开放全量。
5. **前端改动面过大**：采用功能开关与模块化迁移。

---

## 15. 最终落地建议（简版决策）

- 后端技术优先选：**Golang + PostgreSQL + Redis + WebSocket**。
- 账号策略：仅 Linux.do OAuth2 登录，首次自动注册。
- 充值策略：仅接入 credit.linux.do 积分 API，回调验签后发放灵石。
- 短期目标：先把当前单机版做成“可登录、可同步、可多端”的在线版。
- 中期目标：上线排行榜与聊天，再逐步开放拍卖行（建议拍卖行最后上线）。
- 长期目标：引入赛季机制、跨服排行、工会/宗门体系。

---

## 16. 你可以直接执行的下一步

1. 先确定 Golang 技术细节与部署环境（框架选型、云厂商、容器方案）。
2. 我可以继续给你输出：
   - **数据库 ER 图（详细字段级）**
   - **API 协议文档（OpenAPI 草案）**
   - **前端改造任务拆解（按页面和 store）**
   - **排行榜 / 拍卖行 / 聊天 的最小可用版本（MVP）详细实现清单**



---

## 17. 对你新增约束的最终确认

- 登录：仅 Linux.do OAuth2.0（不再支持本地注册/密码登录/游客登录）。
- 注册：首次 OAuth 成功后自动创建角色。
- 充值：仅通过 credit.linux.do 积分系统完成，支付成功以服务端回调验签为准。
- 货币发放：仅服务端事务写入“灵石”，并写审计流水，前端不可直接改值。

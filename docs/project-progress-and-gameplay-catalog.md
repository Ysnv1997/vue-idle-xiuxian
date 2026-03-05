# 项目开发进度与玩法清单（2026-03-05）

> 数据来源：后端权威配置（`backend/internal/service/*` + `backend/internal/http/handler/*`）。

## 一、开发进度总览

| 阶段 | 状态 | 当前结论 | 关键落点 |
|---|---|---|---|
| Phase 0 基础设施 | 已完成 | Go 服务、PostgreSQL 迁移、统一路由与中间件已跑通 | `backend/cmd/api/main.go`、`backend/internal/http/router/router.go` |
| Phase 1 账号与快照 | 已完成（充值未接） | Linux.do OAuth、JWT 会话、玩家快照已完成；充值仅有表结构，业务接口未接 | `auth_handler.go`、`player_handler.go`、`0001_initial_schema.sql` |
| Phase 2 基础玩法迁移 | 已完成 | 修炼/突破/探索/背包基础读写后端化 | `game_service.go`、`exploration_service.go`、`inventory_service.go` |
| Phase 3 随机与养成迁移 | 已完成 | 抽卡、炼丹、装备强化/洗练、灵宠养成后端化 | `gacha_service.go`、`alchemy_service.go`、`equipment_service.go` |
| Phase 4 秘境战斗迁移 | 已完成 | 秘境开始、回合推进、奖励与失败结算后端化 | `dungeon_service.go` |
| Phase 5 在线系统 | 已完成（MVP） | 排行榜、拍卖行、聊天接口与前端页面已接入 | `ranking_service.go`、`auction_service.go`、`chat_service.go` |
| 成就后端化（你刚完成） | 已完成 | 成就判定/发奖/进度已后端权威执行，前端改为 API 驱动；游戏动作后自动同步成就 | `achievement_service.go`、`Achievements.vue`、`game_handler.go` |
| Phase 6 灰度与收尾 | 未开始 | 风控、运营后台、灰度开关完善待推进 | `docs/backend-migration-plan.md` |
| 充值系统（credit.linux.do） | 未开始 | 当前仅有数据表，尚无服务与路由 | `recharge_*` tables in `0001_initial_schema.sql` |

## 二、等级/境界经验清单

共 **126** 级（到大罗九重）。

| 等级 | 境界 | 当前境界修为上限（maxCultivation） |
|---:|---|---:|
| 1 | 练气一重 | 100 |
| 2 | 练气二重 | 200 |
| 3 | 练气三重 | 300 |
| 4 | 练气四重 | 400 |
| 5 | 练气五重 | 500 |
| 6 | 练气六重 | 600 |
| 7 | 练气七重 | 700 |
| 8 | 练气八重 | 800 |
| 9 | 练气九重 | 900 |
| 10 | 筑基一重 | 1000 |
| 11 | 筑基二重 | 1200 |
| 12 | 筑基三重 | 1400 |
| 13 | 筑基四重 | 1600 |
| 14 | 筑基五重 | 1800 |
| 15 | 筑基六重 | 2000 |
| 16 | 筑基七重 | 2200 |
| 17 | 筑基八重 | 2400 |
| 18 | 筑基九重 | 2600 |
| 19 | 金丹一重 | 3000 |
| 20 | 金丹二重 | 3500 |
| 21 | 金丹三重 | 4000 |
| 22 | 金丹四重 | 4500 |
| 23 | 金丹五重 | 5000 |
| 24 | 金丹六重 | 5500 |
| 25 | 金丹七重 | 6000 |
| 26 | 金丹八重 | 6500 |
| 27 | 金丹九重 | 7000 |
| 28 | 元婴一重 | 8000 |
| 29 | 元婴二重 | 9000 |
| 30 | 元婴三重 | 10000 |
| 31 | 元婴四重 | 11000 |
| 32 | 元婴五重 | 12000 |
| 33 | 元婴六重 | 13000 |
| 34 | 元婴七重 | 14000 |
| 35 | 元婴八重 | 15000 |
| 36 | 元婴九重 | 16000 |
| 37 | 化神一重 | 18000 |
| 38 | 化神二重 | 20000 |
| 39 | 化神三重 | 22000 |
| 40 | 化神四重 | 24000 |
| 41 | 化神五重 | 26000 |
| 42 | 化神六重 | 28000 |
| 43 | 化神七重 | 30000 |
| 44 | 化神八重 | 32000 |
| 45 | 化神九重 | 35000 |
| 46 | 返虚一重 | 40000 |
| 47 | 返虚二重 | 45000 |
| 48 | 返虚三重 | 50000 |
| 49 | 返虚四重 | 55000 |
| 50 | 返虚五重 | 60000 |
| 51 | 返虚六重 | 65000 |
| 52 | 返虚七重 | 70000 |
| 53 | 返虚八重 | 75000 |
| 54 | 返虚九重 | 80000 |
| 55 | 合体一重 | 90000 |
| 56 | 合体二重 | 100000 |
| 57 | 合体三重 | 110000 |
| 58 | 合体四重 | 120000 |
| 59 | 合体五重 | 130000 |
| 60 | 合体六重 | 140000 |
| 61 | 合体七重 | 150000 |
| 62 | 合体八重 | 160000 |
| 63 | 合体九重 | 170000 |
| 64 | 大乘一重 | 200000 |
| 65 | 大乘二重 | 230000 |
| 66 | 大乘三重 | 260000 |
| 67 | 大乘四重 | 290000 |
| 68 | 大乘五重 | 320000 |
| 69 | 大乘六重 | 350000 |
| 70 | 大乘七重 | 380000 |
| 71 | 大乘八重 | 410000 |
| 72 | 大乘九重 | 450000 |
| 73 | 渡劫一重 | 500000 |
| 74 | 渡劫二重 | 550000 |
| 75 | 渡劫三重 | 600000 |
| 76 | 渡劫四重 | 650000 |
| 77 | 渡劫五重 | 700000 |
| 78 | 渡劫六重 | 750000 |
| 79 | 渡劫七重 | 800000 |
| 80 | 渡劫八重 | 850000 |
| 81 | 渡劫九重 | 900000 |
| 82 | 仙人一重 | 1000000 |
| 83 | 仙人二重 | 1200000 |
| 84 | 仙人三重 | 1400000 |
| 85 | 仙人四重 | 1600000 |
| 86 | 仙人五重 | 1800000 |
| 87 | 仙人六重 | 2000000 |
| 88 | 仙人七重 | 2200000 |
| 89 | 仙人八重 | 2400000 |
| 90 | 仙人九重 | 2600000 |
| 91 | 真仙一重 | 3000000 |
| 92 | 真仙二重 | 3500000 |
| 93 | 真仙三重 | 4000000 |
| 94 | 真仙四重 | 4500000 |
| 95 | 真仙五重 | 5000000 |
| 96 | 真仙六重 | 5500000 |
| 97 | 真仙七重 | 6000000 |
| 98 | 真仙八重 | 6500000 |
| 99 | 真仙九重 | 7000000 |
| 100 | 金仙一重 | 8000000 |
| 101 | 金仙二重 | 9000000 |
| 102 | 金仙三重 | 10000000 |
| 103 | 金仙四重 | 11000000 |
| 104 | 金仙五重 | 12000000 |
| 105 | 金仙六重 | 13000000 |
| 106 | 金仙七重 | 14000000 |
| 107 | 金仙八重 | 15000000 |
| 108 | 金仙九重 | 16000000 |
| 109 | 太乙一重 | 20000000 |
| 110 | 太乙二重 | 24000000 |
| 111 | 太乙三重 | 28000000 |
| 112 | 太乙四重 | 32000000 |
| 113 | 太乙五重 | 36000000 |
| 114 | 太乙六重 | 40000000 |
| 115 | 太乙七重 | 44000000 |
| 116 | 太乙八重 | 48000000 |
| 117 | 太乙九重 | 52000000 |
| 118 | 大罗一重 | 60000000 |
| 119 | 大罗二重 | 70000000 |
| 120 | 大罗三重 | 80000000 |
| 121 | 大罗四重 | 90000000 |
| 122 | 大罗五重 | 100000000 |
| 123 | 大罗六重 | 110000000 |
| 124 | 大罗七重 | 120000000 |
| 125 | 大罗八重 | 130000000 |
| 126 | 大罗九重 | 140000000 |

## 三、探索地图清单

| 地图ID | 名称 | 最低等级 | 灵力消耗 | 奖励池 |
|---|---|---:|---:|---|
| newbie_village | 新手村 | 1 | 50 | spirit_stone 30% (1~3)；herb 30% (1~2)；cultivation 20% (5~10)；pill_fragment 20% (1~1) |
| celestial_mountain | 天阙峰 | 10 | 1500 | spirit_stone 25% (30~60)；herb 30% (15~25)；cultivation 25% (150~300)；pill_fragment 20% (6~10) |
| phoenix_valley | 凤凰谷 | 19 | 2000 | spirit_stone 25% (50~100)；herb 30% (20~35)；cultivation 25% (250~500)；pill_fragment 20% (8~12) |
| dragon_abyss | 龙渊 | 28 | 3000 | spirit_stone 25% (80~150)；herb 30% (30~50)；cultivation 25% (400~800)；pill_fragment 20% (10~15) |
| immortal_realm | 仙界入口 | 37 | 5000 | spirit_stone 25% (150~300)；herb 30% (50~100)；cultivation 25% (800~1500)；pill_fragment 20% (15~20) |

## 四、灵草清单

| 灵草ID | 名称 | 分类 | 基础价值 | 基础权重 |
|---|---|---|---:|---:|
| spirit_grass | 灵精草 | spirit | 10 | 0.4 |
| cloud_flower | 云雾花 | cultivation | 15 | 0.3 |
| thunder_root | 雷击根 | attribute | 25 | 0.15 |
| dragon_breath_herb | 龙息草 | special | 40 | 0.1 |
| immortal_jade_grass | 仙玉草 | special | 60 | 0.05 |
| dark_yin_grass | 玄阴草 | spirit | 30 | 0.2 |
| nine_leaf_lingzhi | 九叶灵芝 | cultivation | 45 | 0.12 |
| purple_ginseng | 紫金参 | attribute | 50 | 0.08 |
| frost_lotus | 寒霜莲 | spirit | 55 | 0.07 |
| fire_heart_flower | 火心花 | attribute | 35 | 0.15 |
| moonlight_orchid | 月华兰 | spirit | 70 | 0.04 |
| sun_essence_flower | 日精花 | cultivation | 75 | 0.03 |
| five_elements_grass | 五行草 | attribute | 80 | 0.02 |
| phoenix_feather_herb | 凤羽草 | special | 85 | 0.015 |
| celestial_dew_grass | 天露草 | special | 90 | 0.01 |

### 灵草品质系数

| 品质ID | 显示名 | 价值系数 | 掉落概率（代码逻辑） |
|---|---|---:|---|
| common | 普通 | 1 | 50% |
| uncommon | 优质 | 1.5 | 30% |
| rare | 稀有 | 2 | 15% |
| epic | 极品 | 3 | 4% |
| legendary | 仙品 | 5 | 1% |

## 五、丹方清单

| 丹方ID | 名称 | 品阶 | 类型 | 残页需求 | 材料 | 基础效果 | 持续时长(s) |
|---|---|---|---|---:|---|---|---:|
| spirit_gathering | 聚灵丹 | 一品 | 灵力类 | 10 | spirit_grass x2；cloud_flower x1 | spiritRate +0.2 | 3600 |
| cultivation_boost | 聚气丹 | 二品 | 修炼类 | 15 | cloud_flower x2；thunder_root x1 | cultivationRate +0.3 | 1800 |
| thunder_power | 雷灵丹 | 三品 | 属性类 | 20 | thunder_root x2；dragon_breath_herb x1 | combatBoost +0.4 | 900 |
| immortal_essence | 仙灵丹 | 四品 | 特殊类 | 25 | dragon_breath_herb x2；immortal_jade_grass x1 | allAttributes +0.5 | 600 |
| five_elements_pill | 五行丹 | 五品 | 属性类 | 30 | five_elements_grass x2；phoenix_feather_herb x1 | allAttributes +0.8 | 1200 |
| celestial_essence_pill | 天元丹 | 六品 | 修炼类 | 35 | celestial_dew_grass x2；moonlight_orchid x1 | cultivationRate +1 | 1800 |
| sun_moon_pill | 日月丹 | 七品 | 灵力类 | 40 | sun_essence_flower x2；moonlight_orchid x2 | spiritCap +1.5 | 2400 |
| phoenix_rebirth_pill | 涅槃丹 | 八品 | 特殊类 | 45 | phoenix_feather_herb x3；celestial_dew_grass x1 | autoHeal +0.1 | 3600 |
| spirit_recovery | 回灵丹 | 二品 | 灵力类 | 15 | dark_yin_grass x2；frost_lotus x1 | spiritRecovery +0.4 | 1200 |
| essence_condensation | 凝元丹 | 三品 | 修炼类 | 20 | nine_leaf_lingzhi x2；purple_ginseng x1 | cultivationEfficiency +0.5 | 1500 |
| mind_clarity | 清心丹 | 三品 | 特殊类 | 20 | frost_lotus x2；fire_heart_flower x1 | comprehension +0.3 | 2400 |
| fire_essence | 火元丹 | 四品 | 属性类 | 25 | fire_heart_flower x2；dragon_breath_herb x1 | fireAttribute +0.6 | 1800 |

## 六、抽卡配置清单

### 装备品质概率

| 品质ID | 名称 | 概率 | 属性倍率(StatMod) | 颜色 |
|---|---|---:|---:|---|
| common | 凡品 | 50.00% | 1 | #9e9e9e |
| uncommon | 下品 | 30.00% | 1.2 | #4caf50 |
| rare | 中品 | 12.00% | 1.5 | #2196f3 |
| epic | 上品 | 5.00% | 2 | #9c27b0 |
| legendary | 极品 | 2.00% | 2.5 | #ff9800 |
| mythic | 仙品 | 1.00% | 3 | #e91e63 |

### 装备部位池

| 类型ID | 名称 | 对应槽位 |
|---|---|---|
| weapon | 武器 | weapon |
| head | 头部 | head |
| body | 衣服 | body |
| legs | 裤子 | legs |
| feet | 鞋子 | feet |
| shoulder | 肩甲 | shoulder |
| hands | 手套 | hands |
| wrist | 护腕 | wrist |
| necklace | 项链 | necklace |
| ring1 | 戒指1 | ring1 |
| ring2 | 戒指2 | ring2 |
| belt | 腰带 | belt |
| artifact | 法宝 | artifact |

### 灵宠稀有度概率

| 稀有度ID | 名称 | 概率 | 放生精华奖励 | 颜色 |
|---|---|---:|---:|---|
| divine | 神品 | 0.20% | 50 | #FF0000 |
| celestial | 仙品 | 5.81% | 30 | #FFD700 |
| mystic | 玄品 | 16.01% | 20 | #9932CC |
| spiritual | 灵品 | 28.01% | 10 | #1E90FF |
| mortal | 凡品 | 49.97% | 5 | #32CD32 |

### 抽卡规则

| 规则项 | 当前值 |
|---|---|
| 支持类型 | `all` / `equipment` / `pet` |
| 单抽基础消耗 | 100 灵石 |
| 开启心愿单消耗 | 200 灵石/抽 |
| 单次抽卡上限 | 100 抽 |
| 灵宠仓库上限（非装备池） | 100 只 |

## 七、秘境清单

### 难度与结算

| 项目 | 规则 |
|---|---|
| 可选难度 | 1 / 2 / 5 / 10 / 100 |
| 每层灵石奖励 | `10 * floor * difficulty` |
| 精英层（5,15,25...） | 额外 `difficulty` 个洗练石 |
| BOSS层（10,20,30...） | 记一次 `boss_kills` |
| 增益选择层 | 第1层 + 每逢5层 |
| 可刷新次数 | 开局随机 1~3 次（在可选增益层生效） |

### 秘境增益池

| 增益ID | 名称 | 品质 | 描述 |
|---|---|---|---|
| heal | 气血增加 | common | 增加10%血量 |
| small_buff | 小幅强化 | common | 增加10%伤害 |
| defense_boost | 铁壁 | common | 提升20%防御力 |
| speed_boost | 疾风 | common | 提升15%速度 |
| crit_boost | 会心 | common | 提升15%暴击率 |
| dodge_boost | 轻身 | common | 提升15%闪避率 |
| vampire_boost | 吸血 | common | 提升10%吸血率 |
| combat_boost | 战意 | common | 提升10%战斗属性 |
| defense_master | 防御大师 | rare | 防御力提升10% |
| crit_mastery | 会心精通 | rare | 暴击率提升10%，暴击伤害提升20% |
| dodge_master | 无影 | rare | 闪避率提升10% |
| combo_master | 连击精通 | rare | 连击率提升10% |
| vampire_master | 血魔 | rare | 吸血率提升5% |
| stun_master | 震慑 | rare | 眩晕率提升5% |
| ultimate_power | 极限突破 | epic | 所有战斗属性提升50% |
| divine_protection | 天道庇护 | epic | 最终减伤提升30% |
| combat_master | 战斗大师 | epic | 所有战斗属性和抗性提升25% |
| immortal_body | 不朽之躯 | epic | 生命上限提升100%，最终减伤提升20% |
| celestial_might | 天人合一 | epic | 所有战斗属性提升40%，生命值增加50% |
| battle_sage_supreme | 战圣至尊 | epic | 暴击率提升40%，暴击伤害提升80%，最终伤害提升20% |

## 八、成就清单（分类统计）

| 分类 | 数量 | 说明 |
|---|---:|---|
| equipment | 10 | 初获装备 ~ 全能装备师 |
| dungeon_explore | 10 | 初探秘境 ~ 秘境之主 |
| dungeon_combat | 10 | 初战告捷 ~ 无尽战神 |
| cultivation | 10 | 初入修仙 ~ 修炼至尊 |
| breakthrough | 10 | 初窥门径 ~ 突破之神 |
| exploration | 10 | 初探世界 ~ 探索之神 |
| collection | 10 | 初识灵草 ~ 灵草大师 |
| resources | 10 | 初获灵石 ~ 灵石之神 |
| alchemy | 10 | 初识丹道 ~ 仙丹炼师 |
| **合计** | **90** | 后端已迁移并由服务端判定发奖 |

### 成就分类映射

| 分类ID | 中文名 |
|---|---|
| equipment | 装备成就 |
| dungeon_explore | 秘境探索 |
| dungeon_combat | 秘境战斗 |
| cultivation | 修炼成就 |
| breakthrough | 突破成就 |
| exploration | 探索成就 |
| collection | 收集成就 |
| resources | 资源成就 |
| alchemy | 炼丹成就 |

## 九、玩法逻辑（后端权威）

### 1) 通用执行模型

| 环节 | 当前规则 |
|---|---|
| 事务模型 | 核心玩法均在单事务中执行，关键表 `FOR UPDATE` 锁行，避免并发脏写。 |
| 快照口径 | 返回给前端的 `snapshot` 以 `userRepo.GetSnapshot` 为准。 |
| 灵力读值 | 多处按 `spirit + (now-updated_at)*spirit_rate` 计算“当前可用灵力”，不是只读库里静态值。 |
| 错误模型 | 资源不足、条件不满足、非法参数均由后端返回结构化错误，前端只展示。 |
| 成就联动 | 绝大部分 `/game/*` 动作执行后会自动触发一次 `achievementService.Sync`。 |

### 2) 修炼与突破（`game_service.go`）

| 环节 | 当前规则 |
|---|---|
| 单次修炼消耗 | `spiritCost = floor(10 * 1.5^(level-1))` |
| 单次修炼收益 | `baseGain = floor(1 * 1.2^(level-1))`，再乘 `cultivationRate`（向下取整，最少 1） |
| 双倍修为概率 | `0.3 * luck`，并钳制到 `[0,1]` |
| 连续修炼 | 按 `ceil((maxCultivation-cultivation)/baseGain)` 预估次数，一次性校验总灵力是否足够 |
| 突破条件 | `cultivation >= maxCultivation` |
| 突破效果 | `level+1`、`realm/maxCultivation` 更新、`cultivation=0`、`spirit += 100*level`、`spiritRate *= 1.2` |
| 上限行为 | 到最终境界后不再突破（返回已到顶层语义错误） |

### 3) 探索（`exploration_service.go`）

| 环节 | 当前规则 |
|---|---|
| 进入校验 | 地图存在、等级达标、灵力足够。 |
| 事件触发 | `rand < 0.3 * luck` 走事件分支（当前未钳制上限）。 |
| 非事件倍率 | `rand < 0.5 * luck` 时奖励倍率 `1.5`，否则 `1.0`（当前未钳制上限）。 |
| 常规奖励池 | 从地图 reward table 按概率抽 `spirit_stone/herb/cultivation/pill_fragment`。 |
| 灵草奖励 | `herbRate > 1` 时放大数量；每株灵草再独立 roll 品质：50/30/15/4/1。 |
| 修为奖励联动 | 探索给修为时若到达上限会自动触发突破（突破效果与主修炼一致）。 |
| 丹方残页 | 随机丹方 `+1`；达到 `FragmentsNeeded` 自动扣除残页并解锁丹方。 |

### 4) 炼丹（`alchemy_service.go`）

| 环节 | 当前规则 |
|---|---|
| 解锁条件 | `recipeId` 必须存在于 `pill_recipes`。 |
| 材料校验 | 缺料会返回具体缺失明细（草药 ID、需要数量、当前数量）。 |
| 成功率 | `grade.SuccessRate * luck * alchemyRate`，钳制到 `[0,1]`。 |
| 成功结果 | 消耗材料、生成丹药物品、累计炼丹统计。 |
| 失败结果 | 仅返回失败，不消耗材料。 |
| 丹药数值 | `effect.value = baseEffect.value * typeMultiplier * (1 + (level-1)*0.1)`，保留 4 位小数。 |

### 5) 抽卡（`gacha_service.go`）

| 环节 | 当前规则 |
|---|---|
| 基础校验 | `times` 仅允许 `1~100`；`gachaType` 仅允许 `all/equipment/pet`。 |
| 灵宠容量 | 非纯装备池时，抽卡前检查灵宠数是否 `>=100`；当前只在开抽前检查一次。 |
| 消耗 | 普通 `100` 灵石/抽；开启心愿单 `200` 灵石/抽。 |
| 全池逻辑 | `all` 每抽 50% 装备、50% 灵宠；当前实现下不使用心愿单定向。 |
| 心愿单调权 | 目标项概率乘 `(1 + min(1, 0.2/baseProb))`，其余项按比例缩放回总和 1。 |
| 自动处理 | 装备可自动卖出（转强化石）；灵宠可自动放生（不入背包）。 |
| 灵宠精华 | 抽到灵宠即按稀有度加 `petEssence`；不是“放生时才加”。 |

### 6) 背包与灵宠养成（`inventory_service.go`）

| 环节 | 当前规则 |
|---|---|
| 使用物品 | `pill`：写入 `active_effects` 并移除道具；`pet`：切换出战/召回并增减属性。 |
| 卖装备 | 价格来自品质映射（common=1 ... mythic=6），收入为强化石。 |
| 批量卖装备 | 可按品质/类型过滤；只处理装备类型。 |
| 放生灵宠 | 单放/批放均会移除灵宠；批放会跳过当前出战灵宠。 |
| 放生收益 | 当前后端放生不返还灵宠精华。 |
| 升级灵宠 | 消耗 `level*10` 精华，`level+1`；基础属性和百分比属性按稀有度系数成长。 |
| 升星灵宠 | 必须“同名+同稀有度”作为材料；主宠 `star+1`，材料宠移除，并返还部分精华。 |

### 7) 装备（`equipment_service.go`）

| 环节 | 当前规则 |
|---|---|
| 穿戴 | 需满足 `player.level >= requiredRealm`；同槽位旧装备自动卸下回背包。 |
| 卸下 | 从槽位移回背包，并回退对应属性增量。 |
| 强化消耗 | `cost = (enhanceLevel+1) * 10`（强化石）。 |
| 强化成功率 | `1 - 0.05*enhanceLevel`（最低 0）；强化上限 `+100`。 |
| 强化结果 | 成功后每条词条 `*1.1`（百分比词条保留 2 位小数）；失败当前实现不扣资源。 |
| 洗练消耗 | 固定 `10` 洗练石。 |
| 洗练逻辑 | 随机改 1~3 条词条，值在原值约 `±30%` 区间；30% 概率换成该部位可洗新词条。 |
| 属性落地 | 装备词条会实时写入玩家四组属性（基础/战斗/抗性/特殊）。 |

### 8) 秘境（`dungeon_service.go`）

| 环节 | 当前规则 |
|---|---|
| 难度 | 仅允许 `1/2/5/10/100`。 |
| 开始层数 | 按对应难度历史最高层继续（`startFloor=历史最高层`，本次目标层 `start+1`）。 |
| 增益选择层 | 第 1 层和每逢 5 层；每次出现时初始刷新次数随机 `1~3`。 |
| 增益品质概率 | 常规层：rare 25%、epic 5%；5 层：rare 35%、epic 15%；10 层：rare 30%、epic 20%。 |
| 战斗回合 | 最多 10 回合，超时判负；按速度先手，含暴击/连击/眩晕/吸血/闪避/反击。 |
| 胜利结算 | 灵石 `10*floor*difficulty`；精英层额外 `difficulty` 洗练石；更新最高层与击杀统计。 |
| 失败结算 | 结束本次 run；非 100 倍难度损失 10%~50% 当前修为；100 倍难度随机掉 1~3 级。 |

### 9) 成就（`achievement_service.go`）

| 环节 | 当前规则 |
|---|---|
| `List` | 只读当前状态 + 进度，不发奖励。 |
| `Sync` | 遍历全部 90 条成就，满足条件即“完成并立即领取”。 |
| 奖励类型 | `spirit` 加法；`spiritRate/herbRate/alchemyRate/luck` 乘法叠加。 |
| 同步时机 | `game_handler.go` 在多数 `POST /game/*` 成功后自动调用 `Sync` 并把新成就合并回响应。 |
| 进度算法 | `progress = round(min(current/target,1)*100, 2)`。 |

成就判定核心指标（按分类）：

| 分类 | 主要指标 |
|---|---|
| equipment | 装备总数、部位覆盖数、极品/仙品数量、最高强化等级 |
| dungeon_explore | 最高层、进入次数 |
| dungeon_combat | 总击杀、连斩、精英/BOSS 击杀 |
| cultivation | 累计修炼次数（以 `total_cultivation_time` 统计） |
| breakthrough | 突破次数、到达等级里程碑 |
| exploration | 探索次数、事件触发次数、道具发现数 |
| collection | 灵草总数、灵草种类数、稀有/极品/仙品草数量 |
| resources | 灵石持有量里程碑 |
| alchemy | 炼丹总数、高品丹数量、已解锁丹方数 |

### 10) 在线系统（`ranking/auction/chat`）

| 模块 | 当前规则 |
|---|---|
| 排行榜 | 类型支持 `realm/cultivation/dungeon/wealth/power`；范围支持 `global/friends`；`limit` 上限 100。 |
| 关注关系 | 可关注/取关，禁止关注自己；好友榜由“自己 + 我关注的人”组成。 |
| 拍卖行上架 | 仅可交易 `pill + 装备`；上架时物品先从背包移除；时长仅 `6/12/24h`。 |
| 拍卖手续费 | 固定 5%；`sellerIncome = price - fee`。 |
| 拍卖成交 | 一口价购买或卖家接受最高出价；成交后更新双方灵石、道具归属、订单状态与经济日志。 |
| 拍卖出价 | 出价需 `>= 起拍价` 且 `> 当前最高价`；同单只保留 1 条 active 最高出价。 |
| 过期订单 | `SweepExpired` 扫描过期 open 订单，回退物品给卖家并关闭订单。 |
| 聊天发言 | 默认世界频道，内容最长 200 字符，用户最小发言间隔 3 秒。 |
| 聊天风控 | 支持禁言（最长 7 天）、敏感词替换、举报写入 `risk_events`。 |

## 十、补充说明

- 上述“清单 + 逻辑”均以当前后端实现为准，不是策划草案。
- 需要调数值时，优先改 `backend/internal/service/*_data.go`；需要改行为时改对应 `*_service.go`。
- 充值（`credit.linux.do`）仍未接入业务流程，目前只存在数据库表结构。

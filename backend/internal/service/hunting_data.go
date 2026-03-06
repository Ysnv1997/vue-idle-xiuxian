package service

type huntingMapConfig struct {
	ID           string
	Name         string
	Description  string
	MinLevel     int
	RewardFactor float64
	Monsters     []string
}

var huntingMapCatalog = []huntingMapConfig{
	{
		ID:           "qingmu_forest",
		Name:         "青木林",
		Description:  "灵木成荫，常有山猪与灰狼出没，适合初入仙途的修士磨练战技。",
		MinLevel:     1,
		RewardFactor: 1.00,
		Monsters:     []string{"山猪", "灰狼", "野猴"},
	},
	{
		ID:           "heifeng_slope",
		Name:         "黑风坡",
		Description:  "阴风呼啸，妖气渐重，出没的妖兽更为凶悍。",
		MinLevel:     8,
		RewardFactor: 1.10,
		Monsters:     []string{"黑鬃狼", "裂齿虎", "毒尾蜥"},
	},
	{
		ID:           "chiyan_cave",
		Name:         "赤岩洞",
		Description:  "洞中炽热难耐，妖兽皮甲坚硬，战斗节奏更快。",
		MinLevel:     16,
		RewardFactor: 1.22,
		Monsters:     []string{"熔岩蜥", "赤甲蝎", "火鬃猿"},
	},
	{
		ID:           "luoxia_marsh",
		Name:         "落霞泽",
		Description:  "泽气弥漫，幻影重重，若不谨慎容易被群妖围攻。",
		MinLevel:     28,
		RewardFactor: 1.35,
		Monsters:     []string{"雾沼妖鳄", "鬼面蛙王", "腐骨蛇"},
	},
	{
		ID:           "hanyue_valley",
		Name:         "寒月谷",
		Description:  "终年寒气不散，妖兽速度极快，需以攻代守。",
		MinLevel:     40,
		RewardFactor: 1.50,
		Monsters:     []string{"霜牙豹", "寒羽枭", "冰壳龟"},
	},
	{
		ID:           "leiming_plain",
		Name:         "雷鸣原",
		Description:  "雷光游走于天地之间，妖兽狂暴，生死只在一瞬。",
		MinLevel:     55,
		RewardFactor: 1.68,
		Monsters:     []string{"雷角犀", "电爪狼", "暴雷蟒"},
	},
	{
		ID:           "xuanming_rift",
		Name:         "玄冥裂谷",
		Description:  "裂谷幽深，魔气侵蚀心神，唯有强者可深入。",
		MinLevel:     72,
		RewardFactor: 1.88,
		Monsters:     []string{"冥骨将", "噬魂鸦", "裂渊魔犬"},
	},
	{
		ID:           "tianhuo_ruins",
		Name:         "天火遗迹",
		Description:  "古战场残火未熄，凶兽遍地，战后感悟尤深。",
		MinLevel:     90,
		RewardFactor: 2.10,
		Monsters:     []string{"焚天狮", "烬翼鹏", "赤炎魔像"},
	},
	{
		ID:           "jiuxiao_battlefield",
		Name:         "九霄战场",
		Description:  "上古强者陨落之地，杀机与机缘并存，胜者可直指大道。",
		MinLevel:     108,
		RewardFactor: 2.35,
		Monsters:     []string{"天渊魔主", "九霄战灵", "混元古兽"},
	},
}

func findHuntingMapByID(mapID string) (huntingMapConfig, bool) {
	for _, cfg := range huntingMapCatalog {
		if cfg.ID == mapID {
			return cfg, true
		}
	}
	return huntingMapConfig{}, false
}

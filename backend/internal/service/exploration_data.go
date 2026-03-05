package service

type locationRewardRule struct {
	RewardType string
	Chance     float64
	MinAmount  int64
	MaxAmount  int64
}

type explorationLocation struct {
	ID          string
	Name        string
	Description string
	MinLevel    int
	SpiritCost  int64
	Rewards     []locationRewardRule
}

type explorationEvent struct {
	ID          string
	Name        string
	Description string
	Chance      float64
}

type herbDefinition struct {
	ID          string
	Name        string
	Description string
	BaseValue   int64
	Category    string
	Chance      float64
}

type pillRecipeDefinition struct {
	ID              string
	Name            string
	FragmentsNeeded int64
}

var explorationLocations = []explorationLocation{
	{
		ID:          "newbie_village",
		Name:        "新手村",
		Description: "灵气稀薄的凡人聚集地，适合初入修仙之道的修士。",
		MinLevel:    1,
		SpiritCost:  50,
		Rewards: []locationRewardRule{
			{RewardType: "spirit_stone", Chance: 0.3, MinAmount: 1, MaxAmount: 3},
			{RewardType: "herb", Chance: 0.3, MinAmount: 1, MaxAmount: 2},
			{RewardType: "cultivation", Chance: 0.2, MinAmount: 5, MaxAmount: 10},
			{RewardType: "pill_fragment", Chance: 0.2, MinAmount: 1, MaxAmount: 1},
		},
	},
	{
		ID:          "celestial_mountain",
		Name:        "天阙峰",
		Description: "云雾缭绕的仙山，传说是远古仙人讲道之地。",
		MinLevel:    10,
		SpiritCost:  1500,
		Rewards: []locationRewardRule{
			{RewardType: "spirit_stone", Chance: 0.25, MinAmount: 30, MaxAmount: 60},
			{RewardType: "herb", Chance: 0.3, MinAmount: 15, MaxAmount: 25},
			{RewardType: "cultivation", Chance: 0.25, MinAmount: 150, MaxAmount: 300},
			{RewardType: "pill_fragment", Chance: 0.2, MinAmount: 6, MaxAmount: 10},
		},
	},
	{
		ID:          "phoenix_valley",
		Name:        "凤凰谷",
		Description: "常年被火焰环绕的神秘山谷，据说有凤凰遗留的道韵。",
		MinLevel:    19,
		SpiritCost:  2000,
		Rewards: []locationRewardRule{
			{RewardType: "spirit_stone", Chance: 0.25, MinAmount: 50, MaxAmount: 100},
			{RewardType: "herb", Chance: 0.3, MinAmount: 20, MaxAmount: 35},
			{RewardType: "cultivation", Chance: 0.25, MinAmount: 250, MaxAmount: 500},
			{RewardType: "pill_fragment", Chance: 0.2, MinAmount: 8, MaxAmount: 12},
		},
	},
	{
		ID:          "dragon_abyss",
		Name:        "龙渊",
		Description: "深不见底的神秘深渊，蕴含远古真龙的气息。",
		MinLevel:    28,
		SpiritCost:  3000,
		Rewards: []locationRewardRule{
			{RewardType: "spirit_stone", Chance: 0.25, MinAmount: 80, MaxAmount: 150},
			{RewardType: "herb", Chance: 0.3, MinAmount: 30, MaxAmount: 50},
			{RewardType: "cultivation", Chance: 0.25, MinAmount: 400, MaxAmount: 800},
			{RewardType: "pill_fragment", Chance: 0.2, MinAmount: 10, MaxAmount: 15},
		},
	},
	{
		ID:          "immortal_realm",
		Name:        "仙界入口",
		Description: "传说中通往仙界的神秘之地，充满无尽机缘。",
		MinLevel:    37,
		SpiritCost:  5000,
		Rewards: []locationRewardRule{
			{RewardType: "spirit_stone", Chance: 0.25, MinAmount: 150, MaxAmount: 300},
			{RewardType: "herb", Chance: 0.3, MinAmount: 50, MaxAmount: 100},
			{RewardType: "cultivation", Chance: 0.25, MinAmount: 800, MaxAmount: 1500},
			{RewardType: "pill_fragment", Chance: 0.2, MinAmount: 15, MaxAmount: 20},
		},
	},
}

var explorationEvents = []explorationEvent{
	{ID: "ancient_tablet", Name: "古老石碑", Description: "发现一块刻有上古功法的石碑。", Chance: 0.08},
	{ID: "spirit_spring", Name: "灵泉", Description: "偶遇一处天然灵泉。", Chance: 0.12},
	{ID: "ancient_master", Name: "古修遗府", Description: "意外发现一位上古大能的洞府。", Chance: 0.03},
	{ID: "monster_attack", Name: "妖兽袭击", Description: "遭遇一只实力强大的妖兽。", Chance: 0.15},
	{ID: "cultivation_deviation", Name: "走火入魔", Description: "修炼出现偏差，走火入魔。", Chance: 0.12},
	{ID: "treasure_trove", Name: "秘境宝藏", Description: "发现一处上古修士遗留的宝藏。", Chance: 0.05},
	{ID: "enlightenment", Name: "顿悟", Description: "修炼中突然顿悟。", Chance: 0.08},
	{ID: "qi_deviation", Name: "心魔侵扰", Description: "遭受心魔侵扰，修为受损。", Chance: 0.15},
}

var herbDefinitions = []herbDefinition{
	{ID: "spirit_grass", Name: "灵精草", Description: "最常见的灵草，蕴含少量灵气", BaseValue: 10, Category: "spirit", Chance: 0.4},
	{ID: "cloud_flower", Name: "云雾花", Description: "生长在云雾缭绕处的灵花，有助于修炼", BaseValue: 15, Category: "cultivation", Chance: 0.3},
	{ID: "thunder_root", Name: "雷击根", Description: "经过雷霆淬炼的灵根，蕴含强大能量", BaseValue: 25, Category: "attribute", Chance: 0.15},
	{ID: "dragon_breath_herb", Name: "龙息草", Description: "吸收龙气孕育的灵草，极为珍贵", BaseValue: 40, Category: "special", Chance: 0.1},
	{ID: "immortal_jade_grass", Name: "仙玉草", Description: "传说中生长在仙境的灵草，可遇不可求", BaseValue: 60, Category: "special", Chance: 0.05},
	{ID: "dark_yin_grass", Name: "玄阴草", Description: "生长在阴暗处的奇特灵草，具有独特的灵气属性", BaseValue: 30, Category: "spirit", Chance: 0.2},
	{ID: "nine_leaf_lingzhi", Name: "九叶灵芝", Description: "传说中的灵芝，拥有九片叶子，蕴含强大的生命力", BaseValue: 45, Category: "cultivation", Chance: 0.12},
	{ID: "purple_ginseng", Name: "紫金参", Description: "千年紫参，散发着淡淡的黄金，大补元气", BaseValue: 50, Category: "attribute", Chance: 0.08},
	{ID: "frost_lotus", Name: "寒霜莲", Description: "生长在极寒之地的莲花，可以提升修炼者的灵力纯度", BaseValue: 55, Category: "spirit", Chance: 0.07},
	{ID: "fire_heart_flower", Name: "火心花", Description: "生长在火山口的奇花，花心似火焰跳动", BaseValue: 35, Category: "attribute", Chance: 0.15},
	{ID: "moonlight_orchid", Name: "月华兰", Description: "只在月圆之夜绽放的神秘兰花，能吸收月华精华", BaseValue: 70, Category: "spirit", Chance: 0.04},
	{ID: "sun_essence_flower", Name: "日精花", Description: "吸收太阳精华的奇花，蕴含纯阳之力", BaseValue: 75, Category: "cultivation", Chance: 0.03},
	{ID: "five_elements_grass", Name: "五行草", Description: "一株草同时具备金木水火土五种属性的奇珍", BaseValue: 80, Category: "attribute", Chance: 0.02},
	{ID: "phoenix_feather_herb", Name: "凤羽草", Description: "传说生长在不死火凤栖息地的神草，具有涅槃之力", BaseValue: 85, Category: "special", Chance: 0.015},
	{ID: "celestial_dew_grass", Name: "天露草", Description: "凝聚天地精华的仙草，千年一遇", BaseValue: 90, Category: "special", Chance: 0.01},
}

var pillRecipeDefinitions = []pillRecipeDefinition{
	{ID: "spirit_gathering", Name: "聚灵丹", FragmentsNeeded: 10},
	{ID: "cultivation_boost", Name: "聚气丹", FragmentsNeeded: 15},
	{ID: "thunder_power", Name: "雷灵丹", FragmentsNeeded: 20},
	{ID: "immortal_essence", Name: "仙灵丹", FragmentsNeeded: 25},
	{ID: "five_elements_pill", Name: "五行丹", FragmentsNeeded: 30},
	{ID: "celestial_essence_pill", Name: "天元丹", FragmentsNeeded: 35},
	{ID: "sun_moon_pill", Name: "日月丹", FragmentsNeeded: 40},
	{ID: "phoenix_rebirth_pill", Name: "涅槃丹", FragmentsNeeded: 45},
	{ID: "spirit_recovery", Name: "回灵丹", FragmentsNeeded: 15},
	{ID: "essence_condensation", Name: "凝元丹", FragmentsNeeded: 20},
	{ID: "mind_clarity", Name: "清心丹", FragmentsNeeded: 20},
	{ID: "fire_essence", Name: "火元丹", FragmentsNeeded: 25},
}

func explorationLocationByID(locationID string) (explorationLocation, bool) {
	for _, location := range explorationLocations {
		if location.ID == locationID {
			return location, true
		}
	}
	return explorationLocation{}, false
}

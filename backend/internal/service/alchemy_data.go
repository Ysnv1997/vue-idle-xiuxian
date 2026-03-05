package service

type alchemyGradeDefinition struct {
	Name        string
	SuccessRate float64
}

type alchemyTypeDefinition struct {
	Name             string
	EffectMultiplier float64
}

type alchemyMaterial struct {
	Herb  string
	Count int64
}

type alchemyBaseEffect struct {
	Type     string
	Value    float64
	Duration int64
}

type alchemyRecipeDefinition struct {
	ID          string
	Name        string
	Description string
	Grade       string
	Type        string
	Materials   []alchemyMaterial
	BaseEffect  alchemyBaseEffect
}

var alchemyGradeDefinitions = map[string]alchemyGradeDefinition{
	"grade1": {Name: "一品", SuccessRate: 0.9},
	"grade2": {Name: "二品", SuccessRate: 0.8},
	"grade3": {Name: "三品", SuccessRate: 0.7},
	"grade4": {Name: "四品", SuccessRate: 0.6},
	"grade5": {Name: "五品", SuccessRate: 0.5},
	"grade6": {Name: "六品", SuccessRate: 0.4},
	"grade7": {Name: "七品", SuccessRate: 0.3},
	"grade8": {Name: "八品", SuccessRate: 0.2},
	"grade9": {Name: "九品", SuccessRate: 0.1},
}

var alchemyTypeDefinitions = map[string]alchemyTypeDefinition{
	"spirit":      {Name: "灵力类", EffectMultiplier: 1},
	"cultivation": {Name: "修炼类", EffectMultiplier: 1.2},
	"attribute":   {Name: "属性类", EffectMultiplier: 1.5},
	"special":     {Name: "特殊类", EffectMultiplier: 2},
}

var alchemyRecipeDefinitions = []alchemyRecipeDefinition{
	{
		ID:          "spirit_gathering",
		Name:        "聚灵丹",
		Description: "提升灵力恢复速度的丹药",
		Grade:       "grade1",
		Type:        "spirit",
		Materials: []alchemyMaterial{
			{Herb: "spirit_grass", Count: 2},
			{Herb: "cloud_flower", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "spiritRate", Value: 0.2, Duration: 3600},
	},
	{
		ID:          "cultivation_boost",
		Name:        "聚气丹",
		Description: "提升修炼速度的丹药",
		Grade:       "grade2",
		Type:        "cultivation",
		Materials: []alchemyMaterial{
			{Herb: "cloud_flower", Count: 2},
			{Herb: "thunder_root", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "cultivationRate", Value: 0.3, Duration: 1800},
	},
	{
		ID:          "thunder_power",
		Name:        "雷灵丹",
		Description: "提升战斗属性的丹药",
		Grade:       "grade3",
		Type:        "attribute",
		Materials: []alchemyMaterial{
			{Herb: "thunder_root", Count: 2},
			{Herb: "dragon_breath_herb", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "combatBoost", Value: 0.4, Duration: 900},
	},
	{
		ID:          "immortal_essence",
		Name:        "仙灵丹",
		Description: "全属性提升的神奇丹药",
		Grade:       "grade4",
		Type:        "special",
		Materials: []alchemyMaterial{
			{Herb: "dragon_breath_herb", Count: 2},
			{Herb: "immortal_jade_grass", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "allAttributes", Value: 0.5, Duration: 600},
	},
	{
		ID:          "five_elements_pill",
		Name:        "五行丹",
		Description: "融合五行之力的神奇丹药，全面提升修炼者素质",
		Grade:       "grade5",
		Type:        "attribute",
		Materials: []alchemyMaterial{
			{Herb: "five_elements_grass", Count: 2},
			{Herb: "phoenix_feather_herb", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "allAttributes", Value: 0.8, Duration: 1200},
	},
	{
		ID:          "celestial_essence_pill",
		Name:        "天元丹",
		Description: "凝聚天地精华的极品丹药，大幅提升修炼速度",
		Grade:       "grade6",
		Type:        "cultivation",
		Materials: []alchemyMaterial{
			{Herb: "celestial_dew_grass", Count: 2},
			{Herb: "moonlight_orchid", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "cultivationRate", Value: 1.0, Duration: 1800},
	},
	{
		ID:          "sun_moon_pill",
		Name:        "日月丹",
		Description: "融合日月精华的丹药，能大幅提升灵力上限",
		Grade:       "grade7",
		Type:        "spirit",
		Materials: []alchemyMaterial{
			{Herb: "sun_essence_flower", Count: 2},
			{Herb: "moonlight_orchid", Count: 2},
		},
		BaseEffect: alchemyBaseEffect{Type: "spiritCap", Value: 1.5, Duration: 2400},
	},
	{
		ID:          "phoenix_rebirth_pill",
		Name:        "涅槃丹",
		Description: "蕴含不死凤凰之力的神丹，能在战斗中自动恢复生命",
		Grade:       "grade8",
		Type:        "special",
		Materials: []alchemyMaterial{
			{Herb: "phoenix_feather_herb", Count: 3},
			{Herb: "celestial_dew_grass", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "autoHeal", Value: 0.1, Duration: 3600},
	},
	{
		ID:          "spirit_recovery",
		Name:        "回灵丹",
		Description: "快速恢复灵力的丹药",
		Grade:       "grade2",
		Type:        "spirit",
		Materials: []alchemyMaterial{
			{Herb: "dark_yin_grass", Count: 2},
			{Herb: "frost_lotus", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "spiritRecovery", Value: 0.4, Duration: 1200},
	},
	{
		ID:          "essence_condensation",
		Name:        "凝元丹",
		Description: "提升修炼效率的高级丹药",
		Grade:       "grade3",
		Type:        "cultivation",
		Materials: []alchemyMaterial{
			{Herb: "nine_leaf_lingzhi", Count: 2},
			{Herb: "purple_ginseng", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "cultivationEfficiency", Value: 0.5, Duration: 1500},
	},
	{
		ID:          "mind_clarity",
		Name:        "清心丹",
		Description: "提升心境和悟性的丹药",
		Grade:       "grade3",
		Type:        "special",
		Materials: []alchemyMaterial{
			{Herb: "frost_lotus", Count: 2},
			{Herb: "fire_heart_flower", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "comprehension", Value: 0.3, Duration: 2400},
	},
	{
		ID:          "fire_essence",
		Name:        "火元丹",
		Description: "提升火属性修炼速度的丹药",
		Grade:       "grade4",
		Type:        "attribute",
		Materials: []alchemyMaterial{
			{Herb: "fire_heart_flower", Count: 2},
			{Herb: "dragon_breath_herb", Count: 1},
		},
		BaseEffect: alchemyBaseEffect{Type: "fireAttribute", Value: 0.6, Duration: 1800},
	},
}

func alchemyRecipeByID(recipeID string) (alchemyRecipeDefinition, bool) {
	for _, recipe := range alchemyRecipeDefinitions {
		if recipe.ID == recipeID {
			return recipe, true
		}
	}
	return alchemyRecipeDefinition{}, false
}

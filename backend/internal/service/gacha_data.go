package service

type gachaEquipmentQuality struct {
	Name        string
	Color       string
	StatMod     float64
	Probability float64
}

type gachaEquipmentType struct {
	ID       string
	Name     string
	Slot     string
	Prefixes []string
}

type gachaStatRange struct {
	Min float64
	Max float64
}

type gachaPetRarity struct {
	Name         string
	Color        string
	Probability  float64
	EssenceBonus int64
}

type gachaPetTemplate struct {
	Name        string
	Description string
}

var gachaEquipmentQualityOrder = []string{"common", "uncommon", "rare", "epic", "legendary", "mythic"}

var gachaEquipmentQualities = map[string]gachaEquipmentQuality{
	"common":    {Name: "凡品", Color: "#9e9e9e", StatMod: 1.0, Probability: 0.38},
	"uncommon":  {Name: "下品", Color: "#4caf50", StatMod: 1.2, Probability: 0.24},
	"rare":      {Name: "中品", Color: "#2196f3", StatMod: 1.5, Probability: 0.08},
	"epic":      {Name: "上品", Color: "#9c27b0", StatMod: 2.0, Probability: 0.015},
	"legendary": {Name: "极品", Color: "#ff9800", StatMod: 2.5, Probability: 0.0015},
	"mythic":    {Name: "仙品", Color: "#e91e63", StatMod: 3.0, Probability: 0.0005},
}

var gachaEquipmentTypes = []gachaEquipmentType{
	{ID: "weapon", Name: "武器", Slot: "weapon", Prefixes: []string{"九天", "太虚", "混沌", "玄天", "紫霄", "青冥", "赤炎", "幽冥"}},
	{ID: "head", Name: "头部", Slot: "head", Prefixes: []string{"天灵", "玄冥", "紫金", "青玉", "赤霞", "幽月", "星辰", "云霄"}},
	{ID: "body", Name: "衣服", Slot: "body", Prefixes: []string{"九霄", "太素", "混元", "玄阳", "紫薇", "青龙", "赤凤", "幽冥"}},
	{ID: "legs", Name: "裤子", Slot: "legs", Prefixes: []string{"天罡", "玄武", "紫电", "青云", "赤阳", "幽灵", "星光", "云雾"}},
	{ID: "feet", Name: "鞋子", Slot: "feet", Prefixes: []string{"天行", "玄风", "紫霞", "青莲", "赤焰", "幽影", "星步", "云踪"}},
	{ID: "shoulder", Name: "肩甲", Slot: "shoulder", Prefixes: []string{"天护", "玄甲", "紫雷", "青锋", "赤羽", "幽岚", "星芒", "云甲"}},
	{ID: "hands", Name: "手套", Slot: "hands", Prefixes: []string{"天罗", "玄玉", "紫晶", "青钢", "赤金", "幽银", "星铁", "云纹"}},
	{ID: "wrist", Name: "护腕", Slot: "wrist", Prefixes: []string{"天绝", "玄铁", "紫玉", "青石", "赤铜", "幽钢", "星晶", "云纱"}},
	{ID: "necklace", Name: "项链", Slot: "necklace", Prefixes: []string{"天珠", "玄圣", "紫灵", "青魂", "赤心", "幽魄", "星魂", "云珠"}},
	{ID: "ring1", Name: "戒指1", Slot: "ring1", Prefixes: []string{"天命", "玄命", "紫命", "青命", "赤命", "幽命", "星命", "云命"}},
	{ID: "ring2", Name: "戒指2", Slot: "ring2", Prefixes: []string{"天道", "玄道", "紫道", "青道", "赤道", "幽道", "星道", "云道"}},
	{ID: "belt", Name: "腰带", Slot: "belt", Prefixes: []string{"天系", "玄系", "紫系", "青系", "赤系", "幽系", "星系", "云系"}},
	{ID: "artifact", Name: "法宝", Slot: "artifact", Prefixes: []string{"天宝", "玄宝", "紫宝", "青宝", "赤宝", "幽宝", "星宝", "云宝"}},
}

var gachaEquipmentBaseStats = map[string]map[string]gachaStatRange{
	"weapon": {
		"attack":          {Min: 10, Max: 20},
		"critRate":        {Min: 0.05, Max: 0.1},
		"critDamageBoost": {Min: 0.1, Max: 0.3},
	},
	"head": {
		"defense":    {Min: 5, Max: 10},
		"health":     {Min: 50, Max: 100},
		"stunResist": {Min: 0.05, Max: 0.1},
	},
	"body": {
		"defense":           {Min: 8, Max: 15},
		"health":            {Min: 80, Max: 150},
		"finalDamageReduce": {Min: 0.05, Max: 0.1},
	},
	"legs": {
		"defense":   {Min: 6, Max: 12},
		"speed":     {Min: 5, Max: 10},
		"dodgeRate": {Min: 0.05, Max: 0.1},
	},
	"feet": {
		"defense":   {Min: 4, Max: 8},
		"speed":     {Min: 8, Max: 15},
		"dodgeRate": {Min: 0.05, Max: 0.1},
	},
	"shoulder": {
		"defense":     {Min: 5, Max: 10},
		"health":      {Min: 40, Max: 80},
		"counterRate": {Min: 0.05, Max: 0.1},
	},
	"hands": {
		"attack":    {Min: 5, Max: 10},
		"critRate":  {Min: 0.03, Max: 0.08},
		"comboRate": {Min: 0.05, Max: 0.1},
	},
	"wrist": {
		"defense":     {Min: 3, Max: 8},
		"counterRate": {Min: 0.05, Max: 0.1},
		"vampireRate": {Min: 0.05, Max: 0.1},
	},
	"necklace": {
		"health":     {Min: 60, Max: 120},
		"healBoost":  {Min: 0.1, Max: 0.2},
		"spiritRate": {Min: 0.1, Max: 0.2},
	},
	"ring1": {
		"attack":           {Min: 5, Max: 10},
		"critDamageBoost":  {Min: 0.1, Max: 0.2},
		"finalDamageBoost": {Min: 0.05, Max: 0.1},
	},
	"ring2": {
		"defense":          {Min: 5, Max: 10},
		"critDamageReduce": {Min: 0.1, Max: 0.2},
		"resistanceBoost":  {Min: 0.05, Max: 0.1},
	},
	"belt": {
		"health":      {Min: 40, Max: 80},
		"defense":     {Min: 4, Max: 8},
		"combatBoost": {Min: 0.05, Max: 0.1},
	},
	"artifact": {
		"attack":    {Min: 0.1, Max: 0.3},
		"critRate":  {Min: 0.1, Max: 0.3},
		"comboRate": {Min: 0.1, Max: 0.3},
	},
}

var gachaPetRarityOrder = []string{"divine", "celestial", "mystic", "spiritual", "mortal"}

var gachaPetRarities = map[string]gachaPetRarity{
	"divine":    {Name: "神品", Color: "#FF0000", Probability: 0.0003, EssenceBonus: 50},
	"celestial": {Name: "仙品", Color: "#FFD700", Probability: 0.0012, EssenceBonus: 30},
	"mystic":    {Name: "玄品", Color: "#9932CC", Probability: 0.02, EssenceBonus: 20},
	"spiritual": {Name: "灵品", Color: "#1E90FF", Probability: 0.10, EssenceBonus: 10},
	"mortal":    {Name: "凡品", Color: "#32CD32", Probability: 0.23, EssenceBonus: 5},
}

var gachaPetPool = map[string][]gachaPetTemplate{
	"divine": {
		{Name: "玄武", Description: "北方守护神兽"},
		{Name: "白虎", Description: "西方守护神兽"},
		{Name: "朱雀", Description: "南方守护神兽"},
		{Name: "青龙", Description: "东方守护神兽"},
		{Name: "应龙", Description: "上古神龙，掌控风雨"},
		{Name: "麒麟", Description: "祥瑞之兽，通晓万物"},
		{Name: "饕餮", Description: "贪婪之兽，吞噬万物，象征无尽的欲望"},
		{Name: "穷奇", Description: "邪恶之兽，背信弃义，象征混乱与背叛"},
		{Name: "梼杌", Description: "凶暴之兽，顽固不化，象征无法驯服的野性"},
		{Name: "混沌", Description: "无序之兽，无形无相，象征原始的混乱"},
	},
	"celestial": {
		{Name: "囚牛", Description: "龙之长子，喜好音乐，常立于琴头"},
		{Name: "睚眦", Description: "龙之次子，性格刚烈，嗜杀好斗，常刻于刀剑之上"},
		{Name: "嘲风", Description: "龙之三子，形似兽，喜好冒险，常立于殿角"},
		{Name: "蒲牢", Description: "龙之四子，形似龙而小，性好鸣，常铸于钟上"},
		{Name: "狻犴", Description: "龙之五子，形似狮子，喜静好坐，常立于香炉"},
		{Name: "霸下", Description: "龙之六子，形似龟，力大无穷，常背负石碑"},
		{Name: "狴犴", Description: "龙之七子，形似虎，明辨是非，常立于狱门"},
		{Name: "负屃", Description: "龙之八子，形似龙，雅好诗文，常盘于碑顶"},
		{Name: "螭吻", Description: "龙之九子，形似鱼，能吞火，常立于屋脊"},
	},
	"mystic": {
		{Name: "火凤凰", Description: "浴火重生的永恒之鸟"},
		{Name: "雷鹰", Description: "雷电的猛禽"},
		{Name: "冰狼", Description: "冰原霸主"},
		{Name: "岩龟", Description: "坚不可摧的守护者"},
	},
	"spiritual": {
		{Name: "玄龟", Description: "擅长防御的水系灵宠"},
		{Name: "风隼", Description: "速度极快的飞行灵宠"},
		{Name: "地甲", Description: "坚固的大地守护者"},
		{Name: "云豹", Description: "敏捷的猎手"},
	},
	"mortal": {
		{Name: "灵猫", Description: "敏捷的小型灵宠"},
		{Name: "幻蝶", Description: "美丽的蝴蝶灵宠"},
		{Name: "火鼠", Description: "活泼的啮齿类灵宠"},
		{Name: "草兔", Description: "温顺的兔类灵宠"},
	},
}

var gachaEquipmentPriceByQuality = map[string]int64{
	"mythic":    6,
	"legendary": 5,
	"epic":      4,
	"rare":      3,
	"uncommon":  2,
	"common":    1,
}

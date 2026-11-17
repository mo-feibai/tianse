package config

// City is a pre-filled location. The table ships with the binary so the app
// never needs a geolocation permission or a network round trip to know where
// the sun sets.
type City struct {
	Name      string
	Province  string
	Latitude  float64
	Longitude float64
	Timezone  string
}

// Cities is ordered roughly north to south so the settings dropdown reads like
// a map rather than an alphabet.
var Cities = []City{
	{"北京", "北京", 39.9042, 116.4074, "Asia/Shanghai"},
	{"天津", "天津", 39.0842, 117.2009, "Asia/Shanghai"},
	{"哈尔滨", "黑龙江", 45.8038, 126.5350, "Asia/Shanghai"},
	{"长春", "吉林", 43.8171, 125.3235, "Asia/Shanghai"},
	{"沈阳", "辽宁", 41.8057, 123.4315, "Asia/Shanghai"},
	{"大连", "辽宁", 38.9140, 121.6147, "Asia/Shanghai"},
	{"呼和浩特", "内蒙古", 40.8424, 111.7490, "Asia/Shanghai"},
	{"石家庄", "河北", 38.0428, 114.5149, "Asia/Shanghai"},
	{"太原", "山西", 37.8706, 112.5489, "Asia/Shanghai"},
	{"济南", "山东", 36.6512, 117.1201, "Asia/Shanghai"},
	{"青岛", "山东", 36.0671, 120.3826, "Asia/Shanghai"},
	{"郑州", "河南", 34.7466, 113.6254, "Asia/Shanghai"},
	{"西安", "陕西", 34.3416, 108.9398, "Asia/Shanghai"},
	{"兰州", "甘肃", 36.0611, 103.8343, "Asia/Shanghai"},
	{"西宁", "青海", 36.6171, 101.7782, "Asia/Shanghai"},
	{"银川", "宁夏", 38.4872, 106.2309, "Asia/Shanghai"},
	{"乌鲁木齐", "新疆", 43.8256, 87.6168, "Asia/Shanghai"},
	{"拉萨", "西藏", 29.6520, 91.1721, "Asia/Shanghai"},
	{"成都", "四川", 30.5728, 104.0668, "Asia/Shanghai"},
	{"重庆", "重庆", 29.5630, 106.5516, "Asia/Shanghai"},
	{"昆明", "云南", 25.0389, 102.7183, "Asia/Shanghai"},
	{"贵阳", "贵州", 26.6470, 106.6302, "Asia/Shanghai"},
	{"南宁", "广西", 22.8170, 108.3665, "Asia/Shanghai"},
	{"武汉", "湖北", 30.5928, 114.3055, "Asia/Shanghai"},
	{"长沙", "湖南", 28.2282, 112.9388, "Asia/Shanghai"},
	{"南昌", "江西", 28.6820, 115.8579, "Asia/Shanghai"},
	{"合肥", "安徽", 31.8206, 117.2272, "Asia/Shanghai"},
	{"南京", "江苏", 32.0603, 118.7969, "Asia/Shanghai"},
	{"苏州", "江苏", 31.2989, 120.5853, "Asia/Shanghai"},
	{"上海", "上海", 31.2304, 121.4737, "Asia/Shanghai"},
	{"杭州", "浙江", 30.2741, 120.1551, "Asia/Shanghai"},
	{"宁波", "浙江", 29.8683, 121.5440, "Asia/Shanghai"},
	{"福州", "福建", 26.0745, 119.2965, "Asia/Shanghai"},
	{"厦门", "福建", 24.4798, 118.0894, "Asia/Shanghai"},
	{"广州", "广东", 23.1291, 113.2644, "Asia/Shanghai"},
	{"深圳", "广东", 22.5431, 114.0579, "Asia/Shanghai"},
	{"东莞", "广东", 23.0207, 113.7518, "Asia/Shanghai"},
	{"海口", "海南", 20.0444, 110.1999, "Asia/Shanghai"},
	{"三亚", "海南", 18.2528, 109.5119, "Asia/Shanghai"},
	{"香港", "香港", 22.3193, 114.1694, "Asia/Hong_Kong"},
	{"澳门", "澳门", 22.1987, 113.5439, "Asia/Macau"},
	{"台北", "台湾", 25.0330, 121.5654, "Asia/Taipei"},
	{"高雄", "台湾", 22.6273, 120.3014, "Asia/Taipei"},
}

// LookupCity returns the entry for a name, or the first city as a fallback.
func LookupCity(name string) City {
	for _, c := range Cities {
		if c.Name == name {
			return c
		}
	}
	return Cities[0]
}

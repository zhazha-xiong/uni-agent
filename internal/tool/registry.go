package tool

var toolSet = make(map[string]Tool)

// Registry 注册工具
func Registry(tool Tool) {
	toolSet[tool.Name] = tool
}

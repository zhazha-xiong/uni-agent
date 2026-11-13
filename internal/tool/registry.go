package tool

var Set = make(map[string]Tool)

// Registry 注册工具
func Registry(tool Tool) {
	Set[tool.Name] = tool
}

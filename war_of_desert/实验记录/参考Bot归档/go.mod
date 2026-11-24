// 归档目录：声明为独立模块，避免根模块的 go build ./... / go test ./...
// 扫描到中文路径下的包（Go 不接受非 ASCII 导入路径）。
module wod-archive

go 1.22

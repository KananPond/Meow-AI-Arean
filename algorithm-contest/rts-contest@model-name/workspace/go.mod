// 选手工作区自己的模块。
//
// 让你在本地直接 `go build ./submission`，也能 `go run ./cmd/arena` 起裁判。
// replace 指向 arena/，所以 `import "desertarena/workspace/botkit"` 解析到
// 包里那份协议库——和裁判编译你的代码时用的是同一份实现。
//
// 不要在这里加第三方依赖：收卷只带走 submission/<选手名>_bot.go 一个文件，
// 加了依赖也会在收卷后编译失败。
module contestant

go 1.22

require desertarena v0.0.0

replace desertarena => ./arena

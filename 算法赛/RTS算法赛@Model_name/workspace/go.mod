// 选手工作区自己的模块。
//
// 目的只有一个：让你在本地就能 `go build ./submission`，不用把代码拷进引擎目录。
// replace 指向引擎，所以 `import "desertarena/workspace/botkit"` 能解析到赛场那份
// （和裁判编译时用的是同一份实现）。
//
// 不要在这里加第三方依赖：收卷只带走 submission/<选手名>_bot.go 一个文件，
// 加了依赖也会在收卷后编译失败。
module contestant

go 1.22

require desertarena v0.0.0

replace desertarena => ../../../war_of_desert

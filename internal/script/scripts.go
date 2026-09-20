package script

import _ "embed" //匿名导入 embed 包

//go:embed seckill.lua
var SeckillLua string

//go:embed unlock.lua
var UnlockLua string

package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hm-dianping/internal/ctx"
)

// 路由中获取字符串
func pathUint(c *gin.Context, name string) (uint64, error) {
	return strconv.ParseUint(c.Param(name), 10, 64) //param:从路由中获取字符串
}

// 查询参数里获取字符串
func queryUint(c *gin.Context, name string) (uint64, error) {
	return strconv.ParseUint(c.Query(name), 10, 64)
}

// 安全的在路由中获取整数,不合法则返回默认值
func queryInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(name, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

// 把 URL 里的字符串转成数字，若前端没给，就用原来的默认值
func parseSscanf(value string, target *int64) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		*target = parsed
	}
}

// 获取当前用户视图id
func viewerID(c *gin.Context) uint64 {
	user, err := ctx.CurrentUser(c)
	if err != nil {
		return 0
	}
	return user.ID
}

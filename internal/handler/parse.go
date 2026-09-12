package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hm-dianping/internal/ctx"
)

func pathUint(c *gin.Context, name string) (uint64, error) {
	return strconv.ParseUint(c.Param(name), 10, 64)
}

func queryUint(c *gin.Context, name string) (uint64, error) {
	return strconv.ParseUint(c.Query(name), 10, 64)
}

func queryInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(name, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func parseSscanf(value string, target *int64) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		*target = parsed
	}
}

func viewerID(c *gin.Context) uint64 {
	user, err := ctx.CurrentUser(c)
	if err != nil {
		return 0
	}
	return user.ID
}

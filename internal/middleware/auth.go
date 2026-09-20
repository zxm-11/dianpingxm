package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"hm-dianping/internal/config"
	"hm-dianping/internal/constants"
	userctx "hm-dianping/internal/ctx"
	"hm-dianping/internal/model"
	"hm-dianping/internal/response"
)

func Auth(cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		//白名单放行
		path := c.FullPath() //获取当前请求匹配到的路由模板 (如"/user/:id")
		if path == "" {
			path = c.Request.URL.Path
		}
		if isPublic(c.Request.Method, path) {
			loadOptionalUser(c, rdb)
			c.Next()
			return
		}
		//token鉴权
		token := strings.TrimSpace(c.GetHeader("authorization"))
		if token == "" {
			//由于yaml里配置的 'compatible_missing_token: false' 这个分支不生效
			if cfg.Auth.CompatibleMissingToken {
				c.Next()
				return
			}
			c.JSON(http.StatusOK, response.Fail("请先登录"))
			c.Abort()
			return
		}

		if !loadUserByToken(c, rdb, token) {
			c.JSON(http.StatusUnauthorized, response.Fail("登录状态已失效"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// 加载可选的用户-有 token 就顺便解析出用户，没有也不报错
func loadOptionalUser(c *gin.Context, rdb *redis.Client) {
	token := strings.TrimSpace(c.GetHeader("authorization")) //trimspace去掉字符串首尾的空白字符
	if token != "" {
		loadUserByToken(c, rdb, token)
	}
}

// 通过 token 加载用户
func loadUserByToken(c *gin.Context, rdb *redis.Client, token string) bool {
	values, err := rdb.HGetAll(c.Request.Context(), constants.LoginUserKey+token).Result()
	if err != nil || len(values) == 0 {
		return false
	}
	var user model.UserView
	if id, ok := values["id"]; ok {
		parsed, err := parseUint(id)
		if err != nil {
			return false
		}
		user.ID = parsed
	}
	user.NickName = values["nickName"]
	user.Icon = values["icon"]
	if user.ID == 0 {
		return false
	}

	userctx.SaveUser(c, user)
	_ = rdb.Expire(c.Request.Context(), constants.LoginUserKey+token, constants.LoginUserTTL).Err() //设置登录有效期(登录状态刷新)
	return true
}

func isPublic(method, path string) bool {
	switch {
	case method == http.MethodPost && path == "/user/code": //发验证码（还没登录，当然要能调）
		return true
	case method == http.MethodPost && path == "/user/login": //登录本身
		return true
	case strings.HasPrefix(path, "/shop") && method == http.MethodGet: //浏览商铺
		return true
	case strings.HasPrefix(path, "/shop-type") && method == http.MethodGet: //商铺分类
		return true
	case strings.HasPrefix(path, "/voucher") && method == http.MethodGet: //看优惠券
		return true
	case strings.HasPrefix(path, "/upload"): //图片访问
		return true
	case path == "/blog/hot" || path == "/blog/:id" || path == "/blog/likes/:id" || path == "/blog/of/user": //看博客
		return method == http.MethodGet
		//给noroute兜底的case
	case strings.HasPrefix(path, "/blog/") && method == http.MethodGet:
		parts := strings.Split(strings.Trim(path, "/"), "/") //trim:路径开头和结尾多余的 / 去掉，避免拆出来有空字符串
		//strings.Split(s, sep) 的作用是：用分隔符 sep 把字符串 s 切成一个切片
		return len(parts) == 2
	default:
		return false
	}
}

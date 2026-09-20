package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"hm-dianping/internal/config"
	"hm-dianping/internal/handler"
	"hm-dianping/internal/middleware"
	"hm-dianping/internal/service"
)

func New(
	cfg *config.Config,
	rdb *redis.Client,
	userSvc *service.UserService,
	shopSvc *service.ShopService,
	shopTypeSvc *service.ShopTypeService,
	blogSvc *service.BlogService,
	followSvc *service.FollowService,
	voucherSvc *service.VoucherService,
	voucherOrderSvc *service.VoucherOrderService,
	uploadSvc *service.UploadService,
) *gin.Engine {
	r := gin.New()

	// === 全局中间件 ===
	r.Use(gin.Logger(), middleware.Recovery())
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "authorization"},
		AllowCredentials: true,
		AllowOriginFunc:  func(string) bool { return true },
		MaxAge:           12 * time.Hour,
	}))
	r.Use(middleware.Auth(cfg, rdb))

	// === 用户模块 ===
	user := r.Group("/user")
	user.POST("/code", handler.HandleUserSendCode(userSvc)) //获取验证码
	user.POST("/login", handler.HandleUserLogin(userSvc))   //用户登录
	user.POST("/logout", handler.HandleUserLogout(userSvc)) //用户登出(auth)
	user.GET("/me", handler.HandleUserMe())                 //获取当前登录者自己的视图信息(auth)
	user.GET("/info/:id", handler.HandleUserInfo(userSvc))  // 查指定用户的详细资料(auth)

	user.POST("/sign", handler.HandleUserSign(userSvc))             // 用户签到(auth)
	user.GET("/sign/count", handler.HandleUserSignCount(userSvc))   // 本月连续签到天数(auth)
	user.GET("/sign/status", handler.HandleUserSignStatus(userSvc)) // 本月签到日历(auth)
	// === 商户模块 ===
	shop := r.Group("/shop")
	shop.POST("", handler.HandleShopCreate(shopSvc))           //新增商铺(auth)
	shop.PUT("", handler.HandleShopUpdate(shopSvc))            //更新商铺(auth)
	shop.POST("/geo/load", handler.HandleShopGeoLoad(shopSvc)) //导入店铺坐标到 Redis GEO(auth)
	shop.GET("/of/type", handler.HandleShopOfType(shopSvc))    //获取对应商铺类型的商铺
	shop.GET("/of/name", handler.HandleShopOfName(shopSvc))    //手动查询商铺

	r.GET("/shop-type/list", handler.HandleShopTypeList(shopTypeSvc)) //获取商铺列表

	// === 博客模块 ===
	blog := r.Group("/blog")
	blog.POST("", handler.HandleBlogSave(blogSvc))              //用户发布探店博客&推送给粉丝(auth)
	blog.PUT("/like/:id", handler.HandleBlogLike(blogSvc))      //博客点赞(auth)
	blog.GET("/of/me", handler.HandleBlogOfMe(blogSvc))         //查询自己写过的探店博客(auth)
	blog.GET("/hot", handler.HandleBlogHot(blogSvc))            //查询热点探店博客
	blog.GET("/likes/:id", handler.HandleBlogLikes(blogSvc))    //查询一条笔记的点赞用户列表(点赞排行榜)
	blog.GET("/of/user", handler.HandleBlogOfUser(blogSvc))     //查询对应用户的博客
	blog.GET("/of/follow", handler.HandleBlogOfFollow(blogSvc)) //关注流(auth)

	// === 关注模块 ===
	follow := r.Group("/follow")
	follow.GET("/or/not/:id", handler.HandleFollowIsFollow(followSvc)) //显示已关注/未关注(auth)
	follow.GET("/common/:id", handler.HandleFollowCommon(followSvc))   //共同关注

	// === 优惠券模块 ===
	voucher := r.Group("/voucher")
	voucher.POST("", handler.HandleVoucherAdd(voucherSvc))                //添加普通券(auth)
	voucher.POST("/seckill", handler.HandleVoucherAddSeckill(voucherSvc)) //添加秒杀券(auth)
	voucher.GET("/list/:shopId", handler.HandleVoucherList(voucherSvc))   //查询对应商铺的优惠券

	r.POST("/voucher-order/seckill/:id", handler.HandleSeckill(voucherOrderSvc)) //实现秒杀下单(auth)
	r.POST("/upload/blog", handler.HandleUploadBlog(uploadSvc))                  //探店博客图片上传(auth)
	r.GET("/upload/blog/delete", handler.HandleUploadDeleteBlog(uploadSvc))      //探店博客删除(auth)

	// === NoRoute 兜底——兼容旧版路径格式 ===
	r.NoRoute(func(c *gin.Context) {
		path := strings.Trim(c.Request.URL.Path, "/")
		parts := strings.Split(path, "/")
		switch {
		case c.Request.Method == http.MethodGet && len(parts) == 2 && parts[0] == "shop":
			c.Params = append(c.Params, gin.Param{Key: "id", Value: parts[1]})
			handler.HandleShopGet(shopSvc)(c) //查看对应id的商铺
		case c.Request.Method == http.MethodGet && len(parts) == 2 && parts[0] == "blog":
			c.Params = append(c.Params, gin.Param{Key: "id", Value: parts[1]})
			handler.HandleBlogGet(blogSvc)(c) //查看对应id探店博客
		case c.Request.Method == http.MethodGet && len(parts) == 2 && parts[0] == "user":
			c.Params = append(c.Params, gin.Param{Key: "id", Value: parts[1]})
			handler.HandleUserGet(userSvc)(c) //查看对应id的客户视图
		case c.Request.Method == http.MethodPut && len(parts) == 3 && parts[0] == "follow":
			c.Params = append(c.Params, gin.Param{Key: "id", Value: parts[1]}, gin.Param{Key: "isFollow", Value: parts[2]})
			handler.HandleFollowFollow(followSvc)(c) //关注/取关某人
		default:
			c.JSON(http.StatusNotFound, gin.H{"success": false, "errorMsg": "接口不存在"})
		}
	})

	return r
}

package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"hm-dianping/internal/ctx"
	"hm-dianping/internal/model"
	"hm-dianping/internal/service"
)

func HandleBlogSave(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		var blog model.Blog
		if err := c.ShouldBindJSON(&blog); err != nil {
			writeFail(c, err)
			return
		}
		if err := svc.Save(c.Request.Context(), &blog, user.ID); err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, blog.ID)
	}
}

func HandleBlogLike(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		id, err := pathUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		if err := svc.Like(c.Request.Context(), id, user.ID); err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, nil)
	}
}

func HandleBlogOfMe(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		blogs, err := svc.QueryByUser(c.Request.Context(), user.ID, queryInt(c, "current", 1), user.ID)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, blogs)
	}
}

func HandleBlogHot(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		viewer := viewerID(c)
		blogs, err := svc.QueryHot(c.Request.Context(), queryInt(c, "current", 1), viewer) //query是获取当前页码
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, blogs)
	}
}

func HandleBlogGet(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := pathUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		blog, err := svc.QueryByID(c.Request.Context(), id, viewerID(c))
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, blog)
	}
}

func HandleBlogLikes(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := pathUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		users, err := svc.QueryLikes(c.Request.Context(), id)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, users)
	}
}

func HandleBlogOfUser(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := queryUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		blogs, err := svc.QueryByUser(c.Request.Context(), id, queryInt(c, "current", 1), viewerID(c))
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, blogs)
	}
}

func HandleBlogOfFollow(svc *service.BlogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		//获取指定时间戳(默认取现在)
		lastID := int64(time.Now().UnixMilli())
		if v := c.Query("lastId"); v != "" {
			parseSscanf(v, &lastID)
		}
		//获取当前偏移量
		offset := int64(0)
		if v := c.Query("offset"); v != "" {
			parseSscanf(v, &offset)
		}
		result, err := svc.QueryFeed(c.Request.Context(), user.ID, lastID, offset)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, result)
	}
}

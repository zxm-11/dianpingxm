package handler

import (
	"github.com/gin-gonic/gin"

	"hm-dianping/internal/ctx"
	"hm-dianping/internal/service"
)

type sendCodeRequest struct {
	Phone string `json:"phone" form:"phone"`
}

type loginRequest struct {
	Phone string `json:"phone" form:"phone"`
	Code  string `json:"code" form:"code"`
}

func HandleUserSendCode(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sendCodeRequest
		// 兼容 JSON body 和 form-data 两种请求
		if err := c.ShouldBind(&req); err != nil { //shouldbind支持表单(form)和json两种
			// 如果 ShouldBind 失败（可能没 Content-Type），回退到 PostForm/Query
			req.Phone = c.PostForm("phone") //从表单获(存在请求body里)取电话号码
			if req.Phone == "" {
				req.Phone = c.Query("phone") //表单查不到—-获取查询参数 (URL 里 '?' 后面的参数，比如 ?phone=xxx&name=yyy)
			}
		}
		code, err := svc.SendCode(c.Request.Context(), req.Phone)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, code)
	}
}

func HandleUserLogin(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBind(&req); err != nil {
			// 兼容：从 PostForm 读取
			req.Phone = c.PostForm("phone")
			req.Code = c.PostForm("code")
		}
		token, err := svc.Login(c.Request.Context(), req.Phone, req.Code)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, token)
	}
}

func HandleUserLogout(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := svc.Logout(c.Request.Context(), c.GetHeader("authorization")); err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, nil)
	}
}

func HandleUserMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, user)
	}
}

func HandleUserInfo(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := pathUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		info, err := svc.GetInfo(c.Request.Context(), id)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, info)
	}
}

func HandleUserGet(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := pathUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		user, err := svc.GetUserView(c.Request.Context(), id)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, user)
	}
}

// HandleUserSign 用户签到
func HandleUserSign(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		ok, err := svc.Sign(c.Request.Context(), user.ID)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, ok)
	}
}

// HandleUserSignCount 本月连续签到天数
func HandleUserSignCount(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		count, err := svc.SignCount(c.Request.Context(), user.ID)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, count)
	}
}

// HandleUserSignStatus 本月签到日历(1 已签 / 0 未签)
func HandleUserSignStatus(svc *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := ctx.CurrentUser(c)
		if err != nil {
			writeFail(c, err)
			return
		}
		days, err := svc.SignStatus(c.Request.Context(), user.ID)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, days)
	}
}

package ctx

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hm-dianping/internal/model"
)

const userKey = "currentUser"

func SaveUser(c *gin.Context, user model.UserView) {
	c.Set(userKey, user)
}

func CurrentUser(c *gin.Context) (model.UserView, error) {
	value, ok := c.Get(userKey)
	if !ok {
		return model.UserView{}, errors.New("用户未登录")
	}
	user, ok := value.(model.UserView) //类型断言 userkey实际上是[map]string
	if !ok || user.ID == 0 {
		return model.UserView{}, errors.New("用户未登录")
	}
	return user, nil
}

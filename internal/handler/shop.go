package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hm-dianping/internal/model"
	"hm-dianping/internal/service"
)

func HandleShopGet(svc *service.ShopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := pathUint(c, "id")
		if err != nil {
			writeFail(c, err)
			return
		}
		shop, err := svc.GetByID(c.Request.Context(), id)
		if err != nil {
			writeFail(c, err)
			return
		}
		if shop == nil {
			writeFail(c, errors.New("商户信息不存在"))
			return
		}
		writeOK(c, shop)
	}
}

func HandleShopCreate(svc *service.ShopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var shop model.Shop
		if err := c.ShouldBindJSON(&shop); err != nil {
			writeFail(c, err)
			return
		}
		if err := svc.Create(c.Request.Context(), &shop); err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, shop.ID)
	}
}

func HandleShopUpdate(svc *service.ShopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var shop model.Shop
		if err := c.ShouldBindJSON(&shop); err != nil {
			writeFail(c, err)
			return
		}
		if err := svc.Update(c.Request.Context(), &shop); err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, nil)
	}
}

func HandleShopOfType(svc *service.ShopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		typeID, err := queryUint(c, "typeId") //例如GET /shop/of/type?typeId=1&current=1&sortBy=&x=120.149993&y=30.334229
		if err != nil {
			writeFail(c, err)
			return
		}
		current := queryInt(c, "current", 1) //current页码
		//带上了坐标(x/y)就走 Redis GEO 按距离查询，否则普通分页
		x, xErr := strconv.ParseFloat(c.Query("x"), 64)
		y, yErr := strconv.ParseFloat(c.Query("y"), 64)
		if xErr == nil && yErr == nil {
			shops, err := svc.PageByTypeWithGeo(c.Request.Context(), typeID, current, x, y)
			if err != nil {
				writeFail(c, err)
				return
			}
			writeOK(c, shops)
			return
		}
		//没带坐标,普通分页
		shops, err := svc.PageByType(c.Request.Context(), typeID, current)
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, shops)
	}
}

// HandleShopGeoLoad 把店铺经纬度导入 Redis GEO(初始化/修复索引用)
func HandleShopGeoLoad(svc *service.ShopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := svc.LoadShopGeoData(c.Request.Context()); err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, nil)
	}
}

func HandleShopOfName(svc *service.ShopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		shops, err := svc.PageByName(c.Request.Context(), c.Query("name"), queryInt(c, "current", 1))
		if err != nil {
			writeFail(c, err)
			return
		}
		writeOK(c, shops)
	}
}

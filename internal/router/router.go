package router

import (
	"github.com/gin-gonic/gin"
	"github.com/kiritosuki/mover/internal/handler"
)

func SetupRouter(r *gin.Engine) {
	// r.Group() 返回 Group 对象 设置请求统一前缀
	// group.GET("", func)
	// 第一个参数拼接剩余请求路径 第二个参数是传递给哪个函数处理请求
	// 下面 {} 只是为了好看和规范

	// poi 接口
	poiGroup := r.Group("/pois")
	{
		poiGroup.GET("", handler.ListPois)
	}
}

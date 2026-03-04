package handler

import "github.com/gin-gonic/gin"

// ListPois 列出 poi 点 支持条件查询
func ListPois(c *gin.Context) {
	// 获取请求路径中的参数
	name := c.Query("name")
	tybe := c.Query("tybe")
	status := c.Query("status")
	
}

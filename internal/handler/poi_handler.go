package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/dto"
	"github.com/kiritosuki/mover/internal/repository"
	"github.com/kiritosuki/mover/internal/result"
	"go.uber.org/zap"
)

// ListPois godoc
// @Summary 筛选 / 获取 poi 列表
// @Tags Poi
// @Accept json
// @Produce json
// @Param name query string false "poi 名称"
// @Param tybe query int false "poi 类型"
// @Param status query int false "poi 状态"
// @Success 200 {object} result.Result{data=[]model.Poi}
// @Failure 400 {object} result.Result
// @Router /pois [get]
func ListPois(c *gin.Context) {
	Logger.Logger.Info("筛选 / 获取 poi 列表")
	// 声明用于接受请求参数的结构体对象
	listPoisReq := &dto.ListPoisReq{}
	// 把请求参数绑定到结构体中
	if err := c.ShouldBindQuery(listPoisReq); err != nil {
		Logger.Logger.Error("请求参数解析失败！", zap.Error(err))
		result.Fail(c, "请求参数解析失败！", err)
		return
	}
	// 查询数据库
	pois, err := repository.ListPois(listPoisReq)
	if err != nil {
		Logger.Logger.Error("数据库查询错误！", zap.Error(err))
		result.Fail(c, "数据库查询错误！", err)
		return
	}
	// 查询成功
	Logger.Logger.Info("筛选 / 获取 poi 列表成功")
	result.Success(c, pois)
}

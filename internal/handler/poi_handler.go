package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/dto"
	"github.com/kiritosuki/mover/internal/repository"
	"github.com/kiritosuki/mover/internal/result"
	"go.uber.org/zap"
)

// ListPois godoc
// @Summary 筛选/获取poi列表
// @Tags Poi
// @Accept json
// @Produce json
// @Param name query string false "poi 名称"
// @Param tybe query int false "poi 类型"
// @Param status query int false "poi 状态"
// @Success 200 {object} result.Result{data=[]model.Poi}
// @Failure 400 {object} result.Result
// @Router /pois [get]
// 目前查询条件有 name tybe status
func ListPois(c *gin.Context) {
	Logger.Logger.Info("筛选/获取poi列表...")
	// 声明用于接受请求参数的结构体对象
	listPoisReq := &dto.ListPoisReq{}
	// 把请求参数绑定到结构体中
	if err := c.ShouldBindQuery(listPoisReq); err != nil {
		result.Fail(c, "请求参数解析失败！", err)
		return
	}
	Logger.Logger.Info("查询条件", zap.Any("listPoisReq", listPoisReq))
	// 查询数据库
	pois, err := repository.ListPois(listPoisReq)
	if err != nil {
		result.Fail(c, "数据库查询错误！", err)
		return
	}
	// 查询成功
	Logger.Logger.Info("筛选/获取poi列表成功", zap.Any("poi列表", pois))
	result.Success(c, pois)
}

// GetPoi godoc
// @Summary 根据id获取poi信息
// @Tags Poi
// @Accept json
// @Produce json
// @Param id path int true "查询id"
// @Success 200 {object} result.Result{data=model.Poi}
// @Failure 400 {object} result.Result
// @Router /pois/{id} [get]
func GetPoi(c *gin.Context) {
	Logger.Logger.Info("根据id获取poi信息...")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		result.Fail(c, "id需要为整数类型！", err)
		return
	}
	Logger.Logger.Info("查询条件", zap.Any("poi-id", id))
	// 查询数据库
	poi, err := repository.GetPoi(id)
	if err != nil {
		result.Fail(c, "数据库查询错误！", err)
		return
	}
	// 查询成功
	Logger.Logger.Info("根据id获取poi信息成功", zap.Any("poi信息", poi))
	result.Success(c, poi)
}

// ListVehicles godoc
// @Summary 查询/筛选车辆列表
// @Tags Vehicle
// @Accept json
// @Produce json
// @Param status query int false "vehicle 状态"
// @Success 200 {object} result.Result{data=[]model.Vehicle}
// @Failure 400 {object} result.Result
// @Router /vehicles [get]
// 目前查询条件有 status
// TODO 后续可能有更新的查询条件
func ListVehicles(c *gin.Context) {
	Logger.Logger.Info("查询/筛选车辆列表...")
	statusStr := c.Query("status")
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		result.Fail(c, "status需要为整数类型", err)
		return
	}
	Logger.Logger.Info("查询条件", zap.Any("status", status))
	// 查询数据库
	vehicles, err := repository.ListVehicles(status)
	if err != nil {
		result.Fail(c, "数据库查询错误！", err)
		return
	}
	// 查询成功
	Logger.Logger.Info("查询/筛选车辆列表成功", zap.Any("vehicles", vehicles))
	result.Success(c, vehicles)
}

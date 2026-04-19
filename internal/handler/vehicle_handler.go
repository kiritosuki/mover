package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/repository"
	"github.com/kiritosuki/mover/internal/result"
	"go.uber.org/zap"
)

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
	statusStr := c.DefaultQuery("status", "0")
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

// WsListVehicles godoc
// @Summary WebSocket查询/筛选车辆列表
// @Tags Vehicle
// @Accept json
// @Produce json
// @Param status query int false "vehicle 状态"
// @Success 101 {string} string "Switching Protocols to websocket"
// @Router /vehicles/ws [get]
func WsListVehicles(c *gin.Context) {
	Logger.Logger.Info("WebSocket查询/筛选车辆列表...")

	// 升级HTTP连接为WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有来源
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		Logger.Logger.Error("WebSocket升级失败", zap.Error(err))
		// 向客户端发送错误响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请使用WebSocket协议连接",
			"error":   err.Error(),
		})
		return
	}
	defer conn.Close()

	// 获取查询参数
	statusStr := c.DefaultQuery("status", "0")
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		Logger.Logger.Error("status参数错误", zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte(`{"code": 400, "message": "status需要为整数类型"}`))
		return
	}

	Logger.Logger.Info("WebSocket查询条件", zap.Any("status", status))

	// 查询数据库
	vehicles, err := repository.ListVehicles(status)
	if err != nil {
		Logger.Logger.Error("数据库查询错误", zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte(`{"code": 500, "message": "数据库查询错误"}`))
		return
	}

	// 查询成功，发送数据
	Logger.Logger.Info("WebSocket查询/筛选车辆列表成功", zap.Any("vehicles", vehicles))

	// 构建响应
	response := result.Result{
		Code:    200,
		Message: "success",
		Data:    vehicles,
	}

	// 发送JSON数据
	data, err := json.Marshal(response)
	if err != nil {
		Logger.Logger.Error("JSON序列化失败", zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte(`{"code": 500, "message": "JSON序列化失败"}`))
		return
	}

	conn.WriteMessage(websocket.TextMessage, data)
}

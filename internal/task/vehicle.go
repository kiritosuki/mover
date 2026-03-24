package task

import (
	"fmt"
	"sync"
	"time"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/database"
	"github.com/kiritosuki/mover/internal/model"
	"github.com/kiritosuki/mover/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type VehicleProgress struct {
	mu   sync.Mutex
	data map[uint]int
}

var vehicleProgress = VehicleProgress{
	data: make(map[uint]int),
}

// SimulateMoving 模拟车辆行驶
func SimulateMoving() {
	for {
		// 查询正在进行中的任务
		tasks, err := repository.GetRunningTasks() // 你需要实现
		if err != nil {
			Logger.Logger.Error("查询正在进行中的任务失败")
			// 防止一直出错 忙循环
			time.Sleep(1 * time.Second)
			continue
		}
		for _, task := range tasks {
			go moveVehicle(task)
		}
		time.Sleep(constant.VehicleSimulateMovingGap * time.Second)
	}
}

// 车辆行驶的核心逻辑
func moveVehicle(task *model.OrderTask) {
	vehicleID := task.VehicleId
	shipmentID := task.ShipmentId
	// 取路径
	routeStore.mu.Lock()
	points := routeStore.RouteMap[int(shipmentID)]
	routeStore.mu.Unlock()
	// 没有路径信息 直接返回
	if len(points) == 0 {
		return
	}
	// 当前走到哪
	vehicleProgress.mu.Lock()
	idx := vehicleProgress.data[vehicleID]
	// 往前走一个点
	idx++
	// 越界判断 如果这辆车已经走到了这段路的末尾
	if idx >= len(points) {
		vehicleProgress.mu.Unlock()
		handlePathFinished(task)
		return
	}
	// 如果车辆还没走完路径点
	// 写回新索引
	vehicleProgress.data[vehicleID] = idx
	vehicleProgress.mu.Unlock()
	// 取当前位置
	point := points[idx]
	// 更新数据库中的车辆位置
	err := repository.UpdateVehicleLocation(int(vehicleID), point.Lon, point.Lat)
	if err != nil {
		Logger.Logger.Error("更新车辆位置失败")
		return
	}

	// WebSocket 把车辆位置信息推送给前端
	// 与普通http不同 websocket不是接受前端请求 而是建立连接后根据状态变化主动把信息推送给前端
	// TODO 后面前后端对接再实现 现在不用管
	// ws.Broadcast(vehicleID, point)
}

// 到达路段终点 做状态更新
func handlePathFinished(task *model.OrderTask) {
	switch task.Sequential {
	// 到达起点（准备装货）
	case constant.OrderTaskSequentialAccepting:
		Logger.Logger.Info("车辆到达起点，开始装货")
		// 装货（更新车辆载货量）
		err := LoadCargo(task)
		if err != nil {
			Logger.Logger.Error("装货逻辑出现错误", zap.Any("err", err))
		}
		// 更新订单状态：待取货 -> 运输中
		// CAS锁一律格式 旧的status放后面的参数
		err = repository.UpdateShipmentStatus(int(task.ShipmentId), constant.ShipmentStatusWorking, constant.ShipmentStatusWaiting)
		if err != nil {
			Logger.Logger.Error("更新订单状态失败")
			return
		}
		// 更新任务阶段：进入运输
		// 这里同样加了CAS锁(不确定到底加不加 防止go程竞争 加了更保险)
		err = repository.UpdateTaskSequential(int(task.Id), constant.OrderTaskSequentialTransporting, constant.OrderTaskSequentialAccepting)
		if err != nil {
			Logger.Logger.Error("更新任务阶段失败")
			return
		}
		// 重新规划路径（起点 -> 终点）
		shipment, err := repository.GetShipment(int(task.ShipmentId))
		if err != nil {
			Logger.Logger.Error("查询订单失败")
			return
		}
		startPoi, err := repository.GetPoi(int(shipment.StartPoiId))
		if err != nil {
			Logger.Logger.Error("查询起点poi失败")
			return
		}
		endPoi, err := repository.GetPoi((int(shipment.EndPoiId)))
		if err != nil {
			Logger.Logger.Error("查询终点poi失败")
			return
		}
		routeResp, err := PlanRoute(startPoi.Lon, startPoi.Lat, endPoi.Lon, endPoi.Lat)
		if err != nil {
			Logger.Logger.Error("重新规划路径失败")
			return
		}

		points := ExtractPoints(routeResp)
		// 替换路径
		routeStore.mu.Lock()
		routeStore.RouteMap[int(task.ShipmentId)] = points
		routeStore.mu.Unlock()
		// 重置进度
		vehicleProgress.mu.Lock()
		vehicleProgress.data[task.VehicleId] = 0
		vehicleProgress.mu.Unlock()

	// 到达终点（完成任务）
	case constant.OrderTaskSequentialTransporting:
		Logger.Logger.Info("车辆到达终点，任务完成")
		// 卸货
		err := UnloadCargo(task)
		if err != nil {
			Logger.Logger.Error("卸货逻辑出现错误", zap.Any("err", err))
		}
		// 更新订单状态：完成
		err = repository.UpdateShipmentStatus(int(task.ShipmentId), constant.ShipmentStatusFinish, constant.ShipmentStatusWorking)
		if err != nil {
			Logger.Logger.Error("更新订单状态为完成失败")
			return
		}
		// 更新任务阶段
		err = repository.UpdateTaskSequential(int(task.Id), constant.OrderTaskSequentialFinish, constant.OrderTaskSequentialTransporting)
		if err != nil {
			Logger.Logger.Error("更新任务阶段为完成失败")
		}
		// 释放车辆
		err = repository.UpdateVehicleStatus(int(task.VehicleId), constant.VehicleStatusFree, constant.VehicleStatusRunning)
		if err != nil {
			Logger.Logger.Error("更新车辆状态为free时失败")
		}
		// 清理内存map中的路线信息
		vehicleProgress.mu.Lock()
		delete(vehicleProgress.data, task.VehicleId)
		vehicleProgress.mu.Unlock()
		routeStore.mu.Lock()
		delete(routeStore.RouteMap, int(task.ShipmentId))
		routeStore.mu.Unlock()
	}
}

// LoadCargo 装货逻辑 TODO 这部分是AI的 后续可能要再审一下
func LoadCargo(task *model.OrderTask) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {

		// 查 shipment
		var shipment model.Shipment
		if err := tx.First(&shipment, task.ShipmentId).Error; err != nil {
			return err
		}

		// 查 cargo
		var cargo model.Cargo
		if err := tx.First(&cargo, shipment.CargoId).Error; err != nil {
			return err
		}

		// 查 vehicle
		var vehicle model.Vehicle
		if err := tx.First(&vehicle, task.VehicleId).Error; err != nil {
			return err
		}

		// 计算重量
		totalWeight := shipment.Count * cargo.Weight

		// 容量校验（很重要）
		if vehicle.Size+totalWeight > vehicle.Capacity {
			return fmt.Errorf("车辆超载")
		}

		// 更新车辆载重 + 状态
		err := tx.Model(&model.Vehicle{}).
			Where("id = ?", task.VehicleId).
			Updates(map[string]interface{}{
				"size":   vehicle.Size + totalWeight,
				"status": constant.VehicleStatusRunning, // 装货后一定是运行中
			}).Error

		if err != nil {
			return err
		}

		return nil
	})
}

// UnloadCargo 卸货逻辑 TODO 这部分是AI的 后续可能要再审一下
func UnloadCargo(task *model.OrderTask) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {

		// 查 vehicle
		var vehicle model.Vehicle
		if err := tx.First(&vehicle, task.VehicleId).Error; err != nil {
			return err
		}

		// 卸货（清空载重）
		err := tx.Model(&model.Vehicle{}).
			Where("id = ?", task.VehicleId).
			Updates(map[string]interface{}{
				"size": 0,
			}).Error

		if err != nil {
			return err
		}

		return nil
	})
}

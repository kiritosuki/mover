package task

import (
	"time"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/model"
	"github.com/kiritosuki/mover/internal/repository"
	"go.uber.org/zap"
)

// ============================================================
// 调度入口模块 —— 将 cost 计算、指标收集、打印串联
// 为 order_task.go 中的 doCreateOrderTask 提供调用接口
// ============================================================

// DispatchContext 调度上下文，在任务创建循环中复用
// 避免每次循环都重复查询全局数据
type DispatchContext struct {
	AllVehicles  []*model.Vehicle          // 所有车辆(含各状态)
	FreeVehicles []*model.Vehicle          // 空闲车辆
	AllShipments []*model.Shipment         // 所有订单
	CargoMap     map[uint]*model.Cargo     // cargo id -> cargo 映射
	CostCfg      CostConfig               // 代价权重配置
	Now          time.Time                 // 快照时间
}

// NewDispatchContext 构建调度上下文，查询全局数据
// 返回 nil 表示构建失败(数据查询出错)
func NewDispatchContext(freeVehicles []*model.Vehicle) *DispatchContext {
	// 查所有车辆(含运行中的)
	allVehicles, err := repository.GetAllVehicles()
	if err != nil {
		Logger.Logger.Warn("查询所有车辆失败，指标统计将不完整", zap.Error(err))
		allVehicles = freeVehicles // 降级使用空闲车辆
	}

	// 查所有订单
	allShipments, err := repository.GetAllShipments()
	if err != nil {
		Logger.Logger.Warn("查询所有订单失败", zap.Error(err))
	}

	// 查所有 cargo 类型，构建映射表
	allCargos, err := repository.GetAllCargos()
	if err != nil {
		Logger.Logger.Warn("查询所有货物类型失败", zap.Error(err))
	}
	cargoMap := make(map[uint]*model.Cargo)
	for _, c := range allCargos {
		cargoMap[c.Id] = c
	}

	return &DispatchContext{
		AllVehicles:  allVehicles,
		FreeVehicles: freeVehicles,
		AllShipments: allShipments,
		CargoMap:     cargoMap,
		CostCfg:      DefaultCostConfig(),
		Now:          time.Now(),
	}
}

// EvaluateVehicle 评估单台候选车辆对指定订单的调度代价
// 包含前置校验(容量、货物类型)和 cost 计算
// 返回:
//   - cost: 调度代价，越低越优
//   - ok: 是否通过前置校验
func (dc *DispatchContext) EvaluateVehicle(
	v *model.Vehicle,
	shipment *model.Shipment,
	startPoi *model.Poi,
	endPoi *model.Poi,
) (cost float64, ok bool) {

	// ---- 前置校验 ----

	// 1. 查询货物信息
	cargo, exists := dc.CargoMap[shipment.CargoId]
	if !exists {
		Logger.Logger.Warn("找不到货物信息", zap.Uint("cargoId", shipment.CargoId))
		return 0, false
	}

	// 2. 货物类型校验：危化品车辆才能运危化品货物
	//    普通车辆(VehicleTybeNormal)只能运普通货物(CargoTybe 与车辆 Tybe 匹配)
	//    危化品车辆(VehicleTybeDanger)可以运所有货物
	if v.Tybe == constant.VehicleTybeNormal && cargo.Tybe == constant.VehicleTybeDanger {
		// 普通车辆不能运危化品
		return 0, false
	}

	// 3. 容量校验：车辆剩余容量 >= 货物总重量
	totalWeight := shipment.Count * cargo.Weight
	if v.Size+totalWeight > v.Capacity {
		return 0, false
	}

	// ---- 构建快照数据 ----

	// 车辆等待时间：自上次更新以来的空闲时长
	waitTime := dc.Now.Sub(v.UpdateTime).Seconds()
	if waitTime < 0 {
		waitTime = 0
	}

	// 空驶里程：车辆当前位置到取货点的直线距离
	emptyDist := HaversineDistance(v.Lon, v.Lat, startPoi.Lon, startPoi.Lat)

	// 运输里程：取货点到卸货点的直线距离
	transDist := HaversineDistance(startPoi.Lon, startPoi.Lat, endPoi.Lon, endPoi.Lat)

	// 运输时间估算：里程 / 车速
	transTime := 0.0
	if v.Speed > 0 {
		transTime = transDist / v.Speed
	}

	// 装卸时间估算
	weightTon := float64(totalWeight)
	loadUnloadTime := EstimateLoadUnloadTime(weightTon)

	// 货物等待时间：自订单创建以来的时长
	cargoWaitTime := dc.Now.Sub(shipment.CreateTime).Seconds()
	if cargoWaitTime < 0 {
		cargoWaitTime = 0
	}

	// 构建车辆快照
	vs := VehicleSnapshot{
		Vehicle:        v,
		WaitTime:       waitTime,
		EmptyDistance:   emptyDist,
		TransDistance:   transDist,
		TransTime:      transTime,
		LoadUnloadTime: loadUnloadTime,
	}

	// 构建订单快照
	ss := ShipmentSnapshot{
		Shipment:      shipment,
		CargoWeight:   weightTon,
		WaitTime:      cargoWaitTime,
		TransDistance:  transDist,
	}

	// ---- 计算单车代价 ----
	cost = CalcSingleVehicleCost(dc.CostCfg, vs, ss, nil)
	return cost, true
}

// CollectAndPrintMetrics 收集全局指标并打印到控制台
// 在选出最优车辆并创建任务后调用
func (dc *DispatchContext) CollectAndPrintMetrics(
	shipment *model.Shipment,
	startPoi *model.Poi,
	endPoi *model.Poi,
) {
	// 构建车辆快照列表(所有空闲车辆)
	var vehicleSnapshots []VehicleSnapshot
	for _, v := range dc.FreeVehicles {
		waitTime := dc.Now.Sub(v.UpdateTime).Seconds()
		if waitTime < 0 {
			waitTime = 0
		}
		emptyDist := HaversineDistance(v.Lon, v.Lat, startPoi.Lon, startPoi.Lat)
		transDist := HaversineDistance(startPoi.Lon, startPoi.Lat, endPoi.Lon, endPoi.Lat)
		transTime := 0.0
		if v.Speed > 0 {
			transTime = transDist / v.Speed
		}

		cargo, exists := dc.CargoMap[shipment.CargoId]
		weightTon := 0.0
		if exists {
			weightTon = float64(shipment.Count * cargo.Weight)
		}

		vehicleSnapshots = append(vehicleSnapshots, VehicleSnapshot{
			Vehicle:        v,
			WaitTime:       waitTime,
			EmptyDistance:   emptyDist,
			TransDistance:   transDist,
			TransTime:      transTime,
			LoadUnloadTime: EstimateLoadUnloadTime(weightTon),
		})
	}

	// 构建订单快照列表
	var shipmentSnapshots []ShipmentSnapshot
	for _, s := range dc.AllShipments {
		cargo, exists := dc.CargoMap[s.CargoId]
		if !exists {
			continue
		}
		weightTon := float64(s.Count) * float64(cargo.Weight)
		cargoWait := dc.Now.Sub(s.CreateTime).Seconds()
		if cargoWait < 0 {
			cargoWait = 0
		}

		sPoi, err1 := repository.GetPoi(int(s.StartPoiId))
		ePoi, err2 := repository.GetPoi(int(s.EndPoiId))
		tDist := 0.0
		if err1 == nil && err2 == nil {
			tDist = HaversineDistance(sPoi.Lon, sPoi.Lat, ePoi.Lon, ePoi.Lat)
		}

		shipmentSnapshots = append(shipmentSnapshots, ShipmentSnapshot{
			Shipment:      s,
			CargoWeight:   weightTon,
			WaitTime:      cargoWait,
			TransDistance:  tDist,
		})
	}

	// 构建全局 CostInput 用于完整 cost 计算
	globalInput := buildGlobalCostInput(vehicleSnapshots, shipmentSnapshots, dc)
	globalResult := CalcTotalCost(dc.CostCfg, globalInput)

	// 收集各维度指标
	vm := CollectVehicleMetrics(dc.AllVehicles, vehicleSnapshots)
	cm := CollectCargoMetrics(dc.AllShipments, dc.CargoMap, shipmentSnapshots)
	om := CollectOtherMetrics(vm, vehicleSnapshots, globalResult.Total)

	allMetrics := AllMetrics{
		VehicleMetrics: vm,
		CargoMetrics:   cm,
		OtherMetrics:   om,
	}

	// 打印到控制台
	PrintAllMetrics(allMetrics)
}

// buildGlobalCostInput 从全局快照数据构建 CostInput
func buildGlobalCostInput(
	vehicleSnaps []VehicleSnapshot,
	shipmentSnaps []ShipmentSnapshot,
	dc *DispatchContext,
) CostInput {
	input := CostInput{}

	for _, vs := range vehicleSnaps {
		input.VehicleWaitTimes = append(input.VehicleWaitTimes, vs.WaitTime)
		input.VehicleEmptyDists = append(input.VehicleEmptyDists, vs.EmptyDistance)
		input.VehicleTransDists = append(input.VehicleTransDists, vs.TransDistance)
		input.VehicleTransTimes = append(input.VehicleTransTimes, vs.TransTime)
	}

	minCap := 0.0
	minActual := 0.0
	first := true
	for _, vs := range vehicleSnaps {
		cap := float64(vs.Vehicle.Capacity) * vs.TransDistance
		input.TotalCapacity += cap

		// 查该车辆是否有关联的运输订单来计算实际运能
		actual := float64(vs.Vehicle.Size) * vs.TransDistance
		input.TotalActualTonKm += actual

		if first || cap < minCap {
			minCap = cap
		}
		if first || actual < minActual {
			minActual = actual
		}
		first = false
	}
	input.MinCapacity = minCap
	input.MinActualTonKm = minActual

	for _, ss := range shipmentSnaps {
		input.CargoTonWaits = append(input.CargoTonWaits, ss.CargoWeight*ss.WaitTime)
		input.CargoWaitTimes = append(input.CargoWaitTimes, ss.WaitTime)
	}

	return input
}

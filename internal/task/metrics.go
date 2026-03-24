package task

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/model"
)

// ============================================================
// 数据统计模块 —— 车辆、货物及其他维度的运营指标
// 用于辅助 cost 计算以及后期运营分析
// ============================================================

// ----- 统计摘要结构体 (最大/最小/平均/中位数) -----

// StatsSummary 通用统计摘要，包含最大值、最小值、平均值、中位数
type StatsSummary struct {
	Max    float64 `json:"max"`
	Min    float64 `json:"min"`
	Avg    float64 `json:"avg"`
	Median float64 `json:"median"`
}

// ----- 车辆指标 -----

// VehicleMetrics 车辆维度的所有统计指标
type VehicleMetrics struct {
	// I. 车辆情况
	TotalCount    int            `json:"totalCount"`    // 车辆总数
	TypeDistrib   map[int]int    `json:"typeDistrib"`   // 按车辆类型(Tybe)的数量分布
	CapDistrib    map[int]int    `json:"capDistrib"`    // 按载重容量(Capacity)的数量分布
	PositionStdDev float64       `json:"positionStdDev"` // 车辆位置标准差(衡量分散/密集)

	// II. 利用率
	UtilByCount    float64 `json:"utilByCount"`    // 按车辆数的利用率 = 运行中车辆数 / 总车辆数
	UtilByCapacity float64 `json:"utilByCapacity"` // 按载重的利用率 = 所有车辆已用载重 / 所有车辆总容量

	// III. 等待时间
	TotalWaitTime    float64      `json:"totalWaitTime"`    // 总等待时间(秒)，所有空闲车辆的累计等待
	WaitTimeSummary  StatsSummary `json:"waitTimeSummary"`  // 个体等待时间摘要

	// IV. 空驶
	TotalEmptyDistance float64      `json:"totalEmptyDistance"` // 总空驶里程(米)
	TotalEmptyTime     float64      `json:"totalEmptyTime"`     // 总空驶时间(秒)
	EmptyDistSummary   StatsSummary `json:"emptyDistSummary"`   // 个体空驶里程摘要
}

// ----- 货物指标 -----

// CargoMetrics 货物维度的所有统计指标
type CargoMetrics struct {
	// I. 货物情况
	TotalDemand   float64 `json:"totalDemand"`   // 总需求量(吨)
	AllocatedQty  float64 `json:"allocatedQty"`  // 已分配量(有车可分配)
	PendingQty    float64 `json:"pendingQty"`    // 待处理量

	// II. 吞吐量
	TotalTransport     float64 `json:"totalTransport"`     // 总运输量(吨)
	TransportEfficiency float64 `json:"transportEfficiency"` // 运输效率 = 已完成 / 总需求

	// III. 运能体量
	TotalTonKm        float64      `json:"totalTonKm"`        // 总运输能力体量(吨*米)
	TonKmSummary      StatsSummary `json:"tonKmSummary"`      // 个体运能体量摘要

	// IV. 待处理损耗
	TotalPendingLoss  float64      `json:"totalPendingLoss"`  // 总损耗(吨*秒)
	PendingLossSummary StatsSummary `json:"pendingLossSummary"` // 个体(工厂)损耗摘要
}

// ----- 其它指标 -----

// OtherMetrics 其他维度的统计指标
type OtherMetrics struct {
	// I. 高阶
	EmptyToLoadedRatio float64 `json:"emptyToLoadedRatio"` // 空满占比 = 空驶距离 / 运货距离
	WaitToTransRatio   float64 `json:"waitToTransRatio"`   // 等运占比 = 等待时间 / 运输时间

	// II. 装卸
	TotalLoadUnloadTime    float64      `json:"totalLoadUnloadTime"`    // 装卸总消耗时间(秒)
	LoadUnloadTimeSummary  StatsSummary `json:"loadUnloadTimeSummary"`  // 个体装卸时间摘要

	// III. 总成本值
	TotalCost float64 `json:"totalCost"` // cost 函数计算结果
}

// ----- 汇总入口 -----

// AllMetrics 所有维度指标的汇总
type AllMetrics struct {
	VehicleMetrics VehicleMetrics `json:"vehicleMetrics"`
	CargoMetrics   CargoMetrics   `json:"cargoMetrics"`
	OtherMetrics   OtherMetrics   `json:"otherMetrics"`
}

// ----- 调度快照：为 cost 计算收集的即时数据 -----

// VehicleSnapshot 单台车辆的快照数据，用于调度 cost 计算
type VehicleSnapshot struct {
	Vehicle       *model.Vehicle // 车辆基本信息
	WaitTime      float64        // 等待时间(秒)：空闲车辆自上次更新至今的时长
	EmptyDistance  float64        // 空驶里程(米)：车辆当前位置到取货点的距离
	TransDistance  float64        // 运输里程(米)：取货点到卸货点的距离
	TransTime     float64        // 运输时间(秒)：预估运输用时
	LoadUnloadTime float64       // 装卸时间(秒)：与货物重量成正比
}

// ShipmentSnapshot 单个订单的快照数据，用于调度 cost 计算
type ShipmentSnapshot struct {
	Shipment     *model.Shipment // 订单基本信息
	CargoWeight  float64         // 货物总重(吨) = count * cargo.weight
	WaitTime     float64         // 货物等待时间(秒)：自订单创建以来的等待
	TransDistance float64        // 运输距离(米)
}

// DispatchSnapshot 调度快照，包含所有候选车辆和待处理订单的数据
type DispatchSnapshot struct {
	Vehicles  []VehicleSnapshot  // 所有候选车辆的快照
	Shipments []ShipmentSnapshot // 所有待处理订单的快照
	Now       time.Time          // 快照时间
}

// ============================================================
// 工具函数
// ============================================================

// CalcStatsSummary 根据一组数值计算统计摘要
// 如果 vals 为空，返回全零的 StatsSummary
func CalcStatsSummary(vals []float64) StatsSummary {
	if len(vals) == 0 {
		return StatsSummary{}
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	sum := 0.0
	for _, v := range sorted {
		sum += v
	}

	median := 0.0
	n := len(sorted)
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2.0
	} else {
		median = sorted[n/2]
	}

	return StatsSummary{
		Max:    sorted[n-1],
		Min:    sorted[0],
		Avg:    sum / float64(n),
		Median: median,
	}
}

// StdDev 计算标准差
func StdDev(vals []float64) float64 {
	if len(vals) <= 1 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))

	variance := 0.0
	for _, v := range vals {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(vals))
	return math.Sqrt(variance)
}

// HaversineDistance 使用 Haversine 公式计算两点间的距离(米)
// 参数为经纬度(度)
func HaversineDistance(lon1, lat1, lon2, lat2 float64) float64 {
	const earthRadius = 6371000.0 // 地球半径(米)
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadius * c
}

// EstimateLoadUnloadTime 估算装卸时间(秒)，与货物重量成正比
// 假设每吨货物需要 LoadUnloadTimePerTon 秒
func EstimateLoadUnloadTime(weightTon float64) float64 {
	return weightTon * constant.LoadUnloadTimePerTon
}

// ============================================================
// 指标收集
// ============================================================

// CollectVehicleMetrics 收集车辆维度的指标
// allVehicles: 所有车辆(不区分状态)
// snapshots: 调度快照中的候选车辆数据
func CollectVehicleMetrics(allVehicles []*model.Vehicle, snapshots []VehicleSnapshot) VehicleMetrics {
	m := VehicleMetrics{
		TypeDistrib: make(map[int]int),
		CapDistrib:  make(map[int]int),
	}

	if len(allVehicles) == 0 {
		return m
	}

	m.TotalCount = len(allVehicles)

	// 按类型和容量分布统计
	runningCount := 0
	totalCap := 0
	totalUsed := 0
	var lons, lats []float64

	for _, v := range allVehicles {
		m.TypeDistrib[v.Tybe]++
		m.CapDistrib[v.Capacity]++
		lons = append(lons, v.Lon)
		lats = append(lats, v.Lat)
		totalCap += v.Capacity
		totalUsed += v.Size
		if v.Status == constant.VehicleStatusRunning {
			runningCount++
		}
	}

	// 位置标准差（经纬度标准差的欧几里得合成）
	lonStd := StdDev(lons)
	latStd := StdDev(lats)
	m.PositionStdDev = math.Sqrt(lonStd*lonStd + latStd*latStd)

	// 利用率
	if m.TotalCount > 0 {
		m.UtilByCount = float64(runningCount) / float64(m.TotalCount)
	}
	if totalCap > 0 {
		m.UtilByCapacity = float64(totalUsed) / float64(totalCap)
	}

	// 等待时间和空驶统计 —— 来自快照数据
	var waitTimes []float64
	var emptyDists []float64
	totalWait := 0.0
	totalEmptyDist := 0.0
	totalEmptyTime := 0.0

	for _, s := range snapshots {
		waitTimes = append(waitTimes, s.WaitTime)
		emptyDists = append(emptyDists, s.EmptyDistance)
		totalWait += s.WaitTime
		totalEmptyDist += s.EmptyDistance
		// 空驶时间 = 空驶距离 / 车速(默认取车辆速度，单位 m/s)
		if s.Vehicle.Speed > 0 {
			totalEmptyTime += s.EmptyDistance / s.Vehicle.Speed
		}
	}

	m.TotalWaitTime = totalWait
	m.WaitTimeSummary = CalcStatsSummary(waitTimes)
	m.TotalEmptyDistance = totalEmptyDist
	m.TotalEmptyTime = totalEmptyTime
	m.EmptyDistSummary = CalcStatsSummary(emptyDists)

	return m
}

// CollectCargoMetrics 收集货物维度的指标
// allShipments: 所有订单
// snapshots: 调度快照中的待处理订单数据
func CollectCargoMetrics(allShipments []*model.Shipment, cargoMap map[uint]*model.Cargo, snapshots []ShipmentSnapshot) CargoMetrics {
	m := CargoMetrics{}

	totalDemand := 0.0
	allocated := 0.0
	pending := 0.0
	finished := 0.0

	var tonKms []float64
	var pendingLosses []float64
	totalTonKm := 0.0
	totalLoss := 0.0

	for _, s := range allShipments {
		cargo, ok := cargoMap[s.CargoId]
		if !ok {
			continue
		}
		weightTon := float64(s.Count) * float64(cargo.Weight)
		totalDemand += weightTon

		switch s.Status {
		case constant.ShipmentStatusSleeping:
			pending += weightTon
			// 待处理损耗 = 吨 × 等待时间(秒)
			waitSec := time.Since(s.CreateTime).Seconds()
			loss := weightTon * waitSec
			pendingLosses = append(pendingLosses, loss)
			totalLoss += loss
		case constant.ShipmentStatusWaiting:
			allocated += weightTon
			waitSec := time.Since(s.CreateTime).Seconds()
			loss := weightTon * waitSec
			pendingLosses = append(pendingLosses, loss)
			totalLoss += loss
		case constant.ShipmentStatusWorking:
			allocated += weightTon
		case constant.ShipmentStatusFinish:
			finished += weightTon
		}
	}

	// 运能体量来自快照
	for _, ss := range snapshots {
		tk := ss.CargoWeight * ss.TransDistance
		tonKms = append(tonKms, tk)
		totalTonKm += tk
	}

	m.TotalDemand = totalDemand
	m.AllocatedQty = allocated
	m.PendingQty = pending
	m.TotalTransport = finished
	if totalDemand > 0 {
		m.TransportEfficiency = finished / totalDemand
	}
	m.TotalTonKm = totalTonKm
	m.TonKmSummary = CalcStatsSummary(tonKms)
	m.TotalPendingLoss = totalLoss
	m.PendingLossSummary = CalcStatsSummary(pendingLosses)

	return m
}

// CollectOtherMetrics 收集其他维度的指标
func CollectOtherMetrics(vm VehicleMetrics, snapshots []VehicleSnapshot, totalCost float64) OtherMetrics {
	m := OtherMetrics{}

	// 空满占比和等运占比
	totalEmptyDist := 0.0
	totalTransDist := 0.0
	totalWaitTime := 0.0
	totalTransTime := 0.0
	var loadUnloadTimes []float64
	totalLoadUnload := 0.0

	for _, s := range snapshots {
		totalEmptyDist += s.EmptyDistance
		totalTransDist += s.TransDistance
		totalWaitTime += s.WaitTime
		totalTransTime += s.TransTime
		loadUnloadTimes = append(loadUnloadTimes, s.LoadUnloadTime)
		totalLoadUnload += s.LoadUnloadTime
	}

	if totalTransDist > 0 {
		m.EmptyToLoadedRatio = totalEmptyDist / totalTransDist
	}
	if totalTransTime > 0 {
		m.WaitToTransRatio = totalWaitTime / totalTransTime
	}

	m.TotalLoadUnloadTime = totalLoadUnload
	m.LoadUnloadTimeSummary = CalcStatsSummary(loadUnloadTimes)
	m.TotalCost = totalCost

	return m
}

// ============================================================
// 控制台打印
// ============================================================

// PrintAllMetrics 将所有指标打印到控制台
func PrintAllMetrics(am AllMetrics) {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("============================================================\n")
	sb.WriteString("               调度系统运营指标报告\n")
	sb.WriteString("============================================================\n")

	// ----- 车辆指标 -----
	vm := am.VehicleMetrics
	sb.WriteString("\n【车辆指标】\n")
	sb.WriteString(fmt.Sprintf("  I. 车辆情况\n"))
	sb.WriteString(fmt.Sprintf("     车辆总数:         %d\n", vm.TotalCount))
	sb.WriteString(fmt.Sprintf("     类型分布(Tybe):   %v\n", formatIntMap(vm.TypeDistrib)))
	sb.WriteString(fmt.Sprintf("     载重容量分布:     %v\n", formatIntMap(vm.CapDistrib)))
	sb.WriteString(fmt.Sprintf("     位置分散度(stddev): %.4f\n", vm.PositionStdDev))
	sb.WriteString(fmt.Sprintf("  II. 利用率\n"))
	sb.WriteString(fmt.Sprintf("     按车辆数:         %.2f%%\n", vm.UtilByCount*100))
	sb.WriteString(fmt.Sprintf("     按载重使用:       %.2f%%\n", vm.UtilByCapacity*100))
	sb.WriteString(fmt.Sprintf("  III. 等待时间\n"))
	sb.WriteString(fmt.Sprintf("     总等待(累计):     %.2f 秒\n", vm.TotalWaitTime))
	sb.WriteString(fmt.Sprintf("     个体等待 -> %s\n", formatSummary(vm.WaitTimeSummary, "秒")))
	sb.WriteString(fmt.Sprintf("  IV. 空驶\n"))
	sb.WriteString(fmt.Sprintf("     总空驶里程:       %.2f 米\n", vm.TotalEmptyDistance))
	sb.WriteString(fmt.Sprintf("     总空驶时间:       %.2f 秒\n", vm.TotalEmptyTime))
	sb.WriteString(fmt.Sprintf("     个体空驶 -> %s\n", formatSummary(vm.EmptyDistSummary, "米")))

	// ----- 货物指标 -----
	cm := am.CargoMetrics
	sb.WriteString("\n【货物指标】\n")
	sb.WriteString(fmt.Sprintf("  I. 货物情况\n"))
	sb.WriteString(fmt.Sprintf("     总需求量:         %.2f\n", cm.TotalDemand))
	sb.WriteString(fmt.Sprintf("     已分配量:         %.2f\n", cm.AllocatedQty))
	sb.WriteString(fmt.Sprintf("     待处理量:         %.2f\n", cm.PendingQty))
	sb.WriteString(fmt.Sprintf("  II. 吞吐量\n"))
	sb.WriteString(fmt.Sprintf("     总运输量:         %.2f\n", cm.TotalTransport))
	sb.WriteString(fmt.Sprintf("     运输效率:         %.2f%%\n", cm.TransportEfficiency*100))
	sb.WriteString(fmt.Sprintf("  III. 运能体量\n"))
	sb.WriteString(fmt.Sprintf("     总运能(吨*米):    %.2f\n", cm.TotalTonKm))
	sb.WriteString(fmt.Sprintf("     个体运能 -> %s\n", formatSummary(cm.TonKmSummary, "吨*米")))
	sb.WriteString(fmt.Sprintf("  IV. 待处理损耗\n"))
	sb.WriteString(fmt.Sprintf("     总损耗(吨*秒):    %.2f\n", cm.TotalPendingLoss))
	sb.WriteString(fmt.Sprintf("     个体损耗 -> %s\n", formatSummary(cm.PendingLossSummary, "吨*秒")))

	// ----- 其它指标 -----
	om := am.OtherMetrics
	sb.WriteString("\n【其它指标】\n")
	sb.WriteString(fmt.Sprintf("  I. 高阶\n"))
	sb.WriteString(fmt.Sprintf("     空满占比:         %.4f\n", om.EmptyToLoadedRatio))
	sb.WriteString(fmt.Sprintf("     等运占比:         %.4f\n", om.WaitToTransRatio))
	sb.WriteString(fmt.Sprintf("  II. 装卸\n"))
	sb.WriteString(fmt.Sprintf("     总消耗时间:       %.2f 秒\n", om.TotalLoadUnloadTime))
	sb.WriteString(fmt.Sprintf("     个体装卸 -> %s\n", formatSummary(om.LoadUnloadTimeSummary, "秒")))
	sb.WriteString(fmt.Sprintf("  III. 总成本值(cost): %.6f\n", om.TotalCost))

	sb.WriteString("\n============================================================\n")

	fmt.Print(sb.String())
}

// formatSummary 格式化 StatsSummary 为可读字符串
func formatSummary(s StatsSummary, unit string) string {
	return fmt.Sprintf("最大=%.2f%s, 最小=%.2f%s, 平均=%.2f%s, 中位数=%.2f%s",
		s.Max, unit, s.Min, unit, s.Avg, unit, s.Median, unit)
}

// formatIntMap 格式化 map[int]int 为可读字符串
func formatIntMap(m map[int]int) string {
	if len(m) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%d:%d", k, v))
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, ", ") + "}"
}

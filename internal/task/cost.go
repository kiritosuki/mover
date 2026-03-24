package task

import (
	"math"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/model"
	"go.uber.org/zap"
)

// CostWeights 成本计算中的各类权重
type CostWeights struct {
	// A: 直接成本权重
	K1A float64 // 默认 0.5
	K2A float64 // 默认 0.5

	// B: 运送快权重
	K1B float64 // 默认 0.6
	K2B float64 // 默认 0.4

	// C: 效率+公平权重
	K1C float64 // 默认 0.4
	K2C float64 // 默认 0.3
	K3C float64 // 默认 0.3

	// D: 平台权重
	K1D float64 // 默认 0.5
	K2D float64 // 默认 0.5

	// E: 风险权重
	R1 float64 // 车辆等待时间标准差权重
	R2 float64 // 货物等待时间标准差权重
	R3 float64 // 运行时间标准差权重 (暂无统计，用0)

	// F: 大综合权重
	Lambda1 float64
	Lambda2 float64
	Lambda3 float64
	Lambda4 float64
	Lambda5 float64
}

var DefaultWeights = CostWeights{
	K1A: 0.5, K2A: 0.5,
	K1B: 0.6, K2B: 0.4,
	K1C: 0.4, K2C: 0.3, K3C: 0.3,
	K1D: 0.5, K2D: 0.5,
	R1: 0.1, R2: 0.1, R3: 0.0,
	Lambda1: 0.2, Lambda2: 0.2, Lambda3: 0.2, Lambda4: 0.2, Lambda5: 0.2,
}

// CalculateCost 根据图片中的多种模型计算综合成本
// v: 候选车辆
// shipment: 待分配订单
// stats: 当前全局统计量
// cargo: 货物详情
// startPoi: 货物起点
func CalculateCost(v *model.Vehicle, shipment *model.Shipment, stats *Statistics, cargo *model.Cargo, startPoi *model.Poi) float64 {
	weights := DefaultWeights

	// 1. 估算候选车辆到达起点的距离和时间 (增量)
	distToStart := haversine(v.Lon, v.Lat, startPoi.Lon, startPoi.Lat)
	speed := v.Speed
	if speed <= 0 {
		speed = 10.0 // 默认 10m/s
	}
	timeToStart := distToStart / speed

	// --- A: 直接成本 ---
	costA := weights.K1A*timeToStart + weights.K2A*distToStart

	// --- B: 运送快 ---
	cargoWeightInTons := float64(cargo.Weight) / 1000.0
	costB := weights.K1B*(cargoWeightInTons*timeToStart) + weights.K2B*math.Max(timeToStart, stats.WaitTimeStats.Max)

	// --- C: 效率+公平 ---
	totalEmpty := stats.EmptyDistStats.TotalEmpty + distToStart
	ratio := stats.EmptyDistStats.EmptyFullRatio
	if ratio <= 0 {
		ratio = 0.5
	}
	estimatedTotalDist := stats.EmptyDistStats.TotalEmpty*(1+1/ratio) + distToStart
	
	costC := weights.K1C*(totalEmpty/estimatedTotalDist) +
		weights.K2C*(stats.WaitTimeStats.WaitTransRatio) +
		weights.K3C*math.Max(timeToStart, stats.WaitTimeStats.Max)

	// --- D: 平台 ---
	remainingTotalCap := float64(stats.UtilizationStats.TotalCapacity - stats.UtilizationStats.UsedCapacity)
	newRemainingTotalCap := math.Max(0, remainingTotalCap-float64(v.Capacity))
	waste := float64(v.Capacity - shipment.Count)
	costD := weights.K1D*newRemainingTotalCap + weights.K2D*waste

	// --- E: 风险 ---
	costE := costA + weights.R1*stats.WaitTimeStats.StdDev + weights.R2*stats.WaitTimeStats.StdDev

	// --- F: 大综合 ---
	costF := weights.Lambda1*costA +
		weights.Lambda2*costB +
		weights.Lambda3*costC +
		weights.Lambda4*costD +
		weights.Lambda5*costE

	Logger.Logger.Debug("Cost Calculation Breakdown",
		zap.Uint("VehicleID", v.Id),
		zap.Float64("CostA", costA),
		zap.Float64("CostB", costB),
		zap.Float64("CostC", costC),
		zap.Float64("CostD", costD),
		zap.Float64("CostE", costE),
		zap.Float64("TotalCostF", costF),
		zap.Float64("TimeToStart", timeToStart),
		zap.Float64("DistToStart", distToStart),
	)

	return costF
}

// haversine 计算两点间的球面距离 (单位: 米)
func haversine(lon1, lat1, lon2, lat2 float64) float64 {
	const R = 6371000 // 地球半径
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

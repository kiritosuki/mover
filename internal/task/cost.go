package task

import (
	"math"

	"github.com/kiritosuki/mover/internal/Logger"
	"go.uber.org/zap"
)

// ============================================================
// 代价计算模块 —— 车辆调度的 cost 函数
//
// 总代价公式:
//   cost = a*A + b*B + c*C + d*D + e*E
//
// 其中各分项含义:
//   A (直接成本):    k1*所有车辆等待时间 + k2*所有空驶里程
//   B (运货快):      k1*所有货物(吨*等待时间) + k2*最慢货物等待时间
//   C (效率和公平):  k1*(总空驶/总里程) + k2*(总等待时间/总运输时间) + k3*(等最久的那台车时间)
//   D (平台侧):      k1*(总运能-总实际运能) + k2*(最低的运能-实际运输)
//   E (风险):        基础成本(A+B+C+D) + r1*等待时间标准差 + r2*货物等待时间标准差 + r3*运输里程标准差
// ============================================================

// ----- 权重参数结构体 -----

// CostWeights 总代价公式的外层权重: cost = a*A + b*B + c*C + d*D + e*E
type CostWeights struct {
	A float64 // 直接成本权重
	B float64 // 运货快权重
	C float64 // 效率和公平权重
	D float64 // 平台侧权重
	E float64 // 风险权重
}

// DirectCostParams A (直接成本) 的子权重
type DirectCostParams struct {
	K1 float64 // 所有车辆等待时间的权重
	K2 float64 // 所有空驶里程的权重
}

// FastDeliveryParams B (运货快) 的子权重
type FastDeliveryParams struct {
	K1 float64 // 货物(吨*等待时间)的权重
	K2 float64 // 最慢货物等待时间的权重
}

// EfficiencyFairnessParams C (效率和公平) 的子权重
type EfficiencyFairnessParams struct {
	K1 float64 // 总空驶/总里程 的权重
	K2 float64 // 总等待时间/总运输时间 的权重
	K3 float64 // 等最久车辆时间 的权重
}

// PlatformParams D (平台侧) 的子权重
type PlatformParams struct {
	K1 float64 // (总运能-总实际运能) 的权重
	K2 float64 // (最低运能-实际运输) 的权重
}

// RiskParams E (风险) 的子权重
type RiskParams struct {
	R1 float64 // 等待时间标准差的权重
	R2 float64 // 货物等待时间标准差的权重
	R3 float64 // 运输里程标准差的权重
}

// CostConfig 代价函数的完整配置
type CostConfig struct {
	Weights    CostWeights          // 外层权重 a, b, c, d, e
	Direct     DirectCostParams     // A 子权重
	Fast       FastDeliveryParams   // B 子权重
	Efficiency EfficiencyFairnessParams // C 子权重
	Platform   PlatformParams       // D 子权重
	Risk       RiskParams           // E 子权重
}

// ----- 代价计算输入数据 -----

// CostInput 代价计算所需的全部输入数据，由调度快照中提取
type CostInput struct {
	// 车辆维度
	VehicleWaitTimes   []float64 // 每台车辆的等待时间(秒)
	VehicleEmptyDists  []float64 // 每台车辆的空驶里程(米)
	VehicleTransDists  []float64 // 每台车辆的运输里程(米)
	VehicleTransTimes  []float64 // 每台车辆的运输时间(秒)

	// 货物维度
	CargoTonWaits      []float64 // 每个订单的 吨×等待时间
	CargoWaitTimes     []float64 // 每个订单的等待时间(秒)

	// 运能维度
	TotalCapacity      float64   // 总运能(吨*米): 所有车辆容量 × 最大可行驶距离
	TotalActualTonKm   float64   // 总实际运能(吨*米): 实际装载 × 实际距离
	MinCapacity        float64   // 最低单车运能
	MinActualTonKm     float64   // 最低单车实际运输
}

// ----- 代价计算结果 -----

// CostResult 代价函数的计算结果，包含各分项和总值
type CostResult struct {
	A     float64 // 直接成本
	B     float64 // 运货快
	C     float64 // 效率和公平
	D     float64 // 平台侧
	E     float64 // 风险
	Total float64 // 总代价 = a*A + b*B + c*C + d*D + e*E
}

// ============================================================
// 默认配置
// ============================================================

// DefaultCostConfig 返回一组默认的代价权重参数
// 实际使用时应根据业务场景调参
func DefaultCostConfig() CostConfig {
	return CostConfig{
		Weights: CostWeights{
			A: 0.25, // 直接成本
			B: 0.25, // 运货快
			C: 0.20, // 效率和公平
			D: 0.15, // 平台侧
			E: 0.15, // 风险
		},
		Direct: DirectCostParams{
			K1: 1.0, // 等待时间权重
			K2: 1.0, // 空驶里程权重
		},
		Fast: FastDeliveryParams{
			K1: 1.0, // 吨*等待时间权重
			K2: 2.0, // 最慢货物等待时间权重(惩罚极端值)
		},
		Efficiency: EfficiencyFairnessParams{
			K1: 1.0, // 空满比权重
			K2: 1.0, // 等运比权重
			K3: 0.5, // 最久等待车辆时间权重
		},
		Platform: PlatformParams{
			K1: 1.0, // 运能差额权重
			K2: 2.0, // 最低运能差额权重(惩罚极端值)
		},
		Risk: RiskParams{
			R1: 0.5, // 等待时间标准差权重
			R2: 0.5, // 货物等待标准差权重
			R3: 0.3, // 里程标准差权重
		},
	}
}

// ============================================================
// 各分项计算函数
// ============================================================

// CalcDirectCost 计算 A (直接成本)
// A = k1 * 所有车辆等待时间 + k2 * 所有空驶里程
func CalcDirectCost(params DirectCostParams, input CostInput) float64 {
	totalWait := sumFloat64(input.VehicleWaitTimes)
	totalEmpty := sumFloat64(input.VehicleEmptyDists)
	return params.K1*totalWait + params.K2*totalEmpty
}

// CalcFastDelivery 计算 B (运货快)
// B = k1 * 所有货物(吨*等待时间) + k2 * 最慢货物等待时间
func CalcFastDelivery(params FastDeliveryParams, input CostInput) float64 {
	totalTonWait := sumFloat64(input.CargoTonWaits)
	maxCargoWait := maxFloat64(input.CargoWaitTimes)
	return params.K1*totalTonWait + params.K2*maxCargoWait
}

// CalcEfficiencyFairness 计算 C (效率和公平)
// C = k1*(总空驶/总里程) + k2*(总等待时间/总运输时间) + k3*(等最久的那台车的时间)
func CalcEfficiencyFairness(params EfficiencyFairnessParams, input CostInput) float64 {
	totalEmpty := sumFloat64(input.VehicleEmptyDists)
	totalDist := sumFloat64(input.VehicleTransDists) + totalEmpty // 总里程 = 运输里程 + 空驶里程
	totalWait := sumFloat64(input.VehicleWaitTimes)
	totalTransTime := sumFloat64(input.VehicleTransTimes)
	maxWait := maxFloat64(input.VehicleWaitTimes)

	emptyRatio := 0.0
	if totalDist > 0 {
		emptyRatio = totalEmpty / totalDist
	}
	waitRatio := 0.0
	if totalTransTime > 0 {
		waitRatio = totalWait / totalTransTime
	}

	return params.K1*emptyRatio + params.K2*waitRatio + params.K3*maxWait
}

// CalcPlatformCost 计算 D (平台侧)
// D = k1*(总运能-总实际运能) + k2*(最低运能-实际运输)
func CalcPlatformCost(params PlatformParams, input CostInput) float64 {
	capGap := math.Max(0, input.TotalCapacity-input.TotalActualTonKm)
	minGap := math.Max(0, input.MinCapacity-input.MinActualTonKm)
	return params.K1*capGap + params.K2*minGap
}

// CalcRiskCost 计算 E (风险)
// E = 基础成本(A+B+C+D) + r1*等待时间标准差 + r2*货物等待时间标准差 + r3*运输里程标准差
func CalcRiskCost(params RiskParams, baseCost float64, input CostInput) float64 {
	waitStd := StdDev(input.VehicleWaitTimes)
	cargoWaitStd := StdDev(input.CargoWaitTimes)
	distStd := StdDev(input.VehicleTransDists)
	return baseCost + params.R1*waitStd + params.R2*cargoWaitStd + params.R3*distStd
}

// ============================================================
// 总代价函数
// ============================================================

// CalcTotalCost 计算总代价
// cost = a*A + b*B + c*C + d*D + e*E
func CalcTotalCost(cfg CostConfig, input CostInput) CostResult {
	a := CalcDirectCost(cfg.Direct, input)
	b := CalcFastDelivery(cfg.Fast, input)
	c := CalcEfficiencyFairness(cfg.Efficiency, input)
	d := CalcPlatformCost(cfg.Platform, input)
	baseCost := a + b + c + d
	e := CalcRiskCost(cfg.Risk, baseCost, input)

	total := cfg.Weights.A*a +
		cfg.Weights.B*b +
		cfg.Weights.C*c +
		cfg.Weights.D*d +
		cfg.Weights.E*e

	result := CostResult{
		A:     a,
		B:     b,
		C:     c,
		D:     d,
		E:     e,
		Total: total,
	}

	Logger.Logger.Info("代价计算完成",
		zap.Float64("A(直接成本)", a),
		zap.Float64("B(运货快)", b),
		zap.Float64("C(效率公平)", c),
		zap.Float64("D(平台侧)", d),
		zap.Float64("E(风险)", e),
		zap.Float64("Total", total),
	)

	return result
}

// ============================================================
// 单车代价快速计算 —— 用于调度循环中对比候选车辆
// ============================================================

// CalcSingleVehicleCost 为单台候选车辆计算简化的调度代价
// 这是为 doCreateOrderTask 中的循环设计的快速评估函数
// 参数:
//   - cfg: 代价权重配置
//   - vs: 候选车辆快照
//   - ss: 当前订单快照
//   - globalInput: 全局快照数据(用于计算 C/D/E 分项中的全局统计)
//
// 返回值:
//   - cost: 如果选择该车辆，对整体代价的贡献值
func CalcSingleVehicleCost(cfg CostConfig, vs VehicleSnapshot, ss ShipmentSnapshot, globalInput *CostInput) float64 {
	// --- A: 直接成本(该车辆的等待+空驶) ---
	a := cfg.Direct.K1*vs.WaitTime + cfg.Direct.K2*vs.EmptyDistance

	// --- B: 运货快(该订单的吨*等待 + 该订单等待时间) ---
	b := cfg.Fast.K1*(ss.CargoWeight*ss.WaitTime) + cfg.Fast.K2*ss.WaitTime

	// --- C: 效率和公平(该车辆的空满比 + 等运比) ---
	totalDist := vs.EmptyDistance + vs.TransDistance
	emptyRatio := 0.0
	if totalDist > 0 {
		emptyRatio = vs.EmptyDistance / totalDist
	}
	waitRatio := 0.0
	if vs.TransTime > 0 {
		waitRatio = vs.WaitTime / vs.TransTime
	}
	c := cfg.Efficiency.K1*emptyRatio + cfg.Efficiency.K2*waitRatio + cfg.Efficiency.K3*vs.WaitTime

	// --- D: 平台侧(该车辆的运能差额) ---
	vehicleCap := float64(vs.Vehicle.Capacity) * vs.TransDistance
	vehicleActual := ss.CargoWeight * ss.TransDistance
	capGap := math.Max(0, vehicleCap-vehicleActual)
	d := cfg.Platform.K1 * capGap

	// --- E: 风险(简化为直接成本的补充) ---
	baseCost := a + b + c + d
	e := baseCost // 单车情况下无法计算标准差，直接用基础成本

	total := cfg.Weights.A*a +
		cfg.Weights.B*b +
		cfg.Weights.C*c +
		cfg.Weights.D*d +
		cfg.Weights.E*e

	return total
}

// ============================================================
// 辅助函数
// ============================================================

// sumFloat64 计算切片元素总和
func sumFloat64(vals []float64) float64 {
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s
}

// maxFloat64 返回切片中的最大值，空切片返回 0
func maxFloat64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

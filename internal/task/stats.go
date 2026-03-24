package task

import (
	"math"
	"sort"
	"time"

	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/model" // 使用 model 代替 repository.model
	"github.com/kiritosuki/mover/internal/repository"
)

// Statistics 统计模块入口结构
type Statistics struct {
	VehicleStats     VehicleOverview    // 车辆概况
	UtilizationStats UtilizationMetrics // 利用率指标
	WaitTimeStats    TimeMetrics        // 等待时间指标
	EmptyDistStats   DistanceMetrics    // 运输/里程指标
	LoadStats        LoadMetrics        // 装卸/载重指标
}

// VehicleOverview 车辆基础情况
type VehicleOverview struct {
	TotalCount   int         // 车辆总数
	TypeCountMap map[int]int // 各类型车辆数量分布 (Tybe -> Count)
}

// UtilizationMetrics 利用率指标
type UtilizationMetrics struct {
	ByVehicleCount float64 // 按车辆数计算的利用率
	ByCapacity     float64 // 按载重/容量计算的利用率
	TotalCapacity  int     // 总运能
	UsedCapacity   int     // 实际已使用运能
}

// TimeMetrics 时间相关统计量 (通用：总计、最大、最小、平均、中位数、标准差)
type TimeMetrics struct {
	Total        float64
	Max          float64
	Min          float64
	Avg          float64
	Median       float64
	StdDev       float64
	WaitTransRatio float64 // 等待时间/运行时间占比
}

// DistanceMetrics 距离/空驶指标
type DistanceMetrics struct {
	TotalEmpty      float64
	MaxEmpty        float64
	MinEmpty        float64
	AvgEmpty        float64
	MedianEmpty     float64
	StdDevEmpty     float64
	EmptyFullRatio float64 // 空满占比 (空驶/重载)
}

// LoadMetrics 装卸相关指标
type LoadMetrics struct {
	TotalTime float64
	MaxTime   float64
	MinTime   float64
	AvgTime   float64
	MedianTime float64
	StdDevTime float64
}

// CalculateGlobalStats 计算并获取全局所有统计指标
func CalculateGlobalStats() (*Statistics, error) {
	stats := &Statistics{}

	// 1. 获取基础数据
	vehicles, err := repository.ListVehicles(0)
	if err != nil {
		return nil, err
	}
	tasks, err := repository.GetAllTasks()
	if err != nil {
		return nil, err
	}

	// 2. 核心模块化计算
	stats.calculateVehicleOverview(vehicles)
	stats.calculateUtilization(vehicles)
	stats.calculateTimeStats(tasks)
	// 注：空驶里程和装卸时间需要更精细的数据记录（如历史轨迹记录和装卸日志），此处暂用模拟逻辑预留接口
	stats.calculateDistanceStats()
	stats.calculateLoadStats()

	return stats, nil
}

// calculateVehicleOverview 计算车辆基础指标
func (s *Statistics) calculateVehicleOverview(vehicles []*model.Vehicle) {
	s.VehicleStats.TotalCount = len(vehicles)
	s.VehicleStats.TypeCountMap = make(map[int]int)
	for _, v := range vehicles {
		s.VehicleStats.TypeCountMap[v.Tybe]++
	}
}

// calculateUtilization 计算利用率指标
func (s *Statistics) calculateUtilization(vehicles []*model.Vehicle) {
	if len(vehicles) == 0 {
		return
	}
	runningCount := 0
	totalCap := 0
	usedCap := 0
	for _, v := range vehicles {
		totalCap += v.Capacity
		if v.Status == constant.VehicleStatusRunning {
			runningCount++
			usedCap += v.Capacity // 简化模型：运行即满载
		}
	}
	s.UtilizationStats.TotalCapacity = totalCap
	s.UtilizationStats.UsedCapacity = usedCap
	s.UtilizationStats.ByVehicleCount = float64(runningCount) / float64(len(vehicles))
	if totalCap > 0 {
		s.UtilizationStats.ByCapacity = float64(usedCap) / float64(totalCap)
	}
}

// calculateTimeStats 计算等待时间及运行时间相关指标
func (s *Statistics) calculateTimeStats(tasks []*model.OrderTask) {
	if len(tasks) == 0 {
		return
	}
	var waitTimes []float64
	var totalWait, totalTrans float64

	now := time.Now()
	for _, t := range tasks {
		// 计算等待时间: 对于待命和接单中的，评估其等待耗时
		wait := now.Sub(t.CreateTime).Seconds()
		waitTimes = append(waitTimes, wait)
		totalWait += wait

		// 运行时间：简化计算逻辑
		if t.Sequential == constant.OrderTaskSequentialTransporting {
			totalTrans += now.Sub(t.UpdateTime).Seconds()
		}
	}

	// 通用指标计算
	m := computeBasicMetrics(waitTimes)
	s.WaitTimeStats = TimeMetrics{
		Total:  totalWait,
		Max:    m.Max,
		Min:    m.Min,
		Avg:    m.Avg,
		Median: m.Median,
		StdDev: m.StdDev,
	}
	if totalTrans > 0 {
		s.WaitTimeStats.WaitTransRatio = totalWait / totalTrans
	}
}

// calculateDistanceStats (预留接口) 计算空驶相关
func (s *Statistics) calculateDistanceStats() {
	// TODO: 需要轨迹表关联计算真实里程
	// 目前提供静态模拟或设为 0
	s.EmptyDistStats.EmptyFullRatio = 0.5 
}

// calculateLoadStats (预留接口) 计算装卸耗时
func (s *Statistics) calculateLoadStats() {
	// TODO: 关联订单装卸日志
}

// --- 通用数学辅助函数 ---

type basicMetrics struct {
	Max    float64
	Min    float64
	Avg    float64
	Median float64
	StdDev float64
}

func computeBasicMetrics(data []float64) basicMetrics {
	if len(data) == 0 {
		return basicMetrics{}
	}
	sort.Float64s(data)
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	avg := sum / float64(len(data))
	
	var sqSum float64
	for _, v := range data {
		sqSum += math.Pow(v-avg, 2)
	}
	stdDev := math.Sqrt(sqSum / float64(len(data)))

	median := data[len(data)/2]
	if len(data)%2 == 0 {
		median = (data[len(data)/2-1] + data[len(data)/2]) / 2
	}

	return basicMetrics{
		Max:    data[len(data)-1],
		Min:    data[0],
		Avg:    avg,
		Median: median,
		StdDev: stdDev,
	}
}

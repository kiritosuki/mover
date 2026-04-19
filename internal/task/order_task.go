package task

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/model"
	"github.com/kiritosuki/mover/internal/repository"
)

type RouteStore struct {
	RouteMap map[int][]Point
	// map并发不安全 要加锁
	mu sync.Mutex
}

type Point struct {
	Lon float64
	Lat float64
}

var routeStore = RouteStore{
	RouteMap: make(map[int][]Point),
}

// 定时任务 给待创建任务的订单创建任务
func CreateOrderTask() {
	var wg sync.WaitGroup
	for {
		shipments, err := repository.GetSleepingShipment()
		if err != nil {
			Logger.Logger.Error("查询待创建任务的订单时出错")
			// 防止一直出错 忙循环
			time.Sleep(1 * time.Second)
			continue
		}
		for _, shipment := range shipments {
			wg.Add(1)
			// 并行提高效率
			go doCreateOrderTask(shipment, &wg)
		}
		// 等待其他go程退出
		wg.Wait()
		time.Sleep(constant.OrderTaskCreatingGap * time.Second)
	}
}

// 实际执行创建任务
func doCreateOrderTask(shipment *model.Shipment, wg *sync.WaitGroup) {
	defer wg.Done()
	// 获取起点poi
	startPoi, err := repository.GetPoi(int(shipment.StartPoiId))
	if err != nil {
		Logger.Logger.Error("查询起点POI失败")
		return
	}
	// 获取终点poi
	endPoi, err := repository.GetPoi(int(shipment.EndPoiId))
	if err != nil {
		Logger.Logger.Error("查询终点POI失败")
		return
	}
	_ = endPoi // 目前创建任务主要用起点，终点供后续使用

	// 获取货物详情用于校验
	cargo, err := repository.GetCargo(int(shipment.CargoId))
	if err != nil {
		Logger.Logger.Error("查询货物详情失败")
		return
	}
	// 获取所有可用车辆
	vehicles, err := repository.ListVehicles(constant.VehicleStatusFree)
	if err != nil || len(vehicles) == 0 {
		Logger.Logger.Warn("没有可用车辆")
		return
	}

	// 在分配前，可选地计算全局统计量以供日志记录或监控数据
	stats, err := CalculateGlobalStats()
	if err != nil {
		Logger.Logger.Error("获取全局统计量失败")
	}

	// 选最优车辆 (目前逻辑：选择第一个符合类型和容量要求的车辆)
	var bestVehicle *model.Vehicle
	bestCost := math.MaxFloat64

	for _, v := range vehicles {
		// 校验1: 车辆类型与货物类型匹配
		if v.Tybe != cargo.Tybe {
			continue
		}
		// 校验2: 车辆容量，按实际货重(kg)做匹配
		if v.Capacity-v.Size < cargo.Weight*shipment.Count {
			continue
		}

		// 调用真实的 Cost 计算
		cost := CalculateCost(v, shipment, stats, cargo, startPoi, endPoi)

		if cost < bestCost {
			bestCost = cost
			bestVehicle = v
		}
	}

	if bestVehicle == nil {
		Logger.Logger.Warn("没有选出合适车辆")
		return
	}
	// 路径规划（车辆当前位置 -> 起点）
	routeResp, err := PlanRoute(
		bestVehicle.Lon,
		bestVehicle.Lat,
		startPoi.Lon,
		startPoi.Lat,
	)
	if err != nil {
		Logger.Logger.Error("路径规划失败")
		return
	}
	// 提取路径规划返回值中的路径点
	// TODO 可能需要一些稀释的算法 这里是GPT实现的 先不用动
	points := ExtractPoints(routeResp)

	// 存路径
	routeStore.mu.Lock()
	routeStore.RouteMap[int(shipment.Id)] = points
	routeStore.mu.Unlock()

	// 创建任务
	task := &model.OrderTask{
		ShipmentId: shipment.Id,
		VehicleId:  bestVehicle.Id,
		Sequential: constant.OrderTaskSequentialAccepting,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	err = repository.CreateOrderTask(task)
	if err != nil {
		Logger.Logger.Error("创建任务失败")
		return
	}

	// 更新订单状态为待取货
	// 第二个status是旧状态
	// 因为用了go程 防止高并发下数据被多次修改 加了一层CAS锁
	// 不知道是不是必须的 但比较保险
	err = repository.UpdateShipmentStatus(int(shipment.Id), constant.ShipmentStatusWaiting, shipment.Status)
	if err != nil {
		Logger.Logger.Error("更新订单状态失败")
		return
	}
	Logger.Logger.Info("任务创建成功")
}

// TODO 这部分是AI写的 提取路径规划返回值的路径点 暂时不用动
func ExtractPoints(resp *RouteResponse) []Point {
	var points []Point
	if resp == nil || len(resp.Route.Paths) == 0 {
		return points
	}
	path := resp.Route.Paths[0]
	var last Point
	first := true
	for _, step := range path.Steps {
		pairs := strings.Split(step.Polyline, ";")

		for _, p := range pairs {
			xy := strings.Split(p, ",")
			if len(xy) != 2 {
				continue
			}

			lon, err1 := strconv.ParseFloat(xy[0], 64)
			lat, err2 := strconv.ParseFloat(xy[1], 64)

			if err1 != nil || err2 != nil {
				continue
			}

			curr := Point{Lon: lon, Lat: lat}

			// 去重：和上一个点一样就跳过
			if !first && curr == last {
				continue
			}

			points = append(points, curr)
			last = curr
			first = false
		}
	}

	return points
}

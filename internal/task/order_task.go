package task

import (
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
	// 获取所有可用车辆
	vehicles, err := repository.ListVehicles(constant.VehicleStatusFree)
	if err != nil || len(vehicles) == 0 {
		Logger.Logger.Warn("没有可用车辆")
		return
	}

	// 构建调度上下文(全局数据查询一次，循环中复用)
	dispatchCtx := NewDispatchContext(vehicles)

	// 选最优车辆：遍历候选车辆，通过前置校验(容量/类型)后计算 cost，取最小值
	var bestVehicle *model.Vehicle
	bestCost := float64(1<<63 - 1)

	for _, v := range vehicles {
		// EvaluateVehicle 内部完成了:
		// 1. 货物类型校验(普通/危化品匹配)
		// 2. 车辆剩余容量校验
		// 3. cost 代价计算(综合考虑等待时间、空驶里程、运货效率、平台运能、风险)
		cost, ok := dispatchCtx.EvaluateVehicle(v, shipment, startPoi, endPoi)
		if !ok {
			// 未通过前置校验(类型不匹配或容量不足)，跳过
			continue
		}
		if cost < bestCost {
			bestCost = cost
			bestVehicle = v
		}
	}

	if bestVehicle == nil {
		Logger.Logger.Warn("没有选出合适车辆(所有候选均未通过校验)")
		return
	}

	// 收集全局指标并打印到控制台
	dispatchCtx.CollectAndPrintMetrics(shipment, startPoi, endPoi)
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

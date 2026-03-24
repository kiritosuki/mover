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
	// TODO 这里代码报错是因为go中变量获取后就必须要使用 这里下面计算cost可能会用所以就留这里了
	// TODO 你们写代码用了之后把这两行TODO删掉就行
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
	// 选最优车辆
	var bestVehicle *model.Vehicle
	bestCost := float64(1<<63 - 1)

	for _, v := range vehicles {

		// TODO 这里做 cost 计算 替换成真实做法
		// TODO 除了 cost 计算 还应当先考虑车剩余容量和车辆类型(普通/危化品) 这里一起计算
		// TODO 装货卸货的逻辑我有实现 但是这是在车辆到达原材料地之后的逻辑
		// TODO 也就是这里创建任务是前置逻辑 要在这里完成货物类型和容量的判断 才能保证后续程序正常运行
		// TODO 相关的状态常量信息都在 mover/internal/constant 里
		// 写的时候尽量不在代码中出现魔法值吧 都放constant里
		// TODO 或许你可能要用到 mover/internal/task/route.go 中的路径规划
		// 这里我不太确定 路径规划调用放到后面了 但不建议在循环里连续调用
		// api限额是每秒只能调用10次 这里开了go程 又并发循环调用 一定会超额的
		// TODO 你可以单独创建新文件在 mover/internal/task/cost.go 中写cost计算的函数
		// 函数计算需要用到的参数我也不确定有哪些 可能需要先做上周的数据统计部分 计算参数
		// 这里用参数来计算cost
		// 建议两个人写 一个人写数据统计部分 一个人写cost计算部分

		cost := 0.0

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

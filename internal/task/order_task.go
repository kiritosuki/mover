package task

import (
	"sync"
	"time"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/model"
	"github.com/kiritosuki/mover/internal/repository"
)

// 定时任务 给待创建任务的订单创建任务
func CreateOrderTask() {
	var wg sync.WaitGroup
	for {
		shipments, err := repository.GetSleepingShipment()
		if err != nil {
			Logger.Logger.Error("查询待创建任务的订单时出错")
			// TODO 这里可能会有额外的错误处理逻辑
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
	
}

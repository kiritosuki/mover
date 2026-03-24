package constant

const (
	ShipmentGetCount = 5 // 获取5条数据

	// 订单状态
	ShipmentStatusSleeping = 1 // 待创建任务
	ShipmentStatusWaiting  = 2 // 待执行
	ShipmentStatusWorking  = 3 // 执行中
	ShipmentStatusFinish   = 4 // 已完成
)

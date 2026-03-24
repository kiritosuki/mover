package constant

const (
	ShipmentGetCount = 5 // 获取5条数据

	// 订单状态
	ShipmentStatusSleeping = 1 // 待创建任务
	ShipmentStatusWaiting  = 2 // 待取货
	ShipmentStatusWorking  = 3 // 运输中
	ShipmentStatusFinish   = 4 // 已完成
)

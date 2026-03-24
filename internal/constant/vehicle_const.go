package constant

const (
	// 模拟车辆移动更新间隔时间
	VehicleSimulateMovingGap = 10 // 10s执行一次

	// vehicle 的类型
	VehicleTybeNormal = 1 // 普通
	VehicleTybeDanger = 2 // 危化品

	// vehicle 的状态
	VehicleStatusRunning = 1 // 行驶中
	VehicleStatusFree    = 2 // 空闲中

	// 装卸时间系数：每吨货物的装卸耗时(秒)
	LoadUnloadTimePerTon = 60.0
)

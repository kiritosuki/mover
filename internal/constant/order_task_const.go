package constant

const (
	// 任务创建间隔时间
	OrderTaskCreatingGap = 20 // 20s执行一次

	// 任务阶段
	OrderTaskSequentialAccepting    = 1 // 出发接单阶段
	OrderTaskSequentialTransporting = 2 // 运货送往目的地阶段
	OrderTaskSequentialFinish       = 3 // 任务完成
)

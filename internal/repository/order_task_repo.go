package repository

import (
	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/database"
	"github.com/kiritosuki/mover/internal/model"
)

// CreateOrderTask 插入一条任务
func CreateOrderTask(task *model.OrderTask) error {
	// 获取数据库连接对象
	db := database.DB.Model(&model.OrderTask{})
	// 插入数据
	err := db.Create(task).Error
	return err
}

// GetRunningTasks 查询正在进行中的任务
func GetRunningTasks() ([]*model.OrderTask, error) {
	// 存放查询结果
	var tasks []*model.OrderTask
	db := database.DB.Model(&model.OrderTask{})
	db.Where("sequential != ?", constant.OrderTaskSequentialFinish)
	err := db.Find(&tasks).Error
	return tasks, err
}

// UpdateTaskSequential 更新任务进入下一阶段
func UpdateTaskSequential(id int, status int, oldStatus int) error {
	// 获取数据库连接对象
	db := database.DB.Model(&model.OrderTask{})
	// CAS锁
	db = db.Where("id = ? and status = ?", id, oldStatus)
	db = db.Update("status", status)
	return db.Error
}

package repository

import (
	"github.com/kiritosuki/mover/internal/constant"
	"github.com/kiritosuki/mover/internal/database"
	"github.com/kiritosuki/mover/internal/model"
)

// GetSleepingShipment 根据更新时间 获取前ShipmentGetCount条待创建任务的订单
func GetSleepingShipment() ([]*model.Shipment, error) {
	// 存放返回结果
	var shipments []*model.Shipment
	// 获取数据库连接对象
	db := database.DB.Model(&model.Shipment{})
	// 条件查询
	db = db.Where("status = ?", constant.ShipmentStatusSleeping)
	db = db.Order("create_time ASC") // 早的在前
	db = db.Limit(10)
	// 写入结果
	err := db.Find(&shipments).Error
	return shipments, err
}

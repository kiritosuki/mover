package repository

import (
	"github.com/kiritosuki/mover/internal/database"
	"github.com/kiritosuki/mover/internal/model"
)

// ListVehicles 筛选/获取车辆列表
func ListVehicles(status int) ([]*model.Vehicle, error) {
	// 存放返回结果
	var vehicles []*model.Vehicle
	// 获取数据库连接对象
	db := database.DB.Model(&model.Vehicle{})
	// 条件查询
	db = db.Where("status = ?", status)
	// 写入结果
	err := db.Find(&vehicles).Error
	return vehicles, err
}

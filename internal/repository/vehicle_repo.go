package repository

import (
	"time"

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
	if status != 0 {
		db = db.Where("status = ?", status)
	}
	// 写入结果
	err := db.Find(&vehicles).Error
	return vehicles, err
}

// GetAllVehicles 获取所有车辆(不区分状态，用于指标统计)
func GetAllVehicles() ([]*model.Vehicle, error) {
	var vehicles []*model.Vehicle
	db := database.DB.Model(&model.Vehicle{})
	err := db.Find(&vehicles).Error
	return vehicles, err
}

// UpdateVehicleLocation 更新车辆位置
func UpdateVehicleLocation(id int, lon float64, lat float64) error {
	// 获取数据库连接对象
	db := database.DB.Model(&model.Vehicle{})
	db = db.Where("id = ?", id)
	err := db.Updates(map[string]interface{}{
		"lon":         lon,
		"lat":         lat,
		"update_time": time.Now(),
	}).Error
	return err
}

// UpdateVehicleStatus 更新车辆状态
func UpdateVehicleStatus(id int, status int, oldStatus int) error {
	// 获取数据库连接对象
	db := database.DB.Model(&model.Vehicle{})
	// CAS锁
	db = db.Where("id = ? and status = ?", id, oldStatus)
	db = db.Update("status", status)
	return db.Error
}

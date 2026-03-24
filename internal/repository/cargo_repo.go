package repository

import (
	"github.com/kiritosuki/mover/internal/database"
	"github.com/kiritosuki/mover/internal/model"
)

// GetCargo 根据id获取cargo
func GetCargo(id int) (*model.Cargo, error) {
	cargo := model.Cargo{}
	db := database.DB.Model(&model.Cargo{})
	err := db.First(&cargo, id).Error
	return &cargo, err
}

// GetAllCargos 获取所有货物类型(用于指标统计)
func GetAllCargos() ([]*model.Cargo, error) {
	var cargos []*model.Cargo
	db := database.DB.Model(&model.Cargo{})
	err := db.Find(&cargos).Error
	return cargos, err
}

package repository

import (
	"github.com/kiritosuki/mover/internal/database"
	"github.com/kiritosuki/mover/internal/model"
)

// GetCargo 根据 id 获取 Cargo 详情
func GetCargo(id int) (*model.Cargo, error) {
	var cargo model.Cargo
	db := database.DB.Model(&model.Cargo{})
	err := db.First(&cargo, id).Error
	return &cargo, err
}


package model

import "time"

// TableName 指明关联的数据库
// grom 中默认会关联复数表名 否则需要手动指明
func (vehicle Vehicle) TableName() string {
	return "vehicle"
}

// Vehicle 车辆
type Vehicle struct {
	Id         uint      `gorm:"primaryKey" json:"id"`
	Lon        float64   `json:"lon"`
	Lat        float64   `json:"lat"`
	Speed      float64   `json:"speed"`
	UpdateTime time.Time `json:"updateTime"`
	Status     int       `json:"status"`
}

package task

import (
	"testing"
	"time"

	"github.com/kiritosuki/mover/internal/Logger"
	"github.com/kiritosuki/mover/internal/model"
)

func TestCalculateCost(t *testing.T) {
	// Initialize Logger
	Logger.InitLogger()

	v := &model.Vehicle{
		Id:       1,
		Lon:      116.397428,
		Lat:      39.90923,
		Speed:    10.0,
		Capacity: 20,
	}

	shipment := &model.Shipment{
		Id:         101,
		CreateTime: time.Now().Add(-1 * time.Hour), // Waiting for 1 hour
		Count:      15,
	}

	stats := &Statistics{
		WaitTimeStats: TimeMetrics{
			Max:    1800, // 30 mins
			Total:  3600,
			Avg:    600,
			StdDev: 100,
			WaitTransRatio: 0.2,
		},
		EmptyDistStats: DistanceMetrics{
			TotalEmpty:     5000,
			EmptyFullRatio: 0.5,
		},
		UtilizationStats: UtilizationMetrics{
			TotalCapacity: 100,
			UsedCapacity:  50,
		},
	}

	cargo := &model.Cargo{
		Weight: 1000, // 1 ton
	}

	startPoi := &model.Poi{
		Lon: 116.407428,
		Lat: 39.91923,
	}

	cost := CalculateCost(v, shipment, stats, cargo, startPoi)
	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}
	t.Logf("Calculated Cost: %f", cost)
}

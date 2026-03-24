package task

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/kiritosuki/mover/internal/config"
	"github.com/kiritosuki/mover/internal/constant"
)

type RouteResponse struct {
	Count string `json:"count"`
	Route struct {
		Paths []struct {
			Distance string `json:"distance"` // 行驶距离 米
			Duration string `json:"duration"` // 估计用时 秒
			Steps    []struct {
				Polyline string `json:"polyline"`
			} `json:"steps"`
		} `json:"paths"`
	} `json:"route"`
}

func PlanRoute(startLon float64, startLat float64, endLon float64, endLat float64) (*RouteResponse, error) {
	// 获取 apikey
	apiKey := config.VP.GetString("amap.key")
	// 获取路线规划api的URL
	baseURL := constant.RoutePlanningURL
	// url参数
	params := url.Values{}
	params.Add("key", apiKey)
	// 注意这里小数点不要超过六位
	params.Add("origin", fmt.Sprintf("%.6f,%.6f", startLon, startLat))
	params.Add("destination", fmt.Sprintf("%.6f,%.6f", endLon, endLat))
	fullURL := baseURL + "?" + params.Encode()
	// 创建连接客户端
	client := &http.Client{
		// 设置超时时间
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status error: %d", resp.StatusCode)
	}

	// 解析 JSON
	var result RouteResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil

}

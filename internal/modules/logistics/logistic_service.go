package logistics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"dispatch-and-delivery/internal/models"

	"github.com/google/uuid"
)

// ServiceInterface 定义物流模块对 Handler 暴露的所有业务方法。
// 与 Handler 一一对应，职责清晰。
type ServiceInterface interface {
	ListMachines(ctx context.Context) ([]*models.Machine, error)
	SetMachineStatus(ctx context.Context, machineID string, req models.MachineStatusUpdateRequest) error
	AssignOrder(ctx context.Context, orderID string) (*models.Machine, error)
	CalculateRouteOptions(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error)
	ComputeRoute(ctx context.Context, orderID string) (*models.Route, error)
	ReportTracking(ctx context.Context, orderID string, req models.TrackingEventRequest) error
	GetTracking(ctx context.Context, orderID string, since time.Time) ([]*models.TrackingEvent, error)
}

// service 是 ServiceInterface 的实现，依赖 Repository。
type service struct {
	logisticRepo RepositoryInterface
	httpClient   *http.Client
	apiKey       string
}

const (
	droneMaxWeightKG = 3.0
	droneMaxDimM     = 0.5
	robotMaxWeightKG = 10.0
	robotMaxDimM     = 1.0
)

// NewService 构造函数，注入仓库与 Google Maps API Key
func NewService(logisticRepo RepositoryInterface, apiKey string) ServiceInterface {
	return &service{
		logisticRepo: logisticRepo,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		apiKey:       apiKey,
	}
}

// ListMachines 直接代理到 repo.ListMachines
func (s *service) ListMachines(ctx context.Context) ([]*models.Machine, error) {
	return s.logisticRepo.ListMachines(ctx)
}

// SetMachineStatus 先查询旧记录，再更新状态与位置，保持电量不变
func (s *service) SetMachineStatus(ctx context.Context, machineID string, req models.MachineStatusUpdateRequest) error {
	m, err := s.logisticRepo.FindMachineByID(ctx, machineID)
	if err != nil {
		return err
	}
	m.Status = req.Status
	m.Latitude = req.Latitude
	m.Longitude = req.Longitude
	// BatteryLevel 保持原值
	return s.logisticRepo.UpdateMachine(ctx, m)
}

// AssignOrder 为订单分配一台空闲机器并更新数据库
func (s *service) AssignOrder(ctx context.Context, orderID string) (*models.Machine, error) {
	machines, err := s.logisticRepo.ListIdleMachines(ctx)
	if err != nil {
		return nil, err
	}
	if len(machines) == 0 {
		return nil, fmt.Errorf("no idle machines available")
	}
	m := machines[0]
	if err := s.logisticRepo.AssignOrder(ctx, orderID, m.ID); err != nil {
		return nil, err
	}
	if err := s.logisticRepo.UpdateMachineStatus(ctx, m.ID, models.StatusInTransit); err != nil {
		return nil, err
	}
	m.Status = models.StatusInTransit
	return m, nil
}

// CalculateRouteOptions 调用地图 API 并计算两种报价
func (s *service) CalculateRouteOptions(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error) {
	// 调用 Google Maps
	pickup := req.PickupLocation.StreetAddress
	dropoff := req.DeliveryLocation.StreetAddress
	dMeters, dSeconds, polyline, err := s.callGoogleMaps(ctx, pickup, dropoff)
	if err != nil {
		return nil, fmt.Errorf("CalculateRouteOptions: maps API: %w", err)
	}
	// 高峰判断
	peak := isPeakHour(req.RequestedTime)

	if req.WeightKG > robotMaxWeightKG ||
		req.Dimensions.Length > robotMaxDimM ||
		req.Dimensions.Width > robotMaxDimM ||
		req.Dimensions.Height > robotMaxDimM {
		return nil, models.ErrPackageTooLarge
	}

	useDrone := req.WeightKG <= droneMaxWeightKG &&
		req.Dimensions.Length <= droneMaxDimM &&
		req.Dimensions.Width <= droneMaxDimM &&
		req.Dimensions.Height <= droneMaxDimM

	// “最快” 使用 DRONE
	fastest := models.RouteOption{
		ID:               uuid.NewString(),
		PickupLocation:   req.PickupLocation,
		DeliveryLocation: req.DeliveryLocation,
		Polyline:         polyline,
		DistanceMeters:   dMeters,
		DurationSeconds:  dSeconds,
		Strategy:         models.FastestStrategy,
		EstimatedCost:    computeCost(dMeters, dSeconds, models.MachineTypeDrone, peak),
		MachineType:      models.MachineTypeDrone,
	}

	//  “最便宜” 使用 ROBOT
	cheapest := models.RouteOption{
		ID:               uuid.NewString(),
		PickupLocation:   req.PickupLocation,
		DeliveryLocation: req.DeliveryLocation,
		Polyline:         polyline,
		DistanceMeters:   dMeters,
		DurationSeconds:  int(math.Ceil(float64(dSeconds)*2)), // 假设地面速度为飞行一半
		Strategy:         models.CheapestStrategy,
		EstimatedCost:    computeCost(dMeters, dSeconds, models.MachineTypeRobot, peak),
		MachineType:      models.MachineTypeRobot,
	}

	options := []models.RouteOption{}
	if useDrone {
		options = append(options, fastest)
	}
	options = append(options, cheapest)
	return options, nil
}

// ComputeRoute 生成并持久化实际路线
func (s *service) ComputeRoute(ctx context.Context, orderID string) (*models.Route, error) {
	// 1) 获取地址
	pickup, dropoff, err := s.logisticRepo.GetOrderAddresses(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("ComputeRoute: fetch addresses: %w", err)
	}
	// 2) 调用 Google Maps
	dMeters, dSeconds, polyline, err := s.callGoogleMaps(ctx, pickup, dropoff)
	if err != nil {
		return nil, fmt.Errorf("ComputeRoute: maps API: %w", err)
	}
	// 3) 构造模型
	route := &models.Route{
		OrderID:         orderID,
		Polyline:        polyline,
		DistanceMeters:  dMeters,
		DurationSeconds: dSeconds,
	}
	// 4) 持久化
	if err := s.logisticRepo.SaveRoute(ctx, route); err != nil {
		return nil, fmt.Errorf("ComputeRoute: save route: %w", err)
	}
	return route, nil
}

// ReportTracking 上报轨迹事件
func (s *service) ReportTracking(ctx context.Context, orderID string, req models.TrackingEventRequest) error {
	return s.logisticRepo.CreateTrackingEvent(ctx, &models.TrackingEvent{
		OrderID:   orderID,
		MachineID: req.MachineID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	})
}

// GetTracking 查询轨迹事件列表
func (s *service) GetTracking(ctx context.Context, orderID string, since time.Time) ([]*models.TrackingEvent, error) {
	return s.logisticRepo.ListTrackingEvents(ctx, orderID, since)
}

// callGoogleMaps 调用 Google Maps Directions API 获取路线信息
// 返回距离（米）、时长（秒）和多段线编码
func (s *service) callGoogleMaps(ctx context.Context, origin, destination string) (int, int, string, error) {
	u := "https://maps.googleapis.com/maps/api/directions/json"
	params := url.Values{}
	params.Set("origin", origin)
	params.Set("destination", destination)
	params.Set("key", s.apiKey)
	resp, err := s.httpClient.Get(u + "?" + params.Encode())
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	var out struct {
		Routes []struct {
			OverviewPolyline struct{ Points string } `json:"overview_polyline"`
			Legs             []struct {
				Distance struct{ Value int } `json:"distance"`
				Duration struct{ Value int } `json:"duration"`
			} `json:"legs"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, 0, "", err
	}
	if len(out.Routes) == 0 || len(out.Routes[0].Legs) == 0 {
		return 0, 0, "", fmt.Errorf("no route data")
	}
	leg := out.Routes[0].Legs[0]
	return leg.Distance.Value, leg.Duration.Value, out.Routes[0].OverviewPolyline.Points, nil
}

// computeCost 根据距离、时长、机器类型和是否高峰期计算价格
// 说明：
//  1) 基础费 base + 单位距离费/Km * km
//  2) 高峰期乘以 peakMultiplier
//  3) 根据机器类型(drone/robot)应用不同 base/perKm
func computeCost(distanceMeters, durationSeconds int, machineType string, peak bool) float64 {
	// 1) 转换距离为公里
	km := float64(distanceMeters) / 1000.0
	// 2) 机器类型参数
	var base, perKm float64
	switch machineType {
	case models.MachineTypeDrone:
		base, perKm = 2.0, 0.5 // Drone 起步价和单位公里费
	default:
		base, perKm = 1.0, 0.3 // Robot 起步价和单位公里费
	}
	price := base + perKm*km // 3) 计算初始价格
	// 4) 高峰期加价20%
	if peak {
		price *= 1.2
	}
	// 5) 保留两位小数
	return math.Round(price*100) / 100
}
// isPeakHour 判断给定时间是否属于高峰期
// 支持传入请求时间，当为零值时使用当前时间
func isPeakHour(requestedTime time.Time) bool {
	t := requestedTime
	if t.IsZero() {
		t = time.Now()
	}
	h := t.Hour()
	return (h >= 8 && h <= 10) || (h >= 17 && h <= 19)
}

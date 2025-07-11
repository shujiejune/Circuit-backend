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

// GoogleMapsAPIKey 用于调用 Directions API，请替换为真实 Key
const GoogleMapsAPIKey = "AIzaSyDC1lgkLFVv8kBH7Veaj7ywNFrvDWvWexE"

// ServiceInterface 定义物流模块对 Handler 暴露的所有业务方法。
// 与 Handler 一一对应，职责清晰。
type ServiceInterface interface {
	ListMachines(ctx context.Context) ([]*models.Machine, error)
	SetMachineStatus(ctx context.Context, machineID string, req models.MachineStatusUpdateRequest) error
	AssignOrder(ctx context.Context, orderID string) (*models.Machine, error)
	CalculateRouteOptions(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error)
	ComputeRoute(ctx context.Context, orderID string) (*models.Route, error)
	ReportTracking(ctx context.Context, orderID string, req models.TrackingEventRequest) error
	GetTracking(ctx context.Context, orderID string) ([]*models.TrackingEvent, error)
}

// 为物流服务的依赖注入添加了 AssignServiceInterface，使分配逻辑保持可插入
type AssignServiceInterface interface {
	AssignOrder(ctx context.Context, orderID string) (*models.Machine, error)
}

// service 是 ServiceInterface 的实现，依赖 Repository 和 AssignService。
type service struct {
	repo          RepositoryInterface
	assignService AssignServiceInterface
	httpClient    *http.Client
	apiKey        string
}

// NewService 构造函数，注入 repo、assignService 与 HTTP 客户端
func NewService(repo RepositoryInterface, assignSvc AssignServiceInterface) ServiceInterface {
	return &service{
		repo:          repo,
		assignService: assignSvc,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		apiKey:        GoogleMapsAPIKey,
	}
}

// ListMachines 直接代理到 repo.ListMachines
func (s *service) ListMachines(ctx context.Context) ([]*models.Machine, error) {
	return s.repo.ListMachines(ctx)
}

// SetMachineStatus 先查询旧记录，再更新状态与位置，保持电量不变
func (s *service) SetMachineStatus(ctx context.Context, machineID string, req models.MachineStatusUpdateRequest) error {
	m, err := s.repo.FindMachineByID(ctx, machineID)
	if err != nil {
		return err
	}
	m.Status = req.Status
	m.Latitude = req.Latitude
	m.Longitude = req.Longitude
	// BatteryLevel 保持原值
	return s.repo.UpdateMachine(ctx, m)
}

// AssignOrder 手动或支付后自动派单逻辑复用 AssignService
func (s *service) AssignOrder(ctx context.Context, orderID string) (*models.Machine, error) {
	return s.assignService.AssignOrder(ctx, orderID)
}

// CalculateRouteOptions 调用地图 API 并计算两种报价
func (s *service) CalculateRouteOptions(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error) {
	// 1) 查询地址
	pickup, dropoff, err := s.repo.GetOrderAddresses(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("CalculateRouteOptions: fetch addresses: %w", err)
	}
	// 2) 调用 Google Maps
	dMeters, dSeconds, polyline, err := s.callGoogleMaps(ctx, pickup, dropoff)
	if err != nil {
		return nil, fmt.Errorf("CalculateRouteOptions: maps API: %w", err)
	}
	// 3) 高峰判断
	peak := isPeakHour(req.RequestedTime)

	// 4) “最快” 使用 DRONE
	fastest := models.RouteOption{
		ID:               uuid.NewString(),
		PickupLocation:   pickup,
		DeliveryLocation: dropoff,
		Polyline:         polyline,
		DistanceMeters:   dMeters,
		DurationSeconds:  dSeconds,
		Strategy:         models.FastestStrategy,
		EstimatedCost:    computeCost(dMeters, dSeconds, models.MachineTypeDrone, peak),
		MachineType:      models.MachineTypeDrone,
	}

	// 5) “最便宜” 使用 ROBOT
	cheapest := models.RouteOption{
		ID:               uuid.NewString(),
		PickupLocation:   pickup,
		DeliveryLocation: dropoff,
		Polyline:         polyline,
		DistanceMeters:   dMeters,
		DurationSeconds:  int(math.Ceil(float64(dSeconds) * 2)), // 假设地面速度为飞行一半
		Strategy:         models.CheapestStrategy,
		EstimatedCost:    computeCost(dMeters, dSeconds, models.MachineTypeRobot, peak),
		MachineType:      models.MachineTypeRobot,
	}

	return []models.RouteOption{fastest, cheapest}, nil
}

// ComputeRoute 生成并持久化实际路线
func (s *service) ComputeRoute(ctx context.Context, orderID string) (*models.Route, error) {
	// 1) 获取地址
	pickup, dropoff, err := s.repo.GetOrderAddresses(ctx, orderID)
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
	if err := s.repo.SaveRoute(ctx, route); err != nil {
		return nil, fmt.Errorf("ComputeRoute: save route: %w", err)
	}
	return route, nil
}

// ReportTracking 上报轨迹事件
func (s *service) ReportTracking(ctx context.Context, orderID string, req models.TrackingEventRequest) error {
	return s.repo.CreateTrackingEvent(ctx, &models.TrackingEvent{
		OrderID:   orderID,
		MachineID: req.MachineID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	})
}

// GetTracking 查询轨迹事件列表
func (s *service) GetTracking(ctx context.Context, orderID string) ([]*models.TrackingEvent, error) {
	return s.repo.ListTrackingEvents(ctx, orderID)
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
//  1. 基础费 base + 单位距离费/Km * km
//  2. 高峰期乘以 peakMultiplier
//  3. 根据机器类型(drone/robot)应用不同 base/perKm
func computeCost(distanceMeters, durationSeconds int, machineType string, peak bool) float64 {
	// 1) 转换距离为公里
	km := float64(distanceMeters) / 1000.0
	// 2) 机器类型参数
	var base, perKm float64
	switch machineType {
	case models.MachineTypeDrone:
		base, perKm = 5.0, 1.2 // Drone 起步价和单位公里费
	default:
		base, perKm = 3.0, 0.8 // Robot 起步价和单位公里费
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

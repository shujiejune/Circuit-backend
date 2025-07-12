package logistics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"dispatch-and-delivery/internal/models"
)

// ----------------------------------------------------------------------------
// Mock HTTP RoundTrip: 用于模拟外部路由 API 响应
// ----------------------------------------------------------------------------
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// ----------------------------------------------------------------------------
// fakeRepo: 全权模拟后台存储行为
// - machines: 存放 Machine 对象的 map
// - orderDest: 存放 orderID → destination 的 map
// - ordersAssigned: 记录 AssignOrder 调用情况
// - routes: 存储 SaveRoute 调用产生的 Route 对象列表
// - trackingEvents: 存储 CreateTrackingEvent 调用产生的 TrackingEvent 列表
// ----------------------------------------------------------------------------
type fakeRepo struct {
	machines       map[string]*models.Machine
	orderDest      map[string]string
	ordersAssigned map[string]string
	routes         []*models.Route
	trackingEvents []*models.TrackingEvent
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		machines:       make(map[string]*models.Machine),
		orderDest:      make(map[string]string),
		ordersAssigned: make(map[string]string),
	}
}

// ----------------------------------------------------------------------------
// fakeRepo 方法实现：模仿真实 Repo 层行为，并记录调用结果供测试断言
// ----------------------------------------------------------------------------
func (f *fakeRepo) FindMachineByID(ctx context.Context, id string) (*models.Machine, error) {
	m, ok := f.machines[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *m
	return &cp, nil
}

func (f *fakeRepo) UpdateMachine(ctx context.Context, m *models.Machine) error {
	if _, ok := f.machines[m.ID]; !ok {
		return models.ErrNotFound
	}
	cp := *m
	f.machines[m.ID] = &cp
	return nil
}

func (f *fakeRepo) ListMachines(ctx context.Context) ([]*models.Machine, error) {
	out := make([]*models.Machine, 0, len(f.machines))
	for _, m := range f.machines {
		cp := *m
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeRepo) GetOrderAddresses(ctx context.Context, orderID string) (string, string, error) {
	dest, ok := f.orderDest[orderID]
	if !ok {
		return "", "", models.ErrNotFound
	}
	return "pickup-"+orderID, dest, nil
}

func (f *fakeRepo) SaveRoute(ctx context.Context, r *models.Route) error {
	// 模拟生成 ID 和时间戳
	r.ID = fmt.Sprintf("route-%d", len(f.routes)+1)
	r.CreatedAt = time.Now()
	f.routes = append(f.routes, r)
	return nil
}

func (f *fakeRepo) GetOrderDestination(ctx context.Context, orderID string) (string, error) {
	dest, ok := f.orderDest[orderID]
	if !ok {
		return "", models.ErrNotFound
	}
	return dest, nil
}

func (f *fakeRepo) ListIdleMachines(ctx context.Context) ([]*models.Machine, error) {
	out := []*models.Machine{}
	for _, m := range f.machines {
		if m.Status == models.StatusIdle {
			cp := *m
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeRepo) AssignOrder(ctx context.Context, orderID, machineID string) error {
	if _, ok := f.machines[machineID]; !ok {
		return models.ErrNotFound
	}
	f.ordersAssigned[orderID] = machineID
	return nil
}

func (f *fakeRepo) UpdateMachineStatus(ctx context.Context, machineID, status string) error {
	m, ok := f.machines[machineID]
	if !ok {
		return models.ErrNotFound
	}
	m.Status = status
	m.UpdatedAt = time.Now()
	return nil
}

func (f *fakeRepo) CreateTrackingEvent(ctx context.Context, ev *models.TrackingEvent) error {
	ev.ID = fmt.Sprintf("track-%d", len(f.trackingEvents)+1)
	ev.CreatedAt = time.Now()
	f.trackingEvents = append(f.trackingEvents, ev)
	return nil
}

func (f *fakeRepo) ListTrackingEvents(ctx context.Context, orderID string, since time.Time) ([]*models.TrackingEvent, error) {
	out := []*models.TrackingEvent{}
	for _, ev := range f.trackingEvents {
		if ev.OrderID == orderID && ev.CreatedAt.After(since) {
			cp := *ev
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// newTestService: 构造带有 FakeRepo 和可定制 HTTP 模拟响应的 Service 实例
// ----------------------------------------------------------------------------
func newTestService(fr *fakeRepo, respBody string) ServiceInterface {
	svc := NewService(fr, "test").(*service)
	svc.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// 模拟 API 返回 JSON 格式的路线数据
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(respBody)),
				Header:     http.Header{},
			}, nil
		}),
	}
	return svc
}

// ----------------------------------------------------------------------------
// 单元测试：针对各业务函数的功能与 FakeRepo 状态变更做完整覆盖
// ----------------------------------------------------------------------------

func TestIsPeakHour(t *testing.T) {
	// 验证早上 9 点属于高峰期，14 点不属于
	times := []struct {
		at   time.Time
		want bool
	}{
		{time.Date(2023, 1, 1, 9, 0, 0, 0, time.UTC), true},
		{time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC), false},
	}
	for _, tt := range times {
		got := isPeakHour(tt.at)
		if got != tt.want {
			t.Errorf("isPeakHour(%v) = %v; want %v", tt.at, got, tt.want)
		}
	}
}

func TestComputeCost(t *testing.T) {
	// 非高峰：Drone 1000m、600s → 单价 0.0025 → 总价 2.50
	c := computeCost(1000, 600, models.MachineTypeDrone, false)
	if c != 2.5 {
		t.Errorf("computeCost non-peak drone = %.2f; want 2.50", c)
	}
	// 高峰：Robot 1000m、600s → 基价 1.0 + 高峰倍率 1.2 = 1.2 → 四舍五入 1.2
	c2 := computeCost(1000, 600, models.MachineTypeRobot, true)
	if c2 != 1.2 {
		t.Errorf("computeCost peak robot = %.2f; want 1.20", c2)
	}
}

func TestCalculateRouteOptions(t *testing.T) {
	fr := newFakeRepo()
	// 预置 orderID → 地址映射
	fr.orderDest["order123"] = "DEST-123"
	// 模拟 Google Routes API 返回的数据
	resp := `{"routes":[{"overview_polyline":{"points":"abc"},"legs":[{"distance":{"value":1000},"duration":{"value":600}}]},{"overview_polyline":{"points":"def"},"legs":[{"distance":{"value":2000},"duration":{"value":1200}}]}]}`
	svc := newTestService(fr, resp)

	// 构造请求：1000m/600s 的第一条为最快，第二条为最便宜
	req := models.RouteRequest{
		PickupLocation:   models.Address{StreetAddress: "A"},
		DeliveryLocation: models.Address{StreetAddress: "B"},
		WeightKG:         2,
		Dimensions:       models.Dimensions{Length: 0.3, Width: 0.3, Height: 0.3},
		RequestedTime:    time.Date(2023, 1, 1, 9, 0, 0, 0, time.UTC),
	}
	opts, err := svc.CalculateRouteOptions(context.Background(), req)
	if err != nil {
		t.Fatalf("CalculateRouteOptions error: %v", err)
	}
	// 返回了 2 种选项：最快和最便宜
	if len(opts) != 2 {
		t.Fatalf("got %d options; want 2", len(opts))
	}

	// Fastest: Drone
	fast := opts[0]
	if fast.MachineType != models.MachineTypeDrone {
		t.Errorf("fastest MachineType = %s; want Drone", fast.MachineType)
	}
	if fast.DurationSeconds != 600 {
		t.Errorf("fastest DurationSeconds = %d; want 600", fast.DurationSeconds)
	}
	if fast.EstimatedCost != computeCost(1000, 600, models.MachineTypeDrone, true) {
		t.Errorf("fastest EstimatedCost = %.2f; want %.2f", fast.EstimatedCost, computeCost(1000, 600, models.MachineTypeDrone, true))
	}

	// Cheapest: Robot
	cheap := opts[1]
	if cheap.MachineType != models.MachineTypeRobot {
		t.Errorf("cheapest MachineType = %s; want Robot", cheap.MachineType)
	}
	if cheap.DurationSeconds != 1200 {
		t.Errorf("cheapest DurationSeconds = %d; want 1200", cheap.DurationSeconds)
	}
	if cheap.EstimatedCost != computeCost(2000, 1200, models.MachineTypeRobot, true) {
		t.Errorf("cheapest EstimatedCost = %.2f; want %.2f", cheap.EstimatedCost, computeCost(2000, 1200, models.MachineTypeRobot, true))
	}

	// 确认 SaveRoute 被调用，fakeRepo 中 routes 列表新增了 2 条
	if len(fr.routes) != 2 {
		t.Errorf("fakeRepo.routes length = %d; want 2", len(fr.routes))
	}
}

func TestAssignOrderAndStatusUpdate(t *testing.T) {
	fr := newFakeRepo()
	// 预置两台空闲机器
	fr.machines["m1"] = &models.Machine{ID: "m1", Status: models.StatusIdle}
	fr.machines["m2"] = &models.Machine{ID: "m2", Status: models.StatusIdle}
	svc := NewService(fr, "test")

	// 分配订单 o1，应挑选 m1
	m, err := svc.AssignOrder(context.Background(), "o1")
	if err != nil {
		t.Fatalf("AssignOrder error: %v", err)
	}
	// 返回正确的机器，并写入 repo
	if m.ID != "m1" {
		t.Errorf("AssignOrder returned ID = %s; want m1", m.ID)
	}
	if got, ok := fr.ordersAssigned["o1"]; !ok || got != "m1" {
		t.Errorf("fakeRepo.ordersAssigned[\"o1\"] = %s; want m1", got)
	}
	// 机器状态应更新为 InTransit
	if fr.machines["m1"].Status != models.StatusInTransit {
		t.Errorf("machine m1 Status = %s; want InTransit", fr.machines["m1"].Status)
	}
}

func TestSetMachineStatus(t *testing.T) {
	fr := newFakeRepo()
	// 预置一台机器
	fr.machines["m1"] = &models.Machine{
		ID:        "m1",
		Status:    models.StatusIdle,
		Latitude:  1.0,
		Longitude: 2.0,
	}
	svc := NewService(fr, "test")

	// 更新状态及位置
	req := models.MachineStatusUpdateRequest{
		Status:    models.StatusCharging,
		Latitude:  3.0,
		Longitude: 4.0,
	}
	if err := svc.SetMachineStatus(context.Background(), "m1", req); err != nil {
		t.Fatalf("SetMachineStatus error: %v", err)
	}
	// 验证 repo 中的机器记录已被更新
	updated := fr.machines["m1"]
	if updated.Status != models.StatusCharging {
		t.Errorf("updated.Status = %s; want Charging", updated.Status)
	}
	if updated.Latitude != 3.0 || updated.Longitude != 4.0 {
		t.Errorf("updated coords = (%f,%f); want (3,4)", updated.Latitude, updated.Longitude)
	}
}

func TestComputeRoute(t *testing.T) {
	fr := newFakeRepo()
	// 预置目的地映射
	fr.orderDest["o1"] = "dest-X"
	resp := `{"routes":[{"overview_polyline":{"points":"xyz"},"legs":[{"distance":{"value":500},"duration":{"value":300}}]}]}`
	svc := newTestService(fr, resp)

	route, err := svc.ComputeRoute(context.Background(), "o1")
	if err != nil {
		t.Fatalf("ComputeRoute error: %v", err)
	}
	// 验证返回值及 repo 保存行为
	if route.DistanceMeters != 500 {
		t.Errorf("ComputeRoute DistanceMeters = %d; want 500", route.DistanceMeters)
	}
	if len(fr.routes) != 1 {
		t.Errorf("fakeRepo.routes length = %d; want 1", len(fr.routes))
	}
}

func TestTrackingEvents(t *testing.T) {
    fr := newFakeRepo()
    svc := NewService(fr, "test")
    ctx := context.Background()

  
    err := svc.ReportTracking(ctx, "order-1", models.TrackingEventRequest{
        MachineID: "",         
        Latitude:  12.34,
        Longitude: 56.78,
    })
    if err != nil {
        t.Fatalf("ReportTracking error: %v", err)
    }
    err = svc.ReportTracking(ctx, "order-1", models.TrackingEventRequest{
        MachineID: "",
        Latitude:  98.76,
        Longitude: 54.32,
    })
    if err != nil {
        t.Fatalf("ReportTracking error: %v", err)
    }

    evs, err := svc.GetTracking(ctx, "order-1", time.Time{})
    if err != nil {
        t.Fatalf("GetTracking error: %v", err)
    }

    if len(evs) != 2 {
        t.Errorf("GetTracking returned %d; want 2", len(evs))
    }
    if len(fr.trackingEvents) != 2 {
        t.Errorf("fakeRepo.trackingEvents length = %d; want 2", len(fr.trackingEvents))
    }
}

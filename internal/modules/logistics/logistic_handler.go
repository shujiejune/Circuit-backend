package logistics

import (
	"fmt"
	"net/http"
	"time"

	"dispatch-and-delivery/internal/models"
	"github.com/labstack/echo/v4"
)

// Handler 聚合了物流模块所有 HTTP 接口，
// 负责参数校验、调用 Service 层，并返回规范化的 JSON 响应。
// 错误响应的 Message 字段为 English，方便前端统一处理；
// 所有逻辑注释均为中文，详述每一步算法和流程。
type Handler struct {
	svc ServiceInterface
}

// NewHandler 构造函数，注入 Service，便于单元测试与扩展。
// svc 必须实现以下方法：
//   ListMachines(ctx) ([]*models.Machine, error)
//   SetMachineStatus(ctx, machineID, req) error
//   AssignOrder(ctx, orderID) (*models.Machine, error)
//   CalculateRouteOptions(ctx, req) ([]*models.RouteOption, error)
//   ComputeRoute(ctx, orderID) (*models.Route, error)
//   ReportTracking(ctx, orderID, req) error
//   GetTracking(ctx, orderID) ([]*models.TrackingEvent, error)
func NewHandler(svc ServiceInterface) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 在给定的 Echo 路由组中挂载所有物流相关路由。
// 顺序：机器管理 → 管理后台手动分配 → 客户报价 → 支付后自动派单 → 路线持久化 → 实时轨迹
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// 1) 机器管理
	g.GET("/fleet", h.GetFleet)
	g.PUT("/fleet/:machineId/status", h.SetMachineStatus)

	// 2) 管理后台手动分配
	g.POST("/admin/orders/:orderId/reassign", h.ReassignOrder)

	// 3) 客户端下单前报价
	g.POST("/orders/quote", h.CalculateQuote)

	// 4) 客户端支付完成后派单
	g.POST("/orders/:orderId/assign", h.ReassignOrder)

	// 5) 纯路线计算并持久化
	g.POST("/orders/:orderId/route", h.ComputeRoute)

	// 6) 轨迹上报与查询
	g.POST("/orders/:orderId/track", h.ReportTracking)
	g.GET("/orders/:orderId/track", h.GetTracking)
}

// ---- 1) 机器管理 ----

// GetFleet 返回所有机器的当前状态、位置和电量，供后台监控或展示。
// 调用 svc.ListMachines 并直接返回结果。
func (h *Handler) GetFleet(c echo.Context) error {
	ctx := c.Request().Context()
	machines, err := h.svc.ListMachines(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to list machines"})
	}
	return c.JSON(http.StatusOK, machines)
}

// SetMachineStatus 更新指定机器的状态与坐标。
//  1) 提取 path 中 machineId；
//  2) Bind JSON 为 models.MachineStatusUpdateRequest；
//  3) validate status；
//  4) 调用 svc.SetMachineStatus；
//  5) 返回 204 No Content。
func (h *Handler) SetMachineStatus(c echo.Context) error {
	ctx := c.Request().Context()
	machineID := c.Param("machineId")
	// 解析请求体
	var req models.MachineStatusUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "invalid request body"})
	}
	// 校验状态值是否合法
	if err := validateMachineStatus(req.Status); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: err.Error()})
	}
	// 调用服务层更新机器状态和位置
	if err := h.svc.SetMachineStatus(ctx, machineID, req); err != nil {
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "machine not found"})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to update machine"})
	}
	return c.NoContent(http.StatusNoContent)
}
// validateMachineStatus 用于校验机器状态值
func validateMachineStatus(status string) error {
	switch status {
	case models.StatusIdle, models.StatusInTransit, models.StatusMaintenance:
		return nil
	}
	return fmt.Errorf("invalid machine status: %s", status)
}

// ---- 2) 管理后台：手动重新分配 ----
// ReassignOrder 管理员调用以在异常场景下手动触发分配。
//  1) 提取 path 中 orderId；
//  2) 调用 svc.AssignOrder（内部完成验证、查询、选择与更新）；
//  3) 返回分配到的机器信息。
func (h *Handler) ReassignOrder(c echo.Context) error {
	ctx := c.Request().Context()
	orderID := c.Param("orderId")

	machine, err := h.svc.AssignOrder(ctx, orderID)
	if err != nil {
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "order or machine not found"})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to reassign order"})
	}
	return c.JSON(http.StatusOK, machine)
}

// ---- 3) 客户端：下单前报价 ----

// CalculateQuote 向前端返回“最快”和“最便宜”两种配送方案的估算。
//  1) Bind JSON 为 models.RouteRequest；
//  2) 默认 RequestedTime；
//  3) 调用 svc.CalculateRouteOptions；
//  4) 返回结果列表。
func (h *Handler) CalculateQuote(c echo.Context) error {
	ctx := c.Request().Context()
	var req models.RouteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "invalid request body"})
	}
	// 若客户端未指定期望取件/送达时间，则使用当前时间
	if req.RequestedTime.IsZero() {
		req.RequestedTime = time.Now()
	}

	options, err := h.svc.CalculateRouteOptions(ctx, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to calculate quote"})
	}
	return c.JSON(http.StatusOK, options)
}

// ---- 4) 客户端：支付完成后自动派单 ----
// AssignOrderAlias 为保持路由一致，复用 ReassignOrder 逻辑
func (h *Handler) AssignOrderAlias(c echo.Context) error {
	return h.ReassignOrder(c)
}

// ---- 5) 纯路线计算与持久化 ----

// ComputeRoute 生成并保存路径至 routes 表。
//  1) 提取 orderId；
//  2) 调用 svc.ComputeRoute；
//  3) 返回 models.Route 对象。
func (h *Handler) ComputeRoute(c echo.Context) error {
	ctx := c.Request().Context()
	orderID := c.Param("orderId")

	route, err := h.svc.ComputeRoute(ctx, orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to compute route"})
	}
	return c.JSON(http.StatusOK, route)
}

// ---- 6) 轨迹上报与查询 ----
// ReportTracking 持久化单次定位事件，用于实时或事后跟踪。
// Bind JSON → svc.ReportTracking → 201 Created
func (h *Handler) ReportTracking(c echo.Context) error {
	ctx := c.Request().Context()
	orderID := c.Param("orderId")

	var req models.TrackingEventRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.svc.ReportTracking(ctx, orderID, req); err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to record tracking"})
	}
	return c.NoContent(http.StatusCreated)
}

// GetTracking 返回指定订单的所有轨迹事件，按时间升序。
// 算法：svc.GetTracking → JSON 返回
func (h *Handler) GetTracking(c echo.Context) error {
	ctx := c.Request().Context()
	orderID := c.Param("orderId")

	events, err := h.svc.GetTracking(ctx, orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "failed to get tracking"})
	}
	return c.JSON(http.StatusOK, events)
}

// HandleTracking 目前仅作为占位实现，防止build error for WebSocket path。
func (h *Handler) HandleTracking(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
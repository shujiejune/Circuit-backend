package logistics

import (
    "context"
    "fmt"
    "time"

    "dispatch-and-delivery/internal/models"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface 定义物流模块所需的所有数据库操作。
// 包括：机器状态管理、路线存储、订单分配、轨迹追踪等方法。
type RepositoryInterface interface {
    // ===== Machine Status =====
    // FindMachineByID 根据机器 UUID 查询机器详情。
    FindMachineByID(ctx context.Context, id string) (*models.Machine, error)
    // UpdateMachine 更新机器状态、位置、以及电量等字段。
    UpdateMachine(ctx context.Context, m *models.Machine) error
    // ListMachines 查询所有机器信息，并按创建时间排序返回。
    ListMachines(ctx context.Context) ([]*models.Machine, error)

    // ===== Route =====
    // GetOrderAddresses 查询指定订单的取件地址和投递地址。
    GetOrderAddresses(ctx context.Context, orderID string) (pickup, dropoff string, err error)
    // SaveRoute 持久化计算出的路线数据（polyline、距离、时长）。
    SaveRoute(ctx context.Context, route *models.Route) error

    // ===== Assignment =====
    // GetOrderDestination 查询订单的投递地点（delivery_location 字段）。
    GetOrderDestination(ctx context.Context, orderID string) (string, error)
    // ListIdleMachines 查询所有当前状态为 'IDLE' 的机器列表。
    ListIdleMachines(ctx context.Context) ([]*models.Machine, error)
    // AssignOrder 将机器分配给订单：设置订单的 machine_id 与 status，并更新更新时间。
    AssignOrder(ctx context.Context, orderID, machineID string) error
    // UpdateMachineStatus 单独更新机器的 status 字段（不修改位置、电量等）。
    UpdateMachineStatus(ctx context.Context, machineID, status string) error

    // ===== Tracking =====
    // CreateTrackingEvent 新增一条订单轨迹事件，将机器位置写入 tracking_events 表。
    CreateTrackingEvent(ctx context.Context, event *models.TrackingEvent) error
    // ListTrackingEvents 按时间升序查询指定订单的所有轨迹事件，可选起始时间
    ListTrackingEvents(ctx context.Context, orderID string, since time.Time) ([]*models.TrackingEvent, error)
}

// Repository 实现 RepositoryInterface，使用 PostgreSQL (pgxpool.Pool) 与数据库交互。
type Repository struct {
    db *pgxpool.Pool // pgx 连接池
}

// NewRepository 创建 Repository 实例，传入已初始化的 *pgxpool.Pool。
func NewRepository(db *pgxpool.Pool) RepositoryInterface {
    return &Repository{db: db}
}

// ===== Machine Status 实现 =====

// FindMachineByID 根据机器 ID 从 machines 表中查询机器详情。
// 若未找到，返回 models.ErrNotFound；其他错误封装后返回。
func (r *Repository) FindMachineByID(ctx context.Context, id string) (*models.Machine, error) {
    const query = `
        SELECT id, type, status,
               COALESCE(ST_Y(current_location::geometry), 0) AS lat,
               COALESCE(ST_X(current_location::geometry), 0) AS lon,
               battery_level, created_at, updated_at
        FROM machines
        WHERE id = $1`
    row := r.db.QueryRow(ctx, query, id)

    m := &models.Machine{}
    if err := row.Scan(
        &m.ID, &m.Type, &m.Status,
        &m.Latitude, &m.Longitude,
        &m.BatteryLevel, &m.CreatedAt, &m.UpdatedAt,
    ); err != nil {
        if err == pgx.ErrNoRows {
            return nil, models.ErrNotFound
        }
        return nil, fmt.Errorf("FindMachineByID failed: %w", err)
    }
    return m, nil
}

// UpdateMachine 将机器的状态、位置和电量写回数据库。
// 使用 ST_SetSRID/ST_MakePoint 更新地理位置字段。
func (r *Repository) UpdateMachine(ctx context.Context, m *models.Machine) error {
    const query = `
        UPDATE machines
        SET status = $2,
            current_location = ST_SetSRID(ST_MakePoint($3, $4), 4326),
            battery_level = $5,
            updated_at = now()
        WHERE id = $1`
    cmd, err := r.db.Exec(ctx, query,
        m.ID, m.Status,
        m.Longitude, m.Latitude,
        m.BatteryLevel,
    )
    if err != nil {
        return fmt.Errorf("UpdateMachine failed: %w", err)
    }
    if cmd.RowsAffected() == 0 {
        return models.ErrNotFound
    }
    return nil
}

// ListMachines 查询所有机器信息，并按 created_at 升序排序返回。
// 完整加载每台机器的地理位置、电量和状态。
func (r *Repository) ListMachines(ctx context.Context) ([]*models.Machine, error) {
    const query = `
        SELECT id, type, status,
               COALESCE(ST_Y(current_location::geometry), 0) AS lat,
               COALESCE(ST_X(current_location::geometry), 0) AS lon,
               battery_level, created_at, updated_at
        FROM machines
        ORDER BY created_at`
    rows, err := r.db.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("ListMachines failed: %w", err)
    }
    defer rows.Close()

    var machines []*models.Machine
    for rows.Next() {
        m := &models.Machine{}
        if err := rows.Scan(
            &m.ID, &m.Type, &m.Status,
            &m.Latitude, &m.Longitude,
            &m.BatteryLevel, &m.CreatedAt, &m.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("ListMachines Scan failed: %w", err)
        }
        machines = append(machines, m)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("ListMachines rows failed: %w", err)
    }
    return machines, nil
}

// ===== Route 实现 =====

// GetOrderAddresses 从 orders 表中获取取件(pickup_location)和投递(delivery_location)地址。
// 常用于报价计算前的数据准备。
func (r *Repository) GetOrderAddresses(ctx context.Context, orderID string) (string, string, error) {
    const query = `
        SELECT pickup_location, delivery_location
        FROM orders
        WHERE id = $1`
    var pickup, dropoff string
    if err := r.db.QueryRow(ctx, query, orderID).Scan(&pickup, &dropoff); err != nil {
        return "", "", fmt.Errorf("GetOrderAddresses failed: %w", err)
    }
    return pickup, dropoff, nil
}

// SaveRoute 将计算出的路线数据持久化到 routes 表。
// polyline: Google Maps Polyline 编码；distance_meters: 距离；duration_seconds: 时长。
func (r *Repository) SaveRoute(ctx context.Context, route *models.Route) error {
    const query = `
        INSERT INTO routes (order_id, polyline, distance_meters, duration_seconds)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at`
    return r.db.QueryRow(ctx, query,
        route.OrderID, route.Polyline,
        route.DistanceMeters, route.DurationSeconds,
    ).Scan(&route.ID, &route.CreatedAt)
}

// ===== Assignment 实现 =====

// GetOrderDestination 查询订单的 delivery_location 字段，用于机器分配时获取目的地。
func (r *Repository) GetOrderDestination(ctx context.Context, orderID string) (string, error) {
    const query = `
        SELECT delivery_location
        FROM orders
        WHERE id = $1`
    var dest string
    if err := r.db.QueryRow(ctx, query, orderID).Scan(&dest); err != nil {
        if err == pgx.ErrNoRows {
            return "", models.ErrNotFound
        }
        return "", fmt.Errorf("GetOrderDestination failed: %w", err)
    }
    return dest, nil
}

// ListIdleMachines 查询 machines 表中所有 status = 'IDLE' 的机器，用于可用机器列表。
func (r *Repository) ListIdleMachines(ctx context.Context) ([]*models.Machine, error) {
    const query = `
        SELECT id, type, status,
               COALESCE(ST_Y(current_location::geometry), 0) AS lat,
               COALESCE(ST_X(current_location::geometry), 0) AS lon,
               battery_level, created_at, updated_at
        FROM machines
        WHERE status = 'IDLE'`
    rows, err := r.db.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("ListIdleMachines failed: %w", err)
    }
    defer rows.Close()

    var machines []*models.Machine
    for rows.Next() {
        m := &models.Machine{}
        if err := rows.Scan(
            &m.ID, &m.Type, &m.Status,
            &m.Latitude, &m.Longitude,
            &m.BatteryLevel, &m.CreatedAt, &m.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("ListIdleMachines Scan failed: %w", err)
        }
        machines = append(machines, m)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("ListIdleMachines rows failed: %w", err)
    }
    return machines, nil
}

// AssignOrder 将机器分配给订单：更新 orders.machine_id, orders.status, 并设置 updated_at。
func (r *Repository) AssignOrder(ctx context.Context, orderID, machineID string) error {
    const query = `
        UPDATE orders
        SET machine_id = $2,
            status = 'IN_PROGRESS',
            updated_at = now()
        WHERE id = $1`
    cmd, err := r.db.Exec(ctx, query, orderID, machineID)
    if err != nil {
        return fmt.Errorf("AssignOrder failed: %w", err)
    }
    if cmd.RowsAffected() == 0 {
        return models.ErrNotFound
    }
    return nil
}

// UpdateMachineStatus 单独更新 machines.status 字段及更新时间，用于分配后快速切换状态。
func (r *Repository) UpdateMachineStatus(ctx context.Context, machineID, status string) error {
    const query = `
        UPDATE machines
        SET status = $2,
            updated_at = now()
        WHERE id = $1`
    cmd, err := r.db.Exec(ctx, query, machineID, status)
    if err != nil {
        return fmt.Errorf("UpdateMachineStatus failed: %w", err)
    }
    if cmd.RowsAffected() == 0 {
        return models.ErrNotFound
    }
    return nil
}

// ===== Tracking 实现 =====

// CreateTrackingEvent 在 tracking_events 表中插入一条新记录，保存机器、位置和时间戳。
// location 字段使用 PostGIS 函数 ST_SetSRID(ST_MakePoint(lon, lat), 4326)。
func (r *Repository) CreateTrackingEvent(ctx context.Context, event *models.TrackingEvent) error {
    const query = `
        INSERT INTO tracking_events (order_id, machine_id, location)
        VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326))
        RETURNING id, created_at`
    return r.db.QueryRow(ctx, query,
        event.OrderID, event.MachineID,
        event.Longitude, event.Latitude,
    ).Scan(&event.ID, &event.CreatedAt)
}

// ListTrackingEvents 按 created_at 升序查询指定订单的所有轨迹事件，
// 并将经纬度解析为模型字段。
func (r *Repository) ListTrackingEvents(ctx context.Context, orderID string, since time.Time) ([]*models.TrackingEvent, error) {
    const query = `
        SELECT id, order_id, machine_id,
               COALESCE(ST_Y(location::geometry), 0) AS lat,
               COALESCE(ST_X(location::geometry), 0) AS lon,
               created_at
        FROM tracking_events
        WHERE order_id = $1 AND created_at > $2
        ORDER BY created_at`
    rows, err := r.db.Query(ctx, query, orderID, since)
    if err != nil {
        return nil, fmt.Errorf("ListTrackingEvents failed: %w", err)
    }
    defer rows.Close()

    var events []*models.TrackingEvent
    for rows.Next() {
        ev := &models.TrackingEvent{}
        if err := rows.Scan(
            &ev.ID, &ev.OrderID, &ev.MachineID,
            &ev.Latitude, &ev.Longitude,
            &ev.CreatedAt,
        ); err != nil {
            return nil, fmt.Errorf("ListTrackingEvents Scan failed: %w", err)
        }
        events = append(events, ev)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("ListTrackingEvents rows failed: %w", err)
    }
    return events, nil
}

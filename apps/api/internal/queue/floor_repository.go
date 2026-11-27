package queue

import (
	"context"
	"dinequeue.local/api/internal/auth"
)

func (r *Repository) floorSnapshot(ctx context.Context, queueID string) (FloorSnapshot, error) {
	snapshot := FloorSnapshot{Zones: []FloorZone{}, Assignments: []struct{}{}}
	result := r.db.WithContext(ctx).Raw(`SELECT q.id AS queue_id,q.restaurant_id,q.operations_mode,q.hold_ttl_seconds,
 COALESCE(f.layout_version,1) AS layout_version,transaction_timestamp() AS server_now
 FROM service_queues q LEFT JOIN restaurant_floor_state f ON f.restaurant_id=q.restaurant_id WHERE q.id=?`, queueID).Scan(&snapshot)
	if result.Error != nil {
		return snapshot, result.Error
	}
	if result.RowsAffected != 1 {
		return snapshot, auth.ErrForbidden
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT id,name,is_active FROM floor_zones WHERE restaurant_id=? ORDER BY id`, snapshot.RestaurantID).Scan(&snapshot.Zones).Error; err != nil {
		return snapshot, err
	}
	var tables []struct {
		FloorTable
		ZoneID string
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT id,zone_id,code,capacity,status,position_x,position_y,version FROM dining_tables WHERE restaurant_id=? ORDER BY code,id`, snapshot.RestaurantID).Scan(&tables).Error; err != nil {
		return snapshot, err
	}
	var edges []struct{ ZoneID, TableLowID, TableHighID string }
	if err := r.db.WithContext(ctx).Raw(`SELECT zone_id,table_low_id,table_high_id FROM table_connections WHERE restaurant_id=? AND is_active ORDER BY table_low_id,table_high_id`, snapshot.RestaurantID).Scan(&edges).Error; err != nil {
		return snapshot, err
	}
	for i := range snapshot.Zones {
		zone := &snapshot.Zones[i]
		zone.Tables = []FloorTable{}
		zone.Connections = []FloorConnection{}
		for _, table := range tables {
			if table.ZoneID == zone.ID {
				zone.Tables = append(zone.Tables, table.FloorTable)
			}
		}
		for _, edge := range edges {
			if edge.ZoneID == zone.ID {
				zone.Connections = append(zone.Connections, FloorConnection{[2]string{edge.TableLowID, edge.TableHighID}})
			}
		}
	}
	snapshot.ServerNow = snapshot.ServerNow.UTC()
	if snapshot.Zones == nil {
		snapshot.Zones = []FloorZone{}
	}
	snapshot.Assignments = []struct{}{}
	return snapshot, nil
}

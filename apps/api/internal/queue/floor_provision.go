package queue

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"sort"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrFloorConfiguration = errors.New("floor fixture conflicts with existing configuration")

// ProvisionFloor is an explicit CLI operation for a confirmed synthetic database.
// It does not seed accounts, activate seating, or rewrite existing table states.
func ProvisionFloor(ctx context.Context, database *gorm.DB, queueID, confirmedDatabase string) error {
	if !canonicalID(queueID) || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,51}_(demo|uat|test)$`).MatchString(confirmedDatabase) {
		return errors.New("floor provisioning requires a queue UUID and exact demo/UAT/test database confirmation")
	}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var actual string
		if err := tx.Raw("SELECT current_database()").Scan(&actual).Error; err != nil {
			return err
		}
		if actual != confirmedDatabase {
			return errors.New("database confirmation does not match")
		}
		var restaurantID string
		result := tx.Raw("SELECT restaurant_id FROM service_queues WHERE id=?", queueID).Scan(&restaurantID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		if err := tx.Exec("INSERT INTO restaurant_floor_state(restaurant_id) VALUES (?) ON CONFLICT DO NOTHING", restaurantID).Error; err != nil {
			return err
		}
		var version int64
		if err := tx.Raw("SELECT layout_version FROM restaurant_floor_state WHERE restaurant_id=? FOR UPDATE", restaurantID).Scan(&version).Error; err != nil {
			return err
		}
		// Configuration follows floor -> queue order, including a scope recheck.
		var lockedRestaurant string
		if err := tx.Raw("SELECT restaurant_id FROM service_queues WHERE id=? FOR UPDATE", queueID).Scan(&lockedRestaurant).Error; err != nil {
			return err
		}
		if lockedRestaurant != restaurantID {
			return ErrFloorConfiguration
		}
		expected := floorFixture(restaurantID)
		existing, err := NewRepository(tx).floorSnapshot(ctx, queueID)
		if err != nil {
			return err
		}
		if len(existing.Zones) > 0 {
			var edgeCount int64
			if err := tx.Table("table_connections").Where("restaurant_id=?", restaurantID).Count(&edgeCount).Error; err != nil {
				return err
			}
			for i := range existing.Zones {
				for j := range existing.Zones[i].Tables {
					existing.Zones[i].Tables[j].Status = ""
					existing.Zones[i].Tables[j].Version = 0
				}
			}
			if edgeCount != 4 || !reflect.DeepEqual(existing.Zones, expected) {
				return ErrFloorConfiguration
			}
			return nil
		}
		for _, zone := range expected {
			if err := tx.Exec("INSERT INTO floor_zones(id,restaurant_id,name) VALUES (?,?,?)", zone.ID, restaurantID, zone.Name).Error; err != nil {
				return err
			}
			for _, table := range zone.Tables {
				if err := tx.Exec(`INSERT INTO dining_tables(id,restaurant_id,zone_id,code,capacity,position_x,position_y) VALUES (?,?,?,?,?,?,?)`, table.ID, restaurantID, zone.ID, table.Code, table.Capacity, table.PositionX, table.PositionY).Error; err != nil {
					return err
				}
			}
			for _, edge := range zone.Connections {
				if err := tx.Exec(`INSERT INTO table_connections(restaurant_id,zone_id,table_low_id,table_high_id) VALUES (?,?,?,?)`, restaurantID, zone.ID, edge.TableIDs[0], edge.TableIDs[1]).Error; err != nil {
					return err
				}
			}
		}
		return tx.Exec("UPDATE restaurant_floor_state SET layout_version=layout_version+1 WHERE restaurant_id=?", restaurantID).Error
	})
}

// Stable identifiers derive from the restaurant, never a running sequence.
func floorFixture(restaurantID string) []FloorZone {
	namespace := uuid.MustParse(restaurantID)
	id := func(label string) string {
		return uuid.NewSHA1(namespace, []byte("DineQueue floor v1:"+label)).String()
	}
	zones := []FloorZone{}
	for _, spec := range []struct {
		name       string
		codes      []string
		capacities []int
	}{{"Main", []string{"T1", "T2", "T3"}, []int{2, 4, 8}}, {"Terrace", []string{"T4", "T5", "T6"}, []int{6, 4, 2}}, {"Private", []string{"T7"}, []int{8}}} {
		zone := FloorZone{ID: id(spec.name), Name: spec.name, IsActive: true, Tables: []FloorTable{}, Connections: []FloorConnection{}}
		for i, code := range spec.codes {
			zone.Tables = append(zone.Tables, FloorTable{ID: id(code), Code: code, Capacity: spec.capacities[i], PositionX: float64(20 + i*30), PositionY: 50})
		}
		for i := 1; i < len(zone.Tables); i++ {
			pair := []string{zone.Tables[i-1].ID, zone.Tables[i].ID}
			sort.Strings(pair)
			zone.Connections = append(zone.Connections, FloorConnection{[2]string{pair[0], pair[1]}})
		}
		sort.Slice(zone.Connections, func(i, j int) bool {
			a, b := zone.Connections[i].TableIDs, zone.Connections[j].TableIDs
			if a[0] != b[0] {
				return a[0] < b[0]
			}
			return a[1] < b[1]
		})
		zones = append(zones, zone)
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	return zones
}

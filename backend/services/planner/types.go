package planner

import "conexiuni-cluj/models"

const (
	earthRadiusMeters   = 6_371_000.0
	walkingSpeedMPerSec = 80.0 / 60.0
	transferWalkRadius  = 500.0
	endpointWalkRadius  = 1500.0
	maxEndpointStops    = 8
	maxBusLegs          = 3
	maxResults          = 12
	transferPenaltySec  = 60.0
	walkCostFactor      = 1.6
	maxTotalWalkMeters  = 2200.0
	maxTotalTimeSec     = 75 * 60.0
)

type ShapeKey struct {
	RouteID   int
	Direction models.DirectionType
}

type ShapeStop struct {
	StopID            int
	StopSequence      int
	OffsetArrivalTime float64
}

type ShapePath struct {
	Key            ShapeKey
	RouteShortName string
	TripIDs        []string
	Stops          []ShapeStop
}

type stopShapeRef struct {
	Key     ShapeKey
	StopIdx int
	Offset  float64
}

type walkNeighbor struct {
	StopID   int
	Distance float64
}

type PlanRequest struct {
	OriginLat float64
	OriginLon float64
	DestLat   float64
	DestLon   float64
}

type PlannedLegResp struct {
	RouteID             int     `json:"route_id"`
	TripID              string  `json:"trip_id"`
	StartStopID         int     `json:"start_stop_id"`
	DestStopID          int     `json:"dest_stop_id"`
	RideSeconds         float64 `json:"ride_seconds"`
	IntermediateStopIDs []int   `json:"intermediate_stop_ids"`
}

type PlannedRouteResp struct {
	Legs               []PlannedLegResp `json:"legs"`
	IsDirect           bool             `json:"is_direct"`
	WalkStartMeters    float64          `json:"walk_start_meters"`
	WalkEndMeters      float64          `json:"walk_end_meters"`
	WalkTransferMeters float64          `json:"walk_transfer_meters"`
	TransitDurationSec float64          `json:"transit_duration_sec"`
	TotalDistance      float64          `json:"total_distance"`
}

type PlanResponse struct {
	Plans  []PlannedRouteResp       `json:"plans"`
	Stops  map[int]models.Stop      `json:"stops"`
	Shapes map[int]models.ShapeInfo `json:"shapes"`
}

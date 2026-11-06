package models

// In stop order.
type Traffic struct {
	Stretches []TrafficStretch `json:"stretches"`
	Jams      []Jam            `json:"jams"`
}

type TrafficStretch struct {
	Path string `json:"path"`
	Band int    `json:"band"`
}

type Jam struct {
	FromStopID int `json:"from_stop_id"`
	ToStopID   int `json:"to_stop_id"`
	Band       int `json:"band"`
	DelaySec   int `json:"delay_sec"`
}

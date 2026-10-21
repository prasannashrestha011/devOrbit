package traffic_inspector

type startRequest struct {
	Kind       string `json:"kind"`
	ServiceKey string `json:"service_key"`
	PortKey    string `json:"port_key"`
	File       string `json:"file"`
}

type recordsResponse struct {
	Records        []Record `json:"records"`
	LatestSequence uint64   `json:"latest_sequence"`
	Gap            bool     `json:"gap"`
}

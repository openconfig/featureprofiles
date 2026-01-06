package textfsm

// Nested structure types for transformed output

type SensorGroup struct {
	ID                string `json:"ID"`
	SampleInterval    string `json:"SAMPLE_INTERVAL"`
	HeartbeatInterval string `json:"HEARTBEAT_INTERVAL"`
	SensorPath        string `json:"SENSOR_PATH"`
	SensorPathState   string `json:"SENSOR_PATH_STATE"`
}

type DestinationGroup struct {
	GroupID          string `json:"GROUP_ID"`
	IP               string `json:"IP"`
	Port             string `json:"PORT"`
	DscpQos          string `json:"DSCP_QOS"`
	Compression      string `json:"COMPRESSION"`
	Encoding         string `json:"ENCODING"`
	Transport        string `json:"TRANSPORT"`
	State            string `json:"STATE"`
	TlsMutual        string `json:"TLS_MUTUAL"`
	TotalBytesSent   string `json:"TOTAL_BYTES_SENT"`
	TotalPacketsSent string `json:"TOTAL_PACKETS_SENT"`
	LastSentTime     string `json:"LAST_SENT_TIME"`
	Endpoint         string `json:"ENDPOINT"`
	InitialUpdates   string `json:"INITIAL_UPDATES"`
}

type CollectionGroup struct {
	ID             string `json:"ID"`
	SampleInterval string `json:"SAMPLE_INTERVAL"`
	Heartbeat      string `json:"HEARTBEAT"`
	NumCollection  string `json:"NUM_COLLECTION"`
	Path           string `json:"PATH"`
}

type SubscriptionNested struct {
	SubscriptionName string            `json:"SUBSCRIPTION_NAME"`
	SubscriptionID   string            `json:"SUBSCRIPTION_ID"`
	State            string            `json:"STATE"`
	SensorGroups     []SensorGroup     `json:"SENSOR_GROUPS"`
	DestinationGroup DestinationGroup  `json:"DESTINATION_GROUP"`
	CollectionGroups []CollectionGroup `json:"COLLECTION_GROUPS"`
}

// ToNested transforms parallel arrays into nested structures
func (row *ShowTelemetryModelDrivenSubscriptionRow) ToNested() SubscriptionNested {
	nested := SubscriptionNested{
		SubscriptionName: row.SubscriptionName,
		SubscriptionID:   row.SubscriptionId,
		State:            row.State,
		SensorGroups:     []SensorGroup{},
		CollectionGroups: []CollectionGroup{},
	}

	// Transform sensor groups from parallel arrays to nested objects
	for i := 0; i < len(row.SensorGroupId); i++ {
		sensor := SensorGroup{
			ID: row.SensorGroupId[i],
		}
		if i < len(row.SampleInterval) {
			sensor.SampleInterval = row.SampleInterval[i]
		}
		if i < len(row.HeartbeatInterval) {
			sensor.HeartbeatInterval = row.HeartbeatInterval[i]
		}
		if i < len(row.SensorPath) {
			sensor.SensorPath = row.SensorPath[i]
		}
		if i < len(row.SensorPathState) {
			sensor.SensorPathState = row.SensorPathState[i]
		}
		nested.SensorGroups = append(nested.SensorGroups, sensor)
	}

	// Transform destination group (single object)
	nested.DestinationGroup = DestinationGroup{
		GroupID:          row.DestGroupId,
		IP:               row.DestIp,
		Port:             row.DestPort,
		DscpQos:          row.DscpQos,
		Compression:      row.Compression,
		Encoding:         row.Encoding,
		Transport:        row.Transport,
		State:            row.DestState,
		TlsMutual:        row.TlsMutual,
		TotalBytesSent:   row.TotalBytesSent,
		TotalPacketsSent: row.TotalPacketsSent,
		LastSentTime:     row.LastSentTime,
		Endpoint:         row.DestEndpoint,
		InitialUpdates:   row.InitialUpdates,
	}

	// Transform collection groups from parallel arrays to nested objects
	for i := 0; i < len(row.CollectionId); i++ {
		collection := CollectionGroup{
			ID: row.CollectionId[i],
		}
		if i < len(row.CollectionSampleInterval) {
			collection.SampleInterval = row.CollectionSampleInterval[i]
		}
		if i < len(row.CollectionHeartbeat) {
			collection.Heartbeat = row.CollectionHeartbeat[i]
		}
		if i < len(row.NumCollection) {
			collection.NumCollection = row.NumCollection[i]
		}
		if i < len(row.CollectionPath) {
			collection.Path = row.CollectionPath[i]
		}
		nested.CollectionGroups = append(nested.CollectionGroups, collection)
	}

	return nested
}

// ToNestedSlice transforms all rows to nested structures
func (p *ShowTelemetryModelDrivenSubscription) ToNestedSlice() []SubscriptionNested {
	result := make([]SubscriptionNested, 0, len(p.Rows))
	for _, row := range p.Rows {
		result = append(result, row.ToNested())
	}
	return result
}

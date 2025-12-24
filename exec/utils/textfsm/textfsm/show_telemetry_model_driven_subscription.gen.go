// Code generated from textfsm file
package textfsm

import (
	"reflect"

	"github.com/sirikothe/gotextfsm"
)

var templateShowTelemetryModelDrivenSubscription string = `Value Required SUBSCRIPTION_NAME (\S+)
Value SUBSCRIPTION_ID (\d+)
Value STATE (\S+)
Value List SENSOR_GROUP_ID (\S+)
Value List SAMPLE_INTERVAL (\d+)
Value List HEARTBEAT_INTERVAL (\S+)
Value List SENSOR_PATH (.+)
Value List SENSOR_PATH_STATE (\S+)
Value DEST_GROUP_ID (\S+)
Value DEST_IP (\S+)
Value DEST_PORT (\d+)
Value DSCP_QOS (\S+)
Value COMPRESSION (\S+)
Value ENCODING ([\w-]+)
Value TRANSPORT (\S+)
Value DEST_STATE (\S+)
Value TLS_MUTUAL (\S+)
Value TOTAL_BYTES_SENT (\d+)
Value TOTAL_PACKETS_SENT (\d+)
Value LAST_SENT_TIME (.+)
Value DEST_ENDPOINT (\d+)
Value INITIAL_UPDATES (\S+)
Value List COLLECTION_ID (\d+)
Value List COLLECTION_SAMPLE_INTERVAL (\d+)
Value List COLLECTION_HEARTBEAT (\S+)
Value List NUM_COLLECTION (\d+)
Value List COLLECTION_PATH (.+)

Start
  ^Subscription:\s+${SUBSCRIPTION_NAME}\s*$ -> SubscriptionBody
  ^\s*$
  ^[A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+\d+\s+\d+:\d+:\d+
  ^. -> Ignore

SubscriptionBody
  ^Subscription:\s+\S+ -> Continue.Record
  ^Subscription:\s+${SUBSCRIPTION_NAME}\s*$ -> SubscriptionBody
  ^Subscription ID:\s+${SUBSCRIPTION_ID}\s*$
  ^-+\s*$
  ^\s+State:\s+${STATE}\s*$
  ^\s+Sensor groups:\s*$ -> SensorGroups
  ^\s+Destination Groups:\s*$ -> DestinationGroups
  ^\s+Collection Groups:\s*$ -> CollectionGroups
  ^\s*$
  ^. -> Ignore
  ^\s+Sensor groups:\s*$ -> SensorGroups
  ^\s+Destination Groups:\s*$ -> DestinationGroups
  ^\s+Collection Groups:\s*$ -> CollectionGroups
  ^\s*$
  ^. -> Ignore

SensorGroups
  ^\s+Id:\s+${SENSOR_GROUP_ID}\s*$
  ^\s+Sample Interval:\s+${SAMPLE_INTERVAL}\s+ms\s*$
  ^\s+Heartbeat Interval:\s+${HEARTBEAT_INTERVAL}\s*$
  ^\s+Sensor Path:\s+${SENSOR_PATH}\s*$
  ^\s+Sensor Path State:\s+${SENSOR_PATH_STATE}\s*$
  ^\s+Destination Groups:\s*$ -> DestinationGroups
  ^\s*$
  ^. -> Ignore

DestinationGroups
  ^\s+Group Id:\s+${DEST_GROUP_ID}\s*$
  ^\s+Destination IP:\s+${DEST_IP}\s*$
  ^\s+Destination Port:\s+${DEST_PORT}\s*$
  ^\s+DSCP/Qos setting:\s+${DSCP_QOS}\s*$
  ^\s+Compression:\s+${COMPRESSION}\s*$
  ^\s+Encoding:\s+${ENCODING}\s*$
  ^\s+Transport:\s+${TRANSPORT}\s*$
  ^\s+State:\s+${DEST_STATE}\s*$
  ^\s+TLS-mutual:\s+${TLS_MUTUAL}\s*$
  ^\s+Total bytes sent:\s+${TOTAL_BYTES_SENT}\s*$
  ^\s+Total packets sent:\s+${TOTAL_PACKETS_SENT}\s*$
  ^\s+Last Sent time:\s+${LAST_SENT_TIME}\s*$
  ^\s+Destination endpoint:\s+${DEST_ENDPOINT}\s*$
  ^\s+Initial updates:\s+${INITIAL_UPDATES}\s*$
  ^\s+Collection Groups:\s*$ -> CollectionGroups
  ^\s*$
  ^. -> Ignore

CollectionGroups
  ^Subscription:\s+\S+ -> Continue.Record
  ^Subscription:\s+${SUBSCRIPTION_NAME}\s*$ -> SubscriptionBody
  ^\s+--+\s*$
  ^\s+Id:\s+${COLLECTION_ID}\s*$
  ^\s+Sample Interval:\s+${COLLECTION_SAMPLE_INTERVAL}\s+ms\s*$
  ^\s+Heartbeat Interval:\s+${COLLECTION_HEARTBEAT}\s*$
  ^\s+Heartbeat always:
  ^\s+Encoding:
  ^\s+Num of collection:\s+${NUM_COLLECTION}\s*$
  ^\s+Incremental updates:
  ^\s+Collection time:
  ^\s+Total time:
  ^\s+Total Deferred:
  ^\s+Total Send Errors:
  ^\s+Total Send Drops:
  ^\s+Total Other Errors:
  ^\s+No data Instances:
  ^\s+Initial updates:
  ^\s+Last Collection Start:
  ^\s+Last Collection End:
  ^\s+Collections:
  ^\s+Collection splits:
  ^\s+Last collection max:
  ^\s+Sensor Path:\s+${COLLECTION_PATH}\s*$
  ^\s+Bundle:
  ^\s*$
  ^RP/\d+
  ^[A-Z][a-z]{2}\s+[A-Z][a-z]{2}
  ^. -> Ignore

Ignore
  ^.* -> Ignore


`

type ShowTelemetryModelDrivenSubscriptionRow struct {
	CollectionHeartbeat      []string
	CollectionId             []string
	CollectionPath           []string
	CollectionSampleInterval []string
	Compression              string
	DestEndpoint             string
	DestGroupId              string
	DestIp                   string
	DestPort                 string
	DestState                string
	DscpQos                  string
	Encoding                 string
	HeartbeatInterval        []string
	InitialUpdates           string
	LastSentTime             string
	NumCollection            []string
	SampleInterval           []string
	SensorGroupId            []string
	SensorPath               []string
	SensorPathState          []string
	State                    string
	SubscriptionId           string
	SubscriptionName         string
	TlsMutual                string
	TotalBytesSent           string
	TotalPacketsSent         string
	Transport                string
}

type ShowTelemetryModelDrivenSubscription struct {
	Rows []ShowTelemetryModelDrivenSubscriptionRow
}

func (p *ShowTelemetryModelDrivenSubscription) IsGoTextFSMStruct() {}

func (p *ShowTelemetryModelDrivenSubscription) Parse(cliOutput string) error {
	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(templateShowTelemetryModelDrivenSubscription); err != nil {
		return err
	}

	parser := gotextfsm.ParserOutput{}
	if err := parser.ParseTextString(string(cliOutput), fsm, true); err != nil {
		return err
	}

	for _, row := range parser.Dict {
		p.Rows = append(p.Rows,
			ShowTelemetryModelDrivenSubscriptionRow{
				CollectionHeartbeat:      row["COLLECTION_HEARTBEAT"].([]string),
				CollectionId:             row["COLLECTION_ID"].([]string),
				CollectionPath:           row["COLLECTION_PATH"].([]string),
				CollectionSampleInterval: row["COLLECTION_SAMPLE_INTERVAL"].([]string),
				Compression:              row["COMPRESSION"].(string),
				DestEndpoint:             row["DEST_ENDPOINT"].(string),
				DestGroupId:              row["DEST_GROUP_ID"].(string),
				DestIp:                   row["DEST_IP"].(string),
				DestPort:                 row["DEST_PORT"].(string),
				DestState:                row["DEST_STATE"].(string),
				DscpQos:                  row["DSCP_QOS"].(string),
				Encoding:                 row["ENCODING"].(string),
				HeartbeatInterval:        row["HEARTBEAT_INTERVAL"].([]string),
				InitialUpdates:           row["INITIAL_UPDATES"].(string),
				LastSentTime:             row["LAST_SENT_TIME"].(string),
				NumCollection:            row["NUM_COLLECTION"].([]string),
				SampleInterval:           row["SAMPLE_INTERVAL"].([]string),
				SensorGroupId:            row["SENSOR_GROUP_ID"].([]string),
				SensorPath:               row["SENSOR_PATH"].([]string),
				SensorPathState:          row["SENSOR_PATH_STATE"].([]string),
				State:                    row["STATE"].(string),
				SubscriptionId:           row["SUBSCRIPTION_ID"].(string),
				SubscriptionName:         row["SUBSCRIPTION_NAME"].(string),
				TlsMutual:                row["TLS_MUTUAL"].(string),
				TotalBytesSent:           row["TOTAL_BYTES_SENT"].(string),
				TotalPacketsSent:         row["TOTAL_PACKETS_SENT"].(string),
				Transport:                row["TRANSPORT"].(string),
			},
		)
	}
	return nil
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) Compare(expected ShowTelemetryModelDrivenSubscriptionRow) bool {
	return reflect.DeepEqual(*m, expected)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetCollectionHeartbeat() []string {
	return m.CollectionHeartbeat
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetCollectionId() []string {
	return m.CollectionId
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetCollectionPath() []string {
	return m.CollectionPath
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetCollectionSampleInterval() []string {
	return m.CollectionSampleInterval
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetCompression() string {
	return m.Compression
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetDestEndpoint() string {
	return m.DestEndpoint
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetDestGroupId() string {
	return m.DestGroupId
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetDestIp() string {
	return m.DestIp
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetDestPort() string {
	return m.DestPort
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetDestState() string {
	return m.DestState
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetDscpQos() string {
	return m.DscpQos
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetEncoding() string {
	return m.Encoding
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetHeartbeatInterval() []string {
	return m.HeartbeatInterval
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetInitialUpdates() string {
	return m.InitialUpdates
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetLastSentTime() string {
	return m.LastSentTime
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetNumCollection() []string {
	return m.NumCollection
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetSampleInterval() []string {
	return m.SampleInterval
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetSensorGroupId() []string {
	return m.SensorGroupId
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetSensorPath() []string {
	return m.SensorPath
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetSensorPathState() []string {
	return m.SensorPathState
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetState() string {
	return m.State
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetSubscriptionId() string {
	return m.SubscriptionId
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetSubscriptionName() string {
	return m.SubscriptionName
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetTlsMutual() string {
	return m.TlsMutual
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetTotalBytesSent() string {
	return m.TotalBytesSent
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetTotalPacketsSent() string {
	return m.TotalPacketsSent
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) GetTransport() string {
	return m.Transport
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllCollectionHeartbeat() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.CollectionHeartbeat)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllCollectionId() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.CollectionId)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllCollectionPath() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.CollectionPath)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllCollectionSampleInterval() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.CollectionSampleInterval)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllCompression() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Compression)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllDestEndpoint() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DestEndpoint)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllDestGroupId() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DestGroupId)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllDestIp() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DestIp)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllDestPort() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DestPort)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllDestState() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DestState)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllDscpQos() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DscpQos)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllEncoding() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Encoding)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllHeartbeatInterval() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.HeartbeatInterval)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllInitialUpdates() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.InitialUpdates)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllLastSentTime() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.LastSentTime)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllNumCollection() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.NumCollection)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllSampleInterval() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SampleInterval)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllSensorGroupId() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SensorGroupId)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllSensorPath() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SensorPath)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllSensorPathState() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SensorPathState)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllState() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.State)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllSubscriptionId() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SubscriptionId)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllSubscriptionName() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SubscriptionName)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllTlsMutual() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TlsMutual)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllTotalBytesSent() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TotalBytesSent)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllTotalPacketsSent() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TotalPacketsSent)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscription) GetAllTransport() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Transport)
	}
	return arr
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyCollectionHeartbeat(value []string) bool {
	return reflect.DeepEqual(m.CollectionHeartbeat, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyCollectionId(value []string) bool {
	return reflect.DeepEqual(m.CollectionId, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyCollectionPath(value []string) bool {
	return reflect.DeepEqual(m.CollectionPath, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyCollectionSampleInterval(value []string) bool {
	return reflect.DeepEqual(m.CollectionSampleInterval, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyCompression(value string) bool {
	return reflect.DeepEqual(m.Compression, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyDestEndpoint(value string) bool {
	return reflect.DeepEqual(m.DestEndpoint, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyDestGroupId(value string) bool {
	return reflect.DeepEqual(m.DestGroupId, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyDestIp(value string) bool {
	return reflect.DeepEqual(m.DestIp, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyDestPort(value string) bool {
	return reflect.DeepEqual(m.DestPort, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyDestState(value string) bool {
	return reflect.DeepEqual(m.DestState, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyDscpQos(value string) bool {
	return reflect.DeepEqual(m.DscpQos, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyEncoding(value string) bool {
	return reflect.DeepEqual(m.Encoding, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyHeartbeatInterval(value []string) bool {
	return reflect.DeepEqual(m.HeartbeatInterval, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyInitialUpdates(value string) bool {
	return reflect.DeepEqual(m.InitialUpdates, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyLastSentTime(value string) bool {
	return reflect.DeepEqual(m.LastSentTime, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyNumCollection(value []string) bool {
	return reflect.DeepEqual(m.NumCollection, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifySampleInterval(value []string) bool {
	return reflect.DeepEqual(m.SampleInterval, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifySensorGroupId(value []string) bool {
	return reflect.DeepEqual(m.SensorGroupId, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifySensorPath(value []string) bool {
	return reflect.DeepEqual(m.SensorPath, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifySensorPathState(value []string) bool {
	return reflect.DeepEqual(m.SensorPathState, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyState(value string) bool {
	return reflect.DeepEqual(m.State, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifySubscriptionId(value string) bool {
	return reflect.DeepEqual(m.SubscriptionId, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifySubscriptionName(value string) bool {
	return reflect.DeepEqual(m.SubscriptionName, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyTlsMutual(value string) bool {
	return reflect.DeepEqual(m.TlsMutual, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyTotalBytesSent(value string) bool {
	return reflect.DeepEqual(m.TotalBytesSent, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyTotalPacketsSent(value string) bool {
	return reflect.DeepEqual(m.TotalPacketsSent, value)
}

func (m *ShowTelemetryModelDrivenSubscriptionRow) VerifyTransport(value string) bool {
	return reflect.DeepEqual(m.Transport, value)
}

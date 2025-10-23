// Code generated from textfsm file
package textfsm

import (
	"reflect"

	"github.com/sirikothe/gotextfsm"
)

var templatePingEthernet string = `Value Key dest_addr (\S+)
Value Filldown count (\d+) 
Value timeout (\d+) 
Value tx_count (\d+)
Value rx_count (\d+)
Value success_rate ([\d\.]+)
Value round_trip_min (\d+)
Value round_trip_avg (\d+)
Value round_trip_max (\d+)
Value domain_name (\S+)
Value service_name (\S+)
Value source_mep_id (\d+)
Value source_interface (\S+)
Value target_mac_address (\S+)
Value target_mep_id (\d+)
Value level_number (\d+)

Start
  ^Sending ${count} CFM ${dest_addr}, timeout is ${timeout} seconds -
  ^Domain ${domain_name} \(level ${level_number}\), Service ${service_name}
  ^Source: MEP ID ${source_mep_id}, interface ${source_interface}
  ^Target: ${target_mac_address} \(MEP ID ${target_mep_id}\)
  ^\s*Success\s+rate\s+is\s+${success_rate}\s+percent\s+\(${rx_count}/${tx_count}\),\s+round-trip\s+min/avg/max\s+=\s+${round_trip_min}/${round_trip_avg}/${round_trip_max}\s+ms -> Record
  ^\s*Success\s+rate\s+is\s+${success_rate}\s+percent\s+\(${rx_count}/${tx_count}\) -> Record`

type PingEthernetRow struct {
	Count            string
	DestAddr         string
	DomainName       string
	LevelNumber      string
	RoundTripAvg     string
	RoundTripMax     string
	RoundTripMin     string
	RxCount          string
	ServiceName      string
	SourceInterface  string
	SourceMepId      string
	SuccessRate      string
	TargetMacAddress string
	TargetMepId      string
	Timeout          string
	TxCount          string
}

type PingEthernet struct {
	Rows []PingEthernetRow
}

func (p *PingEthernet) IsGoTextFSMStruct() {}

func (p *PingEthernet) Parse(cliOutput string) error {
	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(templatePingEthernet); err != nil {
		return err
	}

	parser := gotextfsm.ParserOutput{}
	if err := parser.ParseTextString(string(cliOutput), fsm, true); err != nil {
		return err
	}

	for _, row := range parser.Dict {
		p.Rows = append(p.Rows,
			PingEthernetRow{
				Count:            row["count"].(string),
				DestAddr:         row["dest_addr"].(string),
				DomainName:       row["domain_name"].(string),
				LevelNumber:      row["level_number"].(string),
				RoundTripAvg:     row["round_trip_avg"].(string),
				RoundTripMax:     row["round_trip_max"].(string),
				RoundTripMin:     row["round_trip_min"].(string),
				RxCount:          row["rx_count"].(string),
				ServiceName:      row["service_name"].(string),
				SourceInterface:  row["source_interface"].(string),
				SourceMepId:      row["source_mep_id"].(string),
				SuccessRate:      row["success_rate"].(string),
				TargetMacAddress: row["target_mac_address"].(string),
				TargetMepId:      row["target_mep_id"].(string),
				Timeout:          row["timeout"].(string),
				TxCount:          row["tx_count"].(string),
			},
		)
	}
	return nil
}

func (m *PingEthernetRow) Compare(expected PingEthernetRow) bool {
	return reflect.DeepEqual(*m, expected)
}

func (m *PingEthernetRow) GetCount() string {
	return m.Count
}

func (m *PingEthernetRow) GetDestAddr() string {
	return m.DestAddr
}

func (m *PingEthernetRow) GetDomainName() string {
	return m.DomainName
}

func (m *PingEthernetRow) GetLevelNumber() string {
	return m.LevelNumber
}

func (m *PingEthernetRow) GetRoundTripAvg() string {
	return m.RoundTripAvg
}

func (m *PingEthernetRow) GetRoundTripMax() string {
	return m.RoundTripMax
}

func (m *PingEthernetRow) GetRoundTripMin() string {
	return m.RoundTripMin
}

func (m *PingEthernetRow) GetRxCount() string {
	return m.RxCount
}

func (m *PingEthernetRow) GetServiceName() string {
	return m.ServiceName
}

func (m *PingEthernetRow) GetSourceInterface() string {
	return m.SourceInterface
}

func (m *PingEthernetRow) GetSourceMepId() string {
	return m.SourceMepId
}

func (m *PingEthernetRow) GetSuccessRate() string {
	return m.SuccessRate
}

func (m *PingEthernetRow) GetTargetMacAddress() string {
	return m.TargetMacAddress
}

func (m *PingEthernetRow) GetTargetMepId() string {
	return m.TargetMepId
}

func (m *PingEthernetRow) GetTimeout() string {
	return m.Timeout
}

func (m *PingEthernetRow) GetTxCount() string {
	return m.TxCount
}

func (m *PingEthernet) GetAllCount() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Count)
	}
	return arr
}

func (m *PingEthernet) GetAllDestAddr() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DestAddr)
	}
	return arr
}

func (m *PingEthernet) GetAllDomainName() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DomainName)
	}
	return arr
}

func (m *PingEthernet) GetAllLevelNumber() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.LevelNumber)
	}
	return arr
}

func (m *PingEthernet) GetAllRoundTripAvg() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.RoundTripAvg)
	}
	return arr
}

func (m *PingEthernet) GetAllRoundTripMax() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.RoundTripMax)
	}
	return arr
}

func (m *PingEthernet) GetAllRoundTripMin() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.RoundTripMin)
	}
	return arr
}

func (m *PingEthernet) GetAllRxCount() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.RxCount)
	}
	return arr
}

func (m *PingEthernet) GetAllServiceName() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ServiceName)
	}
	return arr
}

func (m *PingEthernet) GetAllSourceInterface() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SourceInterface)
	}
	return arr
}

func (m *PingEthernet) GetAllSourceMepId() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SourceMepId)
	}
	return arr
}

func (m *PingEthernet) GetAllSuccessRate() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.SuccessRate)
	}
	return arr
}

func (m *PingEthernet) GetAllTargetMacAddress() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TargetMacAddress)
	}
	return arr
}

func (m *PingEthernet) GetAllTargetMepId() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TargetMepId)
	}
	return arr
}

func (m *PingEthernet) GetAllTimeout() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Timeout)
	}
	return arr
}

func (m *PingEthernet) GetAllTxCount() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TxCount)
	}
	return arr
}

func (m *PingEthernetRow) VerifyCount(value string) bool {
	return reflect.DeepEqual(m.Count, value)
}

func (m *PingEthernetRow) VerifyDestAddr(value string) bool {
	return reflect.DeepEqual(m.DestAddr, value)
}

func (m *PingEthernetRow) VerifyDomainName(value string) bool {
	return reflect.DeepEqual(m.DomainName, value)
}

func (m *PingEthernetRow) VerifyLevelNumber(value string) bool {
	return reflect.DeepEqual(m.LevelNumber, value)
}

func (m *PingEthernetRow) VerifyRoundTripAvg(value string) bool {
	return reflect.DeepEqual(m.RoundTripAvg, value)
}

func (m *PingEthernetRow) VerifyRoundTripMax(value string) bool {
	return reflect.DeepEqual(m.RoundTripMax, value)
}

func (m *PingEthernetRow) VerifyRoundTripMin(value string) bool {
	return reflect.DeepEqual(m.RoundTripMin, value)
}

func (m *PingEthernetRow) VerifyRxCount(value string) bool {
	return reflect.DeepEqual(m.RxCount, value)
}

func (m *PingEthernetRow) VerifyServiceName(value string) bool {
	return reflect.DeepEqual(m.ServiceName, value)
}

func (m *PingEthernetRow) VerifySourceInterface(value string) bool {
	return reflect.DeepEqual(m.SourceInterface, value)
}

func (m *PingEthernetRow) VerifySourceMepId(value string) bool {
	return reflect.DeepEqual(m.SourceMepId, value)
}

func (m *PingEthernetRow) VerifySuccessRate(value string) bool {
	return reflect.DeepEqual(m.SuccessRate, value)
}

func (m *PingEthernetRow) VerifyTargetMacAddress(value string) bool {
	return reflect.DeepEqual(m.TargetMacAddress, value)
}

func (m *PingEthernetRow) VerifyTargetMepId(value string) bool {
	return reflect.DeepEqual(m.TargetMepId, value)
}

func (m *PingEthernetRow) VerifyTimeout(value string) bool {
	return reflect.DeepEqual(m.Timeout, value)
}

func (m *PingEthernetRow) VerifyTxCount(value string) bool {
	return reflect.DeepEqual(m.TxCount, value)
}

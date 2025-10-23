// Code generated from textfsm file
package textfsm

import (
	"reflect"

	"github.com/sirikothe/gotextfsm"
)

var templateShowInterface string = `#**************************************************
# Copyright (c) 2017 Cisco Systems, Inc.
# All rights reserved.
#**************************************************
Value intf ([^\s]+)
Value intf_state ([^,]+)
Value intf_line_protocol ([^\n]+)
Value intf_state_transitions ([^\n]+)
Value hw ([^,]+)
Value hw_addr ([^\n]+)
Value internet_addr ([^\n]+)
Value mtu ([^,]+)
Value bw ([\s\d\w]+)
Value max_bw ([\s\d\w]+)
Value mtu_reliability ([^\,]+)
Value mtu_txload ([^\,]+)
Value mtu_rxload ([^\n]+)
Value encapsulation ([^\,]+)
Value loopback ([^\,]+)
Value arp_type ([^\,]+)
Value arp_timeout ([^\n]+)
Value l2_overhead ([^\n]+)
Value generic_intf_list ([^\n]+)
Value five_min_bit_input_rate ([^\,]+)
Value five_min_packet_input_rate ([^\n]+)
Value five_min_bit_output_rate ([^\,]+)
Value five_min_packet_output_rate ([^\n]+)
Value thirty_sec_input_bps (\d+)
Value thirty_sec_input_pps (\d+)
Value thirty_sec_output_bps (\d+)
Value thirty_sec_output_pps (\d+)
Value num_bytes_input (\d+)
Value total_input_drops (\d+)
Value unrecognized_ul_protocol_drops (\d+)
Value broadcast_packets_received (\d+)
Value multicast_packets_received (\d+)
Value runts ([0-9]+)
Value giants ([0-9]+)
Value throttles ([0-9]+)
Value parity ([0-9]+)
Value input_errors ([0-9]+)
Value crc ([0-9]+)
Value frame ([0-9]+)
Value overrun ([0-9]+)
Value ignored ([0-9]+)
Value abort ([0-9]+)
Value num_packets_output (\d+)
Value num_bytes_output (\d+)
Value total_output_drops (\d+)
Value broadcast_packets_output (\d+)
Value multicast_packets_output (\d+)
Value output_errors ([0-9]+)
Value underruns ([0-9]+)
Value applique ([0-9]+)
Value resets ([0-9]+)
Value output_buffer_failures ([0-9]+)
Value output_buffers_swapped_out ([0-9]+)
Value carrier_transitions ([0-9]+)
Value speed ([0-9]+)
Value duplex (\w+)

Start
  ^${intf} is\s${intf_state}, line protocol is\s${intf_line_protocol}
  ^\s*Interface state transitions:\s${intf_state_transitions}
  ^\s*Hardware is\s${hw}, address is\s${hw_addr}
  ^\s*Internet address is\s${internet_addr}
  ^\s*MTU\s${mtu}, BW\s${bw}\s\(Max:\s${max_bw}\)
  ^\s*reliability\s${mtu_reliability}, txload\s${mtu_txload}, rxload\s${mtu_rxload}
  ^\s*Encapsulation\s${encapsulation},  loopback\s${loopback},
  ^\s*${duplex}-duplex,\s${speed}Mb/s,
  ^\s*ARP type\s${arp_type}, ARP timeout\s${arp_timeout}
  ^\s*L2Overhead:\s${l2_overhead}
  ^\s*Generic-Interface-List:\s${generic_intf_list}
  ^\s*5 minute input rate\s${five_min_bit_input_rate},\s${five_min_packet_input_rate}
  ^\s*5 minute output rate\s${five_min_bit_output_rate},\s${five_min_packet_output_rate}
  ^\s*30\s+second\s+input\s+rate\s+${thirty_sec_input_bps}\s+bits/sec,\s+${thirty_sec_input_pps}\s+packets/sec
  ^\s*30\s+second\s+output\s+rate\s+${thirty_sec_output_bps}\s+bits/sec,\s+${thirty_sec_output_pps}\s+packets/sec
  ^\s*${num_packets_input}\spackets input,\s${num_bytes_input}\sbytes,\s${total_input_drops}\stotal input drops
  ^\s*${unrecognized_ul_protocol_drops}\sdrops for unrecognized upper-level protocol
  ^\s*Received\s${broadcast_packets_received}\sbroadcast packets,\s${multicast_packets_received}\smulticast packets
  ^\s*${runts}\s*runts, ${giants} giants, ${throttles} throttles, ${parity} parity
  ^\s*${input_errors}\s+input\s+errors,\s+${crc}\s+CRC,\s+${frame}\s+frame,\s+${overrun}\s+overrun,\s+${ignored}\s+ignored,\s+${abort}\s+abort
  ^\s*${num_packets_output}\spackets output,\s${num_bytes_output}\sbytes,\s${total_output_drops}\stotal output drops
  ^\s*Output\s${broadcast_packets_output}\sbroadcast packets,\s${multicast_packets_output}\smulticast packets
  ^\s*${output_errors} output errors, ${underruns} underruns, ${applique} applique, ${resets} resets
  ^\s*${output_buffer_failures} output buffer failures, ${output_buffers_swapped_out} output buffers swapped out
  ^\s*${carrier_transitions} carrier transitions -> Record`

type ShowInterfaceRow struct {
	Abort                       string
	Applique                    string
	ArpTimeout                  string
	ArpType                     string
	BroadcastPacketsOutput      string
	BroadcastPacketsReceived    string
	Bw                          string
	CarrierTransitions          string
	Crc                         string
	Duplex                      string
	Encapsulation               string
	FiveMinBitInputRate         string
	FiveMinBitOutputRate        string
	FiveMinPacketInputRate      string
	FiveMinPacketOutputRate     string
	Frame                       string
	GenericIntfList             string
	Giants                      string
	Hw                          string
	HwAddr                      string
	Ignored                     string
	InputErrors                 string
	InternetAddr                string
	Intf                        string
	IntfLineProtocol            string
	IntfState                   string
	IntfStateTransitions        string
	L2Overhead                  string
	Loopback                    string
	MaxBw                       string
	Mtu                         string
	MtuReliability              string
	MtuRxload                   string
	MtuTxload                   string
	MulticastPacketsOutput      string
	MulticastPacketsReceived    string
	NumBytesInput               string
	NumBytesOutput              string
	NumPacketsOutput            string
	OutputBufferFailures        string
	OutputBuffersSwappedOut     string
	OutputErrors                string
	Overrun                     string
	Parity                      string
	Resets                      string
	Runts                       string
	Speed                       string
	ThirtySecInputBps           string
	ThirtySecInputPps           string
	ThirtySecOutputBps          string
	ThirtySecOutputPps          string
	Throttles                   string
	TotalInputDrops             string
	TotalOutputDrops            string
	Underruns                   string
	UnrecognizedUlProtocolDrops string
}

type ShowInterface struct {
	Rows []ShowInterfaceRow
}

func (p *ShowInterface) IsGoTextFSMStruct() {}

func (p *ShowInterface) Parse(cliOutput string) error {
	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(templateShowInterface); err != nil {
		return err
	}

	parser := gotextfsm.ParserOutput{}
	if err := parser.ParseTextString(string(cliOutput), fsm, true); err != nil {
		return err
	}

	for _, row := range parser.Dict {
		p.Rows = append(p.Rows,
			ShowInterfaceRow{
				Abort:                       row["abort"].(string),
				Applique:                    row["applique"].(string),
				ArpTimeout:                  row["arp_timeout"].(string),
				ArpType:                     row["arp_type"].(string),
				BroadcastPacketsOutput:      row["broadcast_packets_output"].(string),
				BroadcastPacketsReceived:    row["broadcast_packets_received"].(string),
				Bw:                          row["bw"].(string),
				CarrierTransitions:          row["carrier_transitions"].(string),
				Crc:                         row["crc"].(string),
				Duplex:                      row["duplex"].(string),
				Encapsulation:               row["encapsulation"].(string),
				FiveMinBitInputRate:         row["five_min_bit_input_rate"].(string),
				FiveMinBitOutputRate:        row["five_min_bit_output_rate"].(string),
				FiveMinPacketInputRate:      row["five_min_packet_input_rate"].(string),
				FiveMinPacketOutputRate:     row["five_min_packet_output_rate"].(string),
				Frame:                       row["frame"].(string),
				GenericIntfList:             row["generic_intf_list"].(string),
				Giants:                      row["giants"].(string),
				Hw:                          row["hw"].(string),
				HwAddr:                      row["hw_addr"].(string),
				Ignored:                     row["ignored"].(string),
				InputErrors:                 row["input_errors"].(string),
				InternetAddr:                row["internet_addr"].(string),
				Intf:                        row["intf"].(string),
				IntfLineProtocol:            row["intf_line_protocol"].(string),
				IntfState:                   row["intf_state"].(string),
				IntfStateTransitions:        row["intf_state_transitions"].(string),
				L2Overhead:                  row["l2_overhead"].(string),
				Loopback:                    row["loopback"].(string),
				MaxBw:                       row["max_bw"].(string),
				Mtu:                         row["mtu"].(string),
				MtuReliability:              row["mtu_reliability"].(string),
				MtuRxload:                   row["mtu_rxload"].(string),
				MtuTxload:                   row["mtu_txload"].(string),
				MulticastPacketsOutput:      row["multicast_packets_output"].(string),
				MulticastPacketsReceived:    row["multicast_packets_received"].(string),
				NumBytesInput:               row["num_bytes_input"].(string),
				NumBytesOutput:              row["num_bytes_output"].(string),
				NumPacketsOutput:            row["num_packets_output"].(string),
				OutputBufferFailures:        row["output_buffer_failures"].(string),
				OutputBuffersSwappedOut:     row["output_buffers_swapped_out"].(string),
				OutputErrors:                row["output_errors"].(string),
				Overrun:                     row["overrun"].(string),
				Parity:                      row["parity"].(string),
				Resets:                      row["resets"].(string),
				Runts:                       row["runts"].(string),
				Speed:                       row["speed"].(string),
				ThirtySecInputBps:           row["thirty_sec_input_bps"].(string),
				ThirtySecInputPps:           row["thirty_sec_input_pps"].(string),
				ThirtySecOutputBps:          row["thirty_sec_output_bps"].(string),
				ThirtySecOutputPps:          row["thirty_sec_output_pps"].(string),
				Throttles:                   row["throttles"].(string),
				TotalInputDrops:             row["total_input_drops"].(string),
				TotalOutputDrops:            row["total_output_drops"].(string),
				Underruns:                   row["underruns"].(string),
				UnrecognizedUlProtocolDrops: row["unrecognized_ul_protocol_drops"].(string),
			},
		)
	}
	return nil
}

func (m *ShowInterfaceRow) Compare(expected ShowInterfaceRow) bool {
	return reflect.DeepEqual(*m, expected)
}

func (m *ShowInterfaceRow) GetAbort() string {
	return m.Abort
}

func (m *ShowInterfaceRow) GetApplique() string {
	return m.Applique
}

func (m *ShowInterfaceRow) GetArpTimeout() string {
	return m.ArpTimeout
}

func (m *ShowInterfaceRow) GetArpType() string {
	return m.ArpType
}

func (m *ShowInterfaceRow) GetBroadcastPacketsOutput() string {
	return m.BroadcastPacketsOutput
}

func (m *ShowInterfaceRow) GetBroadcastPacketsReceived() string {
	return m.BroadcastPacketsReceived
}

func (m *ShowInterfaceRow) GetBw() string {
	return m.Bw
}

func (m *ShowInterfaceRow) GetCarrierTransitions() string {
	return m.CarrierTransitions
}

func (m *ShowInterfaceRow) GetCrc() string {
	return m.Crc
}

func (m *ShowInterfaceRow) GetDuplex() string {
	return m.Duplex
}

func (m *ShowInterfaceRow) GetEncapsulation() string {
	return m.Encapsulation
}

func (m *ShowInterfaceRow) GetFiveMinBitInputRate() string {
	return m.FiveMinBitInputRate
}

func (m *ShowInterfaceRow) GetFiveMinBitOutputRate() string {
	return m.FiveMinBitOutputRate
}

func (m *ShowInterfaceRow) GetFiveMinPacketInputRate() string {
	return m.FiveMinPacketInputRate
}

func (m *ShowInterfaceRow) GetFiveMinPacketOutputRate() string {
	return m.FiveMinPacketOutputRate
}

func (m *ShowInterfaceRow) GetFrame() string {
	return m.Frame
}

func (m *ShowInterfaceRow) GetGenericIntfList() string {
	return m.GenericIntfList
}

func (m *ShowInterfaceRow) GetGiants() string {
	return m.Giants
}

func (m *ShowInterfaceRow) GetHw() string {
	return m.Hw
}

func (m *ShowInterfaceRow) GetHwAddr() string {
	return m.HwAddr
}

func (m *ShowInterfaceRow) GetIgnored() string {
	return m.Ignored
}

func (m *ShowInterfaceRow) GetInputErrors() string {
	return m.InputErrors
}

func (m *ShowInterfaceRow) GetInternetAddr() string {
	return m.InternetAddr
}

func (m *ShowInterfaceRow) GetIntf() string {
	return m.Intf
}

func (m *ShowInterfaceRow) GetIntfLineProtocol() string {
	return m.IntfLineProtocol
}

func (m *ShowInterfaceRow) GetIntfState() string {
	return m.IntfState
}

func (m *ShowInterfaceRow) GetIntfStateTransitions() string {
	return m.IntfStateTransitions
}

func (m *ShowInterfaceRow) GetL2Overhead() string {
	return m.L2Overhead
}

func (m *ShowInterfaceRow) GetLoopback() string {
	return m.Loopback
}

func (m *ShowInterfaceRow) GetMaxBw() string {
	return m.MaxBw
}

func (m *ShowInterfaceRow) GetMtu() string {
	return m.Mtu
}

func (m *ShowInterfaceRow) GetMtuReliability() string {
	return m.MtuReliability
}

func (m *ShowInterfaceRow) GetMtuRxload() string {
	return m.MtuRxload
}

func (m *ShowInterfaceRow) GetMtuTxload() string {
	return m.MtuTxload
}

func (m *ShowInterfaceRow) GetMulticastPacketsOutput() string {
	return m.MulticastPacketsOutput
}

func (m *ShowInterfaceRow) GetMulticastPacketsReceived() string {
	return m.MulticastPacketsReceived
}

func (m *ShowInterfaceRow) GetNumBytesInput() string {
	return m.NumBytesInput
}

func (m *ShowInterfaceRow) GetNumBytesOutput() string {
	return m.NumBytesOutput
}

func (m *ShowInterfaceRow) GetNumPacketsOutput() string {
	return m.NumPacketsOutput
}

func (m *ShowInterfaceRow) GetOutputBufferFailures() string {
	return m.OutputBufferFailures
}

func (m *ShowInterfaceRow) GetOutputBuffersSwappedOut() string {
	return m.OutputBuffersSwappedOut
}

func (m *ShowInterfaceRow) GetOutputErrors() string {
	return m.OutputErrors
}

func (m *ShowInterfaceRow) GetOverrun() string {
	return m.Overrun
}

func (m *ShowInterfaceRow) GetParity() string {
	return m.Parity
}

func (m *ShowInterfaceRow) GetResets() string {
	return m.Resets
}

func (m *ShowInterfaceRow) GetRunts() string {
	return m.Runts
}

func (m *ShowInterfaceRow) GetSpeed() string {
	return m.Speed
}

func (m *ShowInterfaceRow) GetThirtySecInputBps() string {
	return m.ThirtySecInputBps
}

func (m *ShowInterfaceRow) GetThirtySecInputPps() string {
	return m.ThirtySecInputPps
}

func (m *ShowInterfaceRow) GetThirtySecOutputBps() string {
	return m.ThirtySecOutputBps
}

func (m *ShowInterfaceRow) GetThirtySecOutputPps() string {
	return m.ThirtySecOutputPps
}

func (m *ShowInterfaceRow) GetThrottles() string {
	return m.Throttles
}

func (m *ShowInterfaceRow) GetTotalInputDrops() string {
	return m.TotalInputDrops
}

func (m *ShowInterfaceRow) GetTotalOutputDrops() string {
	return m.TotalOutputDrops
}

func (m *ShowInterfaceRow) GetUnderruns() string {
	return m.Underruns
}

func (m *ShowInterfaceRow) GetUnrecognizedUlProtocolDrops() string {
	return m.UnrecognizedUlProtocolDrops
}

func (m *ShowInterface) GetAllAbort() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Abort)
	}
	return arr
}

func (m *ShowInterface) GetAllApplique() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Applique)
	}
	return arr
}

func (m *ShowInterface) GetAllArpTimeout() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ArpTimeout)
	}
	return arr
}

func (m *ShowInterface) GetAllArpType() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ArpType)
	}
	return arr
}

func (m *ShowInterface) GetAllBroadcastPacketsOutput() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.BroadcastPacketsOutput)
	}
	return arr
}

func (m *ShowInterface) GetAllBroadcastPacketsReceived() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.BroadcastPacketsReceived)
	}
	return arr
}

func (m *ShowInterface) GetAllBw() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Bw)
	}
	return arr
}

func (m *ShowInterface) GetAllCarrierTransitions() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.CarrierTransitions)
	}
	return arr
}

func (m *ShowInterface) GetAllCrc() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Crc)
	}
	return arr
}

func (m *ShowInterface) GetAllDuplex() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Duplex)
	}
	return arr
}

func (m *ShowInterface) GetAllEncapsulation() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Encapsulation)
	}
	return arr
}

func (m *ShowInterface) GetAllFiveMinBitInputRate() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.FiveMinBitInputRate)
	}
	return arr
}

func (m *ShowInterface) GetAllFiveMinBitOutputRate() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.FiveMinBitOutputRate)
	}
	return arr
}

func (m *ShowInterface) GetAllFiveMinPacketInputRate() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.FiveMinPacketInputRate)
	}
	return arr
}

func (m *ShowInterface) GetAllFiveMinPacketOutputRate() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.FiveMinPacketOutputRate)
	}
	return arr
}

func (m *ShowInterface) GetAllFrame() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Frame)
	}
	return arr
}

func (m *ShowInterface) GetAllGenericIntfList() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.GenericIntfList)
	}
	return arr
}

func (m *ShowInterface) GetAllGiants() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Giants)
	}
	return arr
}

func (m *ShowInterface) GetAllHw() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Hw)
	}
	return arr
}

func (m *ShowInterface) GetAllHwAddr() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.HwAddr)
	}
	return arr
}

func (m *ShowInterface) GetAllIgnored() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Ignored)
	}
	return arr
}

func (m *ShowInterface) GetAllInputErrors() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.InputErrors)
	}
	return arr
}

func (m *ShowInterface) GetAllInternetAddr() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.InternetAddr)
	}
	return arr
}

func (m *ShowInterface) GetAllIntf() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Intf)
	}
	return arr
}

func (m *ShowInterface) GetAllIntfLineProtocol() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.IntfLineProtocol)
	}
	return arr
}

func (m *ShowInterface) GetAllIntfState() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.IntfState)
	}
	return arr
}

func (m *ShowInterface) GetAllIntfStateTransitions() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.IntfStateTransitions)
	}
	return arr
}

func (m *ShowInterface) GetAllL2Overhead() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.L2Overhead)
	}
	return arr
}

func (m *ShowInterface) GetAllLoopback() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Loopback)
	}
	return arr
}

func (m *ShowInterface) GetAllMaxBw() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.MaxBw)
	}
	return arr
}

func (m *ShowInterface) GetAllMtu() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Mtu)
	}
	return arr
}

func (m *ShowInterface) GetAllMtuReliability() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.MtuReliability)
	}
	return arr
}

func (m *ShowInterface) GetAllMtuRxload() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.MtuRxload)
	}
	return arr
}

func (m *ShowInterface) GetAllMtuTxload() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.MtuTxload)
	}
	return arr
}

func (m *ShowInterface) GetAllMulticastPacketsOutput() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.MulticastPacketsOutput)
	}
	return arr
}

func (m *ShowInterface) GetAllMulticastPacketsReceived() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.MulticastPacketsReceived)
	}
	return arr
}

func (m *ShowInterface) GetAllNumBytesInput() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.NumBytesInput)
	}
	return arr
}

func (m *ShowInterface) GetAllNumBytesOutput() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.NumBytesOutput)
	}
	return arr
}

func (m *ShowInterface) GetAllNumPacketsOutput() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.NumPacketsOutput)
	}
	return arr
}

func (m *ShowInterface) GetAllOutputBufferFailures() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OutputBufferFailures)
	}
	return arr
}

func (m *ShowInterface) GetAllOutputBuffersSwappedOut() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OutputBuffersSwappedOut)
	}
	return arr
}

func (m *ShowInterface) GetAllOutputErrors() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OutputErrors)
	}
	return arr
}

func (m *ShowInterface) GetAllOverrun() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Overrun)
	}
	return arr
}

func (m *ShowInterface) GetAllParity() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Parity)
	}
	return arr
}

func (m *ShowInterface) GetAllResets() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Resets)
	}
	return arr
}

func (m *ShowInterface) GetAllRunts() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Runts)
	}
	return arr
}

func (m *ShowInterface) GetAllSpeed() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Speed)
	}
	return arr
}

func (m *ShowInterface) GetAllThirtySecInputBps() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ThirtySecInputBps)
	}
	return arr
}

func (m *ShowInterface) GetAllThirtySecInputPps() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ThirtySecInputPps)
	}
	return arr
}

func (m *ShowInterface) GetAllThirtySecOutputBps() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ThirtySecOutputBps)
	}
	return arr
}

func (m *ShowInterface) GetAllThirtySecOutputPps() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.ThirtySecOutputPps)
	}
	return arr
}

func (m *ShowInterface) GetAllThrottles() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Throttles)
	}
	return arr
}

func (m *ShowInterface) GetAllTotalInputDrops() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TotalInputDrops)
	}
	return arr
}

func (m *ShowInterface) GetAllTotalOutputDrops() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.TotalOutputDrops)
	}
	return arr
}

func (m *ShowInterface) GetAllUnderruns() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Underruns)
	}
	return arr
}

func (m *ShowInterface) GetAllUnrecognizedUlProtocolDrops() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.UnrecognizedUlProtocolDrops)
	}
	return arr
}

func (m *ShowInterfaceRow) VerifyAbort(value string) bool {
	return reflect.DeepEqual(m.Abort, value)
}

func (m *ShowInterfaceRow) VerifyApplique(value string) bool {
	return reflect.DeepEqual(m.Applique, value)
}

func (m *ShowInterfaceRow) VerifyArpTimeout(value string) bool {
	return reflect.DeepEqual(m.ArpTimeout, value)
}

func (m *ShowInterfaceRow) VerifyArpType(value string) bool {
	return reflect.DeepEqual(m.ArpType, value)
}

func (m *ShowInterfaceRow) VerifyBroadcastPacketsOutput(value string) bool {
	return reflect.DeepEqual(m.BroadcastPacketsOutput, value)
}

func (m *ShowInterfaceRow) VerifyBroadcastPacketsReceived(value string) bool {
	return reflect.DeepEqual(m.BroadcastPacketsReceived, value)
}

func (m *ShowInterfaceRow) VerifyBw(value string) bool {
	return reflect.DeepEqual(m.Bw, value)
}

func (m *ShowInterfaceRow) VerifyCarrierTransitions(value string) bool {
	return reflect.DeepEqual(m.CarrierTransitions, value)
}

func (m *ShowInterfaceRow) VerifyCrc(value string) bool {
	return reflect.DeepEqual(m.Crc, value)
}

func (m *ShowInterfaceRow) VerifyDuplex(value string) bool {
	return reflect.DeepEqual(m.Duplex, value)
}

func (m *ShowInterfaceRow) VerifyEncapsulation(value string) bool {
	return reflect.DeepEqual(m.Encapsulation, value)
}

func (m *ShowInterfaceRow) VerifyFiveMinBitInputRate(value string) bool {
	return reflect.DeepEqual(m.FiveMinBitInputRate, value)
}

func (m *ShowInterfaceRow) VerifyFiveMinBitOutputRate(value string) bool {
	return reflect.DeepEqual(m.FiveMinBitOutputRate, value)
}

func (m *ShowInterfaceRow) VerifyFiveMinPacketInputRate(value string) bool {
	return reflect.DeepEqual(m.FiveMinPacketInputRate, value)
}

func (m *ShowInterfaceRow) VerifyFiveMinPacketOutputRate(value string) bool {
	return reflect.DeepEqual(m.FiveMinPacketOutputRate, value)
}

func (m *ShowInterfaceRow) VerifyFrame(value string) bool {
	return reflect.DeepEqual(m.Frame, value)
}

func (m *ShowInterfaceRow) VerifyGenericIntfList(value string) bool {
	return reflect.DeepEqual(m.GenericIntfList, value)
}

func (m *ShowInterfaceRow) VerifyGiants(value string) bool {
	return reflect.DeepEqual(m.Giants, value)
}

func (m *ShowInterfaceRow) VerifyHw(value string) bool {
	return reflect.DeepEqual(m.Hw, value)
}

func (m *ShowInterfaceRow) VerifyHwAddr(value string) bool {
	return reflect.DeepEqual(m.HwAddr, value)
}

func (m *ShowInterfaceRow) VerifyIgnored(value string) bool {
	return reflect.DeepEqual(m.Ignored, value)
}

func (m *ShowInterfaceRow) VerifyInputErrors(value string) bool {
	return reflect.DeepEqual(m.InputErrors, value)
}

func (m *ShowInterfaceRow) VerifyInternetAddr(value string) bool {
	return reflect.DeepEqual(m.InternetAddr, value)
}

func (m *ShowInterfaceRow) VerifyIntf(value string) bool {
	return reflect.DeepEqual(m.Intf, value)
}

func (m *ShowInterfaceRow) VerifyIntfLineProtocol(value string) bool {
	return reflect.DeepEqual(m.IntfLineProtocol, value)
}

func (m *ShowInterfaceRow) VerifyIntfState(value string) bool {
	return reflect.DeepEqual(m.IntfState, value)
}

func (m *ShowInterfaceRow) VerifyIntfStateTransitions(value string) bool {
	return reflect.DeepEqual(m.IntfStateTransitions, value)
}

func (m *ShowInterfaceRow) VerifyL2Overhead(value string) bool {
	return reflect.DeepEqual(m.L2Overhead, value)
}

func (m *ShowInterfaceRow) VerifyLoopback(value string) bool {
	return reflect.DeepEqual(m.Loopback, value)
}

func (m *ShowInterfaceRow) VerifyMaxBw(value string) bool {
	return reflect.DeepEqual(m.MaxBw, value)
}

func (m *ShowInterfaceRow) VerifyMtu(value string) bool {
	return reflect.DeepEqual(m.Mtu, value)
}

func (m *ShowInterfaceRow) VerifyMtuReliability(value string) bool {
	return reflect.DeepEqual(m.MtuReliability, value)
}

func (m *ShowInterfaceRow) VerifyMtuRxload(value string) bool {
	return reflect.DeepEqual(m.MtuRxload, value)
}

func (m *ShowInterfaceRow) VerifyMtuTxload(value string) bool {
	return reflect.DeepEqual(m.MtuTxload, value)
}

func (m *ShowInterfaceRow) VerifyMulticastPacketsOutput(value string) bool {
	return reflect.DeepEqual(m.MulticastPacketsOutput, value)
}

func (m *ShowInterfaceRow) VerifyMulticastPacketsReceived(value string) bool {
	return reflect.DeepEqual(m.MulticastPacketsReceived, value)
}

func (m *ShowInterfaceRow) VerifyNumBytesInput(value string) bool {
	return reflect.DeepEqual(m.NumBytesInput, value)
}

func (m *ShowInterfaceRow) VerifyNumBytesOutput(value string) bool {
	return reflect.DeepEqual(m.NumBytesOutput, value)
}

func (m *ShowInterfaceRow) VerifyNumPacketsOutput(value string) bool {
	return reflect.DeepEqual(m.NumPacketsOutput, value)
}

func (m *ShowInterfaceRow) VerifyOutputBufferFailures(value string) bool {
	return reflect.DeepEqual(m.OutputBufferFailures, value)
}

func (m *ShowInterfaceRow) VerifyOutputBuffersSwappedOut(value string) bool {
	return reflect.DeepEqual(m.OutputBuffersSwappedOut, value)
}

func (m *ShowInterfaceRow) VerifyOutputErrors(value string) bool {
	return reflect.DeepEqual(m.OutputErrors, value)
}

func (m *ShowInterfaceRow) VerifyOverrun(value string) bool {
	return reflect.DeepEqual(m.Overrun, value)
}

func (m *ShowInterfaceRow) VerifyParity(value string) bool {
	return reflect.DeepEqual(m.Parity, value)
}

func (m *ShowInterfaceRow) VerifyResets(value string) bool {
	return reflect.DeepEqual(m.Resets, value)
}

func (m *ShowInterfaceRow) VerifyRunts(value string) bool {
	return reflect.DeepEqual(m.Runts, value)
}

func (m *ShowInterfaceRow) VerifySpeed(value string) bool {
	return reflect.DeepEqual(m.Speed, value)
}

func (m *ShowInterfaceRow) VerifyThirtySecInputBps(value string) bool {
	return reflect.DeepEqual(m.ThirtySecInputBps, value)
}

func (m *ShowInterfaceRow) VerifyThirtySecInputPps(value string) bool {
	return reflect.DeepEqual(m.ThirtySecInputPps, value)
}

func (m *ShowInterfaceRow) VerifyThirtySecOutputBps(value string) bool {
	return reflect.DeepEqual(m.ThirtySecOutputBps, value)
}

func (m *ShowInterfaceRow) VerifyThirtySecOutputPps(value string) bool {
	return reflect.DeepEqual(m.ThirtySecOutputPps, value)
}

func (m *ShowInterfaceRow) VerifyThrottles(value string) bool {
	return reflect.DeepEqual(m.Throttles, value)
}

func (m *ShowInterfaceRow) VerifyTotalInputDrops(value string) bool {
	return reflect.DeepEqual(m.TotalInputDrops, value)
}

func (m *ShowInterfaceRow) VerifyTotalOutputDrops(value string) bool {
	return reflect.DeepEqual(m.TotalOutputDrops, value)
}

func (m *ShowInterfaceRow) VerifyUnderruns(value string) bool {
	return reflect.DeepEqual(m.Underruns, value)
}

func (m *ShowInterfaceRow) VerifyUnrecognizedUlProtocolDrops(value string) bool {
	return reflect.DeepEqual(m.UnrecognizedUlProtocolDrops, value)
}

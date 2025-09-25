// Code generated from textfsm file
package textfsm

import (
	"reflect"

	"github.com/sirikothe/gotextfsm"
)

var templateShowCefVrfDetail string = `#**************************************************
# Copyright (c) 2019 Cisco Systems, Inc.
# All rights reserved.
#**************************************************
Value Filldown ip_address ([0-9a-fA-f:/.]+)
Value Filldown version (\d+)
Value nexthopsid ([a-fA-F0-9:/._]+)
Value overlay_sid ([a-fA-F0-9:]+)
Value overlay_transit (\S+)
Value List underlay_sid ([a-fA-F0-9:]+)
Value List underlay_transit (\S+)
Value backup_path (backup)
Value bgp_multipath (bgp-multipath)
Value local_label (\d+)
Value List next_hop (\b[\da-fA-F:.]+\b)
Value List out_interface ([\w\-/]+(?:[\.\d]+)?)
Value List weight (\d+)
Value List in_label (\w+)
Value List out_labels ([^\}]+)
#
# Hash
#
Value List Hash_value ([\d\-]+)
Value List Ok_value (\S+)
Value List Interface_value ([a-zA-Z0-9-\/\._]+)
Value List Address_val (\S+)
Value List pd_context (.*)

Start
  ^\s*${ip_address}, version ${version}
  ^\s*via\s*local-srv6-sid\s*${nexthopsid},\s*\d+\s*dependencies,\s*recursive(,\s*(${backup_path}|$bgp_multipath))?
  ^\s*via\s*${nexthopsid},\s*\d+\s*dependencies,\s*recursive(,\s*(${backup_path}|$bgp_multipath))?
  ^\s*via local-label ${local_label}, \d+ dependencies,\s+recursive
  ^\s*via [\da-f:\.]+(?:/\d+)?(?:, ${out_interface})?, \d+ (dependencies|dependency),\s+weight ${weight},
  ^\s*next hop $^\s*next hop (?P<next_hop>\b[\da-fA-F:.]+\b)/\d+\s*$
  ^\s*next hop ${next_hop}/\d+\s+${out_interface}\s+labels imposed\s+\{${out_labels}\}
  ^\s*next hop ${next_hop}.*\s*[a-zA-Z\d\.\/]+\s*labels imposed\s*\{${out_labels}\}
  ^\s*next hop ${next_hop}\s*labels imposed\s*\{${out_labels}\}
  ^\s*next hop ${next_hop}\s*
  ^\s*labels imposed \{${out_labels}\}
  ^\s*local label ${in_label}\s+labels imposed {${out_labels}}
  ^\s*local label ${local_label}?
  ^\s{4}SRv6 $overlay_transit SID-list\s+\{${overlay_sid}\}
  ^\s{6}SRv6 $underlay_transit SID-list\s+\{${underlay_sid}\}
  ^\s*Hash -> Hash
  ^\s*via\s*${nexthopsid},\s*\d+\s*dependencies,\s*recursive(,\s*(${backup_path}|$bgp_multipath))? -> Start
  ^\s*via\s*local-srv6-sid\s*${nexthopsid},\s*\d+\s*dependencies,\s*recursive(,\s*(${backup_path}|$bgp_multipath))? -> Start

Hash
  ^\s*${Hash_value}\s*${Ok_value}\s*${Interface_value}\s*${Address_val}
  ^\s*via -> Continue.Record
  ^\s*via\s*${nexthopsid},\s*\d+\s*dependencies,\s*recursive(,\s*(${backup_path}|$bgp_multipath))? -> Start
  ^\s*via\s*local-srv6-sid\s*${nexthopsid},\s*\d+\s*dependencies,\s*recursive(,\s*(${backup_path}|$bgp_multipath))? -> Start`

type ShowCefVrfDetailRow struct {
	AddressVal      []string
	HashValue       []string
	InterfaceValue  []string
	OkValue         []string
	BackupPath      string
	BgpMultipath    string
	InLabel         []string
	IpAddress       string
	LocalLabel      string
	NextHop         []string
	Nexthopsid      string
	OutInterface    []string
	OutLabels       []string
	OverlaySid      string
	OverlayTransit  string
	PdContext       []string
	UnderlaySid     []string
	UnderlayTransit []string
	Version         string
	Weight          []string
}

type ShowCefVrfDetail struct {
	Rows []ShowCefVrfDetailRow
}

func (p *ShowCefVrfDetail) IsGoTextFSMStruct() {}

func (p *ShowCefVrfDetail) Parse(cliOutput string) error {
	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(templateShowCefVrfDetail); err != nil {
		return err
	}

	parser := gotextfsm.ParserOutput{}
	if err := parser.ParseTextString(string(cliOutput), fsm, true); err != nil {
		return err
	}

	for _, row := range parser.Dict {
		p.Rows = append(p.Rows,
			ShowCefVrfDetailRow{
				AddressVal:      row["Address_val"].([]string),
				HashValue:       row["Hash_value"].([]string),
				InterfaceValue:  row["Interface_value"].([]string),
				OkValue:         row["Ok_value"].([]string),
				BackupPath:      row["backup_path"].(string),
				BgpMultipath:    row["bgp_multipath"].(string),
				InLabel:         row["in_label"].([]string),
				IpAddress:       row["ip_address"].(string),
				LocalLabel:      row["local_label"].(string),
				NextHop:         row["next_hop"].([]string),
				Nexthopsid:      row["nexthopsid"].(string),
				OutInterface:    row["out_interface"].([]string),
				OutLabels:       row["out_labels"].([]string),
				OverlaySid:      row["overlay_sid"].(string),
				OverlayTransit:  row["overlay_transit"].(string),
				PdContext:       row["pd_context"].([]string),
				UnderlaySid:     row["underlay_sid"].([]string),
				UnderlayTransit: row["underlay_transit"].([]string),
				Version:         row["version"].(string),
				Weight:          row["weight"].([]string),
			},
		)
	}
	return nil
}

func (m *ShowCefVrfDetailRow) Compare(expected ShowCefVrfDetailRow) bool {
	return reflect.DeepEqual(*m, expected)
}

func (m *ShowCefVrfDetailRow) GetAddressVal() []string {
	return m.AddressVal
}

func (m *ShowCefVrfDetailRow) GetHashValue() []string {
	return m.HashValue
}

func (m *ShowCefVrfDetailRow) GetInterfaceValue() []string {
	return m.InterfaceValue
}

func (m *ShowCefVrfDetailRow) GetOkValue() []string {
	return m.OkValue
}

func (m *ShowCefVrfDetailRow) GetBackupPath() string {
	return m.BackupPath
}

func (m *ShowCefVrfDetailRow) GetBgpMultipath() string {
	return m.BgpMultipath
}

func (m *ShowCefVrfDetailRow) GetInLabel() []string {
	return m.InLabel
}

func (m *ShowCefVrfDetailRow) GetIpAddress() string {
	return m.IpAddress
}

func (m *ShowCefVrfDetailRow) GetLocalLabel() string {
	return m.LocalLabel
}

func (m *ShowCefVrfDetailRow) GetNextHop() []string {
	return m.NextHop
}

func (m *ShowCefVrfDetailRow) GetNexthopsid() string {
	return m.Nexthopsid
}

func (m *ShowCefVrfDetailRow) GetOutInterface() []string {
	return m.OutInterface
}

func (m *ShowCefVrfDetailRow) GetOutLabels() []string {
	return m.OutLabels
}

func (m *ShowCefVrfDetailRow) GetOverlaySid() string {
	return m.OverlaySid
}

func (m *ShowCefVrfDetailRow) GetOverlayTransit() string {
	return m.OverlayTransit
}

func (m *ShowCefVrfDetailRow) GetPdContext() []string {
	return m.PdContext
}

func (m *ShowCefVrfDetailRow) GetUnderlaySid() []string {
	return m.UnderlaySid
}

func (m *ShowCefVrfDetailRow) GetUnderlayTransit() []string {
	return m.UnderlayTransit
}

func (m *ShowCefVrfDetailRow) GetVersion() string {
	return m.Version
}

func (m *ShowCefVrfDetailRow) GetWeight() []string {
	return m.Weight
}

func (m *ShowCefVrfDetail) GetAllAddressVal() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.AddressVal)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllHashValue() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.HashValue)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllInterfaceValue() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.InterfaceValue)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllOkValue() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OkValue)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllBackupPath() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.BackupPath)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllBgpMultipath() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.BgpMultipath)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllInLabel() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.InLabel)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllIpAddress() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.IpAddress)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllLocalLabel() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.LocalLabel)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllNextHop() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.NextHop)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllNexthopsid() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Nexthopsid)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllOutInterface() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OutInterface)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllOutLabels() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OutLabels)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllOverlaySid() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OverlaySid)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllOverlayTransit() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.OverlayTransit)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllPdContext() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.PdContext)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllUnderlaySid() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.UnderlaySid)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllUnderlayTransit() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.UnderlayTransit)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllVersion() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Version)
	}
	return arr
}

func (m *ShowCefVrfDetail) GetAllWeight() [][]string {
	arr := [][]string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Weight)
	}
	return arr
}

func (m *ShowCefVrfDetailRow) VerifyAddressVal(value []string) bool {
	return reflect.DeepEqual(m.AddressVal, value)
}

func (m *ShowCefVrfDetailRow) VerifyHashValue(value []string) bool {
	return reflect.DeepEqual(m.HashValue, value)
}

func (m *ShowCefVrfDetailRow) VerifyInterfaceValue(value []string) bool {
	return reflect.DeepEqual(m.InterfaceValue, value)
}

func (m *ShowCefVrfDetailRow) VerifyOkValue(value []string) bool {
	return reflect.DeepEqual(m.OkValue, value)
}

func (m *ShowCefVrfDetailRow) VerifyBackupPath(value string) bool {
	return reflect.DeepEqual(m.BackupPath, value)
}

func (m *ShowCefVrfDetailRow) VerifyBgpMultipath(value string) bool {
	return reflect.DeepEqual(m.BgpMultipath, value)
}

func (m *ShowCefVrfDetailRow) VerifyInLabel(value []string) bool {
	return reflect.DeepEqual(m.InLabel, value)
}

func (m *ShowCefVrfDetailRow) VerifyIpAddress(value string) bool {
	return reflect.DeepEqual(m.IpAddress, value)
}

func (m *ShowCefVrfDetailRow) VerifyLocalLabel(value string) bool {
	return reflect.DeepEqual(m.LocalLabel, value)
}

func (m *ShowCefVrfDetailRow) VerifyNextHop(value []string) bool {
	return reflect.DeepEqual(m.NextHop, value)
}

func (m *ShowCefVrfDetailRow) VerifyNexthopsid(value string) bool {
	return reflect.DeepEqual(m.Nexthopsid, value)
}

func (m *ShowCefVrfDetailRow) VerifyOutInterface(value []string) bool {
	return reflect.DeepEqual(m.OutInterface, value)
}

func (m *ShowCefVrfDetailRow) VerifyOutLabels(value []string) bool {
	return reflect.DeepEqual(m.OutLabels, value)
}

func (m *ShowCefVrfDetailRow) VerifyOverlaySid(value string) bool {
	return reflect.DeepEqual(m.OverlaySid, value)
}

func (m *ShowCefVrfDetailRow) VerifyOverlayTransit(value string) bool {
	return reflect.DeepEqual(m.OverlayTransit, value)
}

func (m *ShowCefVrfDetailRow) VerifyPdContext(value []string) bool {
	return reflect.DeepEqual(m.PdContext, value)
}

func (m *ShowCefVrfDetailRow) VerifyUnderlaySid(value []string) bool {
	return reflect.DeepEqual(m.UnderlaySid, value)
}

func (m *ShowCefVrfDetailRow) VerifyUnderlayTransit(value []string) bool {
	return reflect.DeepEqual(m.UnderlayTransit, value)
}

func (m *ShowCefVrfDetailRow) VerifyVersion(value string) bool {
	return reflect.DeepEqual(m.Version, value)
}

func (m *ShowCefVrfDetailRow) VerifyWeight(value []string) bool {
	return reflect.DeepEqual(m.Weight, value)
}

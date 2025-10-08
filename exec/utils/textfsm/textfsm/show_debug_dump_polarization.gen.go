// Code generated from textfsm file
package textfsm

import (
	"reflect"

	"github.com/sirikothe/gotextfsm"
)

var templateShowDebugDumpPolarization string = `Value Filldown NPU (\d+)
Value Filldown ECMP_HASH_SEED (0x[0-9a-fA-F]+)
Value Filldown BUNDLE_SEED (0x[0-9a-fA-F]+)
Value Filldown LB_NODE_ID (\d+)
Value Filldown HARD (\d+)
Value Filldown SOFT (\d+)
Value Filldown RTF (\d+)
Value Filldown DEFAULT_OVERLAY_MODE (\w+)
Value TYPE (\S+)
Value OFFSET1 (\d+)
Value WIDTH1 (\d+)
Value OFFSET2 (\d+)
Value WIDTH2 (\d+)

Start
  ^Load Balancing Hash Info for NPU:\s+${NPU}
  ^ECMP Hash seed:\s+${ECMP_HASH_SEED}\s+Bundle seed:\s+${BUNDLE_SEED}
  ^Load balancing config:\s+LB node id = ${LB_NODE_ID}\s+hard:\s+${HARD},\s+soft:\s+${SOFT},\s+rtf:\s+${RTF}
  ^default_overlay_mode:\s+${DEFAULT_OVERLAY_MODE}
  ^Configured values -> Configured
  ^Load Balancing Hash Info for NPU:\s+${NPU} -> Start
  ^================================================= -> Start
  ^\s*$ -> Continue

Configured
  ^${TYPE}\s+Offset:\s+${OFFSET1}\s+Width:\s+${WIDTH1}\s+Offset:\s+${OFFSET2}\s+Width:\s+${WIDTH2} -> Record
  ^Load Balancing Hash Info for NPU:\s+${NPU} -> Start
  ^================================================= -> Start
  ^\s*$ -> Continue`

type ShowDebugDumpPolarizationRow struct {
	BundleSeed         string
	DefaultOverlayMode string
	EcmpHashSeed       string
	Hard               string
	LbNodeId           string
	Npu                string
	Offset1            string
	Offset2            string
	Rtf                string
	Soft               string
	Type               string
	Width1             string
	Width2             string
}

type ShowDebugDumpPolarization struct {
	Rows []ShowDebugDumpPolarizationRow
}

func (p *ShowDebugDumpPolarization) IsGoTextFSMStruct() {}

func (p *ShowDebugDumpPolarization) Parse(cliOutput string) error {
	fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(templateShowDebugDumpPolarization); err != nil {
		return err
	}

	parser := gotextfsm.ParserOutput{}
	if err := parser.ParseTextString(string(cliOutput), fsm, true); err != nil {
		return err
	}

	for _, row := range parser.Dict {
		p.Rows = append(p.Rows,
			ShowDebugDumpPolarizationRow{
				BundleSeed:         row["BUNDLE_SEED"].(string),
				DefaultOverlayMode: row["DEFAULT_OVERLAY_MODE"].(string),
				EcmpHashSeed:       row["ECMP_HASH_SEED"].(string),
				Hard:               row["HARD"].(string),
				LbNodeId:           row["LB_NODE_ID"].(string),
				Npu:                row["NPU"].(string),
				Offset1:            row["OFFSET1"].(string),
				Offset2:            row["OFFSET2"].(string),
				Rtf:                row["RTF"].(string),
				Soft:               row["SOFT"].(string),
				Type:               row["TYPE"].(string),
				Width1:             row["WIDTH1"].(string),
				Width2:             row["WIDTH2"].(string),
			},
		)
	}
	return nil
}

func (m *ShowDebugDumpPolarizationRow) Compare(expected ShowDebugDumpPolarizationRow) bool {
	return reflect.DeepEqual(*m, expected)
}

func (m *ShowDebugDumpPolarizationRow) GetBundleSeed() string {
	return m.BundleSeed
}

func (m *ShowDebugDumpPolarizationRow) GetDefaultOverlayMode() string {
	return m.DefaultOverlayMode
}

func (m *ShowDebugDumpPolarizationRow) GetEcmpHashSeed() string {
	return m.EcmpHashSeed
}

func (m *ShowDebugDumpPolarizationRow) GetHard() string {
	return m.Hard
}

func (m *ShowDebugDumpPolarizationRow) GetLbNodeId() string {
	return m.LbNodeId
}

func (m *ShowDebugDumpPolarizationRow) GetNpu() string {
	return m.Npu
}

func (m *ShowDebugDumpPolarizationRow) GetOffset1() string {
	return m.Offset1
}

func (m *ShowDebugDumpPolarizationRow) GetOffset2() string {
	return m.Offset2
}

func (m *ShowDebugDumpPolarizationRow) GetRtf() string {
	return m.Rtf
}

func (m *ShowDebugDumpPolarizationRow) GetSoft() string {
	return m.Soft
}

func (m *ShowDebugDumpPolarizationRow) GetType() string {
	return m.Type
}

func (m *ShowDebugDumpPolarizationRow) GetWidth1() string {
	return m.Width1
}

func (m *ShowDebugDumpPolarizationRow) GetWidth2() string {
	return m.Width2
}

func (m *ShowDebugDumpPolarization) GetAllBundleSeed() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.BundleSeed)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllDefaultOverlayMode() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.DefaultOverlayMode)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllEcmpHashSeed() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.EcmpHashSeed)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllHard() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Hard)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllLbNodeId() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.LbNodeId)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllNpu() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Npu)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllOffset1() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Offset1)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllOffset2() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Offset2)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllRtf() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Rtf)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllSoft() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Soft)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllType() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Type)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllWidth1() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Width1)
	}
	return arr
}

func (m *ShowDebugDumpPolarization) GetAllWidth2() []string {
	arr := []string{}
	for _, value := range m.Rows {
		arr = append(arr, value.Width2)
	}
	return arr
}

func (m *ShowDebugDumpPolarizationRow) VerifyBundleSeed(value string) bool {
	return reflect.DeepEqual(m.BundleSeed, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyDefaultOverlayMode(value string) bool {
	return reflect.DeepEqual(m.DefaultOverlayMode, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyEcmpHashSeed(value string) bool {
	return reflect.DeepEqual(m.EcmpHashSeed, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyHard(value string) bool {
	return reflect.DeepEqual(m.Hard, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyLbNodeId(value string) bool {
	return reflect.DeepEqual(m.LbNodeId, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyNpu(value string) bool {
	return reflect.DeepEqual(m.Npu, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyOffset1(value string) bool {
	return reflect.DeepEqual(m.Offset1, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyOffset2(value string) bool {
	return reflect.DeepEqual(m.Offset2, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyRtf(value string) bool {
	return reflect.DeepEqual(m.Rtf, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifySoft(value string) bool {
	return reflect.DeepEqual(m.Soft, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyType(value string) bool {
	return reflect.DeepEqual(m.Type, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyWidth1(value string) bool {
	return reflect.DeepEqual(m.Width1, value)
}

func (m *ShowDebugDumpPolarizationRow) VerifyWidth2(value string) bool {
	return reflect.DeepEqual(m.Width2, value)
}

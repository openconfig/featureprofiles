package ocrpcs

import (
	"fmt"

	"github.com/yoheimuta/go-protoparser/v4/parser"
)

type rpcServiceAccumulator struct {
	packageName    string
	currentService string

	rpcs []string
}

var _ parser.Visitor = &rpcServiceAccumulator{}

func (v *rpcServiceAccumulator) VisitComment(*parser.Comment) {
}

func (v *rpcServiceAccumulator) VisitEmptyStatement(*parser.EmptyStatement) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitEnum(*parser.Enum) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitEnumField(*parser.EnumField) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitExtend(*parser.Extend) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitExtensions(*parser.Extensions) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitField(*parser.Field) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitGroupField(*parser.GroupField) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitImport(*parser.Import) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitMapField(*parser.MapField) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitMessage(*parser.Message) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitOneof(*parser.Oneof) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitOneofField(*parser.OneofField) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitOption(*parser.Option) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitPackage(p *parser.Package) (next bool) {
	v.packageName = p.Name
	return
}

func (v *rpcServiceAccumulator) VisitReserved(*parser.Reserved) (next bool) {
	return
}

func (v *rpcServiceAccumulator) VisitRPC(r *parser.RPC) (next bool) {
	v.rpcs = append(v.rpcs, fmt.Sprintf("%s.%s.%s", v.packageName, v.currentService, r.RPCName))
	return
}

func (v *rpcServiceAccumulator) VisitService(s *parser.Service) (next bool) {
	v.currentService = s.ServiceName
	return true
}

func (v *rpcServiceAccumulator) VisitSyntax(*parser.Syntax) (next bool) {
	return
}

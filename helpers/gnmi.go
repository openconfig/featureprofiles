package helpers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/open-traffic-generator/snappi/gosnappi"
	gnmiclient "github.com/openconfig/gnmi/client"
	gnmiproto "github.com/openconfig/gnmi/proto/gnmi"

	// DO NOT REMOVE
	_ "github.com/openconfig/gnmi/client/gnmi"
)

type GnmiClient struct {
	client gnmiclient.BaseClient
	query  *gnmiclient.Query
	ctx    context.Context
	cfg    gosnappi.Config
}

func NewGnmiClient(query *gnmiclient.Query, cfg gosnappi.Config) (*GnmiClient, error) {
	log.Println("Creating gNMI client for server ...")

	client := gnmiclient.BaseClient{}
	log.Println("Successfully created gNMI client !")

	return &GnmiClient{
		client: client,
		query:  query,
		cfg:    cfg,
		ctx:    context.Background(),
	}, nil
}

func (c *GnmiClient) Close() {
	log.Println("Closing gNMI connection with")
	c.client.Close()
}

func (c *GnmiClient) SetQueries(prefix string, names []string) {
	c.query.Queries = []gnmiclient.Path{}
	for _, name := range names {
		c.query.Queries = append(
			c.query.Queries,
			[]string{fmt.Sprintf("%s[name=%s]", prefix, name)},
		)
	}

	log.Printf("GNMI Query: %v\n", c.query.Queries)
}

func (c *GnmiClient) GetFlowMetrics(flowNames []string) (gosnappi.MetricsResponseFlowMetricIter, error) {
	defer Timer(time.Now(), "GetFlowMetrics GNMI")
	if len(flowNames) == 0 {
		flowNames = []string{}
		for _, f := range c.cfg.Flows().Items() {
			flowNames = append(flowNames, f.Name())
		}
	}
	c.SetQueries("flow_metrics", flowNames)
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().FlowMetrics()

	c.query.ProtoHandler = func(msg proto.Message) error {
		response := msg.(*gnmiproto.SubscribeResponse)
		notification := response.GetUpdate()

		for _, update := range notification.GetUpdate() {
			jsonBytes := update.GetVal().GetJsonVal()
			flowMetric := metrics.Add()
			if err := flowMetric.FromJson(string(jsonBytes)); err != nil {
				return fmt.Errorf("could not marshal json to protobuf: %v", err)
			}
		}

		return nil
	}

	log.Println("Getting flow metrics ...")
	if err := c.client.Subscribe(c.ctx, *c.query); err != nil {
		return nil, fmt.Errorf("could not subscribe to gNMI server for flow metrics: %v", err)
	}

	return metrics, nil
}

func (c *GnmiClient) GetPortMetrics(portNames []string) (gosnappi.MetricsResponsePortMetricIter, error) {
	defer Timer(time.Now(), "GetPortMetrics GNMI")
	if len(portNames) == 0 {
		portNames = []string{}
		for _, p := range c.cfg.Ports().Items() {
			portNames = append(portNames, p.Name())
		}
	}
	c.SetQueries("port_metrics", portNames)
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().PortMetrics()

	c.query.ProtoHandler = func(msg proto.Message) error {
		response := msg.(*gnmiproto.SubscribeResponse)
		notification := response.GetUpdate()

		for _, update := range notification.GetUpdate() {
			jsonBytes := update.GetVal().GetJsonVal()
			portMetric := metrics.Add()

			if err := portMetric.FromJson(string(jsonBytes)); err != nil {
				return fmt.Errorf("could not marshal json to gosnappi object: %v", err)
			}
		}

		return nil
	}

	log.Println("Getting port metrics ...")
	if err := c.client.Subscribe(c.ctx, *c.query); err != nil {
		return nil, fmt.Errorf("could not subscribe to gNMI server for port metrics: %v", err)
	}

	return metrics, nil
}

func (c *GnmiClient) GetBgpv4Metrics(deviceNames []string) (gosnappi.MetricsResponseBgpv4MetricIter, error) {
	defer Timer(time.Now(), "GetBgpv4Metrics GNMI")
	if len(deviceNames) == 0 {
		deviceNames = []string{}
		for _, d := range c.cfg.Devices().Items() {
			bgp := d.Bgp()
			for _, ip := range bgp.Ipv4Interfaces().Items() {
				for _, peer := range ip.Peers().Items() {
					deviceNames = append(deviceNames, peer.Name())
				}
			}
		}
	}
	c.SetQueries("bgpv4_metrics", deviceNames)
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().Bgpv4Metrics()

	c.query.ProtoHandler = func(msg proto.Message) error {
		response := msg.(*gnmiproto.SubscribeResponse)
		notification := response.GetUpdate()

		for _, update := range notification.GetUpdate() {
			jsonBytes := update.GetVal().GetJsonVal()
			bgpv4Metric := metrics.Add()
			if err := bgpv4Metric.FromJson(string(jsonBytes)); err != nil {
				return fmt.Errorf("could not marshal json to protobuf: %v", err)
			}
		}

		return nil
	}

	log.Println("Getting bgpv4 metrics ...")
	if err := c.client.Subscribe(c.ctx, *c.query); err != nil {
		return nil, fmt.Errorf("could not subscribe to gNMI server for bgpv4 metrics: %v", err)
	}

	return metrics, nil
}

func (c *GnmiClient) GetBgpv6Metrics(deviceNames []string) (gosnappi.MetricsResponseBgpv6MetricIter, error) {
	defer Timer(time.Now(), "GetBgpv6Metrics GNMI")
	if len(deviceNames) == 0 {
		deviceNames = []string{}
		for _, d := range c.cfg.Devices().Items() {
			bgp := d.Bgp()
			for _, ipv6 := range bgp.Ipv6Interfaces().Items() {
				for _, peer := range ipv6.Peers().Items() {
					deviceNames = append(deviceNames, peer.Name())
				}
			}
		}
	}
	c.SetQueries("bgpv6_metrics", deviceNames)
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().Bgpv6Metrics()

	c.query.ProtoHandler = func(msg proto.Message) error {
		response := msg.(*gnmiproto.SubscribeResponse)
		notification := response.GetUpdate()

		for _, update := range notification.GetUpdate() {
			jsonBytes := update.GetVal().GetJsonVal()
			bgpv6Metric := metrics.Add()
			if err := bgpv6Metric.FromJson(string(jsonBytes)); err != nil {
				return fmt.Errorf("could not marshal json to protobuf: %v", err)
			}
		}

		return nil
	}

	log.Println("Getting bgpv6 metrics ...")
	if err := c.client.Subscribe(c.ctx, *c.query); err != nil {
		return nil, fmt.Errorf("could not subscribe to gNMI server for bgpv6 metrics: %v", err)
	}

	return metrics, nil
}

func (c *GnmiClient) GetIsisMetrics(routerNames []string) (gosnappi.MetricsResponseIsisMetricIter, error) {
	defer Timer(time.Now(), "GetIsisMetrics GNMI")
	if len(routerNames) == 0 {
		routerNames = []string{}
		for _, d := range c.cfg.Devices().Items() {
			isis := d.Isis()
			routerNames = append(routerNames, isis.Name())
		}
	}
	c.SetQueries("isis_metrics", routerNames)
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().IsisMetrics()
	c.query.ProtoHandler = func(msg proto.Message) error {
		response := msg.(*gnmiproto.SubscribeResponse)
		notification := response.GetUpdate()
		for _, update := range notification.GetUpdate() {
			jsonBytes := update.GetVal().GetJsonVal()
			isisMetric := metrics.Add()
			if err := isisMetric.FromJson(string(jsonBytes)); err != nil {
				return fmt.Errorf("could not marshal json to protobuf: %v", err)
			}
		}
		return nil
	}
	log.Println("Getting ISIS metrics ...")
	if err := c.client.Subscribe(c.ctx, *c.query); err != nil {
		return nil, fmt.Errorf("could not subscribe to gNMI server for ISIS metrics: %v", err)
	}
	return metrics, nil
}

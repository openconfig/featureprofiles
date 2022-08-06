package lwotgtelem

import (
	"context"
	"fmt"
	"sync"

	"github.com/openconfig/featureprofiles/tools/traffic/lwotg"
	"github.com/openconfig/lemming/gnmi/gnmit"
	"k8s.io/klog/v2"
)

// LWOTGGNMIServer is a gNMI server that can be used with the lightweight
// OTG implementation.
type LWOTGGNMIServer struct {
	c          *gnmit.Collector
	GNMIServer *gnmit.GNMIServer

	hintCh chan lwotg.Hint

	hintsMu sync.RWMutex
	hints   map[string]map[string]string
}

// New returns a new LWOTG gNMI implementation acting for an OTG server with hostname
// "hostname". It returns the instantiated server, or an error if experienced.
func New(ctx context.Context, hostname string) (*LWOTGGNMIServer, error) {
	defaultTasks := []gnmit.Task{{
		Run: interfaceState,
	}, {
		Run: dateTime,
	}}
	c, g, err := gnmit.NewServer(ctx, hostname, true, defaultTasks)
	if err != nil {
		return nil, fmt.Errorf("cannot create gnmit server, %v", err)
	}

	l := &LWOTGGNMIServer{
		c:          c,
		GNMIServer: g,
		hints:      map[string]map[string]string{},
	}

	l.GNMIServer.RegisterTask(gnmit.Task{
		Run: makeARPNeighborTask(ctx, l.hints),
	})

	return l, nil
}

func (l *LWOTGGNMIServer) SetHintChannel(ctx context.Context, ch chan lwotg.Hint) {
	l.hintCh = ch
	go func() {
		for {
			select {
			case h := <-l.hintCh:
				l.SetHint(h.Group, h.Key, h.Val)
			case <-ctx.Done():
				return
			}
		}

	}()
}

func (l *LWOTGGNMIServer) SetHint(group, key, val string) {
	l.hintsMu.Lock()
	defer l.hintsMu.Unlock()

	klog.Infof("Setting hint %s %s = %s", group, key, val)
	if _, ok := l.hints[group]; !ok {
		l.hints[group] = map[string]string{}
	}
	l.hints[group][key] = val
	return
}

// AddTask adds the task t to the current tasks run by the OTG implementation.
// It can be used to request the OTG server to return new telemetry.
func (l *LWOTGGNMIServer) AddTask(t gnmit.Task) error {
	return l.GNMIServer.RegisterTask(t)
}

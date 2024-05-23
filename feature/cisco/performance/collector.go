package performance

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PerformanceData struct {
	Timestamp          string `bson:"timestamp,omitempty" json:"timestamp"`
	PartNo             string `bson:"partNo,omitempty" json:"partNo"`
	SoftwareVersion    string `bson:"softwareVersion,omitempty" json:"softwareVersion"`
	Release            string `bson:"release,omitempty" json:"release"`
	Name               string `bson:"name,omitempty" json:"name"`
	SerialNo           string `bson:"serialNo,omitempty" json:"serialNo"`
	Location           string `bson:"location,omitempty" json:"location"`
	Type               string `bson:"type,omitempty" json:"type"`
	Feature            string `bson:"feature,omitempty" json:"feature"`
	Trigger            string `bson:"trigger,omitempty" json:"trigger"`
	FinishedCollecting FinishedCollecting
	ProcessData        []ProcessData `bson:"processData,omitempty" json:"processData"`
	MemoryUsed         float64       `bson:"memoryUsed,omitempty" json:"memoryUsed"`
	MemoryFree         float64       `bson:"memoryFree,omitempty" json:"memoryFree"`
	CpuUser            float64       `bson:"cpuUser,omitempty" json:"cpuUser"`
	CpuKernel          float64       `bson:"cpuKernel,omitempty" json:"cpuKernel"`
}

type ProcessData struct {
	ProcessName string  `bson:"processName,omitempty" json:"processName"`
	ProcessMem  float64 `bson:"processMem,omitempty" json:"processMem"`
	ProcessCpu  float64 `bson:"processCpu,omitempty" json:"processCpu"`
}

type FinishedCollecting bool

const (
	During FinishedCollecting = false
	Done   FinishedCollecting = true
)

type FinishCollection func()

// Starts background data collection. returns function which stops the collection when called
func RunCollector(t *testing.T, dut *ondatra.DUTDevice, featureName string, triggerName string, frequency time.Duration) (FinishCollection, error) {
	t.Logf("Collector starting at %s", time.Now())

	// openconfig-system:system/state
	sys := gnmi.Get(t, dut, gnmi.OC().System().State())

	// openconfig-platform:components/component/state
	platform := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())

	var dataEntries []PerformanceData

	data := PerformanceData{
		// Platform: "8201",
		SoftwareVersion: sys.GetSoftwareVersion(),
		// Release:  "24.2.1",
		Feature: featureName,
		Trigger: triggerName,
	}

	for _, component := range platform {
		switch component.GetType() {
		case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD:
			if component.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				data.Type = "RP"
			}
		case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD:
			data.Type = "LC"
		default:
			continue
		}
		data.PartNo = component.GetPartNo()
		data.Name = component.GetName()
		data.Location = component.GetLocation()
		data.SerialNo = component.GetSerialNo()
		dataEntries = append(dataEntries, data)
	}

	t.Logf("Image: %+v", data)

	for _, entry := range dataEntries {
		fmt.Printf("data entry: %+v\n", entry)
	}

	completedChan := make(chan FinishedCollecting, 1)

	// begin collection thread
	resultChan := collectAllData(t, dut, completedChan, dataEntries, frequency)

	var results []PerformanceData

	finish := func() {
		t.Logf("Collector finished at %s", time.Now())
		completedChan <- Done
		results = <-resultChan

		err := pushToDB(results)
		if err != nil {
			t.Fatalf("Could not push to DB: %s", err)
		}
	}

	return finish, nil
}

func collectAllData(t *testing.T, dut *ondatra.DUTDevice, stageChan chan FinishedCollecting, perfData []PerformanceData, frequency time.Duration) chan []PerformanceData {
	wg := &sync.WaitGroup{}
	resultChan := make(chan []PerformanceData)
	results := []PerformanceData{}
	mu := &sync.Mutex{}
	stageChannelsMultiplexed := []chan FinishedCollecting{}
	for _, data := range perfData {
		wg.Add(1)

		stageChanCurrent := make(chan FinishedCollecting, 1)
		stageChannelsMultiplexed = append(stageChannelsMultiplexed, stageChanCurrent)

		go func(dataCurrent PerformanceData, stageChanCurrent chan FinishedCollecting) {
			defer wg.Done()
			t.Logf("Starting collection for component: %s", dataCurrent.Name)
			cliClient := dut.RawAPIs().CLI(t)
			t.Logf("Established client for component: %s", dataCurrent.Name)
			ticker := time.NewTicker(frequency)
			done := false
			for !done {
				select {
				case dataCurrent.FinishedCollecting = <-stageChanCurrent:
					if dataCurrent.FinishedCollecting == Done {
						t.Logf("Received done signal on component: %s", dataCurrent.Name)
						done = true
					}
				case <-ticker.C:
					if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
						var finalData *PerformanceData
						var err error
						switch dataCurrent.Type {
						case "RP":
							finalData, err = TopCpuMemoryUtilization(t, dut, cliClient, dataCurrent)
							if finalData != nil {
								t.Logf("component %s:, collected: %+v", dataCurrent.Name, finalData)
							}
						case "LC":
							finalData, err = TopLineCardCpuMemoryUtilization(t, dut, cliClient, dataCurrent)
							if finalData != nil {
								t.Logf("component %s:, collected: %+v", dataCurrent.Name, finalData)
							}
						}

						if err != nil {
							t.Logf("Could not get response on component %s (%s), attempting to reestablish client", dataCurrent.Name, err)
							cliClient = dut.RawAPIs().CLI(t)
						}

						if finalData != nil {
							mu.Lock()
							results = append(results, *finalData)
							mu.Unlock()
						}

					}); errMsg != nil {
						t.Logf(*errMsg)
						continue
					}
				}
			}
		}(data, stageChanCurrent)
	}

	// multiplex message of trigger status
	go func() {
		for stage := range stageChan {
			for _, channel := range stageChannelsMultiplexed {
				channel <- stage
			}
		}
	}()

	go func() {
		wg.Wait()
		resultChan <- results
	}()

	return resultChan
}

// CLI Parser that runs the top linux command on the DUT
func TopCpuMemoryUtilization(t testing.TB, dut *ondatra.DUTDevice, cliClient binding.CLIClient, data PerformanceData) (*PerformanceData, error) {
	if cliClient == nil {
		return nil, errors.New("CLI client not established")
	}
	command := "run top -b | head -n 30"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	cliOutput, err := cliClient.RunCommand(ctx, command)

	// if ctx.Err() == context.DeadlineExceeded {
	// 	return nil, errors.New("context deadline exceeded")
	// }

	if err != nil {
		return nil, err
	}

	lines := strings.Split(cliOutput.Output(), "\n")
	// procRe := regexp.MustCompile(`^\s*\d+\s+\w+\s+\d+\s+-?\d+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\d+\.\d+)\s+(\d+\.\d+)`)
	memRe := regexp.MustCompile(`MiB Mem :\s+(\d+\.\d+) total,\s+(\d+\.\d+) free,\s+(\d+\.\d+) used`)
	cpuRe := regexp.MustCompile(`\%Cpu\(s\):\s+(\d+\.\d+)\s+us,\s+(\d+\.\d+)\s+sy,.+`)

	var freeMem, usedMem, cpuUser, cpuKernel float64

	for _, line := range lines {
		// process usage
		// if processMatches := procRe.FindStringSubmatch(line); len(processMatches) > 2 {
		// 	procCpuUsage, err := strconv.ParseFloat(processMatches[1], 64)
		// 	if err != nil {
		// 		continue
		// 	}
		// 	procMemUsage, err := strconv.ParseFloat(processMatches[2], 64)
		// 	if err != nil {
		// 		continue
		// 	}
		// }

		// Check for total, free, and used memory
		if memMatches := memRe.FindStringSubmatch(line); len(memMatches) > 3 {
			freeMem, err = strconv.ParseFloat(memMatches[2], 64)
			if err != nil {
				return nil, err
			}
			usedMem, err = strconv.ParseFloat(memMatches[3], 64)
			if err != nil {
				return nil, err
			}
		}

		// Check for cpu usage in user and kernel spaces
		if cpuMatches := cpuRe.FindStringSubmatch(line); len(cpuMatches) > 2 {
			cpuUser, err = strconv.ParseFloat(cpuMatches[1], 64)
			if err != nil {
				return nil, err
			}
			cpuKernel, err = strconv.ParseFloat(cpuMatches[2], 64)
			if err != nil {
				return nil, err
			}
		}
	}

	data.Timestamp = time.Now().Format(time.RFC3339)
	data.MemoryUsed = usedMem
	data.MemoryFree = freeMem
	data.CpuUser = cpuUser
	data.CpuKernel = cpuKernel

	return &data, nil
}

// CLI Parser that runs the top linux command on each line card in the DUT
func TopLineCardCpuMemoryUtilization(t testing.TB, dut *ondatra.DUTDevice, cliClient binding.CLIClient, data PerformanceData) (*PerformanceData, error) {
	if cliClient == nil {
		return nil, errors.New("CLI client not established")
	}
	var node int64
	nodesRe := regexp.MustCompile(`^\d+\/(\d+)\/CPU\d+`)

	if nodeStr := nodesRe.FindStringSubmatch(data.Name); len(nodeStr) > 1 {
		nodeVal, err := strconv.ParseInt(nodeStr[1], 10, 64)
		if err != nil {
			return nil, err
		}
		node = nodeVal
	}
	t.Logf("collecting line card slot %d", node)

	commandFormat := "run ssh 172.0.%d.1 top -b | head -n 30"

	command := fmt.Sprintf(commandFormat, node)

	t.Logf("Running: `%s`", command)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	cliOutput, err := cliClient.RunCommand(ctx, command)

	// if ctx.Err() == context.DeadlineExceeded {
	// 	return nil, errors.New("context deadline exceeded")
	// }

	if err != nil {
		return nil, err
	}

	t.Log("successfully ran command")

	var freeMem, usedMem, cpuUser, cpuKernel float64

	lines := strings.Split(cliOutput.Output(), "\n")
	// procRe := regexp.MustCompile(`^\s*\d+\s+\w+\s+\d+\s+-?\d+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\d+\.\d+)\s+(\d+\.\d+)`)
	memRe := regexp.MustCompile(`MiB Mem :\s+(\d+\.\d+) total,\s+(\d+\.\d+) free,\s+(\d+\.\d+) used`)
	cpuRe := regexp.MustCompile(`\%Cpu\(s\):\s+(\d+\.\d+)\s+us,\s+(\d+\.\d+)\s+sy,.+`)

	for _, line := range lines {
		// Check for CPU and MEM usage
		// if processMatches := procRe.FindStringSubmatch(line); len(processMatches) > 2 {
		// 	procCpuUsage, err := strconv.ParseFloat(processMatches[1], 64)
		// 	if err != nil {
		// 		continue
		// 	}
		// 	procMemUsage, err := strconv.ParseFloat(processMatches[2], 64)
		// 	if err != nil {
		// 		continue
		// 	}
		// }

		// Check for total, free, and used memory
		if memMatches := memRe.FindStringSubmatch(line); len(memMatches) > 3 {
			freeMem, err = strconv.ParseFloat(memMatches[2], 64)
			if err != nil {
				return nil, err
			}
			usedMem, err = strconv.ParseFloat(memMatches[3], 64)
			if err != nil {
				return nil, err
			}
		}

		// Check for cpu usage in user and kernel spaces
		if cpuMatches := cpuRe.FindStringSubmatch(line); len(cpuMatches) > 2 {
			cpuUser, err = strconv.ParseFloat(cpuMatches[1], 64)
			if err != nil {
				return nil, err
			}
			cpuKernel, err = strconv.ParseFloat(cpuMatches[2], 64)
			if err != nil {
				return nil, err
			}
		}
	}

	data.Timestamp = time.Now().Format(time.RFC3339)
	data.MemoryUsed = usedMem
	data.MemoryFree = freeMem
	data.CpuUser = cpuUser
	data.CpuKernel = cpuKernel

	return &data, nil

}

func castSliceToInterface(data []PerformanceData) []interface{} {
	castData := make([]interface{}, len(data))
	for i, entry := range data {
		entry.Timestamp = options.TimeSeries().TimeField
		castData[i] = entry
	}
	return castData
}

func pushToDB(data []PerformanceData) error {
	// Set client options
	clientOptions := options.Client().ApplyURI("mongodb://xr-sf-npi-lnx:27017")

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // cancel when we are finished consuming the context
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return err
	}

	// Access the collection
	collection := client.Database("XR_SF_NPI_OC_MODELS").Collection("oc-perf-sandbox")

	// topts := options.TimeSeries().SetTimeField("timestamp")
	// opts := options.CreateCollection()

	wrappedData := struct {
		TimeSeries []PerformanceData `bson:"timeseries" json:"timeseries"`
	}{
		TimeSeries: data,
	}

	// Insert the document into the collection
	// result, err := collection.InsertMany(ctx, castSliceToInterface(data))
	result, err := collection.InsertOne(ctx, wrappedData)

	fmt.Printf("result: %v\n", result)
	if err != nil {
		return err
	}

	return nil
}

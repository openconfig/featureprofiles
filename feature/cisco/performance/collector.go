package performance

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/openconfig/featureprofiles/feature/cisco/performance/flagUtils"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PerformanceData struct {
	Timestamp       string           `bson:"timestamp,omitempty" json:"timestamp"`
	PartNo          string           `bson:"partNo,omitempty" json:"partNo"`
	SoftwareRelease string           `bson:"softwareVersion,omitempty" json:"softwareVersion"`
	SoftwareImage   string           `bson:"SoftwareImage,omitempty" json:"SoftwareImage"`
	Release         string           `bson:"release,omitempty" json:"release"`
	Name            string           `bson:"name,omitempty" json:"name"`
	SerialNo        string           `bson:"serialNo,omitempty" json:"serialNo"`
	Location        string           `bson:"location,omitempty" json:"location"`
	Chassis         string           `bson:"chassis,omitempty" json:"chassis"`
	Type            string           `bson:"type,omitempty" json:"type"`
	Feature         string           `bson:"feature,omitempty" json:"feature"`
	Trigger         string           `bson:"trigger,omitempty" json:"trigger"`
	ProcessData     []ProcessData    `bson:"processData,omitempty" json:"processData"`
	ScaleValues     *ScaleAttributes `bson:"scaleValues"`
	MemoryUsed      float64          `bson:"memoryUsed,omitempty" json:"memoryUsed"`
	MemoryFree      float64          `bson:"memoryFree,omitempty" json:"memoryFree"`
	CpuUser         float64          `bson:"cpuUser,omitempty" json:"cpuUser"`
	CpuKernel       float64          `bson:"cpuKernel,omitempty" json:"cpuKernel"`
}

type ProcessData struct {
	ProcessName string  `bson:"processName,omitempty" json:"processName"`
	ProcessMem  float64 `bson:"processMem,omitempty" json:"processMem"`
	ProcessCpu  float64 `bson:"processCpu,omitempty" json:"processCpu"`
}

type ScaleAttributes struct {
	TotalIPv4Routes      int `bson:"totalIPv4Routes,omitempty" json:"totalIPv4Routes"`
	TotalIPv6Routes      int `bson:"totalIPv6Routes,omitempty" json:"totalIPv6Routes"`
	MplsLabelScale       int `bson:"mplsLabelScale,omitempty" json:"mplsLabelScale"`
	ConfiguredInterfaces int `bson:"configuredInterfaces,omitempty" json:"configuredInterfaces"`
	BGPNeighbors         int `bson:"bgpNeighbors,omitempty" json:"bgpNeighbors"`
	IsisNeighbors        int `bson:"isisNeighbors,omitempty" json:"isisNeighbors"`
	OspfNeighbors        int `bson:"ospfNeighbors,omitempty" json:"ospfNeighbors"`
	L2vpnXConnect        int `bson:"l2vpnXConnect,omitempty" json:"l2vpnXConnect"`
}

type Collector struct {
	results       []PerformanceData
	objectID      primitive.ObjectID
	doneChan      chan struct{}
	wg            sync.WaitGroup
	flagOptions   flagUtils.FlagOptions
	mu            sync.Mutex
	cliClientLock sync.Mutex
	clientPool    map[string]*binding.CLIClient
	sys           *oc.System
	platform      []*oc.Component
	scale         *ScaleAttributes
	chassis       string
}

type FinishCollection func() ([]PerformanceData, error)

// package-level variable to store the collector instance
var collector Collector

func (c *Collector) getClient(t *testing.T, dut *ondatra.DUTDevice, componentName string) binding.CLIClient {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	c.cliClientLock.Lock()
	defer c.cliClientLock.Unlock()

	if c.clientPool == nil {
		c.clientPool = make(map[string]*binding.CLIClient)
	}

	if client, exists := c.clientPool[componentName]; exists {
		t.Logf("Reusing existing CLI client for component: %s", componentName)
		return *client
	}

	var client binding.CLIClient
	for i := 0; i < maxRetries; i++ {
		client = dut.RawAPIs().CLI(t)
		if client != nil {
			c.clientPool[componentName] = &client
			t.Logf("Successfully created new CLI client for component: %s on attempt %d", componentName, i+1)
			return client
		}
		t.Logf("Failed to create CLI client on attempt %d for component %s: %v", i+1, componentName, client)
		time.Sleep(retryDelay)
	}

	t.Fatalf("Failed to create CLI client for component %s after %d attempts", componentName, maxRetries)
	return nil // This line will never be reached due to the t.Fatalf call
}

// RunCollector starts the collector
func RunCollector(t *testing.T, dut *ondatra.DUTDevice, featureName string, triggerName string, frequency time.Duration, flagOptions flagUtils.FlagOptions) error {
	collector.doneChan = make(chan struct{})
	collector.results = nil
	collector.flagOptions = flagOptions

	t.Logf("Collector starting at %s", time.Now())

	if collector.scale == nil {
		cliClient := dut.RawAPIs().CLI(t)
		scale, err := CollectScaleAttributes(t, dut, cliClient)
		if err != nil {
			return err
		}
		collector.scale = scale
	}

	if collector.sys == nil || collector.platform == nil {
		collector.sys = gnmi.Get(t, dut, gnmi.OC().System().State())
		collector.platform = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	}

	// Collect chassis information if it hasn't been collected yet
	if collector.chassis == "" {
		cliClient := dut.RawAPIs().CLI(t)
		chassisCmdResult, err := cliClient.RunCommand(context.Background(), "show inventory chassis")
		if err != nil {
			t.Fatalf("Failed to run command: %v", err)
		}

		// Extract the output as a string
		chassisStrOutput := chassisCmdResult.Output()
		chassisPattern := regexp.MustCompile(`PID:\s+(\d+)`)
		chassisMatch := chassisPattern.FindStringSubmatch(chassisStrOutput)
		if len(chassisMatch) < 2 {
			t.Fatalf("Failed to find chassis PID in the output")
		}
		collector.chassis = chassisMatch[1]
	}

	var dataEntries []PerformanceData

	imageStr := collector.sys.GetSoftwareVersion()
	imagePattern := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	release := imagePattern.FindString(imageStr)

	data := PerformanceData{
		SoftwareRelease: release,
		SoftwareImage:   imageStr,
		Feature:         featureName,
		Trigger:         triggerName,
		ScaleValues:     collector.scale,
		Chassis:         collector.chassis, // Use the collected chassis information
	}

	for _, component := range collector.platform {
		switch component.GetType() {
		case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD:
			if component.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				if component.RedundantRole == oc.Platform_ComponentRedundantRole_SECONDARY {
					continue
				}
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

	collector.wg.Add(1)
	go collector.collectAllData(t, dut, dataEntries, frequency)

	return nil
}

func StopCollector(t *testing.T) ([]PerformanceData, error) {
	t.Logf("Signaling goroutines to stop at %s", time.Now())
	close(collector.doneChan)
	collector.wg.Wait()
	t.Logf("All goroutines finished at %s", time.Now())

	if collector.results == nil {
		return nil, fmt.Errorf("no data collected")
	}

	if collector.flagOptions.NoDBRun {
		t.Logf("Not pushing results to database as flagOptions.NoDBRun is set to %v", collector.flagOptions.NoDBRun)
	} else {
		err, dbBool := collector.pushToDB(t, collector.results)
		if err != nil && dbBool != true {
			return collector.results, err
		}
	}

	return collector.results, nil
}

func (c *Collector) collectAllData(t *testing.T, dut *ondatra.DUTDevice, perfData []PerformanceData, frequency time.Duration) {
	defer c.wg.Done()
	mu := &sync.Mutex{}
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for {
		select {
		case <-c.doneChan:
			t.Logf("Received done signal")
			return
		case <-ticker.C:
			timestamp := time.Now().Format(time.RFC3339)
			t.Logf("Collecting data at %s", timestamp)
			for _, dataCurrent := range perfData {
				if dataCurrent.Type == "RP" && dataCurrent.Name == "standby" {
					t.Logf("Skipping standby RP component: %s", dataCurrent.Name)
					continue
				}
				t.Logf("Starting collection for component: %s", dataCurrent.Name)
				cliClient := c.getClient(t, dut, dataCurrent.Name)
				t.Logf("Established client for component: %s", dataCurrent.Name)
				errMsg := testt.CaptureFatal(t, func(tb testing.TB) {
					var finalData *PerformanceData
					var err error
					switch dataCurrent.Type {
					case "RP":
						finalData, err = TopCpuMemoryUtilization(tb, dut, cliClient, dataCurrent)
						if finalData != nil {
							finalData.Timestamp = timestamp
							t.Logf("component %s:, collected: %+v", dataCurrent.Name, finalData)
						}
					case "LC":
						finalData, err = TopLineCardCpuMemoryUtilization(tb, dut, cliClient, dataCurrent)
						if finalData != nil {
							finalData.Timestamp = timestamp
							t.Logf("component %s:, collected: %+v", dataCurrent.Name, finalData)
						}
					}

					if err != nil {
						t.Logf("Could not get response on component %s (%s), attempting to reestablish client", dataCurrent.Name, err)
						cliClient = c.getClient(t, dut, dataCurrent.Name)
					}

					if finalData != nil {
						mu.Lock()
						c.results = append(c.results, *finalData)
						mu.Unlock()
					}
				})
				if errMsg != nil {
					t.Logf("Error during collection for component %s: %s", dataCurrent.Name, *errMsg)
					return
				}
			}
		}
	}
}

// CollectCpuMemoryUtilization collects CPU and memory utilization data from the DUT.
func CollectCpuMemoryUtilization(t testing.TB, dut *ondatra.DUTDevice, cliClient binding.CLIClient, data PerformanceData, command string) (*PerformanceData, error) {
	if cliClient == nil {
		return nil, errors.New("CLI client not established")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	cliOutput, err := cliClient.RunCommand(ctx, command)

	if err != nil {
		return nil, err
	}

	lines := strings.Split(cliOutput.Output(), "\n")
	procRe := regexp.MustCompile(`^\s*\d+\s+\w+\s+[\w+-]+\s+[\d-]+\s+\S+\s+\S+\s+\S+\s+\S\s+(\d+\.\d+)\s+(\d+\.\d+)\s+\S+\s+(.+)$`)
	memRe := regexp.MustCompile(`MiB Mem :\s+(\d+\.\d+) total,\s+(\d+\.\d+) free,\s+(\d+\.\d+) used`)
	cpuRe := regexp.MustCompile(`\%Cpu\(s\):\s+(\d+\.\d+)\s+us,\s+(\d+\.\d+)\s+sy,.+`)

	var freeMem, usedMem, cpuUser, cpuKernel float64
	var processes []ProcessData

	memFound, cpuFound := false, false

	for _, line := range lines {
		// Process usage
		if processMatches := procRe.FindStringSubmatch(line); len(processMatches) > 3 {
			procCpuUsage, err := strconv.ParseFloat(processMatches[1], 64)
			if err != nil {
				continue
			}
			procMemUsage, err := strconv.ParseFloat(processMatches[2], 64)
			if err != nil {
				continue
			}

			// Skip appending if both CPU and memory usage are 0
			if procCpuUsage != 0 || procMemUsage != 0 {
				processes = append(processes, ProcessData{
					ProcessName: processMatches[3],
					ProcessCpu:  procCpuUsage,
					ProcessMem:  procMemUsage,
				})
			}
		}

		// Check for total, free, and used memory
		if memMatches := memRe.FindStringSubmatch(line); len(memMatches) > 3 {
			memFound = true
			freeMem, err = strconv.ParseFloat(memMatches[2], 64)
			if err != nil {
				return nil, err
			}
			usedMem, err = strconv.ParseFloat(memMatches[3], 64)
			if err != nil {
				return nil, err
			}
		}

		// Check for CPU usage in user and kernel spaces
		if cpuMatches := cpuRe.FindStringSubmatch(line); len(cpuMatches) > 2 {
			cpuFound = true
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

	if !memFound {
		return nil, errors.New("memory information not found")
	}
	if !cpuFound {
		return nil, errors.New("CPU information not found")
	}

	data.MemoryUsed = usedMem
	data.MemoryFree = freeMem
	data.CpuUser = cpuUser
	data.CpuKernel = cpuKernel
	data.ProcessData = processes

	return &data, nil
}

// TopCpuMemoryUtilization Wrapper function to handle RP
func TopCpuMemoryUtilization(t testing.TB, dut *ondatra.DUTDevice, cliClient binding.CLIClient, data PerformanceData) (*PerformanceData, error) {
	command := "run top -b | head -n 30"
	return CollectCpuMemoryUtilization(t, dut, cliClient, data, command)
}

// TopLineCardCpuMemoryUtilization Wrapper function to handle LineCard
func TopLineCardCpuMemoryUtilization(t testing.TB, dut *ondatra.DUTDevice, cliClient binding.CLIClient, data PerformanceData) (*PerformanceData, error) {
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

	return CollectCpuMemoryUtilization(t, dut, cliClient, data, command)
}

// ConnectToMongo sources MongoDB environment variables from a remote machine via SSH
func ConnectToMongo(t *testing.T, flagOptions flagUtils.FlagOptions) (*mongo.Collection, error) {

	var mongoClient *mongo.Client
	var databaseName string
	var collectionName string

	user := os.Getenv("USER")
	host := "sjc-ads-9292:22"
	remotePath := "/auto/tftp-sjc-users2/mastarke/oc-aft-mongo-env/"
	var envFile string

	userPath := fmt.Sprintf("/Users/%s/.ssh/id_rsa", user)
	key, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("Failed to read SSH private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		t.Fatalf("Failed to parse SSH private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		t.Fatalf("Failed to dial SSH: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Detect the shell on the remote machine
	var shellCheck bytes.Buffer
	session.Stdout = &shellCheck
	if err := session.Run("echo $SHELL"); err != nil {
		t.Fatalf("Failed to detect remote shell: %v", err)
	}

	if flagOptions.LocalRun {
		// sourcing env files for local runs to connect to database
		remoteShell := strings.TrimSpace(shellCheck.String())
		t.Logf("Detected remote shell: %s\n", remoteShell)

		if filepath.Base(remoteShell) == "bash" {
			envFile = filepath.Join(remotePath, "mongo-env-bashrc")
		} else if filepath.Base(remoteShell) == "csh" {
			envFile = filepath.Join(remotePath, "mongo-env-cshrc")
		} else if filepath.Base(remoteShell) == "zsh" {
			envFile = filepath.Join(remotePath, "mongo-env-zshrc")
		} else {
			t.Fatalf("Unknown remote shell: %s", remoteShell)
		}

		t.Logf("Using environment file: %s\n", envFile)

		session, err = client.NewSession()
		if err != nil {
			t.Logf("Failed to create SSH session: %v", err)
		}
		defer session.Close()

		var sourceEnv bytes.Buffer
		session.Stdout = &sourceEnv
		sourceCommand := fmt.Sprintf("source %s && env", envFile)
		if err := session.Run(sourceCommand); err != nil {
			t.Fatalf("Failed to source env file: %v", err)
		}

		scanner := bufio.NewScanner(&sourceEnv)
		for scanner.Scan() {
			line := scanner.Text()
			parts := bytes.SplitN([]byte(line), []byte("="), 2)
			if len(parts) == 2 {
				key := string(parts[0])
				value := string(parts[1])
				os.Setenv(key, value)
			}
		}

		if err := scanner.Err(); err != nil {
			t.Fatalf("Failed to parse env variables: %v", err)
		}

		// Connect to MongoDB
		mongoURI := os.Getenv("MONGODB_URI")
		if mongoURI == "" {
			return nil, fmt.Errorf("MONGODB_URI not set in environment variables")
		}

		clientOptions := options.Client().ApplyURI(mongoURI)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		mongoClient, err = mongo.Connect(ctx, clientOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
		}

		databaseName = os.Getenv("MONGO_DATABASE")
		if databaseName == "" {
			return nil, fmt.Errorf("MONGO_DATABASE not set in environment variables")
		}
		collectionName = os.Getenv("MONGO_COLLECTION")
		if collectionName == "" {
			return nil, fmt.Errorf("MONGO_COLLECTION not set in environment variables")
		}

	} else if flagOptions.FirexRun {

		// TODO: figure out firex requirements

	} else {
		return nil, errors.New("invalid options")
	}

	db := mongoClient.Database(databaseName)
	collection := db.Collection(collectionName)

	return collection, nil

}

// validateDbSchema checks that the necessary fields in the PerformanceData struct are populated correctly.
func validateDbSchema(data PerformanceData) error {
	if data.Timestamp == "" {
		return fmt.Errorf("missing Timestamp in PerformanceData")
	}
	if data.PartNo == "" {
		return fmt.Errorf("missing PartNo in PerformanceData")
	}
	if data.SoftwareRelease == "" {
		return fmt.Errorf("missing SoftwareRelease in PerformanceData")
	}
	if data.SoftwareImage == "" {
		return fmt.Errorf("missing SoftwareImage in PerformanceData")
	}
	if data.Name == "" {
		return fmt.Errorf("missing Name in PerformanceData")
	}
	if data.SerialNo == "" {
		return fmt.Errorf("missing SerialNo in PerformanceData")
	}
	if data.Location == "" {
		return fmt.Errorf("missing Location in PerformanceData")
	}
	if data.Chassis == "" {
		return fmt.Errorf("missing Chassis in PerformanceData")
	}
	if data.Type == "" {
		return fmt.Errorf("missing Type in PerformanceData")
	}
	if data.Feature == "" {
		return fmt.Errorf("missing Feature in PerformanceData")
	}
	if data.Trigger == "" {
		return fmt.Errorf("missing Trigger in PerformanceData")
	}
	if data.ScaleValues == nil {
		return fmt.Errorf("missing ScaleValues in PerformanceData")
	}
	if data.MemoryUsed == 0 && data.MemoryFree == 0 && data.CpuUser == 0 && data.CpuKernel == 0 {
		return fmt.Errorf("all values in memory and cpu are 0 should not accept this")
	}

	// Validate ProcessData
	for i, process := range data.ProcessData {
		if process.ProcessName == "" {
			return fmt.Errorf("ProcessName should not be an empty string %v", i)
		}
		if process.ProcessMem == 0 && process.ProcessCpu == 0 {
			return fmt.Errorf("all values in ProcessData at index %d are 0, should not accept this", i)
		}
	}

	// Validate ScaleAttributes
	if data.ScaleValues == nil {
		return fmt.Errorf("missing ScaleValues")
	}

	return nil
}

func (c *Collector) pushToDB(t *testing.T, results []PerformanceData) (error, bool) {
	var uploadToDb bool
	var schemaErr error

	// Validate each data entry
	for _, data := range results {
		if err := validateDbSchema(data); err != nil {
			uploadToDb = false
			schemaErr = err
			t.Logf("Validation failed for PerformanceData: %v", err)
			return schemaErr, uploadToDb
		} else {
			uploadToDb = true
		}
	}

	// Connect to MongoDB
	collection, errDB := ConnectToMongo(t, c.flagOptions)
	if errDB != nil {
		return errDB, false
	}

	// Prepare data for insertion
	wrappedData := struct {
		TimeSeries []PerformanceData `bson:"timeseries" json:"timeseries"`
	}{
		TimeSeries: results,
	}

	if uploadToDb {
		if c.objectID.IsZero() {
			// Create a new top-level document with the first timeseriesdata array
			timeseriesData := []interface{}{wrappedData}
			document := bson.M{"timeseriesdata": timeseriesData}
			result, err := collection.InsertOne(context.Background(), document)
			if err != nil {
				return err, uploadToDb
			}
			c.objectID = result.InsertedID.(primitive.ObjectID)
		} else {
			filter := bson.M{"_id": c.objectID}
			update := bson.M{"$push": bson.M{"timeseriesdata": wrappedData}}
			_, err := collection.UpdateOne(context.Background(), filter, update)
			if err != nil {
				return err, uploadToDb
			}
		}
	} else {
		t.Logf("Could not upload to database as there are entry errors: %v", schemaErr)
	}

	return nil, uploadToDb
}

func CollectScaleAttributes(t *testing.T, dut *ondatra.DUTDevice, cliClient binding.CLIClient) (*ScaleAttributes, error) {
	if cliClient == nil {
		return nil, errors.New("CLI client not established")
	}

	// Collect running-config
	runningConfig, err := executeCommand(cliClient, "show running-config")
	if err != nil {
		return nil, err
	}

	// Collect route summary
	routeSummary, err := executeCommand(cliClient, "show route summary")
	if err != nil {
		return nil, err
	}

	// Collect IPv6 route summary
	routeIPv6Summary, err := executeCommand(cliClient, "show route ipv6 summary")
	if err != nil {
		return nil, err
	}

	// Parse running-config
	interfaces, bgpNeighbors := parseRunningConfig(runningConfig)

	// Parse route summary
	totalRoutes := parseRouteSummary(routeSummary)

	// Parse IPv6 route summary
	totalIPv6Routes := parseRouteSummary(routeIPv6Summary)

	// Return collected attributes
	return &ScaleAttributes{
		TotalIPv4Routes:      totalRoutes,
		TotalIPv6Routes:      totalIPv6Routes,
		ConfiguredInterfaces: interfaces,
		BGPNeighbors:         bgpNeighbors,
	}, nil
}

func executeCommand(cliClient binding.CLIClient, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	cliOutput, err := cliClient.RunCommand(ctx, command)
	if err != nil {
		return "", err
	}
	return cliOutput.Output(), nil
}

func parseRunningConfig(config string) (int, int) {
	interfacePattern := regexp.MustCompile(`interface\s+\S+\s*\n\s*(ipv[46]\saddress\s+\S+)`)
	bgpNeighborPattern := regexp.MustCompile(`neighbor\s+\S+`)

	interfaces := interfacePattern.FindAllString(config, -1)
	bgpNeighbors := bgpNeighborPattern.FindAllString(config, -1)

	return len(interfaces), len(bgpNeighbors)
}

func parseRouteSummary(summary string) int {
	totalRoutesPattern := regexp.MustCompile(`Total\s+(\d+)`)
	matches := totalRoutesPattern.FindStringSubmatch(summary)
	if len(matches) < 2 {
		return 0
	}
	totalRoutes, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return totalRoutes
}

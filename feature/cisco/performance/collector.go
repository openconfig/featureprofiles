package performance

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/ssh"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	localRun = flag.Bool("local_run", false, "Used for local run")
	firexRun = flag.Bool("firex_run", false, "Set env variables for firex run")
	noDbRun  = flag.Bool("no_db_run", true, "Don't upload to database")
)

type PerformanceData struct {
	Timestamp             string           `bson:"timestamp" json:"timestamp"`
	PartNo                string           `bson:"partNo" json:"partNo"`
	SoftwareRelease       string           `bson:"softwareVersion" json:"softwareVersion"`
	SoftwareImage         string           `bson:"SoftwareImage" json:"SoftwareImage"`
	Name                  string           `bson:"name" json:"name"`
	SerialNo              string           `bson:"serialNo" json:"serialNo"`
	Location              string           `bson:"location" json:"location"`
	Chassis               string           `bson:"chassis" json:"chassis"`
	Type                  string           `bson:"type" json:"type"`
	Feature               string           `bson:"feature" json:"feature"`
	Trigger               string           `bson:"trigger" json:"trigger"`
	ProcessData           []ProcessData    `bson:"processData" json:"processData"`
	ScaleValues           *ScaleAttributes `bson:"scaleValues"`
	MemoryUsed            float64          `bson:"memoryUsed" json:"memoryUsed"`
	MemoryFree            float64          `bson:"memoryFree" json:"memoryFree"`
	CpuUser               float64          `bson:"cpuUser" json:"cpuUser"`
	CpuKernel             float64          `bson:"cpuKernel" json:"cpuKernel"`
	RedisUsedMem          int64            `bson:"redisUsedMem" json:"redisUsedMem"`
	RedisUsedMemHuman     string           `bson:"redisUsedMemHuman" json:"redisUsedMemHuman"`
	RedisUsedMemPeakHuman string           `bson:"redisUsedMemPeakHuman" json:"redisUsedMemPeakHuman"`
	RedisUsedMemPeakPerc  string           `bson:"redisUsedMemPeakPerc" json:"redisUsedMemPeakPerc"`
	RedisTotalMem         int64            `bson:"redisTotalMem" json:"redisTotalMem"`
	RedisTotalMemHuman    string           `bson:"redisTotalMemHuman" json:"redisTotalMemHuman"`
	RedisUsedMemAsPct     float64          `bson:"redisUsedMemAsPct" json:"redisUsedMemAsPct"`
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

type metadata struct {
	sys      *oc.System
	platform []*oc.Component
	scale    *ScaleAttributes
	chassis  string
}

type Collector struct {
	metadata
	EndCollector func() ([]PerformanceData, error)
	pauseWg      *sync.WaitGroup
}

// Checks cache to see if dut already has had metadata gathered. If not then it will gather it.
func (c *Collector) getMetadata(t *testing.T, dut *ondatra.DUTDevice) (*metadata, error) {

	cliClient := dut.RawAPIs().CLI(t)
	scale, err := CollectScaleAttributes(t, dut, cliClient)
	if err != nil {
		return nil, err
	}

	c.scale = scale
	c.sys = gnmi.Get(t, dut, gnmi.OC().System().State())
	c.platform = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())

	chassisCmdResult, err := cliClient.RunCommand(context.Background(), "show inventory chassis")
	if err != nil {
		return nil, fmt.Errorf("failed to run command: %v", err)
	}

	// Extract the output as a string
	chassisStrOutput := chassisCmdResult.Output()
	chassisPattern := regexp.MustCompile(`PID:\s+(\d+)`)
	chassisMatch := chassisPattern.FindStringSubmatch(chassisStrOutput)
	if len(chassisMatch) < 2 {
		return nil, fmt.Errorf("failed to find chassis PID in the output")
	}
	c.chassis = chassisMatch[1]

	return &c.metadata, nil

}

func (c *Collector) PauseCollector() {
	c.pauseWg.Add(1)
}

func (c *Collector) ResumeCollector() {
	c.pauseWg.Done()
}

// RunCollector starts the collector
func RunCollector(t *testing.T, dut *ondatra.DUTDevice, featureName string, triggerName string, frequency time.Duration) (*Collector, error) {
	t.Logf("Collector starting at %s", time.Now().UTC().Format(time.RFC3339))

	c := &Collector{}
	c.pauseWg = &sync.WaitGroup{}

	m, err := c.getMetadata(t, dut)
	if err != nil {
		return c, err
	}

	var dataEntries []PerformanceData

	imageStr := m.sys.GetSoftwareVersion()
	imagePattern := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	release := imagePattern.FindString(imageStr)

	for _, component := range m.platform {
		data := PerformanceData{
			SoftwareRelease: release,
			SoftwareImage:   imageStr,
			Feature:         featureName,
			Trigger:         triggerName,
			ScaleValues:     m.scale,
			Chassis:         m.chassis, // Use the collected chassis information
			PartNo:          component.GetPartNo(),
			Name:            component.GetName(),
			Location:        component.GetLocation(),
			SerialNo:        component.GetSerialNo(),
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		}

		switch component.GetType() {
		case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD:
			if component.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				if component.RedundantRole == oc.Platform_ComponentRedundantRole_SECONDARY {
					continue
				}
				data.Type = "RP"

				// Collect Redis memory information only for RP
				cliClient := dut.RawAPIs().CLI(t)
				redisData, err := CollectRedisMemoryInfo(cliClient, data)
				if err != nil {
					t.Fatalf("Failed to collect Redis memory information: %v", err)
				}
				data = *redisData

			}
		case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD:
			data.Type = "LC"

			// Set default values for Redis fields for LC
			data.RedisUsedMem = 0
			data.RedisUsedMemHuman = "none"
			data.RedisUsedMemPeakHuman = "none"
			data.RedisUsedMemPeakPerc = "none"
			data.RedisTotalMem = 0
			data.RedisTotalMemHuman = "none"
			data.RedisUsedMemAsPct = 0
		default:
			continue
		}

		dataEntries = append(dataEntries, data)
		t.Logf("Image: %+v", data)
	}

	for _, entry := range dataEntries {
		fmt.Printf("data entry: %+v\n", entry)
	}

	completedChan := make(chan bool, 1)

	resultChan := c.collectAllData(t, dut, completedChan, dataEntries, frequency)

	var results []PerformanceData

	c.EndCollector = func() ([]PerformanceData, error) {
		t.Logf("Collector finished at %s", time.Now())
		close(completedChan)
		results = <-resultChan

		if results == nil {
			return nil, fmt.Errorf("no data collected")
		}

		if *noDbRun {
			t.Logf("Not pushing results to database as -noDBRun is set to %v", *noDbRun)
		} else {
			err, dbBool := pushToDB(t, results)
			if err != nil && !dbBool {
				return results, err
			}
		}

		return results, nil
	}

	return c, nil
}

func (c *Collector) collectAllData(t *testing.T, dut *ondatra.DUTDevice, doneChan chan bool, perfData []PerformanceData, frequency time.Duration) chan []PerformanceData {
	wg := &sync.WaitGroup{}
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()
	resultChan := make(chan []PerformanceData)
	results := []PerformanceData{}
	mu := &sync.Mutex{}
	doneChannelsMultiplexed := []chan bool{}

	for _, data := range perfData {
		wg.Add(1)

		doneChanCurrent := make(chan bool, 1)
		doneChannelsMultiplexed = append(doneChannelsMultiplexed, doneChanCurrent)

		go func(dataCurrent PerformanceData, doneChanCurrent chan bool) {
			defer wg.Done()
			t.Logf("Starting collection for component: %s", dataCurrent.Name)
			cliClient := dut.RawAPIs().CLI(t)
			t.Logf("Established client for component: %s", dataCurrent.Name)
			ticker := time.NewTicker(frequency)
			done := false

			for !done {
				c.pauseWg.Wait()
				select {
				case <-doneChanCurrent:
					done = true
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
		}(data, doneChanCurrent)
	}

	// forwards channel closure from parent thread to each thread spawned per component
	go func() {
		<-doneChan
		for _, channel := range doneChannelsMultiplexed {
			close(channel)
		}
	}()

	// waits for results
	go func() {
		wg.Wait()
		resultChan <- results
	}()

	return resultChan

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
func ConnectToMongo(t *testing.T) (*mongo.Collection, error) {

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

	if *localRun {
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

	} else if *firexRun {

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

func pushToDB(t *testing.T, results []PerformanceData) (error, bool) {
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
	collection, errDB := ConnectToMongo(t)
	if errDB != nil {
		t.Logf("Failed to connect to MongoDB: %v", errDB)
		return errDB, false
	}

	// Prepare data for insertion
	wrappedData := struct {
		TimeSeries []PerformanceData `bson:"timeseries" json:"timeseries"`
	}{
		TimeSeries: results,
	}

	if uploadToDb {
		// Create a new top-level document with the first timeseriesdata array
		timeseriesData := []interface{}{wrappedData}
		document := bson.M{"timeseriesdata": timeseriesData}
		_, err := collection.InsertOne(context.Background(), document)
		if err != nil {
			t.Logf("Failed to insert document into MongoDB: %v", err)
			return err, uploadToDb
		}
		// objectID = result.InsertedID.(primitive.ObjectID)
	} else {
		t.Logf("Could not upload to database as there are entry errors: %v", schemaErr)
	}

	return nil, uploadToDb
}

func CollectRedisMemoryInfo(cliClient binding.CLIClient, data PerformanceData) (*PerformanceData, error) {

	if cliClient == nil {
		return nil, errors.New("CLI client not established")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	cliOutput, err := cliClient.RunCommand(ctx, "run redis-cli INFO MEMORY")

	if err != nil {
		return nil, err
	}

	lines := strings.Split(cliOutput.Output(), "\n")
	redisMemRe := regexp.MustCompile(`used_memory:(\d+)`)
	redisMemHumanRe := regexp.MustCompile(`used_memory_human:(\S+)`)
	redisMemPeakHumanRe := regexp.MustCompile(`used_memory_peak_human:(\S+)`)
	redisMemPeakPercRe := regexp.MustCompile(`used_memory_peak_perc:(\S+)`)
	totalMemRe := regexp.MustCompile(`total_system_memory:(\d+)`)
	totalMemHumanRe := regexp.MustCompile(`total_system_memory_human:(\S+)`)

	var usedMem, totalMem int64
	var usedMemFound, totalMemFound bool

	for _, line := range lines {
		if redisMemMatches := redisMemRe.FindStringSubmatch(line); len(redisMemMatches) > 1 {
			var err error
			usedMem, err = strconv.ParseInt(redisMemMatches[1], 10, 64)
			if err != nil {
				return nil, err
			}
			data.RedisUsedMem = usedMem
			usedMemFound = true
		}
		if redisMemHumanMatches := redisMemHumanRe.FindStringSubmatch(line); len(redisMemHumanMatches) > 1 {
			data.RedisUsedMemHuman = redisMemHumanMatches[1]
		}
		if redisMemPeakHumanMatches := redisMemPeakHumanRe.FindStringSubmatch(line); len(redisMemPeakHumanMatches) > 1 {
			data.RedisUsedMemPeakHuman = redisMemPeakHumanMatches[1]
		}
		if redisMemPeakPercMatches := redisMemPeakPercRe.FindStringSubmatch(line); len(redisMemPeakPercMatches) > 1 {
			data.RedisUsedMemPeakPerc = redisMemPeakPercMatches[1]
		}
		if totalMemMatches := totalMemRe.FindStringSubmatch(line); len(totalMemMatches) > 1 {
			var err error
			totalMem, err = strconv.ParseInt(totalMemMatches[1], 10, 64)
			if err != nil {
				return nil, err
			}
			data.RedisTotalMem = totalMem
			totalMemFound = true
		}
		if totalMemHumanMatches := totalMemHumanRe.FindStringSubmatch(line); len(totalMemHumanMatches) > 1 {
			data.RedisTotalMemHuman = totalMemHumanMatches[1]
		}
	}

	// Calculate the percentage of memory used by Redis
	if usedMemFound && totalMemFound {
		usedMemAsPct := float64(usedMem) / float64(totalMem) * 100
		data.RedisUsedMemAsPct = usedMemAsPct
	} else {
		data.RedisUsedMemAsPct = 0.0
	}

	return &data, nil
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

func AveragePerformanceData(data []PerformanceData) map[string]map[string]float64 {
	aggregatedData := make(map[string]map[string]float64)

	for _, entry := range data {
		location := entry.Location

		if _, exists := aggregatedData[location]; !exists {
			aggregatedData[location] = map[string]float64{
				"AvgRedisUsedMemAsPct": 0,
				"AvgMemoryUsedPct":     0,
				"AvgCpuUser":           0,
				"AvgCpuKernel":         0,
				"RedisUsedMemPeak":     0,
				"MemoryUsedPeak":       0,
				"CpuUserPeak":          0,
				"CpuKernelPeak":        0,
				"count":                0,
			}
		}

		// Calculate memory usage percentage
		memoryUsedPct := (entry.MemoryUsed / (entry.MemoryUsed + entry.MemoryFree)) * 100

		// Aggregate the values
		aggregatedData[location]["AvgRedisUsedMemAsPct"] += entry.RedisUsedMemAsPct
		aggregatedData[location]["AvgMemoryUsedPct"] += memoryUsedPct
		aggregatedData[location]["AvgCpuUser"] += entry.CpuUser
		aggregatedData[location]["AvgCpuKernel"] += entry.CpuKernel

		// Update peak values
		if entry.RedisUsedMemAsPct > aggregatedData[location]["RedisUsedMemPeak"] {
			aggregatedData[location]["RedisUsedMemPeak"] = entry.RedisUsedMemAsPct
		}
		if memoryUsedPct > aggregatedData[location]["MemoryUsedPeak"] {
			aggregatedData[location]["MemoryUsedPeak"] = memoryUsedPct
		}
		if entry.CpuUser > aggregatedData[location]["CpuUserPeak"] {
			aggregatedData[location]["CpuUserPeak"] = entry.CpuUser
		}
		if entry.CpuKernel > aggregatedData[location]["CpuKernelPeak"] {
			aggregatedData[location]["CpuKernelPeak"] = entry.CpuKernel
		}

		aggregatedData[location]["count"]++
	}

	// Calculate the average for each location
	for location, metrics := range aggregatedData {
		count := metrics["count"]
		aggregatedData[location]["AvgRedisUsedMemAsPct"] /= count
		aggregatedData[location]["AvgMemoryUsedPct"] /= count
		aggregatedData[location]["AvgCpuUser"] /= count
		aggregatedData[location]["AvgCpuKernel"] /= count

		// Remove count from the final output if not needed
		delete(aggregatedData[location], "count")
	}

	return aggregatedData
}

func ComparePerformanceData(baseline, trigger map[string]map[string]float64) map[string]map[string]map[string]float64 {
	compareData := make(map[string]map[string]map[string]float64)

	for location, baselineMetrics := range baseline {
		if _, exists := trigger[location]; !exists {
			continue
		}

		triggerMetrics := trigger[location]
		compareData[location] = map[string]map[string]float64{
			"RedisUsedMemAsPct": {
				"Baseline": baselineMetrics["RedisUsedMemAsPct"],
				"Trigger":  triggerMetrics["RedisUsedMemAsPct"],
				"Diff":     triggerMetrics["RedisUsedMemAsPct"] - baselineMetrics["RedisUsedMemAsPct"],
			},
			"MemoryUsedPct": {
				"Baseline": baselineMetrics["MemoryUsedPct"],
				"Trigger":  triggerMetrics["MemoryUsedPct"],
				"Diff":     triggerMetrics["MemoryUsedPct"] - baselineMetrics["MemoryUsedPct"],
			},
			"CpuUser": {
				"Baseline": baselineMetrics["CpuUser"],
				"Trigger":  triggerMetrics["CpuUser"],
				"Diff":     triggerMetrics["CpuUser"] - baselineMetrics["CpuUser"],
			},
			"CpuKernel": {
				"Baseline": baselineMetrics["CpuKernel"],
				"Trigger":  triggerMetrics["CpuKernel"],
				"Diff":     triggerMetrics["CpuKernel"] - baselineMetrics["CpuKernel"],
			},
			"RedisUsedMemPeak": {
				"Baseline": baselineMetrics["RedisUsedMemPeak"],
				"Trigger":  triggerMetrics["RedisUsedMemPeak"],
				"Diff":     triggerMetrics["RedisUsedMemPeak"] - baselineMetrics["RedisUsedMemPeak"],
			},
			"MemoryUsedPeak": {
				"Baseline": baselineMetrics["MemoryUsedPeak"],
				"Trigger":  triggerMetrics["MemoryUsedPeak"],
				"Diff":     triggerMetrics["MemoryUsedPeak"] - baselineMetrics["MemoryUsedPeak"],
			},
			"CpuUserPeak": {
				"Baseline": baselineMetrics["CpuUserPeak"],
				"Trigger":  triggerMetrics["CpuUserPeak"],
				"Diff":     triggerMetrics["CpuUserPeak"] - baselineMetrics["CpuUserPeak"],
			},
			"CpuKernelPeak": {
				"Baseline": baselineMetrics["CpuKernelPeak"],
				"Trigger":  triggerMetrics["CpuKernelPeak"],
				"Diff":     triggerMetrics["CpuKernelPeak"] - baselineMetrics["CpuKernelPeak"],
			},
		}
	}

	return compareData
}

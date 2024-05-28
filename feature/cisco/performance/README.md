# Performance Data Collector

## Overview

This project contains a performance data collector implemented in `collector.go`, which is used to gather various metrics from a DUT (Device Under Test) and store them in a MongoDB database. The `flags.go` file is used for managing command-line flags to configure the collector's behavior.

### collector.go

The `collector.go` file defines the following key components and functions:

- **PerformanceData**: Struct that holds the collected performance data such as memory usage, CPU usage, scale attributes, and other metadata.

- **Collector**: Struct that manages the collection process, including maintaining state, managing CLI clients, and storing results.

- **getClient**: Function to get or create a CLI client for a given component, with retry logic.

- **RunCollector**: Function to start the collector, initialize necessary data, and spawn a goroutine to collect data periodically.

- **StopCollector**: Function to stop the collector, signal all goroutines to finish, and optionally push the collected data to a database.

- **collectAllData**: Goroutine function to periodically collect data from the DUT and store it in the collector's results.

- **ConnectToMongo**: Function to connect to a MongoDB database, sourcing necessary environment variables from a remote machine via SSH if needed.

- **pushToDB**: Function to push the collected data to the MongoDB database after validating the data schema.

### flags.go

The `flags.go` file handles the command-line flags used to configure the collector's behavior. It defines the `FlagOptions` struct and initializes the flags:

- **FlagOptions**: Struct that holds the flag values (`LocalRun`, `FirexRun`, `NoDBRun`).

- **init**: Function to define and initialize the command-line flags.

## Usage

### Running the Collector

To use the collector in your feature script, you need to initialize and start it using the provided functions. Here is an example of how to do this:

1. **Import the necessary packages**:

    ```go
    import (
        "testing"
        "time"
        "github.com/openconfig/featureprofiles/feature/cisco/performance"
        "github.com/openconfig/featureprofiles/feature/cisco/performance/flagUtils"
    )
    ```

2. **Initialize the flag options**:

    ```go
    flagOptions := flagUtils.ParseFlags()
    ```

3. **Start the collector**:

    ```go
    t.Run("Start Collector", func(t *testing.T) {
        err := performance.RunCollector(t, dut, "Feature-Name", "Trigger-Name", time.Second*30, flagOptions)
        if err != nil {
            t.Fatalf("Failed to start collector: %v", err)
        }
        t.Log("Starting sleep for baseline collection")
        time.Sleep(time.Minute * 5)
    })
    ```

4. **Stop the collector and handle the results**:

    ```go
    t.Run("Stop Collector", func(t *testing.T) {
        results, err := performance.StopCollector(t)
        if err != nil {
            t.Fatalf("Failed to stop collector: %v", err)
        }
        t.Logf("Collected results: %+v", results)
    })
    ```

### flags.go

The `flags.go` file manages the command-line flags for configuring the collector. The available flags are:

- `local_run`: Used for local runs.
- `firex_run`: Set environment variables for Firex runs.
- `no_db_run`: Don't upload to the database if set to true.

To define and parse these flags, simply import the `flagUtils` package and call the `ParseFlags` function as shown in the usage example above.
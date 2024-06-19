# Performance Data Collector

## Overview

This project contains a performance data collector implemented in `collector.go`, which is used to gather various metrics from a DUT (Device Under Test) and store them in a MongoDB database. The `flags.go` file is used for managing command-line flags to configure the collector's behavior.

### collector.go

The `collector.go` file defines the following key components and functions:

- **PerformanceData**: Struct that holds the collected performance data such as memory usage, CPU usage, scale attributes, and other metadata.

- **Collector**: Reference object that manages the collection process.

- **getClient**: Function to get or create a CLI client for a given component, with retry logic.

- **RunCollector**: Function to start the collector, initialize necessary data, and spawn a goroutine to collect data periodically.

- **EndCollector**: Function to stop the collector, signal all goroutines to finish, and optionally push the collected data to a database.

- **PauseCollector: Function to temporarily halt collection until ResumeCollector is run.

- **ResumeCollector: Function to resume collection after it has been paused by PauseCollector.

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
    )
    ```

2. **Simple usage of the collector**:
    
    ```go
    t.Run("Example Collection to Database", func(t *testing.T) {
        collector, err := performance.RunCollector(t, dut, "Feature-Name", "Trigger-Name", time.Second*30)
        if err != nil {
            t.Fatalf("Failed to start collector: %v", err)
        }
        
        defer collector.EndCollector()
        
        // Collect for two minutes.
        time.Sleep(time.Minute * 2)
    })
    ```

2. **Fine-grained control of collector: (Start/End and Pause/Resume)**:

    ```go
    t.Run("Example Collection", func(t *testing.T) {
        collector, err := performance.RunCollector(t, dut, "Feature-Name", "Trigger-Name", time.Second*30)
        if err != nil {
            t.Fatalf("Failed to start collector: %v", err)
        }
        // Collect for two minutes.
        time.Sleep(time.Minute * 2)
        
        // Pause for 10 seconds.
        collector.PauseCollector()
        time.Sleep(time.Second * 10)

        // Resume collection.
        collector.ResumeCollector()

        // Access results in a variable if necessary
        performanceData, err := collector.EndCollector()
        
    })
    ```

### Run flags

The available flags are:

- `local_run`: Used for local runs. (Default: false)
- `firex_run`: Set environment variables for Firex runs. (Default: false)
- `no_db_run`: Don't upload to the database if set to true. (Default: true)

Overriding the defaults can be done by passing these arguments on the command line with the `go test` command.
e.g. `go test -run TestCollector -local_run=true -binding=<binding_file> -testbed=<testbed_file>`

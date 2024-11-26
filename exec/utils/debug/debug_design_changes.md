# Collect Debug Files

This document captures the changes made through [PR #1257](https://wwwin-github.cisco.com/B4Test/featureprofiles/pull/1257
)

## Improvements made

- Quicker Debug Log Collection:

    The old implementation takes approximately 55 minutes to collect logs, causing Firex to time out after 30 minutes. Consequently, debug file collections are incomplete. The new implementation overcomes this by processing debug collection in parallel using goroutines.


- Core File Decode:

    With the new implementation, core files are decoded automatically, and the results are placed in the Firex debug folder. This will help save manual time in triaging issues.

## Bechmark Data

Bechmark Data for command execution in parllel go routines

![Benchmark data](benchmark_data.png)

## Major Design changes

- Process DUTs Simultaneously:

    Instead of sequentially collecting debug files, the new design collects them in parallel.

- Execute Commands Simultaneously in Each DUT:

    Based on benchmark data, run 4 commands simultaneously in each device. Four is a safe number that does not affect the DUT ssh performance.

- Decode the Core Files in the Background:

    This will not take additional time as it runs in a background shell process on the Firex worker machine. A file with the extension `*.in_progress` is placed in the debug folder, indicating that the process is running in the background. Once the decode is completed, this `*.in_progress` file will be removed, and a `*.decoded.txt` file will have the decoded output.

## Old Design (Sequential)

```mermaid
graph TD
    A[Init and Construct Commands] --> B[Run DUT1]

    subgraph Sequential Execution of Commands for DUT1
        B --> C1[Execute cmd1]
        C1 --> C2[Execute cmd2]
        C2 --> C3[Execute cmd3]
        C3 -.-> C4[Execute cmds]
        C4 -.-> D[Cmd Completed]
    end

    D --> E[Copy Files]
    E --> F[Detect Core File]
    F -.-> G[Process DUT2 Similarly]

    subgraph Sequential Execution of Commands for DUT2
        G
    end
    G -.-> H[Process DUT3 Similarly]

    subgraph Sequential Execution of Commands for DUT3
        H
    end
    H --> I[End]
```

## New Design (Parllel)

```mermaid
graph TD
    A[Init and Construct Commands] --> B[Run Parallel in DUTs]

    subgraph Parallel Execution in DUTs
        B -->|Parallel| C1[DUT1]
        B -->|Parallel| C2[DUT2]
    end

    subgraph Parallel Execution of Commands for DUT1 max 4 goroutines
        C1 -->|cmd1| D1[Execute cmd1]
        C1 -->|cmd2..| D2[Execute cmd .. ..]
        C1 -->|cmd4| D3[Execute ..cmd4]

        D1 --> F[Cmd Completed]
        D2 --> F[Cmd Completed]
        D3 --> F[Cmd Completed]
    end

    F --> G[Copied Debug Files]
    G --> H{Detect Core File?}

    H -->|Found| K[Processing files]
    H -->|Not Found| J[End]

    K -->|Parallel| I1[Decode File 1 in BG]
    K -->|Parallel| I2[Decode File 2 in BG]
    K -->|Parallel| I3[Decode File ..4 in BG]
    I1 --> J
    I2 --> J
    I3 --> J
    subgraph Parallel Decoding
        K
        I1 
        I2 
        I3 
    end

    C2 -.-> |similar to DUT1| J


```
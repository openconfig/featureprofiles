# tblock - Testbed Reservation System

A command-line tool for managing testbed reservations in B4 environments.

## Quick Start

**For most users**, use the default binary which is pre-configured with the latest testbeds file and common lock directory:

```bash
/auto/tftpboot-ottawa/b4/bin/tblock <command> [options]
```

This is the recommended way to use tblock for regular usage.

## Usage

If you need to use a custom testbeds file or lock directory, you can invoke the script directly:

```bash
python tblock.py <testbeds_file> <locks_dir> [options] <command>
```

### Required Arguments

- `testbeds_file` - Path to the YAML file containing testbed definitions
- `locks_dir` - Directory where lock files are stored

### Global Options

- `-j, --json` - Output results in JSON format instead of table format

## Commands

### show

Display all testbeds with their reservation status.

```bash
python tblock.py testbeds.yaml /path/to/locks show
```

**Options:**
- `-a, --available` - Show only available (unlocked) testbeds

**Output columns:**
- Testbed - Testbed identifier
- Owner - Testbed owner
- Reserved By - Username of current reservation holder
- Reason - Reason provided for the reservation

**Example:**
```bash
# Show all testbeds
tblock show

# Or using the full path
/auto/tftpboot-ottawa/b4/bin/tblock show

# Show only available testbeds
tblock show --available

# JSON output
tblock show --json

# Using custom testbeds file
python tblock.py testbeds.yaml /tmp/locks show
```

### lock

Reserve one or more testbeds.

```bash
python tblock.py testbeds.yaml /path/to/locks lock <testbed_id> [options]
```

**Required:**
- `testbed_id` - Testbed ID to lock (supports comma-separated list for multiple testbeds)

**Options:**
- `-w, --wait` - Wait until testbed(s) become available (blocks until successful)
- `-b, --best-effort` - Lock as many testbeds as possible from the list (partial success allowed)
- `-r, --reason REASON` - Provide a reason for the reservation
- `-u, --user USERNAME` - Specify requestor username (defaults to current user)

**Behavior:**
- By default, locks all requested testbeds or fails entirely (atomic operation)
- With `--wait`, continuously retries until all testbeds are locked
- With `--best-effort`, locks whatever testbeds are available
- `--wait` and `--best-effort` are mutually exclusive

**Examples:**
```bash
# Lock a single testbed
tblock lock testbed1

# Lock with reason
tblock lock testbed1 -r "Running integration tests"

# Lock multiple testbeds
tblock lock testbed1,testbed2

# Wait until testbed is available
tblock lock testbed1 --wait

# Best effort lock (partial success)
tblock lock testbed1,testbed2,testbed3 --best-effort

# Lock as different user
tblock lock testbed1 -u john_doe -r "CI pipeline"

# JSON output
tblock lock testbed1 --json

# Using custom testbeds file
python tblock.py testbeds.yaml /tmp/locks lock testbed1
```

### release

Release a reserved testbed.

```bash
python tblock.py testbeds.yaml /path/to/locks release <testbed_id>
```

**Required:**
- `testbed_id` - Testbed ID to release (supports comma-separated list for multiple testbeds)

**Examples:**
```bash
# Release a single testbed
tblock release testbed1

# Release multiple testbeds
tblock release testbed1,testbed2

# JSON output
tblock release testbed1 --json

# Using custom testbeds file
python tblock.py testbeds.yaml /tmp/locks release testbed1
```

## Testbed Configuration

Testbeds are defined in a YAML file with the following structure:

```yaml
testbeds:
  testbed1:
    owner: "user@example.com"
    hw: physical_device_1
  testbed2:
    owner: "user@example.com"
    hw: [physical_device_2, physical_device_3]  # Multiple hardware resources
  sim_testbed:
    owner: "user@example.com"
    sim: true  # Simulation testbed (not subject to locking)
```

**Fields:**
- `owner` - Email or identifier of testbed owner
- `hw` - Hardware identifier(s); can be string or list
- `sim` - Boolean flag indicating simulation testbed (optional, defaults to false)

## Access Control

User access is controlled via a `users.txt` file in the same directory as tblock.py. Only users listed in this file can lock/release testbeds. The file should contain one username per line.

## Lock Files

Lock files are created in the specified locks directory with the hardware ID as the filename. Each lock file contains:
```
<username>,<reason>
```

## Logging

Activity logs are stored in `<locks_dir>/logs.txt` with automatic rotation (max 100MB, 1 backup). Logs include timestamps and record all lock/release operations.

## JSON Output Format

When using `--json` flag, output is structured as follows:

**show command:**
```json
{
  "status": "ok",
  "testbeds": [
    {
      "id": "testbed1",
      "owner": "user@example.com",
      "reserved_by": "john_doe",
      "reason": "Running tests"
    }
  ]
}
```

**lock command (success):**
```json
{
  "status": "ok",
  "testbeds": [...]
}
```

**lock command (failure):**
```json
{
  "status": "fail"
}
```

**release command:**
```json
{
  "status": "ok"
}
```

## Exit Codes

- `0` - Success
- `1` - Failure (testbed not found, lock failed, etc.)

## Notes

- Simulation testbeds (`sim: true`) are not subject to locking and are excluded from `show` output
- Lock operations are atomic by default - all requested testbeds must be available or none are locked
- When locking testbeds with multiple hardware resources, all resources must be available
- Locks persist until explicitly released or the lock file is manually removed

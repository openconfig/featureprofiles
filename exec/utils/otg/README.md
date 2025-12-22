# OTG Container Management

Tool for managing Open Traffic Generator (OTG) containers for testbeds.

## Quick Start

**Most users should use the pre-compiled binary:**
```bash
/auto/tftpboot-ottawa/b4/bin/otgm
```

For development or custom builds, use `manage.py` directly.

## Commands

### start
Start OTG container for a testbed.

```bash
python manage.py start <testbed_id> [options]
```

**Options:**
- `--vxr_out <path>` - Path to vxr.out directory (for sim testbeds)
- `--image <path>` - Path to XR image (for sim testbeds)
- `--topo <path>` - Path to sim topology file
- `--controller <version>` - Docker image version for controller (default: 1.3.0-2)
- `--layer23 <version>` - Docker image version for layer23 (default: 1.3.0-4)
- `--gnmi <version>` - Docker image version for gnmi (default: 1.13.15)
- `--controller_command [args]` - Additional command line arguments for controller (e.g., `--controller_command=[--grpc-max-msg-size 500]`)

**Examples:**
```bash
# Start with default versions
python manage.py start my_testbed

# Start with custom versions
python manage.py start my_testbed --controller=1.20.0-6 --layer23=1.20.0-1 --gnmi=1.20.2

# Start sim testbed
python manage.py start my_sim_testbed --image=/path/to/xr.iso

# Start with controller options
python manage.py start my_testbed --controller_command=[--grpc-max-msg-size 500]
```

### stop
Stop OTG container for a testbed.

```bash
python manage.py stop <testbed_id>
```

### restart
Restart OTG container for a testbed.

```bash
python manage.py restart <testbed_id> [options]
```

Accepts same options as `start` command.

### bindings
Generate Ondatra bindings for a testbed.

```bash
python manage.py bindings <testbed_id> [--out_dir <path>]
```

**Options:**
- `--out_dir <path>` - Output directory (default: `<testbed_id>_bindings`)

**Generates:**
- `otg.binding` - OTG binding file
- `ate.binding` - ATE binding file
- `dut.testbed` - DUT testbed file
- `*.baseconf` - Base configuration files for each DUT
- `setup.sh` - Environment setup script

**Usage:**
```bash
# Generate bindings
python manage.py bindings my_testbed

# Use generated bindings
source my_testbed_bindings/setup.sh
```

The setup script sets:
- `TESTBED_ID` - Testbed identifier
- `TESTBED` - Path to testbed file
- `ATE_BINDING` - Path to ATE binding file
- `OTG_BINDING` - Path to OTG binding file

### logs
Collect OTG container logs.

```bash
python manage.py logs <testbed_id> <out_dir>
```

Collects logs from all OTG containers (controller, layer23, gnmi-server) into the specified directory.

## Configuration

### Testbeds File
Testbeds are defined in `exec/testbeds.yaml`. Each testbed specifies:
- `testbed` - Path to testbed file
- `binding` - Path to binding file
- `baseconf` - Base configuration file(s) for DUT(s)
- `otg` - OTG connection information (host, ports)
- `sim` - Whether testbed is simulated (optional)

### mTLS Certificates
Default certificate paths:
- Trust bundle: `internal/cisco/security/cert/keys/CA/ca.cert.pem`
- Certificate: `internal/cisco/security/cert/keys/clients/cafyauto.cert.pem`
- Key: `internal/cisco/security/cert/keys/clients/cafyauto.key.pem`

## Simulator Support

For simulated testbeds, the tool automatically:
- Starts VXR simulation if `--vxr_out` not provided
- Configures port redirections
- Generates dynamic bindings based on sim topology
- Injects base configurations

**Sim-specific workflow:**
```bash
# Start sim with auto-bringup
python manage.py start my_sim_testbed --image=/path/to/xr.iso

# Or use existing sim
python manage.py start my_sim_testbed --vxr_out=/path/to/vxr.out
```

## Environment Variables

- `FP_REPO_DIR` - Feature profiles repository directory (default: current directory)

## Docker Images

The tool uses these Docker images:
- **Controller:** `ghcr.io/open-traffic-generator/keng-controller`
- **Layer23:** `ghcr.io/open-traffic-generator/keng-layer23-hw-server`
- **gNMI:** `ghcr.io/open-traffic-generator/otg-gnmi-server`

Version format: `x.y.z` or `x.y.z-n`

## Generated Files

When starting a testbed or generating bindings, the tool creates:
- `start_otg.sh` - Script to start OTG containers
- `stop_otg.sh` - Script to stop OTG containers
- Docker compose configuration (temporary)

## Notes

- Requires `go` binary in PATH or at `/auto/firex/bin/go`
- Uses `docker-compose` on remote OTG hosts
- Supports SSH with password authentication via `sshpass`
- Log rotation configured (100MB max size, 10 files)

#!/usr/bin/env python3
"""
Standalone script to generate Docker Compose files for OTG (Open Traffic Generator) simulation.

This script generates docker-compose.yml files for both hardware and simulation setups.
"""

import argparse
import sys
import os

# Docker image constants
DOCKER_KENG_CONTROLLER = 'ghcr.io/open-traffic-generator/keng-controller'
DOCKER_KENG_LAYER23 = 'ghcr.io/open-traffic-generator/keng-layer23-hw-server'
DOCKER_OTG_GNMI = 'ghcr.io/open-traffic-generator/otg-gnmi-server'
DOCKER_TRAFFIC_ENGINE = 'ghcr.io/open-traffic-generator/ixia-c-traffic-engine'
DOCKER_PROTOCOL_ENGINE = 'ghcr.io/open-traffic-generator/ixia-c-protocol-engine'


def generate_hardware_template(control_port, gnmi_port, rest_port, 
                                controller_version='1.3.0-2',
                                layer23_version='1.3.0-4', 
                                gnmi_version='1.13.15',
                                controller_commands=None):
    """
    Generate Docker Compose template for hardware-based OTG setup.
    
    Args:
        control_port: Controller port number
        gnmi_port: gNMI server port number
        rest_port: REST API port number
        controller_version: Version of the Keng controller image
        layer23_version: Version of the Layer23 hardware server image
        gnmi_version: Version of the gNMI server image
        controller_commands: Additional commands for the controller (list of strings)
    
    Returns:
        Docker compose file content as string
    """
    # Format controller commands if provided
    controller_command_formatted = ""
    if controller_commands:
        for cmd in controller_commands:
            controller_command_formatted += f'\n      - "{cmd}"'
    
    docker_file = f"""version: "2.1"
services:
  controller:
    image: {DOCKER_KENG_CONTROLLER}:{controller_version}
    restart: always
    ports:
      - "{control_port}:40051"
      - "{rest_port}:8443"
    depends_on:
      layer23-hw-server:
        condition: service_started
    command:
      - "--accept-eula"
      - "--debug"
      - "--keng-layer23-hw-server"
      - "layer23-hw-server:5001"{controller_command_formatted}
    environment:
      - LICENSE_SERVERS=10.85.70.247
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  layer23-hw-server:
    image: {DOCKER_KENG_LAYER23}:{layer23_version}
    restart: always
    command:
      - "dotnet"
      - "otg-ixhw.dll"
      - "--trace"
      - "--log-level"
      - "trace"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  gnmi-server:
    image: {DOCKER_OTG_GNMI}:{gnmi_version}
    restart: always
    ports:
      - "{gnmi_port}:50051"
    depends_on:
      controller:
        condition: service_started
    command:
      - "-http-server"
      - "https://controller:8443"
      - "--debug"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
"""
    return docker_file


def generate_simulation_template(control_port, gnmi_port, rest_port, num_ports,
                                  controller_version='1.41.0-8',
                                  gnmi_version='1.13.15',
                                  traffic_engine_version='1.8.0.245',
                                  protocol_engine_version='1.00.0.486'):
    """
    Generate Docker Compose template for simulation-based OTG setup.
    
    Args:
        control_port: Controller port number
        gnmi_port: gNMI server port number
        rest_port: REST API port number
        num_ports: Number of network interfaces (eth1 to eth[num_ports])
        controller_version: Version of the Keng controller image
        gnmi_version: Version of the gNMI server image
        traffic_engine_version: Version of the traffic engine image
        protocol_engine_version: Version of the protocol engine image
    
    Returns:
        Docker compose file content as string
    """
    # Generate traffic and protocol engine containers for each interface
    engine_services = ""
    for i in range(1, num_ports + 1):
        host_port = 5554 + i  # Start from 5555 (5554 + 1)
        engine_services += f"""  ixia-c-traffic-engine-eth{i}:
    image: {DOCKER_TRAFFIC_ENGINE}:{traffic_engine_version}
    restart: always
    ports:
      - "{host_port}:5555"
    depends_on:
      controller:
        condition: service_started
    environment:
      OPT_LISTEN_PORT: "5555"
      ARG_IFACE_LIST: "virtual@af_packet,eth{i}"
      OPT_NO_HUGEPAGES: "Yes"
      OPT_NO_PINNING: "Yes"
      WAIT_FOR_IFACE: "Yes"
      OPT_ADAPTIVE_CPU_USAGE: "Yes"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  ixia-c-protocol-engine-eth{i}:
    image: {DOCKER_PROTOCOL_ENGINE}:{protocol_engine_version}
    restart: always
    privileged: true
    depends_on:
      controller:
        condition: service_started
    environment:
      INTF_LIST: "eth{i}"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
"""
    
    docker_file = f"""version: "2.1"
services:
  controller:
    image: {DOCKER_KENG_CONTROLLER}:{controller_version}
    restart: always
    ports:
      - "{control_port}:40051"
      - "{rest_port}:8443"
    environment:
      - LICENSE_SERVERS=10.85.70.247
    command:
      - "--accept-eula"
      - "--debug"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
  gnmi-server:
    image: {DOCKER_OTG_GNMI}:{gnmi_version}
    restart: always
    ports:
      - "{gnmi_port}:50051"
    depends_on:
      controller:
        condition: service_started
    command:
      - "-http-server"
      - "https://controller:8443"
      - "--debug"
    logging:
      driver: "local"
      options:
        max-size: "100m"
        max-file: "10"
        mode: "non-blocking"
{engine_services}"""
    
    return docker_file


def main():
    parser = argparse.ArgumentParser(
        description='Generate Docker Compose files for OTG (Open Traffic Generator)',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Generate simulation setup with 4 ports
  %(prog)s --mode sim --num-ports 4 --control-port 40051 --gnmi-port 50051 --rest-port 8443 -o docker-compose.yml
  
  # Generate hardware setup
  %(prog)s --mode hw --control-port 40051 --gnmi-port 50051 --rest-port 8443 -o docker-compose.yml
  
  # Generate with custom versions
  %(prog)s --mode sim --num-ports 2 --controller-version 1.42.0-1 --traffic-engine-version 1.9.0.100 -o docker-compose.yml
        """
    )
    
    parser.add_argument('--mode', choices=['sim', 'hw'], required=True,
                        help='Mode: sim (simulation) or hw (hardware)')
    parser.add_argument('--control-port', type=int, default=40051,
                        help='Controller port (default: 40051)')
    parser.add_argument('--gnmi-port', type=int, default=50051,
                        help='gNMI server port (default: 50051)')
    parser.add_argument('--rest-port', type=int, default=8443,
                        help='REST API port (default: 8443)')
    parser.add_argument('--num-ports', type=int, default=2,
                        help='Number of ports for simulation mode (default: 2)')
    parser.add_argument('-o', '--output', default='docker-compose.yml',
                        help='Output file path (default: docker-compose.yml)')
    
    # Version arguments
    parser.add_argument('--controller-version',
                        help='Controller image version (sim default: 1.41.0-8, hw default: 1.3.0-2)')
    parser.add_argument('--gnmi-version', default='1.13.15',
                        help='gNMI server image version (default: 1.13.15)')
    parser.add_argument('--layer23-version', default='1.3.0-4',
                        help='Layer23 HW server image version for hw mode (default: 1.3.0-4)')
    parser.add_argument('--traffic-engine-version', default='1.8.0.245',
                        help='Traffic engine image version for sim mode (default: 1.8.0.245)')
    parser.add_argument('--protocol-engine-version', default='1.00.0.486',
                        help='Protocol engine image version for sim mode (default: 1.00.0.486)')
    parser.add_argument('--controller-commands', nargs='*',
                        help='Additional controller commands for hw mode')
    
    args = parser.parse_args()
    
    # Generate the appropriate template
    if args.mode == 'sim':
        controller_version = args.controller_version or '1.41.0-8'
        content = generate_simulation_template(
            args.control_port,
            args.gnmi_port,
            args.rest_port,
            args.num_ports,
            controller_version=controller_version,
            gnmi_version=args.gnmi_version,
            traffic_engine_version=args.traffic_engine_version,
            protocol_engine_version=args.protocol_engine_version
        )
    else:  # hw mode
        controller_version = args.controller_version or '1.3.0-2'
        content = generate_hardware_template(
            args.control_port,
            args.gnmi_port,
            args.rest_port,
            controller_version=controller_version,
            layer23_version=args.layer23_version,
            gnmi_version=args.gnmi_version,
            controller_commands=args.controller_commands
        )
    
    # Write to file
    try:
        with open(args.output, 'w') as f:
            f.write(content)
        print(f"Docker Compose file generated successfully: {args.output}")
        print(f"Mode: {args.mode}")
        if args.mode == 'sim':
            print(f"Number of ports: {args.num_ports}")
        print(f"Control port: {args.control_port}")
        print(f"gNMI port: {args.gnmi_port}")
        print(f"REST port: {args.rest_port}")
        return 0
    except Exception as e:
        print(f"Error writing file: {e}", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())

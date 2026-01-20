#!/bin/bash

# Default version
DEFAULT_VERSION="v1.41.0-8"

export http_proxy=proxy-wsa.esl.cisco.com:80
export https_proxy=proxy-wsa.esl.cisco.com:80

# --- Argument parsing for number of ports and optional version ---
if [ -z "$1" ]; then
    echo "usage: $0 <num_ports> <function> [args...] [--version <version>]"
    echo "  Default version: $DEFAULT_VERSION"
    exit 1
fi

NUM_PORTS=$1
shift 1

# Parse optional --version argument
VERSION="$DEFAULT_VERSION"
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        *)
            break
            ;;
    esac
done

VERSIONS_YAML_LOC="https://github.com/open-traffic-generator/ixia-c/releases/download/${VERSION}/versions.yaml"
VERSIONS_YAML="versions.yaml"
CTRL_IMAGE="ghcr.io/open-traffic-generator/keng-controller"
TE_IMAGE="ghcr.io/open-traffic-generator/ixia-c-traffic-engine"
PE_IMAGE="ghcr.io/open-traffic-generator/ixia-c-protocol-engine"
GNMI_IMAGE="ghcr.io/open-traffic-generator/otg-gnmi-server"

echo "Using ixia-c version: ${VERSION}"

if [ -n "$AUTH_TOKEN" ]; then
    curl -lO -H "Authorization: Bearer $AUTH_TOKEN" $VERSIONS_YAML_LOC
else
    curl -kLO $VERSIONS_YAML_LOC
fi

TIMEOUT_SECONDS=300

# Generate array of interface names (eth1, eth2, ..., ethX)
ETH_PORTS=()
for ((i=1; i<=NUM_PORTS; i++)); do
    ETH_PORTS+=("eth$i")
done

set_docker_permission() {
    if ! groups $USER | grep -q '\bdocker\b'; then
        echo "Adding $USER to docker group (relogin required to take effect)."
        sudo usermod -aG docker $USER
    fi
    docker ps -a
}

set_docker_permission

configq() {
    # echo is needed to further evaluate the 
    # contents extracted from configuration
    eval echo $(yq "${@}" versions.yaml)
}

push_ifc_to_container() {
    # It takes a host NIC (say eth1) and injects it into a container’s 
    # network namespace so the container can directly use that NIC (bypassing Docker’s default bridge). 
    # It symlinks the container’s netns into /var/run/netns, 
    # then moves and configures the interface inside that namespace.
    if [ -z "${1}" ] || [ -z "${2}" ]
    then
        echo "usage: ${0} push_ifc_to_container <ifc-name> <container-name>"
        exit 1
    fi

    # Resolve container metadata
    cid=$(container_id ${2})
    cpid=$(container_pid ${2})

    echo "Changing namespace of ifc ${1} to container ID ${cid} pid ${cpid}"

    # Prepare namespace paths
    orgPath=/proc/${cpid}/ns/net
    newPath=/var/run/netns/${cid}
    
    # Make namespace accessible to ip netns 
    # Move interface into the container’s netns
    # Rename and configure inside the container
    sudo mkdir -p /var/run/netns
    echo "Creating symlink ${orgPath} -> ${newPath}"
    sudo ln -s ${orgPath} ${newPath} \
    && sudo ip link set ${1} netns ${cid} \
    && sudo ip netns exec ${cid} ip link set ${1} name ${1} \
    && sudo ip netns exec ${cid} ip -4 addr add 0/0 dev ${1} \
    && sudo ip netns exec ${cid} ip -4 link set ${1} up \
    && sudo ip netns exec ${cid} ip -4 link set ${1} promisc on \
    && echo "Successfully changed namespace of ifc ${1}"

    sudo rm -rf ${newPath}
}

container_id() {
    docker inspect --format="{{json .Id}}" ${1} | cut -d\" -f 2
}

container_pid() {
    docker inspect --format="{{json .State.Pid}}" ${1} | cut -d\" -f 2
}

container_ip() {
    local container_name=${1}
    local max_retries=${2:-30}
    local retry_count=0
    
    while [ $retry_count -lt $max_retries ]; do
        ip=$(docker inspect --format="{{json .NetworkSettings.IPAddress}}" ${container_name} 2>/dev/null | cut -d\" -f 2)
        if [ -z "$ip" ] || [ "$ip" == "null" ]; then
            ip=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${container_name} 2>/dev/null)
        fi
        
        # Return if we got a valid IP
        if [ -n "$ip" ] && [ "$ip" != "null" ]; then
            echo "$ip"
            return 0
        fi
        
        # Wait before retry
        sleep 1
        retry_count=$((retry_count + 1))
    done
    
    # Failed to get IP after all retries
    echo "ERROR: Failed to get IP for container ${container_name} after ${max_retries} attempts" >&2
    return 1
}

ixia_c_img_tag() {
    tag=$(grep ${1} ${VERSIONS_YAML} | cut -d: -f2 | cut -d\  -f2)
    echo "${tag}"
}

ixia_c_traffic_engine_img() {
    echo "${TE_IMAGE}:$(ixia_c_img_tag ixia-c-traffic-engine)"
}

ixia_c_protocol_engine_img() {
    echo "${PE_IMAGE}:$(ixia_c_img_tag ixia-c-protocol-engine)"
}

keng_controller_img() {
    echo "${CTRL_IMAGE}:$(ixia_c_img_tag keng-controller)"
}

gnmi_server_img() {
    echo "${GNMI_IMAGE}:$(ixia_c_img_tag otg-gnmi-server)"
}

gen_controller_config_b2b_cpdp() {
    configdir=/home/ixia-c/controller/config
    
    # Wait for all ports to be ready
    for eth in "${ETH_PORTS[@]}"; do
        echo "Retrieving IP for container ixia-c-traffic-engine-${eth}..."
        local ip=$(container_ip ixia-c-traffic-engine-${eth})
        if [ -z "$ip" ]; then
            echo "ERROR: Failed to retrieve IP for ixia-c-traffic-engine-${eth}"
            exit 1
        fi
        echo "Container ixia-c-traffic-engine-${eth} has IP: ${ip}"
        wait_for_sock ${ip} 5555
        wait_for_sock ${ip} 50071
    done
    
    # Build YAML configuration dynamically
    yml="location_map:"
    for eth in "${ETH_PORTS[@]}"; do
        local ip=$(container_ip ixia-c-traffic-engine-${eth})
        if [ -z "$ip" ]; then
            echo "ERROR: Failed to retrieve IP for ixia-c-traffic-engine-${eth}"
            exit 1
        fi
        yml+="
          - location: ${eth}
            endpoint: \"${ip}:5555+${ip}:50071\""
    done
    yml+="
        "
    
    echo -n "$yml" | sed "s/^        //g" | tee ./config.yaml > /dev/null \
    && docker exec keng-controller mkdir -p ${configdir} \
    && docker cp ./config.yaml keng-controller:${configdir}/ \
    && rm -rf ./config.yaml
}

gen_controller_config_b2b_dp() {
    configdir=/home/ixia-c/controller/config
    
    # Wait for all ports to be ready
    for eth in "${ETH_PORTS[@]}"; do
        echo "Retrieving IP for container ixia-c-traffic-engine-${eth}..."
        local ip=$(container_ip ixia-c-traffic-engine-${eth})
        if [ -z "$ip" ]; then
            echo "ERROR: Failed to retrieve IP for ixia-c-traffic-engine-${eth}"
            exit 1
        fi
        echo "Container ixia-c-traffic-engine-${eth} has IP: ${ip}"
        wait_for_sock ${ip} 5555
    done
    
    # Build YAML configuration dynamically
    yml="location_map:"
    for eth in "${ETH_PORTS[@]}"; do
        local ip=$(container_ip ixia-c-traffic-engine-${eth})
        if [ -z "$ip" ]; then
            echo "ERROR: Failed to retrieve IP for ixia-c-traffic-engine-${eth}"
            exit 1
        fi
        yml+="
          - location: ${eth}
            endpoint: \"${ip}:5555\""
    done
    yml+="
        "
    
    echo -n "$yml" | sed "s/^        //g" | tee ./config.yaml > /dev/null \
    && docker exec keng-controller mkdir -p ${configdir} \
    && docker cp ./config.yaml keng-controller:${configdir}/ \
    && rm -rf ./config.yaml
}

wait_for_sock() {
    TIMEOUT_SECONDS=120
    if [ ! -z "${3}" ]
    then
        TIMEOUT_SECONDS=${3}
    fi
    echo "Waiting for ${1}:${2} to be ready (timeout=${TIMEOUT_SECONDS}s)..."
    elapsed=0
    TIMEOUT_SECONDS=$(($TIMEOUT_SECONDS * 10))
    while true
    do
        nc -z -v ${1} ${2} && return 0

        elapsed=$(($elapsed+1))
        # echo "Timeout: $TIMEOUT_SECONDS"
        # echo "elapsed time: $elapsed"

        if [ $elapsed -gt ${TIMEOUT_SECONDS} ]
        then
            echo "${1}:${2} to be ready after ${TIMEOUT_SECONDS}"
            exit 1
        fi
        sleep 0.1
    done

}

prepare_eth_pair() {
    if [ -z "${1}" ] || [ -z "${2}" ]
    then
        echo "usage: ${0} create_veth_pair <name1> <name2>"
        exit 1
    fi

    sudo ip link set ${1} up \
    && sudo ip link set ${1} promisc on \
    && sudo ip link set ${2} up \
    && sudo ip link set ${2} promisc on
}

create_ixia_c_b2b_cpdp() {
    # Clean up existing topology if it exists
    echo "Checking for existing topology..."
    if docker ps -a --format '{{.Names}}' | grep -q "keng-controller\|otg-gnmi-server\|ixia-c-traffic-engine\|ixia-c-protocol-engine"; then
        echo "Existing topology found. Cleaning up..."
        rm_ixia_c_b2b_cpdp
        echo "Cleanup complete."
    fi
    
    docker ps -a
    echo "Setting up back-to-back with CP/DP distribution of ixia-c for ${NUM_PORTS} ports..."
    
    # Create controller and gnmi server
    docker run -d                                        \
    --name=keng-controller                              \
    --restart unless-stopped                            \
    --publish 40051:40051                       \
    --publish 8443:8443                         \
    -e LICENSE_SERVERS="10.85.70.247"           \
    $(keng_controller_img)                              \
    --accept-eula                                       \
    --trace                                             \
    --disable-app-usage-reporter
    
    docker run -d                                        \
    --name=otg-gnmi-server                              \
    --restart unless-stopped                            \
    --publish 0.0.0.0:50051:50051                       \
    $(gnmi_server_img)                                  \
    "-http-server" "https://172.17.0.1:8443" "--debug"
    
    # Process each port sequentially: create containers, wait for ready, then push interface
    local port_base=5555
    local timeout=120
    
    for ((i=0; i<${#ETH_PORTS[@]}; i++)); do
        local eth=${ETH_PORTS[$i]}
        local port=$((port_base + i))
        
        # Compute cpuset-cpus: eth1 gets 0,1,2; eth2 gets 0,3,4; eth3 gets 0,5,6, etc.
        # Formula: 0, (i*2+1), (i*2+2)
        local cpu1=$((i * 2 + 1))
        local cpu2=$((i * 2 + 2))
        local cpuset="0,${cpu1},${cpu2}"
        
        echo "Processing port ${eth} ($((i+1))/${#ETH_PORTS[@]}) with cpuset=${cpuset}..."
        
        # Create traffic engine container
        echo "Creating traffic engine for ${eth}..."
        docker run --privileged -d                           \
            --name=ixia-c-traffic-engine-${eth}              \
            --restart unless-stopped                         \
            --cpuset-cpus "${cpuset}"                        \
            --publish 0.0.0.0:${port}:5555                   \
            -e OPT_LISTEN_PORT="5555"                        \
            -e ARG_IFACE_LIST="virtual@af_packet,${eth}"    \
            -e OPT_NO_HUGEPAGES="Yes"                        \
            -e OPT_NO_PINNING="Yes"                           \
            -e WAIT_FOR_IFACE="Yes"                          \
            $(ixia_c_traffic_engine_img)
        
        # Wait for traffic engine to be running
        echo "Waiting for traffic engine ${eth} to be ready..."
        local elapsed=0
        while [ $elapsed -lt $timeout ]; do
            local status=$(docker inspect --format='{{.State.Status}}' ixia-c-traffic-engine-${eth} 2>/dev/null)
            if [ "$status" == "running" ]; then
                echo "Traffic engine ${eth} is running"
                break
            fi
            sleep 1
            elapsed=$((elapsed + 1))
        done
        
        if [ $elapsed -ge $timeout ]; then
            echo "ERROR: Traffic engine ${eth} failed to start within ${timeout} seconds"
            exit 1
        fi
        
        # Create protocol engine if not data plane only
        if [ -z "${DATA_PLANE_ONLY}" ]; then
            echo "Creating protocol engine for ${eth}..."
            docker run --privileged -d                           \
                --net=container:ixia-c-traffic-engine-${eth}     \
                --name=ixia-c-protocol-engine-${eth}             \
                --restart unless-stopped                         \
                -e INTF_LIST="${eth}"                            \
                $(ixia_c_protocol_engine_img)
            
            # Wait for protocol engine to be running
            echo "Waiting for protocol engine ${eth} to be ready..."
            elapsed=0
            while [ $elapsed -lt $timeout ]; do
                local pe_status=$(docker inspect --format='{{.State.Status}}' ixia-c-protocol-engine-${eth} 2>/dev/null)
                if [ "$pe_status" == "running" ]; then
                    echo "Protocol engine ${eth} is running"
                    break
                fi
                sleep 1
                elapsed=$((elapsed + 1))
            done
            
            if [ $elapsed -ge $timeout ]; then
                echo "ERROR: Protocol engine ${eth} failed to start within ${timeout} seconds"
                exit 1
            fi
        fi
        
        # Push interface to container
        echo "Pushing interface ${eth} to container..."
        push_ifc_to_container ${eth} ixia-c-traffic-engine-${eth}
        echo "Successfully configured port ${eth}"
        echo ""
    done
    
    docker ps -a
    
    # Generate controller config
    if [ -z "${DATA_PLANE_ONLY}" ]; then
        gen_controller_config_b2b_cpdp $1
    else 
        gen_controller_config_b2b_dp $1
    fi
    
    docker ps -a
    echo "Successfully deployed !"
}

rm_ixia_c_b2b_cpdp() {
    docker ps -a
    echo "Tearing down back-to-back with CP/DP distribution of ixia-c ..."
    docker stop keng-controller 2>/dev/null && docker rm keng-controller 2>/dev/null
    docker stop otg-gnmi-server 2>/dev/null && docker rm otg-gnmi-server 2>/dev/null

    # Stop and remove all traffic engines
    for eth in "${ETH_PORTS[@]}"; do
        docker stop ixia-c-traffic-engine-${eth} 2>/dev/null
        docker rm ixia-c-traffic-engine-${eth} 2>/dev/null
    done

    # Stop and remove all protocol engines if not data plane only
    if [ -z "${DATA_PLANE_ONLY}" ]; then
        for eth in "${ETH_PORTS[@]}"; do
            docker stop ixia-c-protocol-engine-${eth} 2>/dev/null
            docker rm ixia-c-protocol-engine-${eth} 2>/dev/null
        done
    fi
    
    # Delete veth pairs
    echo "Removing veth pairs..."
    for ((i=0; i<${#ETH_PORTS[@]}; i+=2)); do
        if [ $((i+1)) -lt ${#ETH_PORTS[@]} ]; then
            local eth1=${ETH_PORTS[$i]}
            local eth2=${ETH_PORTS[$((i+1))]}
            sudo ip link delete ${eth1} 2>/dev/null && echo "Deleted veth pair: ${eth1} <-> ${eth2}"
        fi
    done
    
    docker ps -a
}

create() {
    # Create veth pairs for all ports (eth1-eth2, eth3-eth4, etc.)
    for ((i=0; i<${#ETH_PORTS[@]}; i+=2)); do
        if [ $((i+1)) -lt ${#ETH_PORTS[@]} ]; then
            local eth1=${ETH_PORTS[$i]}
            local eth2=${ETH_PORTS[$((i+1))]}
            sudo ip link add $eth1 type veth peer name $eth2
            sudo ip link set $eth1 up
            sudo ip link set $eth2 up
            echo "Created veth pair: $eth1 <-> $eth2"
        fi
    done
}

pull_images() {
    echo "Pulling images for ixia-c version: ${VERSION}"
    
    # Get image names with tags
    local ctrl_img=$(keng_controller_img)
    local te_img=$(ixia_c_traffic_engine_img)
    local pe_img=$(ixia_c_protocol_engine_img)
    local gnmi_img=$(gnmi_server_img)
    
    echo "Pulling controller image: ${ctrl_img}"
    docker pull ${ctrl_img}
    
    echo "Pulling traffic engine image: ${te_img}"
    docker pull ${te_img}
    
    echo "Pulling protocol engine image: ${pe_img}"
    docker pull ${pe_img}
    
    echo "Pulling gNMI server image: ${gnmi_img}"
    docker pull ${gnmi_img}
    
    echo "Successfully pulled all images for version ${VERSION}"
    docker images | grep -E "(keng-controller|ixia-c-traffic-engine|ixia-c-protocol-engine|otg-gnmi-server)"
}

topo() {
    case $1 in
        new )  
            create_ixia_c_b2b_cpdp      
        ;;
        rm  )
            rm_ixia_c_b2b_cpdp
        ;;
        *   )
            exit 1
        ;;
    esac
}


help() {
    grep "() {" ${0} | cut -d\  -f1
}

usage() {
    echo "usage: $0 [name of any function in script]"
    exit 1
}

case $1 in
    *   )
        cmd=${1}
        echo "Hi"
        shift 1
        ${cmd} "$@" || usage
    ;;
esac
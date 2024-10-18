import paramiko
import time
from pygnmi.client import gNMIclient
import json
import grpc
import argparse

def check_ssh_connection(hostname, username, password, port=22):
    try:
        client = paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        client.connect(hostname, username=username, password=password, port=port)
        client.close()
        return True
    except Exception as e:
        print(f"SSH connection failed: {e}")
        return False

def perform_gnmi_subscription(hostname, paths_file, username, password, gnmi_port=35000):
    with open(paths_file, 'r') as file:
        paths = json.load(file)

    # Define the sample interval in nanoseconds (10 seconds)
    sample_interval = 10000000000  # 10 seconds in nanoseconds

    subscription_start_time = time.time()
    print(f"Subscription request sent at: {subscription_start_time}")

    # Configure gRPC options for keepalive and timeout
    grpc_options = [
        ('grpc.keepalive_time_ms', 10000),   # Keepalive time: 10 seconds
        ('grpc.keepalive_timeout_ms', 1000), # Timeout: 1 second
        ('grpc.keepalive_permit_without_calls', 1),  # Keepalive even when no calls in flight
        ('grpc.http2.max_pings_without_data', 0),    # Unlimited pings without data
        ('grpc.http2.min_time_between_pings_ms', 10000), # Minimum time between pings: 10 seconds
        ('grpc.http2.min_ping_interval_without_data_ms', 5000) # 5 seconds minimum interval between pings without data
    ]

    # Establish gNMI connection with keepalive and timeout settings
    with gNMIclient(
        target=(hostname, gnmi_port),
        username=username,
        password=password,
        insecure=False,  # Set to False if using TLS
        options=grpc_options,  # Pass the gRPC options for keepalive and timeout
        skip_verify=True
    ) as gc:
        subscriptions = []
        for path, subscription_mode in paths.items():
            if subscription_mode not in ['ON_CHANGE', 'SAMPLE', 'ONCE', 'TARGET_DEFINED']:
                print(f"Invalid subscription mode '{subscription_mode}' for path '{path}'")
                continue

            subscription_entry = {
                'path': path,
                'mode': subscription_mode
            }

            # Add sample_interval only if the mode is SAMPLE
            if subscription_mode == 'SAMPLE':
                subscription_entry['sample_interval'] = sample_interval

            subscriptions.append(subscription_entry)

        if not subscriptions:
            print("No valid subscriptions to perform.")
            return

        # Wrap subscriptions in a dict with the key 'subscription'
        subscribe_request = {
            'subscription': subscriptions
        }

        # Log the full subscribe request
        print(f"Subscribing to: {subscribe_request}")

        try:
            for response in gc.subscribe(subscribe_request):
                response_time = time.time()
                print(f"Response received at: {response_time}")

                elapsed_time = response_time - subscription_start_time
                print(f"Elapsed time: {elapsed_time:.6f} seconds")

                # Print the entire response for debugging
                print(f"Full response: {response}")

                if hasattr(response, 'update'):
                    for update in response.update:
                        print(f"Message: Path: {update.path}, Value: {update.val}")
                else:
                    print("Received response but no updates available.")

                # Check for sync_response errors
                if hasattr(response, 'sync_response') and response.sync_response:
                    capture_error(response_time, "Sync error in subscription")
                    break

        except Exception as e:
            capture_error(time.time(), f"gNMI subscription failed: {e}")

def capture_error(timestamp, error_message):
    print(f"Error at {timestamp}: {error_message}")

def continuous_ssh_check(hostname, username, password, paths_file, ssh_port=22, gnmi_port=57400):
    while True:
        if check_ssh_connection(hostname, username, password, port=ssh_port):
            print("SSH connection successful. Performing gNMI subscription.")
            perform_gnmi_subscription(hostname, paths_file, username, password, gnmi_port)
        else:
            print("SSH connection failed. Retrying...")
        time.sleep(30)

if __name__ == "__main__":
    # hostname = '10.85.72.52'
    # username = 'cisco'
    # password = 'cisco'
    # paths_file = 'paths.json'  # JSON file containing paths and their subscription modes
    # ssh_port = 22  # Custom SSH port
    # gnmi_port = 8818  # Custom gNMI port

    # D8
    # hostname = '10.85.84.225'
    # username = 'cisco'
    # password = 'cisco123'
    # paths_file = 'paths.json'  # JSON file containing paths and their subscription modes
    # ssh_port = 4999  # Custom SSH port
    # gnmi_port = 35000  # Custom gNMI port

    # Set up argument parser inside the main function
    parser = argparse.ArgumentParser(description="Process hostname, username, password, and other parameters.")

    # Add arguments with default values
    parser.add_argument('--hostname', type=str, default='10.85.72.52', help='Hostname or IP address')
    parser.add_argument('--username', type=str, default='cisco', help='Username for SSH')
    parser.add_argument('--password', type=str, default='cisco', help='Password for SSH')
    parser.add_argument('--paths_file', type=str, default='paths.json',
                        help='JSON file containing paths and their subscription modes')
    parser.add_argument('--ssh_port', type=int, default=22, help='Custom SSH port')
    parser.add_argument('--gnmi_port', type=int, default=8818, help='Custom gNMI port')

    # Parse the arguments inside main
    args = parser.parse_args()

    # Use parsed arguments or fall back to defaults
    hostname = args.hostname
    username = args.username
    password = args.password
    paths_file = args.paths_file
    ssh_port = args.ssh_port
    gnmi_port = args.gnmi_port

    continuous_ssh_check(hostname, username, password, paths_file, ssh_port, gnmi_port)

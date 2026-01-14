#!/bin/bash

hosts=("192.168.105.2" "192.168.105.3")

pids=()

for host in "${hosts[@]}"; do
    echo "Deploying to $host"
    scp -i ~/lab /home/juan/repos/realm/bin/realm ansible@$host:/home/ansible/realm &
    pids+=($!)
done

# Wait for all background jobs to complete
for pid in "${pids[@]}"; do
    wait $pid
    if [ $? -ne 0 ]; then
        echo "Error: Deployment failed for one of the hosts (PID: $pid)"
        exit 1
    fi
done

echo "All deployments completed successfully"

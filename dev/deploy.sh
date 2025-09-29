#!/bin/bash

hosts=("192.168.47.82" "192.168.47.86")

for host in "${hosts[@]}"; do
    echo "Deploying to $host"
    scp -i ~/lab /home/juan/repos/realm/bin/realm ansible@$host:/home/ansible/realm
done

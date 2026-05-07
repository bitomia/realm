# Ansible configurations

Only Debian 12 and Ubuntu 22.04 are supported.

Create first an Ansible inventory with a targets list where agents will be installed:

inventory.ini file:

```toml
[targets]
lab1 ansible_host=192.168.105.2 ansible_user=ansible ansible_ssh_private_key_file=./lab etcd_server=true
lab2 ansible_host=192.168.105.3 ansible_user=ansible ansible_ssh_private_key_file=./lab etcd_server=false
```

Now launch ansible as follows:

```shell
ansible-playbook -i inventory.ini cluster.yml -K
```

Enter the root password and the block device where the ZFS pool will be created.


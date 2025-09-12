# Ansible configurations

## Cluster mode

Only Debian 12 and Ubuntu 22.04 are supported.

Create first an Ansible inventory with a targets list where daemons will be installed:

inventory.ini file:

```toml
[targets]
debian1 ansible_host=172.16.123.135 ansible_user=juan ansible_ssh_private_key_file=~/.ssh/id_rsa
```

Now launch ansible as follows:

```shell
ansible-playbook -i inventory.ini cluster.yml -K
```

Enter the root password and the block device where the ZFS pool will be created.

## Local mode

```
sudo apt-get update
sudo apt-get install -y ansible
sudo ansible-playbook ansible/local.yml
```


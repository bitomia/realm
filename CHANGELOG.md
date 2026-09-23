## 0.2.8

- Agent: can now listen on a unix socket. New `listen_socket_group` option sets the group owning the socket.
- Agent: log level is now a configuration parameter.
- Client: added unix socket support.
- Fix: containerd logs not bridged to the agent log.
- Fix: power and restart client modes not working.
- Fix: invalid poweroff payload in the python client.
- Fix: clarified power state reporting on the vm driver.
- Documentation moved to a dedicated website.

## 0.2.7

- Agent: registry configuration has been moved from the agent configuration to the node configuration. This prevents constant reloading of the agent and now can be configured remotely from control
- Fix: nodes not reporting as ready at bootup

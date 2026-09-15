# realm-client

Python client for the [Realm](https://github.com/bitomia/realm) agent API.

It is a thin wrapper over the agent's HTTP API: one dependency (`httpx`), no
models, no schema. Responses come back as the agent's JSON decoded into plain
dicts and lists, so a new field on the agent side is usable the moment it ships,
without waiting for a client release.

## Install

```bash
pip install realm-client
```

## Use

The agent serves the same API over TCP and, when `agent.listen_socket` is
configured, over a unix socket. Only the constructor differs:

```python
from realm import Realm

with Realm("http://lab1:9000") as agent:
    print(agent.version())

with Realm("unix:///run/realm/agent.sock") as agent:
    print(agent.version())

with Realm(socket="/run/realm/agent.sock") as agent:
    print(agent.version())
```

The URL accepts the same `http://`, `https://` and `unix://` forms as the `url`
of a node in the realm configuration, so a URL read out of that configuration
can be handed to the client as it is.

Read state:

```python
with Realm(socket="/run/realm/agent.sock") as agent:
    node = agent.node()
    print(node["state"]["ncpu"], node["state"]["mem_free_percentage"])

    for deployment in agent.loads():
        print(deployment["load_name"], deployment["deployment_status"]["status"])
```

Deploy a load and follow its output:

```python
deployment_id = agent.provision_load(
    {
        "name": "web",
        "node": "lab1",
        "driver": "container",
        "driver_config": {"image": "docker.io/library/nginx:latest"},
    }
)
agent.start_load("web")

for line in agent.stdout("web"):
    print(line)
```

Run a job — results stream in as NDJSON, one dict per line, each carrying either
`value` or `err`:

```python
for result in agent.run_job({"name": "hello", "driver": "hello"}):
    if "err" in result:
        raise RuntimeError(result["err"])
    print(result["value"])
```

Async works the same way:

```python
from realm import AsyncRealm

async with AsyncRealm(socket="/run/realm/agent.sock") as agent:
    print(await agent.version())
    async for line in agent.stdout("web"):
        print(line)
```

## Authentication

When the agent runs with `JWT_SECRET` set, requests need a bearer token. The
client looks it up the same way the `realm` CLI does — `REALM_BEARER`, then
`~/.realmrc` — or takes it directly:

```python
Realm("http://lab1:9000", token="eyJ..")
```

## Errors

Any non-200 response raises `RealmError`, whose message is the agent's response
body and whose `status` is the HTTP status code. Two statuses that carry
specific meaning get their own subclasses:

```python
from realm import NodeAlreadyConfigured, NodeNotConfigured, RealmError

try:
    config = agent.node_config()
except NodeNotConfigured:  # 204: the agent holds no configuration
    config = None
except RealmError as err:
    print(err.status, err.body)
```

`NodeAlreadyConfigured` is raised by `load_node_config` when the agent already
holds a configuration (409).

## Development

```bash
pip install -e '.[dev]'
pytest                      # unit tests, no agent needed
make all && pytest -m integration    # drives a real agent over a socket
```

`tests/routes.json` is generated from the agent's router by `make python-routes`
and `tests/test_parity.py` asserts the client covers every route in it, so a
route added to the agent with no client operation fails CI.

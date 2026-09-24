"""The agent API described once, as data.

Each function here builds a :class:`Request` for one route and is tagged with
the route it implements. ``Realm`` and ``AsyncRealm`` are thin wrappers that
execute these requests, so the two clients cannot drift apart, and
``tests/test_parity.py`` checks the tags against the router inventory the agent
generates (``tests/routes.json``).

Responses are handed back as decoded JSON — dicts and lists, exactly as the
agent sent them. The only parsing done here is unwrapping single-field
envelopes, where returning ``{"version": "v1"}`` would be noise.
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import quote

JsonDict = dict[str, Any]

# Mirrors the timeouts the Go client uses (cmd/client/client.go).
READ_TIMEOUT = 10.0
REQUEST_TIMEOUT = 120.0

ENDPOINT_ATTR = "__realm_endpoint__"


def endpoint(method: str, path: str) -> Callable[[Any], Any]:
    """Tag an operation with the route template it implements."""

    def decorate(fn: Any) -> Any:
        setattr(fn, ENDPOINT_ATTR, (method, path))
        return fn

    return decorate


@dataclass(frozen=True)
class Request:
    """A single HTTP call, independent of sync/async execution."""

    method: str
    path: str
    json: Any = None
    params: JsonDict = field(default_factory=dict)
    timeout: float | None = READ_TIMEOUT
    # Applied to the decoded JSON body. None returns the body as-is.
    parse: Callable[[Any], Any] | None = None
    # Streaming responses (NDJSON job results, load logs, artifact downloads).
    stream: bool = False


def _as_list(body: Any) -> list[Any]:
    """Normalise a list endpoint's body.

    An empty Go slice marshals to ``null``, so the agent answers ``null``
    rather than ``[]`` for an empty listing. Callers get a list either way.
    """
    return list(body) if body else []


def _seg(value: str) -> str:
    """Escape a path segment, so load names with slashes cannot alter the path."""
    return quote(value, safe="")


# --- Agent ------------------------------------------------------------------


@endpoint("GET", "/version")
def version() -> Request:
    return Request("GET", "/version", parse=lambda body: str(body["version"]))


@endpoint("GET", "/system")
def system() -> Request:
    return Request("GET", "/system")


@endpoint("GET", "/network")
def networks() -> Request:
    return Request("GET", "/network")


@endpoint("GET", "/images")
def images() -> Request:
    return Request("GET", "/images", parse=_as_list)


@endpoint("GET", "/containers")
def containers() -> Request:
    return Request("GET", "/containers")


# --- Node -------------------------------------------------------------------


@endpoint("GET", "/node")
def node(guest: str | None = None) -> Request:
    if guest:
        return Request("GET", f"/node/guests/{_seg(guest)}")
    return Request("GET", "/node")


@endpoint("GET", "/node/config")
def get_node_config() -> Request:
    return Request("GET", "/node/config")


@endpoint("POST", "/node/config")
def load_node_config(config: JsonDict, validate_only: bool = False) -> Request:
    return Request(
        "POST",
        "/node/config",
        json=config,
        params={"validate": "true"} if validate_only else {},
        timeout=REQUEST_TIMEOUT,
    )


@endpoint("DELETE", "/node/config")
def unload_node_config(guest: str | None = None) -> Request:
    if guest:
        return Request("DELETE", f"/node/guests/{_seg(guest)}/config", timeout=REQUEST_TIMEOUT)
    return Request("DELETE", "/node/config", timeout=REQUEST_TIMEOUT)


def _node_path(operation: str, guest: str | None) -> str:
    """Agent path for a node operation, on a guest node when ``guest`` is set."""
    if guest:
        return f"/node/guests/{_seg(guest)}/{operation}"
    return f"/node/{operation}"


@endpoint("POST", "/node/poweron")
def power_on(node_config: JsonDict, guest: str | None = None) -> Request:
    return Request("POST", _node_path("poweron", guest), json=node_config, timeout=REQUEST_TIMEOUT)


@endpoint("POST", "/node/poweroff")
def power_off(guest: str | None = None) -> Request:
    return Request("POST", _node_path("poweroff", guest), timeout=REQUEST_TIMEOUT)


@endpoint("POST", "/node/shutdown")
def shutdown(wall_message: str = "", delay: int = 0, guest: str | None = None) -> Request:
    payload: JsonDict = {"wall_message": wall_message, "time": delay}
    return Request("POST", _node_path("shutdown", guest), json=payload, timeout=REQUEST_TIMEOUT)


@endpoint("POST", "/node/restart")
def restart(wall_message: str = "", delay: int = 0, guest: str | None = None) -> Request:
    payload: JsonDict = {"wall_message": wall_message, "time": delay}
    return Request("POST", _node_path("restart", guest), json=payload, timeout=REQUEST_TIMEOUT)


# --- Loads ------------------------------------------------------------------


@endpoint("GET", "/loads")
def loads() -> Request:
    return Request("GET", "/loads", parse=_as_list)


@endpoint("POST", "/loads/provision")
def provision_load(load: JsonDict) -> Request:
    return Request(
        "POST",
        "/loads/provision",
        json=load,
        timeout=REQUEST_TIMEOUT,
        parse=lambda body: str(body["deployment_id"]),
    )


@endpoint("POST", "/loads/{loadName}/start")
def start_load(name: str) -> Request:
    return Request("POST", f"/loads/{_seg(name)}/start", timeout=REQUEST_TIMEOUT)


@endpoint("POST", "/loads/{loadName}/stop")
def stop_load(name: str) -> Request:
    return Request("POST", f"/loads/{_seg(name)}/stop", timeout=REQUEST_TIMEOUT)


@endpoint("POST", "/loads/{loadName}/kill")
def kill_load(name: str) -> Request:
    return Request("POST", f"/loads/{_seg(name)}/kill", timeout=REQUEST_TIMEOUT)


@endpoint("POST", "/loads/{loadName}/deprovision")
def deprovision_load(name: str) -> Request:
    return Request("POST", f"/loads/{_seg(name)}/deprovision", timeout=REQUEST_TIMEOUT)


@endpoint("GET", "/loads/{loadName}/stdout")
def load_stdout(name: str) -> Request:
    return Request("GET", f"/loads/{_seg(name)}/stdout", timeout=None, stream=True)


@endpoint("GET", "/loads/{loadName}/stderr")
def load_stderr(name: str) -> Request:
    return Request("GET", f"/loads/{_seg(name)}/stderr", timeout=None, stream=True)


# --- Jobs -------------------------------------------------------------------


@endpoint("POST", "/jobs")
def run_job(job: JsonDict) -> Request:
    """Jobs answer with an NDJSON stream: one ``{"value"}``/``{"err"}`` per line."""
    return Request("POST", "/jobs", json=job, timeout=None, stream=True)


# --- Cloud-init and artifacts ----------------------------------------------


@endpoint("GET", "/cloudinit/{nodeName}/{dataType}")
def cloud_init(node_name: str, data_type: str) -> Request:
    if data_type not in ("meta-data", "user-data", "network-config"):
        raise ValueError(
            "data_type must be one of 'meta-data', 'user-data', 'network-config', "
            f"got {data_type!r}"
        )
    return Request("GET", f"/cloudinit/{_seg(node_name)}/{data_type}")


@endpoint("GET", "/artifacts/raw")
def artifacts() -> Request:
    return Request("GET", "/artifacts/raw", parse=_as_list)


@endpoint("GET", "/artifacts/raw/{name}")
def artifact(name: str) -> Request:
    return Request("GET", f"/artifacts/raw/{_seg(name)}", timeout=REQUEST_TIMEOUT, stream=True)

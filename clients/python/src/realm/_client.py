"""Sync and async clients for the Realm agent API."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator, Iterator
from types import TracebackType
from typing import Any

import httpx

from . import _ops as ops
from ._auth import resolve_token
from ._errors import NodeAlreadyConfigured, NodeNotConfigured, RealmError
from ._target import Target, resolve_target

JsonDict = dict[str, Any]


def _prepare(
    url: str | None,
    socket: str | None,
    token: str | None,
    headers: dict[str, str] | None,
) -> tuple[Target, dict[str, str]]:
    target = resolve_target(url, socket)

    resolved = {"Content-Type": "application/json"}
    if token is None:
        token = resolve_token()
    if token:
        resolved["Authorization"] = f"Bearer {token}"
    if headers:
        resolved.update(headers)

    return target, resolved


def _check(response: httpx.Response, body: str) -> None:
    """Raise for a failed response, mapping the statuses that carry meaning.

    204 is a success the agent uses for "nothing to return"; :func:`_decode`
    turns it into :class:`NodeNotConfigured`.
    """
    if response.status_code in (200, 204):
        return
    if response.status_code == 409:
        raise NodeAlreadyConfigured(response.status_code, body.strip())
    raise RealmError(response.status_code, body.strip())


def _decode(request: ops.Request, response: httpx.Response) -> Any:
    """Return the decoded body: JSON where the agent sends JSON, else text."""
    # GET /node/config answers 204 when the agent holds no configuration.
    if response.status_code == 204:
        raise NodeNotConfigured(204, "node is not configured")

    if not response.content:
        return None

    if request.parse is not None:
        return request.parse(response.json())

    if response.headers.get("content-type", "").startswith("application/json"):
        return response.json()
    return response.text


class Realm:
    """Client for a single Realm agent.

    Connect over TCP or over the agent's unix socket (``agent.listen_socket``);
    both expose the same API, so only the constructor differs::

        with Realm("http://lab1:9000") as agent:
            print(agent.version())

        with Realm("unix:///run/realm/agent.sock") as agent:
            print(agent.version())

    The URL takes the same ``http://``, ``https://`` and ``unix://`` forms as
    the ``url`` of a node in the realm configuration. ``socket=`` is a shorthand
    for the ``unix://`` form.

    Responses come back as plain dicts and lists, decoded from the agent's
    JSON. The bearer token is taken from ``REALM_BEARER`` or ``~/.realmrc``
    unless ``token`` is given, matching the ``realm`` CLI.
    """

    def __init__(
        self,
        url: str | None = None,
        *,
        socket: str | None = None,
        token: str | None = None,
        headers: dict[str, str] | None = None,
        transport: httpx.BaseTransport | None = None,
    ) -> None:
        target, resolved_headers = _prepare(url, socket, token, headers)
        if transport is None and target.socket is not None:
            transport = httpx.HTTPTransport(uds=target.socket)
        self._client = httpx.Client(
            base_url=target.base_url,
            headers=resolved_headers,
            transport=transport,
            timeout=ops.READ_TIMEOUT,
        )

    # --- plumbing ----------------------------------------------------------

    def _send(self, request: ops.Request) -> Any:
        response = self._client.request(
            request.method,
            request.path,
            json=request.json,
            params=request.params or None,
            timeout=request.timeout,
        )
        _check(response, response.text)
        return _decode(request, response)

    def _stream_lines(self, request: ops.Request) -> Iterator[str]:
        with self._client.stream(
            request.method,
            request.path,
            json=request.json,
            params=request.params or None,
            timeout=request.timeout,
        ) as response:
            _check(response, response.read().decode(errors="replace"))
            yield from response.iter_lines()

    def _stream_bytes(self, request: ops.Request) -> Iterator[bytes]:
        with self._client.stream(request.method, request.path, timeout=request.timeout) as response:
            _check(response, response.read().decode(errors="replace"))
            yield from response.iter_bytes()

    def close(self) -> None:
        self._client.close()

    def __enter__(self) -> Realm:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        self.close()

    # --- agent -------------------------------------------------------------

    def version(self) -> str:
        """Agent version. The only endpoint that never requires auth."""
        return str(self._send(ops.version()))

    def system(self) -> JsonDict:
        """Static host information and agent capabilities."""
        result: JsonDict = self._send(ops.system())
        return result

    def networks(self) -> JsonDict:
        """Container networks known to the agent."""
        result: JsonDict = self._send(ops.networks())
        return result

    def images(self) -> list[JsonDict]:
        """Images in the agent's containerd namespace."""
        result: list[JsonDict] = self._send(ops.images())
        return result

    def containers(self) -> JsonDict:
        """Running containers, keyed by container id."""
        result: JsonDict = self._send(ops.containers())
        return result

    # --- node --------------------------------------------------------------

    def node(self, guest: str | None = None) -> JsonDict:
        """Live node state and status. ``guest`` queries a guest node instead."""
        result: JsonDict = self._send(ops.node(guest))
        return result

    def node_config(self) -> JsonDict:
        """The loaded node configuration.

        Raises :class:`NodeNotConfigured` when the agent holds none.
        """
        result: JsonDict = self._send(ops.get_node_config())
        return result

    def load_node_config(self, config: JsonDict, validate_only: bool = False) -> None:
        """Push a node configuration. ``validate_only`` checks without applying."""
        self._send(ops.load_node_config(config, validate_only))

    def unload_node_config(self, guest: str | None = None) -> None:
        self._send(ops.unload_node_config(guest))

    def power_on(self, node_config: JsonDict, guest: str | None = None) -> None:
        """Power a node on. ``guest`` targets a guest node."""
        self._send(ops.power_on(node_config, guest))

    def power_off(self, guest: str | None = None) -> None:
        """Power a node off immediately. ``guest`` targets a guest node."""
        self._send(ops.power_off(guest))

    def shutdown(self, wall_message: str = "", delay: int = 0, guest: str | None = None) -> None:
        self._send(ops.shutdown(wall_message, delay, guest))

    def restart(self, wall_message: str = "", delay: int = 0, guest: str | None = None) -> None:
        self._send(ops.restart(wall_message, delay, guest))

    # --- loads -------------------------------------------------------------

    def loads(self) -> list[JsonDict]:
        """Every load deployment the agent knows about."""
        result: list[JsonDict] = self._send(ops.loads())
        return result

    def provision_load(self, load: JsonDict) -> str:
        """Provision a load; returns the deployment id."""
        return str(self._send(ops.provision_load(load)))

    def start_load(self, name: str) -> None:
        self._send(ops.start_load(name))

    def stop_load(self, name: str) -> None:
        self._send(ops.stop_load(name))

    def kill_load(self, name: str) -> None:
        self._send(ops.kill_load(name))

    def deprovision_load(self, name: str) -> None:
        self._send(ops.deprovision_load(name))

    def stdout(self, name: str) -> Iterator[str]:
        """Stream a load's stdout, line by line."""
        return self._stream_lines(ops.load_stdout(name))

    def stderr(self, name: str) -> Iterator[str]:
        """Stream a load's stderr, line by line."""
        return self._stream_lines(ops.load_stderr(name))

    # --- jobs --------------------------------------------------------------

    def run_job(self, job: JsonDict) -> Iterator[JsonDict]:
        """Run a job, yielding each NDJSON result as the agent produces it.

        Each result is ``{"value": ...}`` or ``{"err": ...}``.
        """
        for line in self._stream_lines(ops.run_job(job)):
            if line.strip():
                yield json.loads(line)

    # --- cloud-init and artifacts -----------------------------------------

    def cloud_init(self, node_name: str, data_type: str) -> str:
        """Fetch a cloud-init document (``meta-data``/``user-data``/``network-config``)."""
        return str(self._send(ops.cloud_init(node_name, data_type)))

    def artifacts(self) -> list[str]:
        """Names of the artifacts in the agent's raw repository."""
        result: list[str] = self._send(ops.artifacts())
        return result

    def download_artifact(self, name: str) -> bytes:
        """Download one raw artifact into memory."""
        return b"".join(self._stream_bytes(ops.artifact(name)))

    def stream_artifact(self, name: str) -> Iterator[bytes]:
        """Stream one raw artifact in chunks, for files too big to buffer."""
        return self._stream_bytes(ops.artifact(name))


class AsyncRealm:
    """Async counterpart of :class:`Realm`, with the same methods and results."""

    def __init__(
        self,
        url: str | None = None,
        *,
        socket: str | None = None,
        token: str | None = None,
        headers: dict[str, str] | None = None,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        target, resolved_headers = _prepare(url, socket, token, headers)
        if transport is None and target.socket is not None:
            transport = httpx.AsyncHTTPTransport(uds=target.socket)
        self._client = httpx.AsyncClient(
            base_url=target.base_url,
            headers=resolved_headers,
            transport=transport,
            timeout=ops.READ_TIMEOUT,
        )

    # --- plumbing ----------------------------------------------------------

    async def _send(self, request: ops.Request) -> Any:
        response = await self._client.request(
            request.method,
            request.path,
            json=request.json,
            params=request.params or None,
            timeout=request.timeout,
        )
        _check(response, response.text)
        return _decode(request, response)

    async def _stream_lines(self, request: ops.Request) -> AsyncIterator[str]:
        async with self._client.stream(
            request.method,
            request.path,
            json=request.json,
            params=request.params or None,
            timeout=request.timeout,
        ) as response:
            if response.status_code not in (200, 204):
                _check(response, (await response.aread()).decode(errors="replace"))
            async for line in response.aiter_lines():
                yield line

    async def _stream_bytes(self, request: ops.Request) -> AsyncIterator[bytes]:
        async with self._client.stream(
            request.method, request.path, timeout=request.timeout
        ) as response:
            if response.status_code not in (200, 204):
                _check(response, (await response.aread()).decode(errors="replace"))
            async for chunk in response.aiter_bytes():
                yield chunk

    async def aclose(self) -> None:
        await self._client.aclose()

    async def __aenter__(self) -> AsyncRealm:
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        await self.aclose()

    # --- agent -------------------------------------------------------------

    async def version(self) -> str:
        return str(await self._send(ops.version()))

    async def system(self) -> JsonDict:
        result: JsonDict = await self._send(ops.system())
        return result

    async def networks(self) -> JsonDict:
        result: JsonDict = await self._send(ops.networks())
        return result

    async def images(self) -> list[JsonDict]:
        result: list[JsonDict] = await self._send(ops.images())
        return result

    async def containers(self) -> JsonDict:
        result: JsonDict = await self._send(ops.containers())
        return result

    # --- node --------------------------------------------------------------

    async def node(self, guest: str | None = None) -> JsonDict:
        result: JsonDict = await self._send(ops.node(guest))
        return result

    async def node_config(self) -> JsonDict:
        result: JsonDict = await self._send(ops.get_node_config())
        return result

    async def load_node_config(self, config: JsonDict, validate_only: bool = False) -> None:
        await self._send(ops.load_node_config(config, validate_only))

    async def unload_node_config(self, guest: str | None = None) -> None:
        await self._send(ops.unload_node_config(guest))

    async def power_on(self, node_config: JsonDict, guest: str | None = None) -> None:
        await self._send(ops.power_on(node_config, guest))

    async def power_off(self, guest: str | None = None) -> None:
        await self._send(ops.power_off(guest))

    async def shutdown(
        self, wall_message: str = "", delay: int = 0, guest: str | None = None
    ) -> None:
        await self._send(ops.shutdown(wall_message, delay, guest))

    async def restart(
        self, wall_message: str = "", delay: int = 0, guest: str | None = None
    ) -> None:
        await self._send(ops.restart(wall_message, delay, guest))

    # --- loads -------------------------------------------------------------

    async def loads(self) -> list[JsonDict]:
        result: list[JsonDict] = await self._send(ops.loads())
        return result

    async def provision_load(self, load: JsonDict) -> str:
        return str(await self._send(ops.provision_load(load)))

    async def start_load(self, name: str) -> None:
        await self._send(ops.start_load(name))

    async def stop_load(self, name: str) -> None:
        await self._send(ops.stop_load(name))

    async def kill_load(self, name: str) -> None:
        await self._send(ops.kill_load(name))

    async def deprovision_load(self, name: str) -> None:
        await self._send(ops.deprovision_load(name))

    def stdout(self, name: str) -> AsyncIterator[str]:
        return self._stream_lines(ops.load_stdout(name))

    def stderr(self, name: str) -> AsyncIterator[str]:
        return self._stream_lines(ops.load_stderr(name))

    # --- jobs --------------------------------------------------------------

    async def run_job(self, job: JsonDict) -> AsyncIterator[JsonDict]:
        async for line in self._stream_lines(ops.run_job(job)):
            if line.strip():
                yield json.loads(line)

    # --- cloud-init and artifacts -----------------------------------------

    async def cloud_init(self, node_name: str, data_type: str) -> str:
        return str(await self._send(ops.cloud_init(node_name, data_type)))

    async def artifacts(self) -> list[str]:
        result: list[str] = await self._send(ops.artifacts())
        return result

    async def download_artifact(self, name: str) -> bytes:
        return b"".join([chunk async for chunk in self._stream_bytes(ops.artifact(name))])

    def stream_artifact(self, name: str) -> AsyncIterator[bytes]:
        return self._stream_bytes(ops.artifact(name))

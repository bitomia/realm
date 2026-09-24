"""Client behaviour, exercised against a mock transport (no network, no agent)."""

from __future__ import annotations

import json
from collections.abc import Callable

import httpx
import pytest

from realm import (
    AsyncRealm,
    NodeAlreadyConfigured,
    NodeNotConfigured,
    Realm,
    RealmError,
)

Handler = Callable[[httpx.Request], httpx.Response]


def client(handler: Handler, **kwargs: object) -> Realm:
    return Realm("http://agent", transport=httpx.MockTransport(handler), **kwargs)  # type: ignore[arg-type]


def async_client(handler: Handler) -> AsyncRealm:
    return AsyncRealm("http://agent", transport=httpx.MockTransport(handler))


def json_response(payload: object, status: int = 200) -> httpx.Response:
    return httpx.Response(status, json=payload)


# --- construction -----------------------------------------------------------


def test_requires_exactly_one_target() -> None:
    with pytest.raises(ValueError, match="exactly one"):
        Realm()
    with pytest.raises(ValueError, match="exactly one"):
        Realm("http://agent", socket="/run/realm/agent.sock")


def test_socket_target_uses_a_uds_transport() -> None:
    with Realm(socket="/run/realm/agent.sock") as agent:
        assert isinstance(agent._client._transport, httpx.HTTPTransport)
        assert agent._client.base_url.host == "realm.local"


def test_token_is_sent_as_a_bearer_header() -> None:
    seen: dict[str, str] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen.update(request.headers)
        return json_response({"version": "v1"})

    with client(handler, token="s3cret") as agent:
        agent.version()

    assert seen["authorization"] == "Bearer s3cret"
    assert seen["content-type"] == "application/json"


def test_explicit_token_beats_the_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("REALM_BEARER", "from-env")
    seen: dict[str, str] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen.update(request.headers)
        return json_response({"version": "v1"})

    with client(handler, token="explicit") as agent:
        agent.version()

    assert seen["authorization"] == "Bearer explicit"


def test_token_is_read_from_the_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("REALM_BEARER", "from-env")
    seen: dict[str, str] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen.update(request.headers)
        return json_response({"version": "v1"})

    with client(handler) as agent:
        agent.version()

    assert seen["authorization"] == "Bearer from-env"


# --- responses --------------------------------------------------------------


def test_version_unwraps_the_payload() -> None:
    with client(lambda _: json_response({"version": "v0.3.0-abc123"})) as agent:
        assert agent.version() == "v0.3.0-abc123"


def test_json_responses_are_returned_untouched() -> None:
    payload = {
        "state": {"ncpu": 8, "cpu_usage_percentage": 12.5, "some_new_field": 42},
        "status": {"status": "online", "reason": ""},
    }
    with client(lambda _: json_response(payload)) as agent:
        assert agent.node() == payload


def test_list_responses_are_returned_as_lists() -> None:
    payload = [
        {
            "load_name": "web",
            "deployment_id": "d1",
            "deployment_status": {"status": "running"},
        }
    ]
    with client(lambda _: json_response(payload)) as agent:
        assert agent.loads() == payload


def test_provision_returns_the_deployment_id() -> None:
    with client(lambda _: json_response({"deployment_id": "d42"})) as agent:
        assert agent.provision_load({"name": "web"}) == "d42"


def test_empty_body_is_accepted() -> None:
    # start/stop/kill answer 200 with no body; that must not be an error.
    with client(lambda _: httpx.Response(200)) as agent:
        agent.start_load("web")


def test_cloud_init_returns_text() -> None:
    body = "#cloud-config\nhostname: lab1\n"
    with client(lambda _: httpx.Response(200, text=body)) as agent:
        assert agent.cloud_init("lab1", "user-data") == body


def test_cloud_init_rejects_an_unknown_document() -> None:
    with (
        client(lambda _: httpx.Response(200)) as agent,
        pytest.raises(ValueError, match="data_type"),
    ):
        agent.cloud_init("lab1", "vendor-data")


# --- errors -----------------------------------------------------------------


def test_error_body_becomes_the_message() -> None:
    with (
        client(lambda _: httpx.Response(400, text="load 'web' not found\n")) as agent,
        pytest.raises(RealmError) as caught,
    ):
        agent.start_load("web")

    assert caught.value.status == 400
    assert str(caught.value) == "load 'web' not found"
    assert caught.value.body == "load 'web' not found"


def test_no_content_means_not_configured() -> None:
    with client(lambda _: httpx.Response(204)) as agent, pytest.raises(NodeNotConfigured):
        agent.node_config()


def test_conflict_means_already_configured() -> None:
    with (
        client(lambda _: httpx.Response(409, text="already configured")) as agent,
        pytest.raises(NodeAlreadyConfigured),
    ):
        agent.load_node_config({"name": "lab1"})


def test_stream_errors_are_raised_before_iterating() -> None:
    with (
        client(lambda _: httpx.Response(502, text="load not running")) as agent,
        pytest.raises(RealmError, match="load not running"),
    ):
        list(agent.stdout("web"))


# --- requests ---------------------------------------------------------------


def test_guest_node_uses_guest_path() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return json_response({"state": {}, "status": {}})

    with client(handler) as agent:
        agent.node(guest="vm1")
        agent.node()
        agent.unload_node_config(guest="vm/1")
        agent.unload_node_config()

    assert [(r.method, r.url.raw_path, r.url.params) for r in seen] == [
        ("GET", b"/node/guests/vm1", httpx.QueryParams()),
        ("GET", b"/node", httpx.QueryParams()),
        ("DELETE", b"/node/guests/vm%2F1/config", httpx.QueryParams()),
        ("DELETE", b"/node/config", httpx.QueryParams()),
    ]


def test_validate_only_is_sent_as_a_query_parameter() -> None:
    seen: list[httpx.URL] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.url)
        return httpx.Response(200)

    with client(handler) as agent:
        agent.load_node_config({"name": "lab1"}, validate_only=True)

    assert seen[0].params["validate"] == "true"


def test_load_names_are_escaped_into_the_path() -> None:
    seen: list[str] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.url.raw_path.decode())
        return httpx.Response(200)

    with client(handler) as agent:
        agent.start_load("web/../../etc")

    assert seen[0] == "/loads/web%2F..%2F..%2Fetc/start"


def test_shutdown_sends_message_and_delay() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200)

    with client(handler) as agent:
        agent.shutdown("going down", delay=5)
        agent.shutdown("going down", delay=5, guest="lab1")

    assert [r.url.raw_path for r in seen] == [b"/node/shutdown", b"/node/guests/lab1/shutdown"]
    assert all(json.loads(r.content) == {"wall_message": "going down", "time": 5} for r in seen)


def test_power_operations_target_guest_path() -> None:
    seen: list[bytes] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.url.raw_path)
        return httpx.Response(200)

    with client(handler) as agent:
        agent.power_on({"name": "vm1"}, guest="vm1")
        agent.power_off(guest="vm1")
        agent.restart(guest="vm1")
        agent.power_off()

    assert seen == [
        b"/node/guests/vm1/poweron",
        b"/node/guests/vm1/poweroff",
        b"/node/guests/vm1/restart",
        b"/node/poweroff",
    ]


# --- streaming --------------------------------------------------------------


def test_stdout_yields_lines() -> None:
    with client(lambda _: httpx.Response(200, text="first\nsecond\n")) as agent:
        assert list(agent.stdout("web")) == ["first", "second"]


def test_run_job_yields_ndjson_results() -> None:
    body = '{"value":"step one"}\n{"value":"step two"}\n{"err":"boom"}\n'
    with client(lambda _: httpx.Response(200, text=body)) as agent:
        results = list(agent.run_job({"name": "hello", "driver": "hello"}))

    assert results == [{"value": "step one"}, {"value": "step two"}, {"err": "boom"}]


def test_artifacts_listing_and_download() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/artifacts/raw":
            return json_response(["kernel", "rootfs.ext4"])
        return httpx.Response(200, content=b"binary-bytes")

    with client(handler) as agent:
        assert agent.artifacts() == ["kernel", "rootfs.ext4"]
        assert agent.download_artifact("rootfs.ext4") == b"binary-bytes"


# --- async ------------------------------------------------------------------


async def test_async_client_returns_the_same_results() -> None:
    async with async_client(lambda _: json_response({"version": "v1"})) as agent:
        assert await agent.version() == "v1"


async def test_async_streams_job_results() -> None:
    body = '{"value":"one"}\n{"value":"two"}\n'
    async with async_client(lambda _: httpx.Response(200, text=body)) as agent:
        results = [result async for result in agent.run_job({"name": "hello"})]

    assert results == [{"value": "one"}, {"value": "two"}]


async def test_async_raises_realm_error() -> None:
    async with async_client(lambda _: httpx.Response(400, text="nope")) as agent:
        with pytest.raises(RealmError, match="nope"):
            await agent.start_load("web")


async def test_async_stream_errors_are_raised_before_iterating() -> None:
    async with async_client(lambda _: httpx.Response(502, text="not running")) as agent:
        with pytest.raises(RealmError, match="not running"):
            [line async for line in agent.stdout("web")]


def test_null_listings_become_empty_lists() -> None:
    # An empty Go slice marshals to `null`; callers should still get a list.
    null = httpx.Response(200, content=b"null", headers={"content-type": "application/json"})
    with client(lambda _: null) as agent:
        assert agent.loads() == []
        assert agent.images() == []
        assert agent.artifacts() == []

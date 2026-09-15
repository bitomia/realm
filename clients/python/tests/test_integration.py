"""
End-to-end tests against a real agent, reached over its unix socket:
  pytest -m integration clients/python
"""

from __future__ import annotations

import os
import shutil
import socket
import subprocess
import time
from collections.abc import Iterator
from contextlib import contextmanager
from pathlib import Path

import pytest

from realm import NodeNotConfigured, Realm, RealmError

REPO_ROOT = Path(__file__).resolve().parents[3]
AGENT_BINARY = REPO_ROOT / "bin" / "realm"
STARTUP_TIMEOUT = 30.0

pytestmark = pytest.mark.integration


def agent_binary() -> Path:
    if AGENT_BINARY.exists():
        return AGENT_BINARY
    found = shutil.which("realm")
    if found:
        return Path(found)
    pytest.skip(f"realm binary not found at {AGENT_BINARY}")


def wait_for_socket(path: Path, process: subprocess.Popen[bytes]) -> None:
    deadline = time.monotonic() + STARTUP_TIMEOUT
    while time.monotonic() < deadline:
        if process.poll() is not None:
            output = (process.stdout.read().decode() if process.stdout else "").strip()
            pytest.fail(f"agent exited with {process.returncode}:\n{output}")
        if path.exists():
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as probe:
                try:
                    probe.connect(str(path))
                    return
                except OSError:
                    pass
        time.sleep(0.1)
    pytest.fail(f"agent did not create {path} within {STARTUP_TIMEOUT}s")


@pytest.fixture(scope="module")
def agent_socket(tmp_path_factory: pytest.TempPathFactory) -> Iterator[Path]:
    binary = agent_binary()
    workdir = tmp_path_factory.mktemp("agent")
    socket_path = workdir / "agent.sock"

    config = workdir / "config.yaml"
    config.write_text(
        f"data_path: {workdir / 'data'}\n"
        "agent:\n"
        "  disable_tcp: true\n"
        f"  listen_socket: {socket_path}\n"
        "  log_level: warn\n"
    )

    env = {**os.environ, "REALM_CONFIG_FILE": str(config)}
    env.pop("JWT_SECRET", None)  # the fixture agent runs without auth
    env.pop("REALM_BEARER", None)

    process = subprocess.Popen(
        [str(binary), "agent", "start", "--config", str(config)],
        cwd=workdir,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    try:
        wait_for_socket(socket_path, process)
        yield socket_path
    finally:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()


@pytest.fixture
def agent(agent_socket: Path) -> Iterator[Realm]:
    with Realm(socket=str(agent_socket)) as client:
        yield client


@contextmanager
def configured(agent: Realm) -> Iterator[None]:
    """Load a node configuration for the duration of a test.

    Endpoints that report on the node itself fail until one is loaded, so
    tests that need one push it and unload it again, leaving the shared agent
    as they found it.
    """
    agent.load_node_config({"name": "test-node", "url": "http://127.0.0.1:9000", "driver": "linux"})
    try:
        yield
    finally:
        agent.unload_node_config()


def test_version_over_the_socket(agent: Realm) -> None:
    version = agent.version()
    assert version
    assert isinstance(version, str)


def test_system_info_reports_capabilities(agent: Realm) -> None:
    info = agent.system()
    assert info["os_name"]
    assert "containers_engine" in info["capabilities"]


def test_loads_is_an_empty_list_when_nothing_is_deployed(agent: Realm) -> None:
    # The agent answers `null` rather than `[]`; the client normalises it.
    assert agent.loads() == []


def test_images_is_a_list(agent: Realm) -> None:
    if not agent.system()["capabilities"]["containers_engine"]:
        pytest.skip("no reachable containerd on this host")
    assert isinstance(agent.images(), list)


def test_node_config_is_absent_until_pushed(agent: Realm) -> None:
    with pytest.raises(NodeNotConfigured):
        agent.node_config()


def test_node_config_round_trip(agent: Realm) -> None:
    with configured(agent):
        # GET /node/config answers with the node's driver config, not the
        # whole node definition.
        config = agent.node_config()
        assert config["driver"] == "linux"

    with pytest.raises(NodeNotConfigured):
        agent.node_config()


def test_node_state_has_cpu_and_memory(agent: Realm) -> None:
    with configured(agent):
        state = agent.node()["state"]

    assert state["ncpu"] >= 1
    assert state["mem_total"] > 0


def test_unknown_load_reports_an_error(agent: Realm) -> None:
    with pytest.raises(RealmError) as caught:
        agent.start_load("does-not-exist")

    assert caught.value.status >= 400
    assert caught.value.body


def test_socket_is_group_restricted(agent_socket: Path) -> None:
    assert agent_socket.stat().st_mode & 0o777 == 0o660


def test_socket_is_removed_on_shutdown(tmp_path: Path) -> None:
    """A clean shutdown must not leave the socket behind."""
    binary = agent_binary()
    socket_path = tmp_path / "agent.sock"
    config = tmp_path / "config.yaml"
    config.write_text(
        f"data_path: {tmp_path / 'data'}\n"
        "agent:\n"
        "  disable_tcp: true\n"        
        f"  listen_socket: {socket_path}\n"
        "  log_level: warn\n"
    )

    process = subprocess.Popen(
        [str(binary), "agent", "start", "--config", str(config)],
        cwd=tmp_path,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    wait_for_socket(socket_path, process)
    process.terminate()
    process.wait(timeout=10)

    assert not socket_path.exists()

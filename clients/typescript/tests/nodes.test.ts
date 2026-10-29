import { mkdtempSync, rmSync } from 'node:fs';
import { createServer, type IncomingMessage, type RequestListener, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  Client,
  ErrNodeAlreadyConfigured,
  ErrNodeConfigMismatch,
  ErrNodesValidation,
  RequestError,
} from '../src/client.ts';
import type { Config, NodeConfig } from '../src/models/config.ts';

type Received = { method?: string; url?: string; body: string };

// readBody resolves with the full request body as a string.
async function readBody(req: IncomingMessage): Promise<string> {
  let body = '';
  for await (const chunk of req) body += chunk;
  return body;
}

// startServer serves handler on a random local port and resolves with its URL.
async function startServer(handler: RequestListener): Promise<{ url: string; server: Server }> {
  const server = createServer(handler);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  return { url: `http://127.0.0.1:${port}`, server };
}

const cfg: Config = { nodes: {}, loads: {}, jobs: {} };

describe('nodes', () => {
  let tempHome: string;
  let server: Server | undefined;
  let url: string;
  let received: Received[];
  let reply: (req: Received) => { status: number; body?: string };

  beforeEach(async () => {
    tempHome = mkdtempSync(join(tmpdir(), 'realm-client-'));
    vi.stubEnv('HOME', tempHome);
    vi.stubEnv('REALM_BEARER', undefined);

    received = [];
    reply = () => ({ status: 200 });
    ({ url, server } = await startServer(async (req, res) => {
      const r = { method: req.method, url: req.url, body: await readBody(req) };
      received.push(r);
      const { status, body } = reply(r);
      res.writeHead(status).end(body);
    }));
  });

  afterEach(() => {
    server?.closeAllConnections();
    server?.close();
    server = undefined;
    vi.unstubAllEnvs();
    rmSync(tempHome, { recursive: true, force: true });
  });

  const node = (name: string, driver = 'linux'): NodeConfig => ({
    name,
    url,
    driver,
    driver_config: { key: name },
  });

  describe('loadNodeConfig', () => {
    it('posts the node config', async () => {
      reply = () => ({ status: 201 });
      const n = node('n1');

      await new Client(cfg).loadNodeConfig(n);

      expect(received).toEqual([{ method: 'POST', url: '/node/config', body: JSON.stringify(n) }]);
    });

    it('rejects with ErrNodeAlreadyConfigured on 200', async () => {
      reply = () => ({ status: 200, body: 'node already configured' });

      await expect(new Client(cfg).loadNodeConfig(node('n1'))).rejects.toBe(
        ErrNodeAlreadyConfigured,
      );
    });

    it('rejects with ErrNodeConfigMismatch on 409', async () => {
      reply = () => ({ status: 409, body: 'conflict' });

      await expect(new Client(cfg).loadNodeConfig(node('n1'))).rejects.toBe(
        ErrNodeConfigMismatch,
      );
    });

    it('rejects with the request error on other failures', async () => {
      reply = () => ({ status: 500, body: 'boom' });

      const err = await new Client(cfg).loadNodeConfig(node('n1')).catch((e) => e);

      expect(err).toBeInstanceOf(RequestError);
      expect(err).toMatchObject({ message: 'boom', status: 500 });
    });
  });

  describe('validateNodeConfig', () => {
    it('posts the node config for validation', async () => {
      reply = () => ({ status: 201 });
      const n = node('n1');

      await new Client(cfg).validateNodeConfig(n);

      expect(received).toEqual([
        { method: 'POST', url: '/node/config?validate=true', body: JSON.stringify(n) },
      ]);
    });

    it('rejects with ErrNodeAlreadyConfigured on 200', async () => {
      reply = () => ({ status: 200, body: 'node already configured' });

      await expect(new Client(cfg).validateNodeConfig(node('n1'))).rejects.toBe(
        ErrNodeAlreadyConfigured,
      );
    });

    it('rejects with ErrNodeConfigMismatch on 409', async () => {
      reply = () => ({ status: 409, body: 'conflict' });

      await expect(new Client(cfg).validateNodeConfig(node('n1'))).rejects.toBe(
        ErrNodeConfigMismatch,
      );
    });

    it('rejects with the request error on other failures', async () => {
      reply = () => ({ status: 400, body: 'invalid' });

      const err = await new Client(cfg).validateNodeConfig(node('n1')).catch((e) => e);

      expect(err).toBeInstanceOf(RequestError);
      expect(err).toMatchObject({ message: 'invalid', status: 400 });
    });
  });

  describe('unloadNodeConfig', () => {
    it('deletes the node config', async () => {
      await new Client(cfg).unloadNodeConfig(node('n1'));

      expect(received).toEqual([{ method: 'DELETE', url: '/node/config', body: '' }]);
    });

    it('deletes the guest node config for guest drivers', async () => {
      await new Client(cfg).unloadNodeConfig(node('my vm', 'vm'));

      expect(received).toEqual([
        { method: 'DELETE', url: '/node/guests/my%20vm/config', body: '' },
      ]);
    });

    it('rejects with the request error on failure', async () => {
      reply = () => ({ status: 500, body: 'boom' });

      await expect(new Client(cfg).unloadNodeConfig(node('n1'))).rejects.toMatchObject({
        message: 'boom',
        status: 500,
      });
    });
  });

  describe('validateNodesConfig', () => {
    it('handles every node and resolves when all are valid', async () => {
      reply = () => ({ status: 201 });
      const nodes = { n1: node('n1'), n2: node('n2') };
      const handle = vi.fn();

      await new Client(cfg).validateNodesConfig(nodes, handle);

      expect(handle.mock.calls).toEqual([
        [nodes.n1, undefined],
        [nodes.n2, undefined],
      ]);
    });

    it('handles every node and rejects with ErrNodesValidation when any is invalid', async () => {
      reply = (r) => (r.body.includes('"n1"') ? { status: 400, body: 'invalid' } : { status: 201 });
      const nodes = { n1: node('n1'), n2: node('n2') };
      const handle = vi.fn();

      await expect(new Client(cfg).validateNodesConfig(nodes, handle)).rejects.toBe(
        ErrNodesValidation,
      );

      expect(handle).toHaveBeenCalledTimes(2);
      expect(handle.mock.calls[0][0]).toBe(nodes.n1);
      expect(handle.mock.calls[0][1]).toMatchObject({ message: 'invalid', status: 400 });
      expect(handle.mock.calls[1]).toEqual([nodes.n2, undefined]);
    });
  });

  describe('loadNodesConfig', () => {
    it('loads every node and handles each result', async () => {
      reply = (r) => (r.body.includes('"n2"') ? { status: 200 } : { status: 201 });
      const nodes = { n1: node('n1'), n2: node('n2') };
      const handle = vi.fn();

      await new Client(cfg).loadNodesConfig(nodes, handle);

      expect(received.map((r) => r.url)).toEqual(['/node/config', '/node/config']);
      expect(handle.mock.calls).toEqual([
        [nodes.n1, undefined],
        [nodes.n2, ErrNodeAlreadyConfigured],
      ]);
    });
  });

  describe('unloadNodesConfig', () => {
    it('unloads every node and handles each result', async () => {
      reply = (r) => (r.url === '/node/config' ? { status: 500, body: 'boom' } : { status: 200 });
      const nodes = { n1: node('n1'), vm1: node('vm1', 'vm') };
      const handle = vi.fn();

      await new Client(cfg).unloadNodesConfig(nodes, handle);

      expect(received.map((r) => r.url)).toEqual(['/node/config', '/node/guests/vm1/config']);
      expect(handle).toHaveBeenCalledTimes(2);
      expect(handle.mock.calls[0][0]).toBe(nodes.n1);
      expect(handle.mock.calls[0][1]).toMatchObject({ message: 'boom', status: 500 });
      expect(handle.mock.calls[1]).toEqual([nodes.vm1, undefined]);
    });
  });
});

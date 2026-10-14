import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type RequestListener, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Client, RequestError } from '../src/index.ts';
import type { Config } from '../src/models/config.ts';

class TestClient extends Client {
  getHeaders(): Record<string, string> {
    return this.headers;
  }

  getConfig(): Config {
    return this.cfg;
  }

  request(...args: Parameters<Client['doRequest']>): ReturnType<Client['doRequest']> {
    return this.doRequest(...args);
  }

  jsonRequest(...args: Parameters<Client['doJSONRequest']>): ReturnType<Client['doJSONRequest']> {
    return this.doJSONRequest(...args);
  }

  streamRequest(
    ...args: Parameters<Client['doStreamRequest']>
  ): ReturnType<Client['doStreamRequest']> {
    return this.doStreamRequest(...args);
  }
}

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

describe('client', () => {
  let tempHome: string;
  let server: Server | undefined;

  beforeEach(() => {
    tempHome = mkdtempSync(join(tmpdir(), 'realm-client-'));
    vi.stubEnv('HOME', tempHome);
    vi.stubEnv('REALM_BEARER', undefined);
  });

  afterEach(() => {
    server?.closeAllConnections();
    server?.close();
    server = undefined;
    vi.unstubAllEnvs();
    rmSync(tempHome, { recursive: true, force: true });
  });

  it('stores the config', () => {
    expect(new TestClient(cfg).getConfig()).toBe(cfg);
  });

  it('omits the authorization header without a token', () => {
    expect(new TestClient(cfg).getHeaders()).toEqual({
      'Content-Type': 'application/json',
    });
  });

  it('uses the token from REALM_BEARER', () => {
    vi.stubEnv('REALM_BEARER', 'env-token');

    expect(new TestClient(cfg).getHeaders()).toEqual({
      'Content-Type': 'application/json',
      Authorization: 'Bearer env-token',
    });
  });

  it('uses the token from ~/.realmrc', () => {
    writeFileSync(join(tempHome, '.realmrc'), '  file-token\n');

    expect(new TestClient(cfg).getHeaders()).toEqual({
      'Content-Type': 'application/json',
      Authorization: 'Bearer file-token',
    });
  });

  it('prefers REALM_BEARER over ~/.realmrc', () => {
    vi.stubEnv('REALM_BEARER', 'env-token');
    writeFileSync(join(tempHome, '.realmrc'), 'file-token');

    expect(new TestClient(cfg).getHeaders()['Authorization']).toBe('Bearer env-token');
  });

  it('omits the authorization header when REALM_BEARER is empty', () => {
    vi.stubEnv('REALM_BEARER', '');
    writeFileSync(join(tempHome, '.realmrc'), 'file-token');

    expect(new TestClient(cfg).getHeaders()).not.toHaveProperty('Authorization');
  });

  describe('doRequest', () => {
    it('sends the method, path, headers and payload', async () => {
      vi.stubEnv('REALM_BEARER', 'env-token');
      let received: { method?: string; url?: string; headers?: object; body?: string } = {};
      let url: string;
      ({ url, server } = await startServer((req, res) => {
        let body = '';
        req.on('data', (chunk) => (body += chunk));
        req.on('end', () => {
          received = { method: req.method, url: req.url, headers: req.headers, body };
          res.end();
        });
      }));

      await new TestClient(cfg).request('POST', url, '/loads/web', '{"name":"web"}');

      expect(received).toMatchObject({
        method: 'POST',
        url: '/loads/web',
        headers: { 'content-type': 'application/json', authorization: 'Bearer env-token' },
        body: '{"name":"web"}',
      });
    });

    it('resolves with the body and status on 200', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => res.end('{"ok":true}')));

      const res = await new TestClient(cfg).request('GET', url, '/node');

      expect(res.status).toBe(200);
      expect(new TextDecoder().decode(res.body)).toBe('{"ok":true}');
    });

    it('rejects with the raw body and status on non-200', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => {
        res.statusCode = 404;
        res.end('load not found');
      }));

      const err = await new TestClient(cfg).request('GET', url, '/loads/x').catch((e) => e);

      expect(err).toBeInstanceOf(RequestError);
      expect(err).toMatchObject({ status: 404, message: 'load not found' });
    });

    it('treats non-200 success codes as errors', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => {
        res.statusCode = 204;
        res.end();
      }));

      await expect(new TestClient(cfg).request('GET', url, '/node/config')).rejects.toMatchObject({
        status: 204,
      });
    });

    it('rejects with status -1 when the agent is unreachable', async () => {
      // Grab a free port, then close the server so nothing listens on it
      let url: string;
      ({ url, server } = await startServer(() => {}));
      await new Promise((resolve) => server!.close(resolve));

      await expect(new TestClient(cfg).request('GET', url, '/node')).rejects.toMatchObject({
        status: -1,
        message: expect.stringContaining('failed to make request'),
      });
    });

    it('rejects with status -1 on an invalid agent url', async () => {
      await expect(new TestClient(cfg).request('GET', 'ftp://agent', '/node')).rejects.toMatchObject({
        status: -1,
        message: expect.stringContaining('failed to create request'),
      });
    });

    it('rejects when the response exceeds the timeout', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => {
        res.writeHead(200);
        res.write('partial');
      }));

      await expect(
        new TestClient(cfg).request('GET', url, '/node', undefined, 50),
      ).rejects.toMatchObject({
        status: 200,
        message: expect.stringContaining('timeout of 50ms exceeded'),
      });
    });

    it('connects through a unix socket', async () => {
      const socketPath = join(tempHome, 'agent.sock');
      server = createServer((req, res) => res.end(`unix ${req.url}`));
      await new Promise<void>((resolve) => server!.listen(socketPath, resolve));

      const res = await new TestClient(cfg).request('GET', `unix://${socketPath}`, '/node');

      expect(new TextDecoder().decode(res.body)).toBe('unix /node');
    });
  });

  describe('doJSONRequest', () => {
    it('sends the payload encoded as JSON', async () => {
      let received: { method?: string; url?: string; body?: unknown } = {};
      let url: string;
      ({ url, server } = await startServer(async (req, res) => {
        received = { method: req.method, url: req.url, body: JSON.parse(await readBody(req)) };
        res.end();
      }));

      await new TestClient(cfg).jsonRequest('POST', url, '/jobs', { name: 'backup', arguments: ['-v'] });

      expect(received).toEqual({
        method: 'POST',
        url: '/jobs',
        body: { name: 'backup', arguments: ['-v'] },
      });
    });

    it('sends no body without a payload', async () => {
      let body: string | undefined;
      let url: string;
      ({ url, server } = await startServer(async (req, res) => {
        body = await readBody(req);
        res.end();
      }));

      await new TestClient(cfg).jsonRequest('POST', url, '/node/start');

      expect(body).toBe('');
    });

    it('resolves with the body and status on 200', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => res.end('{"ok":true}')));

      const res = await new TestClient(cfg).jsonRequest('POST', url, '/jobs', {});

      expect(res.status).toBe(200);
      expect(new TextDecoder().decode(res.body)).toBe('{"ok":true}');
    });

    it('rejects with the raw body and status on non-200', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => {
        res.statusCode = 400;
        res.end('invalid job');
      }));

      await expect(
        new TestClient(cfg).jsonRequest('POST', url, '/jobs', {}),
      ).rejects.toMatchObject({ status: 400, message: 'invalid job' });
    });

    it('rejects with status -1 when the payload cannot be encoded', async () => {
      const payload: Record<string, unknown> = {};
      payload['self'] = payload;

      const err = await new TestClient(cfg)
        .jsonRequest('POST', 'http://127.0.0.1:1', '/jobs', payload)
        .catch((e) => e);

      expect(err).toBeInstanceOf(RequestError);
      expect(err.status).toBe(-1);
    });
  });

  describe('doStreamRequest', () => {
    it('sends the method, path, headers and payload', async () => {
      vi.stubEnv('REALM_BEARER', 'env-token');
      let received: { method?: string; url?: string; headers?: object; body?: string } = {};
      let url: string;
      ({ url, server } = await startServer(async (req, res) => {
        received = { method: req.method, url: req.url, headers: req.headers, body: await readBody(req) };
        res.end();
      }));

      const res = await new TestClient(cfg).streamRequest('POST', url, '/jobs', '{"name":"backup"}');
      res.resume();

      expect(received).toMatchObject({
        method: 'POST',
        url: '/jobs',
        headers: { 'content-type': 'application/json', authorization: 'Bearer env-token' },
        body: '{"name":"backup"}',
      });
    });

    it('resolves before the response is complete', async () => {
      let finish!: () => void;
      let url: string;
      ({ url, server } = await startServer((_req, res) => {
        res.write('{"line":1}\n');
        finish = () => res.end('{"line":2}\n');
      }));

      const res = await new TestClient(cfg).streamRequest('GET', url, '/loads/web/stdout');
      const chunks: string[] = [];
      res.setEncoding('utf8');
      res.on('data', (chunk: string) => {
        chunks.push(chunk);
        if (chunks.length === 1) finish();
      });
      await new Promise((resolve) => res.on('end', resolve));

      expect(res.statusCode).toBe(200);
      expect(chunks).toEqual(['{"line":1}\n', '{"line":2}\n']);
    });

    it('rejects with the raw body and status on non-200', async () => {
      let url: string;
      ({ url, server } = await startServer((_req, res) => {
        res.statusCode = 500;
        res.end('job failed to start');
      }));

      const err = await new TestClient(cfg).streamRequest('POST', url, '/jobs').catch((e) => e);

      expect(err).toBeInstanceOf(RequestError);
      expect(err).toMatchObject({ status: 500, message: 'job failed to start' });
    });

    it('rejects with status -1 when the agent is unreachable', async () => {
      let url: string;
      ({ url, server } = await startServer(() => {}));
      await new Promise((resolve) => server!.close(resolve));

      await expect(new TestClient(cfg).streamRequest('GET', url, '/jobs')).rejects.toMatchObject({
        status: -1,
        message: expect.stringContaining('failed to make request'),
      });
    });

    it('rejects with status -1 on an invalid agent url', async () => {
      await expect(
        new TestClient(cfg).streamRequest('GET', 'ftp://agent', '/jobs'),
      ).rejects.toMatchObject({
        status: -1,
        message: expect.stringContaining('failed to create request'),
      });
    });

    it('connects through a unix socket', async () => {
      const socketPath = join(tempHome, 'agent.sock');
      server = createServer((req, res) => res.end(`unix ${req.url}`));
      await new Promise<void>((resolve) => server!.listen(socketPath, resolve));

      const res = await new TestClient(cfg).streamRequest('GET', `unix://${socketPath}`, '/jobs');
      res.setEncoding('utf8');
      let body = '';
      for await (const chunk of res) body += chunk;

      expect(body).toBe('unix /jobs');
    });
  });
});

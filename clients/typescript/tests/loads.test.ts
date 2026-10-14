import { mkdtempSync, rmSync } from 'node:fs';
import { createServer, type IncomingMessage, type RequestListener, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Client, RequestError } from '../src/client.ts';
import type { Config } from '../src/models/config.ts';
import type { LoadConfig, LoadsDeployments } from '../src/models/load.ts';

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

describe('loads', () => {
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

    const load = (name: string, dependsOn: string[] = [], node = 'n1'): LoadConfig => ({
        name,
        node,
        depends_on: dependsOn,
        driver: 'container',
        driver_config: { image: name },
    });

    const config = (...loads: LoadConfig[]): Config => ({
        nodes: { n1: { name: 'n1', url } },
        loads: Object.fromEntries(loads.map((l) => [l.name, l])),
        jobs: {},
    });

    // web depends on api, which depends on db.
    const chain = () => {
        const db = load('db');
        const api = load('api', ['db']);
        const web = load('web', ['api']);
        return { db, api, web, cfg: config(db, api, web) };
    };

    describe('provisionLoad', () => {
        it('posts the load config', async () => {
            const web = load('web');

            await new Client(config(web)).provisionLoad(web);

            expect(received).toEqual([
                { method: 'POST', url: '/loads/provision', body: JSON.stringify(web) },
            ]);
        });

        it('rejects with the request error on failure', async () => {
            reply = () => ({ status: 500, body: '' });
            const web = load('web');

            const err = await new Client(config(web)).provisionLoad(web).catch((e) => e);
            expect(err).toBeInstanceOf(RequestError);
        });
    });

    describe.each([
        ['startLoad', 'start'],
        ['stopLoad', 'stop'],
        ['killLoad', 'kill'],
        ['deprovisionLoad', 'deprovision'],
    ] as const)('%s', (method, action) => {
        it(`posts to the load ${action} endpoint`, async () => {
            const web = load('web');

            await new Client(config(web))[method](web);

            expect(received).toEqual([{ method: 'POST', url: `/loads/web/${action}`, body: '' }]);
        });
    });

    describe('getLoadsDeployments', () => {
        it('gets the deployments of the node', async () => {
            const deployments: LoadsDeployments = [
                {
                    load_name: 'web',
                    deployment_id: 'd1',
                    deployment_status: { status: 'running', reason: '' },
                    driver: 'container',
                    driver_config: {},
                },
            ];
            reply = () => ({ status: 200, body: JSON.stringify(deployments) });

            const got = await new Client(config()).getLoadsDeployments(url);

            expect(got).toEqual(deployments);
            expect(received).toEqual([{ method: 'GET', url: '/loads', body: '' }]);
        });
    });

    describe('provisionLoads', () => {
        it('rejects when there are no loads', async () => {
            const handle = vi.fn();

            await expect(new Client(config()).provisionLoads({}, handle)).rejects.toThrow('no loads');

            expect(handle).not.toHaveBeenCalled();
            expect(received).toEqual([]);
        });
    });

    describe('deprovisionLoads', () => {
        it('deprovisions every load and handles each result', async () => {
            reply = (r) => (r.url === '/loads/web/deprovision' ? { status: 500, body: 'boom' } : { status: 200 });
            const loads = { web: load('web'), db: load('db') };
            const handle = vi.fn();

            await new Client(config(loads.web, loads.db)).deprovisionLoads(loads, handle);

            expect(received.map((r) => r.url)).toEqual(['/loads/web/deprovision', '/loads/db/deprovision']);
            expect(handle).toHaveBeenCalledTimes(2);
        });
    });

    describe('startLoads', () => {
        it('starts each load of the start chains once', async () => {
            const { db, api, web, cfg } = chain();
            const handle = vi.fn();

            await new Client(cfg).startLoads({ web, api }, handle);

            expect(received.map((r) => r.url)).toEqual([
                '/loads/web/start',
                '/loads/api/start',
                '/loads/db/start',
            ]);
            expect(handle.mock.calls).toEqual([
                [web, undefined],
                [api, undefined],
                [db, undefined],
            ]);
        });
    });

    describe('stopLoads', () => {
        it('stops each load of the stop chains once', async () => {
            const { db, api, web, cfg } = chain();
            const handle = vi.fn();

            await new Client(cfg).stopLoads({ web, api }, handle);

            expect(received.map((r) => r.url)).toEqual([
                '/loads/db/stop',
                '/loads/api/stop',
                '/loads/web/stop',
            ]);
            expect(handle.mock.calls).toEqual([
                [db, undefined],
                [api, undefined],
                [web, undefined],
            ]);
        });
    });

    describe('killLoads', () => {
        it('kills each load of the stop chains once', async () => {
            const { web, cfg } = chain();

            await new Client(cfg).killLoads({ web }, vi.fn());

            expect(received.map((r) => r.url)).toEqual([
                '/loads/db/kill',
                '/loads/api/kill',
                '/loads/web/kill',
            ]);
        });
    });

    describe('getLoadsStates', () => {
        it('returns the state of every load sorted by name', async () => {
            const deployments: LoadsDeployments = [
                {
                    load_name: 'web',
                    deployment_id: 'd1',
                    deployment_status: { status: 'ready', reason: '' },
                    driver: 'container',
                    driver_config: {},
                    metadata: { container_name: 'web-1' },
                },
                {
                    load_name: 'web',
                    deployment_id: 'd2',
                    deployment_status: { status: 'running', reason: '' },
                    driver: 'container',
                    driver_config: {},
                    metadata: { container_name: 'web-2' },
                },
            ];
            reply = () => ({ status: 200, body: JSON.stringify(deployments) });
            const web = load('web');
            const worker = load('worker');
            const broken = load('broken', [], 'n2');
            const cfg = config(web, worker, broken);
            cfg.nodes.n2 = { name: 'n2', url: 'http://127.0.0.1:1' };

            const states = await new Client(cfg).getLoadsStates({ web, worker, broken });

            expect(states.map((s) => s.load)).toEqual([broken, web, worker]);
            const [brokenState, webState, workerState] = states;

            expect(brokenState.err).toBeInstanceOf(RequestError);
            expect(brokenState).toMatchObject({ provisioned: false, containerNames: [] });

            expect(webState).toEqual({
                load: web,
                err: undefined,
                provisioned: true,
                status: { status: 'running', reason: '' },
                containerNames: ['web-1', 'web-2'],
            });

            expect(workerState).toEqual({
                load: worker,
                err: undefined,
                provisioned: false,
                containerNames: [],
            });

            // Each node is queried once.
            expect(received).toEqual([{ method: 'GET', url: '/loads', body: '' }]);
        });
    });
});

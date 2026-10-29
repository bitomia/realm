import { request as httpRequest, type ClientRequest, type IncomingMessage } from 'node:http';
import { request as httpsRequest } from 'node:https';
import type { Writable } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { createInterface } from 'node:readline';
import { agentTargetURL, resolveAgentTarget } from './agent.ts';
import { getAuthToken } from './auth.ts';
import type { Config, JobConfig, NodeConfig } from './models/config.ts';
import type { DeploymentStatus, LoadConfig, LoadDeployment, LoadsDeployments } from './models/load.ts';
import type { JobResult } from './models/job.ts';
import type { JobRequest } from './models/job.dto.ts';
import type { SystemInfo } from './models/node.ts';

export const requestTimeout = 120_000; // ms

export type Response = {
    body: Uint8Array;
    status: number;
};

// RequestError carries the HTTP status code of a failed request, or -1 when
// the request failed before a response was received.
export class RequestError extends Error {
    readonly status: number;

    constructor(message: string, status: number, options?: ErrorOptions) {
        super(message, options);
        this.name = 'RequestError';
        this.status = status;
    }
}

export const ErrNodesValidation = new Error('node config validation failed');
export const ErrNodeAlreadyConfigured = new Error('node already configured');
export const ErrNodeConfigMismatch = new Error('node already configured with a different config');

export type NodeResultHandler = (node: NodeConfig, err?: Error) => void;

// LoadResultHandler is called after an operation has been run on a load,
// with the error returned by the operation if any.
export type LoadResultHandler = (load: LoadConfig, err?: Error) => void;

// LoadState is the deployment state of a configured load.
export type LoadState = {
    load: LoadConfig;
    err?: Error;
    provisioned: boolean;
    status?: DeploymentStatus;
    containerNames: string[];
};

// LoadsDependency is an edge of the loads dependency graph.
export type LoadsDependency = {
    from: string;
    to?: string;
};

export class Client {
    protected readonly headers: Record<string, string>;
    protected readonly cfg: Config;

    constructor(cfg: Config) {
        const token = getAuthToken();
        this.headers = { 'Content-Type': 'application/json' };
        if (token !== '') {
            this.headers['Authorization'] = `Bearer ${token}`;
        }
        this.cfg = cfg;
    }

    #newRequest(method: string, agentURL: string, path: string): ClientRequest {
        try {
            const target = resolveAgentTarget(agentURL);
            const url = new URL(agentTargetURL(target, path));
            const request = url.protocol === 'https:' ? httpsRequest : httpRequest;
            return request(url, { method, headers: this.headers, socketPath: target.socketPath });
        } catch (err) {
            throw new RequestError(`failed to create request: ${(err as Error).message}`, -1, {
                cause: err,
            });
        }
    }

    protected async doRequest(
        method: string,
        agentURL: string,
        path: string,
        payload?: string | Uint8Array,
        timeout: number = requestTimeout,
    ): Promise<Response> {
        const req = this.#newRequest(method, agentURL, path);

        return new Promise<Response>((resolve, reject) => {
            let status = -1;
            const fail = (message: string, err?: unknown) =>
                reject(new RequestError(message, status, { cause: err }));

            const timer = setTimeout(() => {
                req.destroy(new Error(`timeout of ${timeout}ms exceeded`));
            }, timeout);

            req.on('error', (err) => {
                clearTimeout(timer);
                fail(
                    status === -1
                        ? `failed to make request: ${err.message}`
                        : `failed to read response body: ${err.message}`,
                    err,
                );
            });

            req.on('response', (res: IncomingMessage) => {
                status = res.statusCode ?? -1;
                const chunks: Buffer[] = [];
                res.on('data', (chunk: Buffer) => chunks.push(chunk));
                res.on('error', (err) => {
                    clearTimeout(timer);
                    fail(`failed to read response body: ${err.message}`, err);
                });
                res.on('end', () => {
                    clearTimeout(timer);
                    const body = new Uint8Array(Buffer.concat(chunks));
                    if (status !== 200 && status !== 201) {
                        fail(new TextDecoder().decode(body));
                        return;
                    }
                    resolve({ body, status });
                });
            });

            req.end(payload);
        });
    }

    protected async doJSONRequest(
        method: string,
        agentURL: string,
        path: string,
        payload?: unknown,
        timeout: number = requestTimeout,
    ): Promise<Response> {
        let body: string | undefined;
        if (payload !== undefined) {
            try {
                body = JSON.stringify(payload);
            } catch (err) {
                throw new RequestError((err as Error).message, -1, { cause: err });
            }
        }
        return this.doRequest(method, agentURL, path, body, timeout);
    }

    protected async doStreamRequest(
        method: string,
        agentURL: string,
        path: string,
        payload?: string | Uint8Array,
        timeout: number = requestTimeout,
    ): Promise<IncomingMessage> {
        const req = this.#newRequest(method, agentURL, path);

        return new Promise<IncomingMessage>((resolve, reject) => {
            const timer = setTimeout(() => {
                req.destroy(new Error(`timeout of ${timeout}ms exceeded`));
            }, timeout);

            req.on('error', (err) => {
                clearTimeout(timer);
                reject(new RequestError(`failed to make request: ${err.message}`, -1, { cause: err }));
            });

            req.on('response', (res: IncomingMessage) => {
                clearTimeout(timer);
                const status = res.statusCode ?? -1;
                if (status === 200) {
                    resolve(res);
                    return;
                }

                const chunks: Buffer[] = [];
                res.on('data', (chunk: Buffer) => chunks.push(chunk));
                res.on('error', () => reject(new RequestError(Buffer.concat(chunks).toString(), status)));
                res.on('end', () => reject(new RequestError(Buffer.concat(chunks).toString(), status)));
            });

            req.end(payload);
        });
    }

    // guestNodePath returns the agent path for a guest node, optionally followed by suffix.
    #guestNodePath(name: string, suffix: string): string {
        return '/node/guests/' + encodeURIComponent(name) + suffix;
    }

    async #runOnNodes(
        nodes: { [k: string]: NodeConfig },
        operation: (node: NodeConfig) => Promise<void>,
        handle: NodeResultHandler,
    ): Promise<void> {
        for (const node of Object.values(nodes)) {
            handle(node, await this.#tryOperation(() => operation(node)));
        }
    }

    async #runChains(
        loads: { [k: string]: LoadConfig },
        chain: (load: LoadConfig) => LoadConfig[],
        operation: (load: LoadConfig) => Promise<void>,
        handle: LoadResultHandler,
    ): Promise<void> {
        const done = new Set<string>();
        for (const load of Object.values(loads)) {
            for (const l of chain(load)) {
                if (done.has(l.name)) {
                    continue;
                }
                done.add(l.name);
                handle(l, await this.#tryOperation(() => operation(l)));
            }
        }
    }

    // startChain returns the loads reached by a depth-first traversal of the
    // dependencies of load, in visit order.
    #startChain(load: LoadConfig): LoadConfig[] {
        const chain: LoadConfig[] = [];
        const stack = [load.name];
        const visited = new Set<string>();
        while (stack.length > 0) {
            const name = stack.pop()!;
            if (visited.has(name)) {
                continue;
            }
            visited.add(name);
            if (name in this.cfg.loads) {
                chain.push(this.cfg.loads[name]);
                stack.push(...this.cfg.loads[name].depends_on);
            }
        }
        return chain;
    }

    // stopChain returns the start chain of load in reverse order.
    #stopChain(load: LoadConfig): LoadConfig[] {
        return this.#startChain(load).reverse();
    }

    async #tryOperation(operation: () => Promise<void>): Promise<Error | undefined> {
        try {
            await operation();
            return undefined;
        } catch (err) {
            return err as Error;
        }
    }

    // containerName extracts the container name from the metadata of a container deployment.
    #containerName(d: LoadDeployment): string {
        if (d.driver !== 'container' || d.metadata == null) {
            return '';
        }
        const metadata = d.metadata as { container_name?: unknown };
        return typeof metadata.container_name === 'string' ? metadata.container_name : '';
    }

    async #streamLoadOutput(load: LoadConfig, output: string, w: Writable, timeout: number): Promise<void> {
        const res = await this.doStreamRequest(
            'GET',
            this.cfg.nodes[load.node].url,
            `/loads/${load.name}/${output}`,
            undefined,
            timeout,
        );

        try {
            await pipeline(res, w, { end: false });
        } catch (err) {
            throw new Error(`failed to stream ${output}: ${(err as Error).message}`, { cause: err });
        }
    }

    async getSystemInfo(node: string, timeout: number = requestTimeout): Promise<SystemInfo> {
        const { body } = await this.doRequest('GET', node, '/system', undefined, timeout);

        try {
            return JSON.parse(new TextDecoder().decode(body)) as SystemInfo;
        } catch (err) {
            throw new Error(`failed to parse JSON: ${(err as Error).message}`, { cause: err });
        }
    }

    async getVersion(node: string, timeout: number = requestTimeout): Promise<string> {
        const { body } = await this.doRequest('GET', node, '/version', undefined, timeout);

        try {
            const res = JSON.parse(new TextDecoder().decode(body)) as { version: string };
            return res.version;
        } catch (err) {
            throw new Error(`failed to parse JSON: ${(err as Error).message}`, { cause: err });
        }
    }

    async loadNodeConfig(node: NodeConfig, timeout: number = requestTimeout): Promise<void> {
        let status: number;
        try {
            status = (await this.doJSONRequest('POST', node.url, '/node/config', node, timeout)).status;
        } catch (err) {
            if (err instanceof RequestError && err.status === 409) {
                throw ErrNodeConfigMismatch;
            }
            throw err;
        }
        if (status === 200) {
            throw ErrNodeAlreadyConfigured;
        }
    }

    async validateNodeConfig(node: NodeConfig, timeout: number = requestTimeout): Promise<void> {
        let status: number;
        try {
            status = (await this.doJSONRequest('POST', node.url, '/node/config?validate=true', node, timeout)).status;
        } catch (err) {
            if (err instanceof RequestError && err.status === 409) {
                throw ErrNodeConfigMismatch;
            }
            throw err;
        }
        if (status === 200) {
            throw ErrNodeAlreadyConfigured;
        }
    }

    async unloadNodeConfig(node: NodeConfig, timeout: number = requestTimeout): Promise<void> {
        let path = '/node/config';
        if (node.driver === 'vm') {
            path = this.#guestNodePath(node.name, '/config');
        }

        await this.doRequest('DELETE', node.url, path, undefined, timeout);
    }

    // validateNodesConfig validates the config of all given nodes, throwing
    // ErrNodesValidation if any of them is invalid.
    async validateNodesConfig(nodes: { [k: string]: NodeConfig }, handle: NodeResultHandler): Promise<void> {
        let valid = true;
        for (const node of Object.values(nodes)) {
            const err = await this.#tryOperation(() => this.validateNodeConfig(node));
            handle(node, err);
            if (err !== undefined && err !== ErrNodeAlreadyConfigured) {
                valid = false;
            }
        }
        if (!valid) {
            throw ErrNodesValidation;
        }
    }

    // loadNodesConfig loads the config to the given nodes.
    async loadNodesConfig(nodes: { [k: string]: NodeConfig }, handle: NodeResultHandler): Promise<void> {
        await this.#runOnNodes(nodes, (n) => this.loadNodeConfig(n), handle);
    }

    // unloadNodesConfig unloads the config from the given nodes.
    async unloadNodesConfig(nodes: { [k: string]: NodeConfig }, handle: NodeResultHandler): Promise<void> {
        await this.#runOnNodes(nodes, (n) => this.unloadNodeConfig(n), handle);
    }

    async provisionLoad(load: LoadConfig, timeout: number = requestTimeout): Promise<void> {
        await this.doJSONRequest('POST', this.cfg.nodes[load.node].url, '/loads/provision', load, timeout);
    }

    async deprovisionLoad(load: LoadConfig, timeout: number = requestTimeout): Promise<void> {
        await this.doRequest('POST', this.cfg.nodes[load.node].url, `/loads/${load.name}/deprovision`, undefined, timeout);
    }

    async startLoad(load: LoadConfig, timeout: number = requestTimeout): Promise<void> {
        await this.doRequest('POST', this.cfg.nodes[load.node].url, `/loads/${load.name}/start`, undefined, timeout);
    }

    async stopLoad(load: LoadConfig, timeout: number = requestTimeout): Promise<void> {
        await this.doRequest('POST', this.cfg.nodes[load.node].url, `/loads/${load.name}/stop`, undefined, timeout);
    }

    async killLoad(load: LoadConfig, timeout: number = requestTimeout): Promise<void> {
        await this.doRequest('POST', this.cfg.nodes[load.node].url, `/loads/${load.name}/kill`, undefined, timeout);
    }

    async getLoadsDeployments(nodeUrl: string, timeout: number = requestTimeout): Promise<LoadsDeployments> {
        const { body } = await this.doRequest('GET', nodeUrl, '/loads', undefined, timeout);

        try {
            return JSON.parse(new TextDecoder().decode(body)) as LoadsDeployments;
        } catch (err) {
            throw new Error(`failed to parse response: ${(err as Error).message}`, { cause: err });
        }
    }

    // provisionLoads provisions the given loads, stopping at the first failure.
    async provisionLoads(
        loads: { [k: string]: LoadConfig },
        handle: LoadResultHandler,
        timeout: number = requestTimeout,
    ): Promise<void> {
        if (Object.keys(loads).length === 0) {
            throw new Error('no loads');
        }
        for (const load of Object.values(loads)) {
            const err = await this.#tryOperation(() => this.provisionLoad(load, timeout));
            handle(load, err);
            if (err !== undefined) {
                throw err;
            }
        }
    }

    // deprovisionLoads deprovisions the given loads.
    async deprovisionLoads(
        loads: { [k: string]: LoadConfig },
        handle: LoadResultHandler,
        timeout: number = requestTimeout,
    ): Promise<void> {
        for (const load of Object.values(loads)) {
            handle(load, await this.#tryOperation(() => this.deprovisionLoad(load, timeout)));
        }
    }

    // startLoads starts the given loads following their start chains.
    async startLoads(
        loads: { [k: string]: LoadConfig },
        handle: LoadResultHandler,
        timeout: number = requestTimeout,
    ): Promise<void> {
        await this.#runChains(loads, (l) => this.#startChain(l), (l) => this.startLoad(l, timeout), handle);
    }

    // stopLoads stops the given loads following their stop chains.
    async stopLoads(
        loads: { [k: string]: LoadConfig },
        handle: LoadResultHandler,
        timeout: number = requestTimeout,
    ): Promise<void> {
        await this.#runChains(loads, (l) => this.#stopChain(l), (l) => this.stopLoad(l, timeout), handle);
    }

    // killLoads kills the given loads following their stop chains.
    async killLoads(
        loads: { [k: string]: LoadConfig },
        handle: LoadResultHandler,
        timeout: number = requestTimeout,
    ): Promise<void> {
        await this.#runChains(loads, (l) => this.#stopChain(l), (l) => this.killLoad(l, timeout), handle);
    }

    // getLoadsStates returns the deployment state of the given loads.
    async getLoadsStates(loads: { [k: string]: LoadConfig }, timeout: number = requestTimeout): Promise<LoadState[]> {
        type NodeDeployments = {
            byLoad: { [k: string]: LoadsDeployments };
            err?: Error;
        };
        const nodes: { [k: string]: NodeDeployments } = {};

        const states: LoadState[] = [];
        for (const name of Object.keys(loads).sort()) {
            const load = loads[name];
            const nodeUrl = this.cfg.nodes[load.node].url;

            let node = nodes[nodeUrl];
            if (node === undefined) {
                let deployments: LoadsDeployments = [];
                let err: Error | undefined;
                try {
                    deployments = await this.getLoadsDeployments(nodeUrl, timeout);
                } catch (e) {
                    err = e as Error;
                }
                node = { byLoad: {}, err };
                for (const d of deployments) {
                    (node.byLoad[d.load_name] ??= []).push(d);
                }
                nodes[nodeUrl] = node;
            }

            const state: LoadState = { load, err: node.err, provisioned: false, containerNames: [] };
            const deployments = node.byLoad[load.name];
            if (deployments !== undefined) {
                state.provisioned = true;
                for (const d of deployments) {
                    const containerName = this.#containerName(d);
                    if (containerName !== '') {
                        state.containerNames.push(containerName);
                    }
                    state.status = d.deployment_status;
                }
            }
            states.push(state);
        }
        return states;
    }

    async streamLoadStdout(load: LoadConfig, w: Writable, timeout: number = requestTimeout): Promise<void> {
        await this.#streamLoadOutput(load, 'stdout', w, timeout);
    }

    async streamLoadStderr(load: LoadConfig, w: Writable, timeout: number = requestTimeout): Promise<void> {
        await this.#streamLoadOutput(load, 'stderr', w, timeout);
    }

    async runJob(
        job: JobConfig,
        handle: (result: JobResult) => void,
        args: string[] = [],
        timeout: number = requestTimeout,
    ): Promise<void> {
        const request: JobRequest = {
            name: job.name,
            driver: job.driver,
            driver_config: job.driver_config,
            arguments: args,
        };

        const res = await this.doStreamRequest(
            'POST',
            this.cfg.nodes[job.node].url,
            '/jobs',
            JSON.stringify(request),
            timeout,
        );

        for await (const line of createInterface({ input: res, crlfDelay: Infinity })) {
            if (line.trim() === '') {
                continue;
            }
            let result: JobResult;
            try {
                result = JSON.parse(line) as JobResult;
            } catch (err) {
                throw new Error(`failed to parse job result: ${(err as Error).message}`, { cause: err });
            }
            handle(result);
        }
    }
}

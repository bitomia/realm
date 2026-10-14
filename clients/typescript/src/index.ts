import { request as httpRequest, type ClientRequest, type IncomingMessage } from 'node:http';
import { request as httpsRequest } from 'node:https';
import { agentTargetURL, resolveAgentTarget } from './agent.ts';
import { getAuthToken } from './auth.ts';
import type { Config } from './models/config.ts';

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
          if (status !== 200) {
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
  ): Promise<IncomingMessage> {
    const req = this.#newRequest(method, agentURL, path);

    return new Promise<IncomingMessage>((resolve, reject) => {
      req.on('error', (err) => {
        reject(new RequestError(`failed to make request: ${err.message}`, -1, { cause: err }));
      });

      req.on('response', (res: IncomingMessage) => {
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
}

export * from './agent.ts';
export * from './models/node.dto.ts';

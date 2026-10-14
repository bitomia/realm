import { isAbsolute } from 'node:path';

export type AgentTarget = {
  baseURL: string;
  socketPath?: string;
};

const unixHost = 'http://unix';

const socketPathError = 'socket path must be absolute, e.g. unix:///run/realm/agent.sock';

export function resolveAgentTarget(agentURL: string): AgentTarget {
  let parsed: URL;
  try {
    parsed = new URL(agentURL.trim());
  } catch (err) {
    throw new Error(`invalid agent url '${agentURL}': ${(err as Error).message}`, { cause: err });
  }

  const scheme = parsed.protocol.replace(/:$/, '');
  switch (scheme) {
    case 'http':
    case 'https':
      if (parsed.host === '') {
        throw new Error(`invalid agent url '${agentURL}': missing host`);
      }
      return { baseURL: agentURL.replace(/\/$/, '') };

    case 'unix': {
      const socketPath = decodeURIComponent(parsed.pathname);
      if (parsed.host !== '' || !isAbsolute(socketPath)) {
        throw new Error(`invalid agent url '${agentURL}': ${socketPathError}`);
      }
      return { baseURL: unixHost, socketPath };
    }

    default:
      throw new Error(
        `unsupported agent url scheme '${scheme}', expected one of http, https or unix`,
      );
  }
}

export function agentTargetURL(target: AgentTarget, path: string): string {
  return target.baseURL + path;
}

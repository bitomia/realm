import type { LoadConfig } from './load.ts';

export type NodeConfig = {
  name: string;
  url: string;
  cloud_init?: {[k: string]: unknown};
  driver?: string;
  driver_config?: unknown;
};

export type JobConfig = {
  name: string;
  node: string;
  driver: string;
  driver_config: unknown;
};

export type NodesConfig = {[k: string]: NodeConfig};
export type LoadsConfig = {[k: string]: LoadConfig};
export type JobsConfig = {[k: string]: JobConfig};

export type MeshTransport = 'udp' | 'websocket';

export type MeshConfig = {
  server: string;
  port?: number;
  transport?: MeshTransport;
  link_code: string;
};

export type Config = {
  nodes: NodesConfig;
  loads: LoadsConfig;
  jobs: JobsConfig;
  mesh?: MeshConfig;
};

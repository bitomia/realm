export type NodeConfig = {
  name: string;
  url: string;
  cloud_init?: Record<string, unknown>;
  driver?: string;
  driver_config?: unknown;
};

export type LoadConfig = {
  name: string;
  node: string;
  depends_on: string[];
  driver: string;
  driver_config: unknown;
  start_chain_hash: string;
  stop_chain_hash: string;
};

export type JobConfig = {
  name: string;
  node: string;
  driver: string;
  driver_config: unknown;
};

export type NodesConfig = Record<string, NodeConfig>;
export type LoadsConfig = Record<string, LoadConfig>;
export type JobsConfig = Record<string, JobConfig>;

export type MeshTransport = 'udp' | 'websocket';

export type MeshConfig = {
  server: string;
  port?: number; // Default: 5000
  transport?: MeshTransport;
  link_code: string;
};

export type Config = {
  nodes: NodesConfig;
  loads: LoadsConfig;
  jobs: JobsConfig;
  mesh?: MeshConfig;
};

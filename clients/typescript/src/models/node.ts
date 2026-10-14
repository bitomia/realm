// Node is the client-side view of a configured node.
export type Node = {
  name: string;
  url: string;
};

export type NodeStatusCode = 'offline' | 'online' | 'ready' | 'error';

export type NodeStatus = {
  status: NodeStatusCode;
  reason: string;
};

export type ContainerState = {
  container_id: string;
  cpu_usage: number; // percentage
  cpu_system: number;
  cpu_user: number;
  mem_usage: number;
  mem_limit: number;
  mem_percentage: number; // percentage
};

export type NetworkInterface = {
  name: string;
  hwaddr?: string;
  addresses?: string[];
};

export type NodeMetrics = {
  ncpu: number;
  cpu_user: number;
  cpu_idle: number;
  cpu_system: number;
  cpu_total: number;
  cpu_usage_percentage: number;

  mem_total: number;
  mem_used: number;
  mem_free: number;
  mem_free_percentage: number;

  free_storage: number;

  swap_total: number; // bytes
  swap_used: number; // bytes
  swap_free: number; // bytes

  cpu_load_avg: number; // from 0 to 1

  proc_total_count: number;
  proc_sleeping_count: number;
  proc_running_count: number;
  proc_zombie_count: number;
  proc_stopped_count: number;
  proc_idle_count: number;
  proc_threads_count: number;

  containers?: ContainerState[];
  network_interfaces?: NetworkInterface[];
};

export type NodeResponse = {
  state: NodeMetrics;
  status: NodeStatus;
};

export const OsKind = {
  Windows: 0,
  Linux: 1,
  Android: 2,
  Apple: 3,
  Unix: 4,
  Posix: 5,
  Other: 6,
} as const;
export type OsKind = (typeof OsKind)[keyof typeof OsKind];

export const CpuArch = {
  X86: 0,
  X64: 1,
  Arm2: 2,
  Arm3: 3,
  Arm4T: 4,
  Arm5: 5,
  Arm6T2: 6,
  Arm6: 7,
  Arm7: 8,
  Arm7a: 9,
  Arm7r: 10,
  Arm7s: 11,
  Arm64: 12,
  Mips: 13,
  SuperH: 14,
  Ppc: 15,
  Ppc64: 16,
  Sparc: 17,
  M68k: 18,
  Other: 19,
} as const;
export type CpuArch = (typeof CpuArch)[keyof typeof CpuArch];

export type Capabilities = {
  containers_engine: boolean;
  containers_networking: boolean;
  volumes: boolean;
  volumes_zfs: boolean;
  vmm: boolean;
};

export type SystemInfo = {
  os_name: string;
  os_kind: OsKind;
  os_arch: CpuArch;
  sys_vendor: string;
  net_default_gateway: string;
  net_host_name: string;
  net_domain_name: string;
  net_primary_dns: string;
  net_secondary_dns: string;
  net_primary_ip_addr: string;
  net_mac_address: string;
  cpu_model: string;
  cpu_start_time: number; // Unix timestamp in nanoseconds
  cpu_total_cores: number;
  cpu_cores_per_socket: number;
  cpu_total_sockets: number;
  cpu_vendor: string;
  cpu_mhz: number;
  ram_total: number; // bytes
  capabilities: Capabilities;
};

export type NodeState = {
  node: Node;
  response: NodeResponse;
  err?: Error;
  systemInfo?: SystemInfo;
  systemInfoErr?: Error;
};

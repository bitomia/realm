import type { NodeMetrics, NodeResponse } from './node.ts';

export type NodeDriverConfig = {
  driver: string;
  driver_config: unknown;
};

// GuestNodeResponse describes a guest node managed by the agent
export type GuestNodeResponse = NodeResponse & {
  name: string;
  config: NodeDriverConfig;
};

export function newNodeMetrics(): NodeMetrics {
  return {
    ncpu: 0,
    cpu_user: 0,
    cpu_idle: 0,
    cpu_system: 0,
    cpu_total: 0,
    cpu_usage_percentage: 0,
    mem_total: 0,
    mem_used: 0,
    mem_free: 0,
    mem_free_percentage: 0,
    free_storage: 0,
    swap_total: 0,
    swap_used: 0,
    swap_free: 0,
    cpu_load_avg: 0,
    proc_total_count: 0,
    proc_sleeping_count: 0,
    proc_running_count: 0,
    proc_zombie_count: 0,
    proc_stopped_count: 0,
    proc_idle_count: 0,
    proc_threads_count: 0,
    containers: [],
    network_interfaces: [],
  };
}

export function newNodeResponse(): NodeResponse {
  return {
    state: newNodeMetrics(),
    status: { status: 'offline', reason: '' },
  };
}

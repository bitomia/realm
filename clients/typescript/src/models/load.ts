export type LoadConfig = {
  name: string;
  node: string;
  depends_on: string[];
  driver: string;
  driver_config: unknown;
};

export type DeploymentStatusCode = 'not ready' | 'ready' | 'running' | 'stopped' | 'error';

export type DeploymentStatus = {
  status: DeploymentStatusCode;
  reason: string;
};

export type LoadDeployment = {
  load_name: string;
  deployment_id: string;
  deployment_status: DeploymentStatus;
  driver: string;
  driver_config: unknown;
  metadata?: unknown;
};

export type LoadsDeployments = LoadDeployment[];

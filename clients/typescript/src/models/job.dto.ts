export type JobDriverConfig = {
  driver: string;
  driver_config?: unknown;
};

export type JobRequest = JobDriverConfig & {
  name: string;
  arguments?: string[];
};

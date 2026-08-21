export type FieldType = 'text' | 'password' | 'int' | 'float' | 'bool' | 'enum';

export interface FieldOption {
  value: string;
  label: string;
}

export interface ProviderField {
  key: string;
  label: string;
  type: FieldType;
  default?: string;
  required?: boolean;
  secret?: boolean;
  min?: number;
  max?: number;
  options?: FieldOption[];
  help?: string;
  advanced?: boolean;
}

export interface ProviderPort {
  container: number;
  protocol: 'tcp' | 'udp';
  defaultHost: number;
  label: string;
  optional?: boolean;
}

export interface ProviderVolume {
  host: string;
  container: string;
  data?: boolean;
}

export interface Provider {
  id: string;
  game: string;
  gameLabel: string;
  short: string;
  image: string;
  description: string;
  docs?: string;
  ports: ProviderPort[] | null;
  volumes: ProviderVolume[];
  defaultMemory: string;
  minMemory: string;
  defaultCpus: number;
  stopGraceSeconds: number;
  fields: ProviderField[] | null;
}

export type State =
  | 'stopped'
  | 'provisioning'
  | 'starting'
  | 'running'
  | 'updating'
  | 'error'
  | 'archived';

export interface PortBinding {
  host: number;
  container: number;
  protocol: 'tcp' | 'udp';
  label?: string;
}

export interface Mount {
  host: string;
  container: string;
  data?: boolean;
}

export interface Stats {
  cpuPercent: number;
  memoryBytes: number;
  memoryLimit: number;
}

export interface Operation {
  kind: string;
  message: string;
  percent?: number;
  startedAt: string;
  error?: string;
}

export interface Instance {
  name: string;
  providerId: string;
  game: string;
  image: string;
  env: Record<string, string>;
  secretKeys?: string[];
  ports: PortBinding[];
  mounts: Mount[];
  memoryLimit: string;
  cpus: number;
  restart: string;
  stopGraceSeconds: number;
  archived?: boolean;
  createdAt: string;
  updatedAt: string;

  dir: string;
  state: State;
  status?: string;
  health?: string;
  exitCode?: number;
  stats?: Stats;
  operation?: Operation;
}

export interface InstancesResponse {
  instances: Instance[];
  states: State[];
}

export interface SystemInfo {
  memoryTotal: number;
  memoryAvailable: number;
  memoryUsed: number;
  diskTotal: number;
  diskFree: number;
  diskUsed: number;
  cpuCount: number;
  cpuPercent: number;
  dockerVersion?: string;
  dockerError?: string;
  memoryReserve: number;
  memoryBudget: number;
  memoryCommitted: number;
  memoryPlanned: number;
  instanceCount: number;
}

export interface SpecRequest {
  name: string;
  providerId: string;
  image?: string;
  values: Record<string, string>;
  ports?: PortBinding[];
  mounts?: Mount[];
  memoryLimit?: string;
  cpus?: number;
  restart?: string;
  secretKeys?: string[];
  start?: boolean;
}

export interface ComposePreview {
  compose: string;
  recreate?: string[];
}

export interface ApiError {
  error: string;
  message: string;
  problems?: string[];
}

export interface ServerEvent {
  type: string;
  instance?: string;
  message?: string;
}

export const STATE_META: Record<State, { title: string; dot: string }> = {
  stopped: { title: 'PARADO', dot: '#6d7280' },
  provisioning: { title: 'PROVISIONANDO', dot: '#6aa6f5' },
  starting: { title: 'INICIANDO', dot: '#e5b567' },
  running: { title: 'RODANDO', dot: '#4fd99b' },
  updating: { title: 'ATUALIZANDO', dot: '#9b8cf5' },
  error: { title: 'ERRO', dot: '#f08a8a' },
  archived: { title: 'ARQUIVADO', dot: '#4e535d' },
};

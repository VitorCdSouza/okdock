import { MessageKey } from './i18n/messages.pt';

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
  tags?: string[];
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

export interface InstanceDNS {
  domain: string;
  hostname: string;
  lastIp?: string;
  lastSync?: string;
  lastError?: string;
}

export interface DnsLink extends InstanceDNS {
  instance: string;
}

export interface DnsStatus {
  token: string;
  suffix: string;
  links: DnsLink[];
  domains: InstanceDNS[];
}

export interface Operation {
  kind: string;
  code?: string;
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
  dns?: InstanceDNS;
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
  root: string;
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

export interface ApiProblem {
  field: string;
  code: string;
  params?: Record<string, string | number>;
}

export interface ApiError {
  error: string;
  message: string;
  params?: Record<string, string | number>;
  problems?: ApiProblem[];
}

export interface ServerEvent {
  type: string;
  instance?: string;
  message?: string;
}

export const STATE_DOT: Record<State, string> = {
  stopped: '#6d7280',
  provisioning: '#6aa6f5',
  starting: '#e5b567',
  running: '#4fd99b',
  updating: '#9b8cf5',
  error: '#f08a8a',
  archived: '#4e535d',
};

export const STATE_KEY: Record<State, MessageKey> = {
  stopped: 'state.stopped',
  provisioning: 'state.provisioning',
  starting: 'state.starting',
  running: 'state.running',
  updating: 'state.updating',
  error: 'state.error',
  archived: 'state.archived',
};

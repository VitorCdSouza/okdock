import { MessageKey } from './i18n/messages.pt';

export type FieldType = 'text' | 'password' | 'int' | 'float' | 'bool' | 'enum';

export interface FieldOption {
  value: string;
  label: string;
}

export interface TemplateField {
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

export interface TemplatePort {
  container: number;
  protocol: 'tcp' | 'udp';
  defaultHost: number;
  label: string;
  optional?: boolean;
}

export interface TemplateVolume {
  host: string;
  container: string;
}

export type BuiltinCategory = 'games' | 'media' | 'database' | 'network' | 'utilities' | 'other';

// a template can carry a category the panel does not ship, so the type is open
export type Category = BuiltinCategory | (string & {});

export interface Template {
  id: string;
  name: string;
  category: Category;
  short: string;
  image: string;
  ports: TemplatePort[] | null;
  volumes: TemplateVolume[];
  defaultMemory: string;
  minMemory: string;
  defaultCpus: number;
  stopGraceSeconds: number;
  fields: TemplateField[] | null;
  freeEnv?: boolean;
  builtin?: boolean;
}

// one repository docker search found, and the registry search reports no tags
export interface ImageHit {
  name: string;
  description: string;
  stars: number;
  official?: boolean;
}

// what the panel could read out of the image, which never says what its entrypoint reads
export interface ImageSuggestion {
  ports: TemplatePort[];
  volumes: TemplateVolume[];
}

export interface TemplatesResponse {
  templates: Template[];
  categories: Category[];
}

export const CATEGORY_KEY: Record<BuiltinCategory, MessageKey> = {
  games: 'category.games',
  media: 'category.media',
  database: 'category.database',
  network: 'category.network',
  utilities: 'category.utilities',
  other: 'category.other',
};

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
  templateId: string;
  category: Category;
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

  networks?: string[];
  external?: boolean;
  project?: string;
  service?: string;
  editable?: boolean;
  composeFile?: string;
  readOnly?: string;
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
  templateId: string;
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

export const COLUMN_OF: Record<State, State> = {
  stopped: 'stopped',
  provisioning: 'running',
  starting: 'running',
  running: 'running',
  updating: 'updating',
  error: 'error',
  archived: 'archived',
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

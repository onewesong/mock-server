export type Endpoint = {
  id: string;
  name: string;
  method: string;
  pathPattern: string;
  enabled: boolean;
  tags: string[];
  description: string;
  createdAt: number;
  updatedAt: number;
};

export type Matcher = {
  source: string;
  key: string;
  op: string;
  value: string | string[];
  caseSensitive: boolean;
};

export type ResponseConfig = {
  status: number;
  headers: Record<string, string>;
  delayMs: number;
  bodyType: string;
  body: string;
  contentType: string;
};

export type Rule = {
  id: string;
  endpointId: string;
  name: string;
  enabled: boolean;
  priority: number;
  weight: number;
  matchers: Matcher[];
  response: ResponseConfig;
  createdAt: number;
  updatedAt: number;
};

export type PreviewRequest = {
  method: string;
  path: string;
  query: Record<string, string>;
  headers: Record<string, string>;
  body: string;
};

export type PreviewResponse = {
  matched: boolean;
  endpointId?: string;
  ruleId?: string;
  explain: string[];
  response?: ResponseConfig;
};

export type ExportBundle = {
  endpoints: Endpoint[];
  rules: Rule[];
};

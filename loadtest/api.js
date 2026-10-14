// k6 load test (PLAN §4 M2): browse, search and read issues under load.
//   KYBER_URL=http://localhost:8080 ISSUES=10000 k6 run loadtest/api.js
// Targets: 50 requests/s with p95 < 100 ms. Run against a dedicated instance with PostgreSQL;
// setup creates a fresh user and project and seeds ISSUES issues.
import http from "k6/http";
import { check } from "k6";

const BASE = __ENV.KYBER_URL || "http://localhost:8080";
const ISSUES = Number(__ENV.ISSUES || 1000);

export const options = {
  setupTimeout: "30m",
  scenarios: {
    browse: { executor: "constant-arrival-rate", rate: 50, timeUnit: "1s", duration: __ENV.DURATION || "2m", preAllocatedVUs: 20, maxVUs: 100 },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{kind:read}": ["p(95)<100"],
  },
};

const json = (token) => ({ headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) } });

export function setup() {
  const id = Date.now().toString(36).toUpperCase();
  const email = `load-${id.toLowerCase()}@example.com`;
  http.post(`${BASE}/api/v1/auth/signup`, JSON.stringify({ email, name: "Load", password: "a very long password" }), json());
  const token = http.post(`${BASE}/api/v1/auth/login`, JSON.stringify({ email, password: "a very long password" }), json()).json("token");
  const key = `L${id.slice(-4)}`;
  http.post(`${BASE}/api/v1/projects`, JSON.stringify({ key, name: "Load" }), json(token));
  const types = ["task", "bug", "story"];
  for (let i = 0; i < ISSUES; i++) {
    http.post(`${BASE}/api/v1/projects/${key}/issues`, JSON.stringify({ title: `Issue ${i}`, type: types[i % 3] }), json(token));
  }
  return { token, key };
}

export default function ({ token, key }) {
  const n = 1 + Math.floor(Math.random() * ISSUES);
  const reqs = [
    ["GET", `${BASE}/api/v1/issues/${key}-${n}`],
    ["GET", `${BASE}/api/v1/projects/${key}/issues?status=todo`],
    ["GET", `${BASE}/api/v1/search?jql=${encodeURIComponent(`project = ${key} AND type = bug ORDER BY key DESC`)}&limit=50`],
  ];
  const [method, url] = reqs[Math.floor(Math.random() * reqs.length)];
  const res = http.request(method, url, null, { ...json(token), tags: { kind: "read" } });
  check(res, { "status 200": (r) => r.status === 200 });
}

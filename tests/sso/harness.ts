// Provisioning harness for the two-app SSO smoke suite.
//
// Everything this file creates is throwaway: two databases with a random
// suffix, the fixed least-privilege runtime roles, two temp directories, the
// Go binaries, and the child processes it starts. Teardown drops exactly the
// objects created here — never anything pre-existing on the cluster.
//
// Safety guards (all fail-fast before any side effect):
//  - REFUSE to run from any path containing "verse-live";
//  - the Postgres admin DSN must point at 127.0.0.1 / ::1 / localhost;
//  - the required roles must not already exist on the cluster (a shared dev
//    cluster means a provisioned runtime role could already serve real data —
//    rotating its password here would be destructive, so the run refuses);
//  - HOME for every child is a fresh directory inside this run's tmp root.

import { execFile, spawn, type ChildProcess } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import net from 'node:net';
import { tmpdir } from 'node:os';
import path from 'node:path';

export const REPO = path.resolve(__dirname, '..', '..');
export const STATE_ENV = 'VERSE_SSO_STATE_DIR';

// ── guards ─────────────────────────────────────────────────────────────────

const FORBIDDEN = ['verse-live', '.ainovel', '.viderse'];
for (const token of FORBIDDEN) {
  if (REPO.includes(token) || tmpdir().includes(token)) {
    throw new Error(`sso fixture: refusing to run near live data (${token} in ${REPO})`);
  }
}

export function adminDSN(): string {
  const raw = process.env.VERSE_SSO_ADMIN_DSN || process.env.TEST_DATABASE_URL ||
    'postgres://dat@127.0.0.1:5432/postgres';
  let u: URL;
  try {
    u = new URL(raw);
  } catch {
    throw new Error(`sso fixture: admin DSN is not a URL`);
  }
  if (u.protocol !== 'postgres:' && u.protocol !== 'postgresql:') {
    throw new Error('sso fixture: admin DSN must be postgres://');
  }
  const host = u.hostname.toLowerCase();
  if (host !== '127.0.0.1' && host !== '::1' && host !== 'localhost' && host !== '[::1]') {
    throw new Error(`sso fixture: admin DSN host must be loopback, got ${u.hostname}`);
  }
  return raw;
}

// ── process helpers ────────────────────────────────────────────────────────

function sleep(ms: number): Promise<void> {
  const { promise, resolve } = Promise.withResolvers<void>();
  setTimeout(resolve, ms);
  return promise;
}

export function run(cmd: string, args: string[], opts: { env?: NodeJS.ProcessEnv; timeoutMs?: number; input?: string } = {}): Promise<string> {
  const { promise, resolve, reject } = Promise.withResolvers<string>();
  const child = execFile(cmd, args, {
    env: { ...process.env, ...opts.env },
    cwd: REPO,
    timeout: opts.timeoutMs ?? 120_000,
    maxBuffer: 8 * 1024 * 1024,
  }, (err, stdout, stderr) => {
    if (err) {
      reject(new Error(redact(`${cmd} failed: ${err.message}\n${stdout}\n${stderr}`).slice(0, 4000)));
    } else {
      resolve(stdout);
    }
  });
  child.stdin?.on('error', () => { /* process failures are reported by execFile */ });
  child.stdin?.end(opts.input);
  return promise;
}

// runExit is `run` for commands whose contract includes a non-zero exit: the
// caller asserts the code, so failure is data, not an exception. Output is
// redacted either way so assertions can safely print it.
export function runExit(cmd: string, args: string[], opts: { env?: NodeJS.ProcessEnv; timeoutMs?: number } = {}): Promise<{ code: number; out: string }> {
  const { promise, resolve } = Promise.withResolvers<{ code: number; out: string }>();
  execFile(cmd, args, {
    env: { ...process.env, ...opts.env },
    cwd: REPO,
    timeout: opts.timeoutMs ?? 120_000,
    maxBuffer: 8 * 1024 * 1024,
  }, (err, stdout, stderr) => {
    const out = redact(`${stdout}\n${stderr}`);
    if (err) {
      const ee = err as { code?: number };
      resolve({ code: typeof ee.code === 'number' ? ee.code : 126, out });
    } else {
      resolve({ code: 0, out });
    }
  });
  return promise;
}

// redact scrubs credential material from output before it can reach logs.
export function redact(text: string): string {
  return text
    .replace(/postgres(?:ql)?:\/\/[^\s/"']+/gi, 'postgres://<redacted>')
    .replace(/(Mã thiết lập:\s*)\S+/g, '$1<redacted>')
    .replace(/(DATABASE_URL|IDENTITY_DATABASE_URL|MIGRATION_DATABASE_URL)=\S+/g, '$1=<redacted>')
    .replace(/\bPASSWORD\s+'(?:''|[^'])*'/gi, 'PASSWORD <redacted>')
    .replace(/\$argon2id\$[^\s"'<>]+/g, '<password-hash>')
    .replace(/\b(?:[0-9a-f]{16}|[0-9a-f-]{36})\.[0-9a-f]{64}\b/gi, '<session-token>')
    .replace(/(password|PASSWORD|token|TOKEN)=\S+/g, '$1=<redacted>');
}

// The suite talks to the throwaway cluster it starts itself (initdb into the
// run dir) unless VERSE_SSO_ADMIN_DSN / TEST_DATABASE_URL points at an
// existing one. pgPort is set during provisioning.
let pgPort = '5432';
const psqlEnv = () => ({ PGHOST: '127.0.0.1', PGPORT: pgPort, PGUSER: 'dat' });

export async function psql(db: string, sql: string): Promise<string> {
  // Some provisioning statements carry role passwords: SQL belongs on stdin,
  // never in process argv or the execFile command attached to an error.
  return run('psql', ['-X', '-tA', '-v', 'ON_ERROR_STOP=1', '-d', db], {
    env: psqlEnv(), input: sql,
  });
}

export async function psqlFile(db: string, file: string, env: NodeJS.ProcessEnv = {}): Promise<string> {
  return run('psql', ['-X', '-tA', '-v', 'ON_ERROR_STOP=1', '-d', db, '-f', file], {
    env: { ...psqlEnv(), ...env },
  });
}

export async function psqlAs(connection: string, sql: string): Promise<string> {
  const target = new URL(connection);
  if (target.hostname !== '127.0.0.1' || !/^\/verse_sso_[a-z0-9_]+$/.test(target.pathname)) {
    throw new Error('sso fixture: runtime connection must target a loopback throwaway database');
  }
  return run('psql', ['-X', '-v', 'ON_ERROR_STOP=1', '-tA', '-c', sql], {
    env: {
      PGHOST: target.hostname, PGPORT: target.port,
      PGDATABASE: target.pathname.slice(1),
      PGUSER: decodeURIComponent(target.username), PGPASSWORD: decodeURIComponent(target.password),
    },
  });
}

export function dsn(user: string, password: string, db: string): string {
  return `postgres://${encodeURIComponent(user)}:${encodeURIComponent(password)}@127.0.0.1:${pgPort}/${db}`;
}

export function dsnUser(dsnText: string): string {
  try { return decodeURIComponent(new URL(dsnText).username); } catch { return ''; }
}

// ── child server manager ───────────────────────────────────────────────────

export interface ServerProc {
  name: string;
  proc: ChildProcess;
  pid: number;
  base: string;
  /** redacted output — safe to print in test failures */
  log: () => string;
  /** the one-time setup code parsed from stdout (throws until printed) */
  setupCode: () => string;
  stop: () => Promise<void>;
}

export async function killGroup(pid: number): Promise<void> {
  try { process.kill(-pid, 'SIGTERM'); } catch { try { process.kill(pid, 'SIGTERM'); } catch { /* gone */ } }
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    try { process.kill(pid, 0); } catch { return; } // gone
    await sleep(150);
  }
  try { process.kill(-pid, 'SIGKILL'); } catch { try { process.kill(pid, 'SIGKILL'); } catch { /* gone */ } }
}

function trackPid(name: string, pid: number): void {
  const dir = process.env[STATE_ENV];
  if (!dir) return;
  try {
    writeFileSync(path.join(dir, 'pids.txt'), `${name} ${pid}\n`, { flag: 'a' });
  } catch { /* state file is best effort */ }
}

const SETUP_CODE_RE = /Mã thiết lập:\s*([A-Z0-9]+-[A-Z0-9]+)/;

export function freePort(): Promise<number> {
  const { promise, resolve, reject } = Promise.withResolvers<number>();
  const srv = net.createServer();
  srv.once('error', reject);
  srv.listen(0, '127.0.0.1', () => {
    const port = (srv.address() as net.AddressInfo).port;
    srv.close(() => resolve(port));
  });
  return promise;
}

export function startServer(name: string, bin: string, args: string[], env: NodeJS.ProcessEnv): ServerProc {
  const root = process.env[STATE_ENV];
  if (!root || !path.basename(root).startsWith('verse-sso-')) {
    throw new Error('sso fixture: server HOME requires an owned temporary root');
  }
  const home = env.HOME ?? path.join(root, `home-${name}`);
  if (!path.resolve(home).startsWith(path.resolve(root) + path.sep)) {
    throw new Error('sso fixture: refusing a server HOME outside its temporary root');
  }
  mkdirSync(home, { recursive: true });
  const inherited = { ...process.env };
  for (const key of Object.keys(inherited)) {
    if (/^(VERSE_|VIDERSE_|NOVERSE_|DATABASE_URL$|MIGRATION_DATABASE_URL$|PUBLIC_URL$|SECURE_COOKIES$|TRUSTED_PROXIES$)/.test(key)) delete inherited[key];
  }
  let output = '';
  // detached: the child gets its own process group so teardown can SIGTERM the
  // whole tree (serve spawns __web-worker children).
  const proc = spawn(bin, args, {
    env: { ...inherited, ...env, HOME: home },
    detached: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const append = (chunk: Buffer) => {
    output += chunk.toString('utf8');
    if (output.length > 256 * 1024) output = output.slice(-128 * 1024);
  };
  proc.stdout!.on('data', append);
  proc.stderr!.on('data', append);
  const srv: ServerProc = {
    name,
    proc,
    pid: proc.pid!,
    base: '',
    log: () => redact(output),
    setupCode: () => {
      const m = SETUP_CODE_RE.exec(output);
      if (!m) throw new Error(`${name}: no setup code in output:\n${redact(output)}`);
      return m[1];
    },
    stop: async () => {
      if (proc.exitCode !== null || proc.signalCode) return;
      try { process.kill(-proc.pid!, 'SIGTERM'); } catch { proc.kill('SIGTERM'); }
      const deadline = Date.now() + 12_000;
      while (proc.exitCode === null && !proc.signalCode && Date.now() < deadline) {
        await sleep(100);
      }
      if (proc.exitCode === null && !proc.signalCode) {
        try { process.kill(-proc.pid!, 'SIGKILL'); } catch { proc.kill('SIGKILL'); }
      }
    },
  };
  trackPid(name, proc.pid!);
  return srv;
}

export async function waitReady(srv: ServerProc, path_ = '/healthz', timeoutMs = 60_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (procExited(srv.proc)) {
      throw new Error(`${srv.name} exited before readiness:\n${srv.log()}`);
    }
    try {
      const res = await fetch(srv.base + path_, { signal: AbortSignal.timeout(2000) });
      if (res.ok) return;
    } catch { /* not up yet */ }
    await sleep(250);
  }
  throw new Error(`${srv.name} never became ready on ${srv.base}${path_}:\n${srv.log()}`);
}

function procExited(proc: ChildProcess): boolean {
  return proc.exitCode !== null || proc.signalCode !== null;
}

// ── the stack ──────────────────────────────────────────────────────────────

export interface Stack {
  dir: string;                 // run tmp root (state file, bins, HOMEs, blobs)
  binNoverse: string;
  binViderse: string;
  binProxy: string;
  identityDB: string;
  viderseDB: string;
  dsn: {
    noverseApp: string;
    noverseWorker: string;
    noverseMigrate: string;
    viderseIdentity: string;
    viderseApp: string;
    viderseMigrate: string;
  };
  passwords: Record<string, string>;
  /** live handles — only inside globalSetup's process */
  noverse: ServerProc;
  viderse: ServerProc;
  hostedNoverse: ServerProc;
  hostedViderse: ServerProc;
  proxy: ServerProc;
  /** process-group ids of the five main servers — always present */
  pids: Record<'noverse' | 'viderse' | 'hostedNoverse' | 'hostedViderse' | 'proxy', number>;
  admin: { username: string; password: string };
  users: Record<string, { username: string; password: string }>;
  urls: {
    noverse: string; viderse: string;
    hostedNoverse: string; hostedViderse: string;
    hostedNoverseLoopback: string;
    proxyHost: string;
  };
  extraServers: ServerProc[];
  extraDBs: string[];
  /** loopback port of the Postgres cluster this run uses */
  pgPort: number;
  rolesCreated?: boolean;
}

const ROLES = ['noverse_migrate', 'noverse_app', 'viderse_migrate', 'viderse_app', 'viderse_identity', 'noverse_engine_worker'];

export async function provision(): Promise<Stack> {
  adminDSN(); // Validate any explicitly supplied connection before side effects.
  const suffix = randomBytes(4).toString('hex');
  const identityDB = `verse_sso_${suffix}`;
  const viderseDB = `verse_sso_v_${suffix}`;
  const dir = mkdtempSync(path.join(tmpdir(), 'verse-sso-'));
  chmodSync(dir, 0o700);
  // Servers spawned from now on append their pgid to dir/pids.txt so a
  // crashed run can still be reaped via cleanup.sh / global-teardown.
  process.env[STATE_ENV] = dir;

  // ── throwaway Postgres cluster ──
  // Starting our own cluster from initdb (trust auth, loopback, random port)
  // means the fixed runtime role names never collide with a developer's
  // cluster, and no pre-existing role or database is ever touched. Set
  // VERSE_SSO_ADMIN_DSN to use an existing cluster instead; the role guard
  // below then keeps that path non-destructive too.
  const useOwnCluster = !process.env.VERSE_SSO_ADMIN_DSN;
  // Register a partial stack before the first side effect so failed setup
  // follows the same teardown path as a completed run.
  setModuleStack({ dir, identityDB, viderseDB, extraServers: [], extraDBs: [] } as unknown as Stack);
  if (useOwnCluster) {
    const pgData = path.join(dir, 'pgdata');
    const port = await freePort();
    pgPort = String(port);
    await run('initdb', ['-D', pgData, '-U', 'dat', '-A', 'trust', '-E', 'UTF8', '--no-locale'], { timeoutMs: 120_000 });
    await run('pg_ctl', ['-D', pgData, '-l', path.join(dir, 'postgres.log'), '-o', `-p ${port} -k ${dir} -c listen_addresses=127.0.0.1`, '-w', '-t', '60', 'start'], { timeoutMs: 90_000 });
  } else {
    pgPort = String(new URL(adminDSN()).port || 5432);
  }

  const stack: Stack = {
    dir,
    binNoverse: path.join(dir, 'ainovel-cli'),
    binViderse: path.join(dir, 'viderse'),
    binProxy: path.join(dir, 'ssoproxy'),
    identityDB, viderseDB,
    dsn: {} as Stack['dsn'],
    passwords: {},
    noverse: null!, viderse: null!, hostedNoverse: null!, hostedViderse: null!, proxy: null!,
    pids: { noverse: 0, viderse: 0, hostedNoverse: 0, hostedViderse: 0, proxy: 0 },
    admin: { username: 'boss', password: `ad-${randomBytes(9).toString('base64url')}` },
    users: {},
    urls: { noverse: '', viderse: '', hostedNoverse: '', hostedViderse: '', proxyHost: '', hostedNoverseLoopback: '' },
    extraServers: [], extraDBs: [], pgPort: Number(pgPort),
  };
  setModuleStack(stack);

  const pw = (role: string) => `pw_${randomBytes(18).toString('base64url')}`;
  for (const role of ROLES) stack.passwords[role] = pw(role);
  stack.dsn = {
    noverseApp: dsn('noverse_app', stack.passwords['noverse_app'], identityDB),
    noverseWorker: dsn('noverse_engine_worker', stack.passwords['noverse_engine_worker'], identityDB),
    noverseMigrate: dsn('noverse_migrate', stack.passwords['noverse_migrate'], identityDB),
    viderseIdentity: dsn('viderse_identity', stack.passwords['viderse_identity'], identityDB),
    viderseApp: dsn('viderse_app', stack.passwords['viderse_app'], viderseDB),
    viderseMigrate: dsn('viderse_migrate', stack.passwords['viderse_migrate'], viderseDB),
  };

  // Guard: every role this run needs must be absent, otherwise the fixture
  // would either rotate a live role's password or silently share it.
  const existing = await psql('postgres',
    `SELECT rolname FROM pg_roles WHERE rolname IN (${ROLES.map(r => `'${r}'`).join(',')})`);
  if (existing.trim()) {
    throw new Error(`sso fixture: cluster already has runtime roles (${existing.trim().replace(/\s+/g, ', ')}); refusing to touch them`);
  }

  // ── build (web first: dist is embedded into the Go binaries) ──
  const goEnv = { GOTOOLCHAIN: 'go1.26.0', GOMAXPROCS: '4' };
  await run('lockf', ['-k', '/tmp/verse-heavy.lock', 'nice', '-n', '10', 'npm', 'run', 'build', '-w', 'apps/noverse/web'], { env: { ...goEnv }, timeoutMs: 1_800_000 });
  await run('lockf', ['-k', '/tmp/verse-heavy.lock', 'nice', '-n', '10', 'npm', 'run', 'build', '-w', 'apps/viderse/web'], { env: { ...goEnv }, timeoutMs: 1_800_000 });
  await run('lockf', ['-k', '/tmp/verse-heavy.lock', 'nice', '-n', '10', 'go', 'build', '-o', stack.binNoverse, './apps/noverse/cmd/ainovel-cli'], { env: goEnv, timeoutMs: 1_800_000 });
  await run('lockf', ['-k', '/tmp/verse-heavy.lock', 'nice', '-n', '10', 'go', 'build', '-o', stack.binViderse, './apps/viderse/cmd/viderse'], { env: goEnv, timeoutMs: 1_800_000 });
  await run('lockf', ['-k', '/tmp/verse-heavy.lock', 'nice', '-n', '10', 'go', 'build', '-o', stack.binProxy, './tests/sso/fixture'], { env: goEnv, timeoutMs: 1_800_000 });

  // ── databases + roles ──
  // The identity DB gets the full hosted role contract: roles + schema
  // ownership (01-roles.sh), goose migrations as noverse_migrate, then the
  // viderse_identity and noverse_engine_worker least-privilege scripts.
  // provisionIdentityDB is reusable for extra empty identity DBs (setup race).
  await psql('postgres', `CREATE DATABASE ${identityDB}`);
  await psql('postgres', `CREATE DATABASE ${viderseDB}`);
  stack.extraDBs.push(identityDB, viderseDB); // owned by this run → dropped on teardown
  stack.rolesCreated = true;
  await provisionIdentityDB(stack, identityDB);

  await psql('postgres', `
    CREATE ROLE viderse_migrate LOGIN PASSWORD '${stack.passwords['viderse_migrate']}';
    CREATE ROLE viderse_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '${stack.passwords['viderse_app']}';
    ALTER DATABASE ${viderseDB} OWNER TO viderse_migrate;
    GRANT CONNECT ON DATABASE ${viderseDB} TO viderse_app;
  `);
  await run(stack.binViderse, ['migrate', 'up'], {
    env: {
      HOME: dir,
      VIDERSE_DATABASE_URL: stack.dsn.viderseMigrate,
      VIDERSE_MIGRATION_DATABASE_URL: stack.dsn.viderseMigrate,
    },
    timeoutMs: 300_000,
  });

  // ── servers ──
  const noversePort = await freePort();
  const vidersePort = await freePort();
  const hostedNoversePort = await freePort();
  const hostedVidersePort = await freePort();
  const proxyPort = await freePort();

  const blobRoot = path.join(dir, 'blobs');
  mkdirSync(blobRoot, { recursive: true });
  const localOrigins = `http://127.0.0.1:${noversePort},http://127.0.0.1:${vidersePort}`;
  const hostedNoverseURL = `https://noverse.verse.test:${proxyPort}`;
  const hostedViderseURL = `https://viderse.verse.test:${proxyPort}`;
  const hostedOrigins = `${hostedNoverseURL},${hostedViderseURL}`;

  stack.urls = {
    noverse: `http://127.0.0.1:${noversePort}`,
    viderse: `http://127.0.0.1:${vidersePort}`,
    hostedNoverse: hostedNoverseURL,
    hostedNoverseLoopback: `http://127.0.0.1:${hostedNoversePort}`,
    hostedViderse: hostedViderseURL,
    proxyHost: `127.0.0.1:${proxyPort}`,
  };

  const noverseEnv = (port: number, hosted: boolean): NodeJS.ProcessEnv => ({
    HOME: path.join(dir, hosted ? 'home-noverse-h' : 'home-noverse'),
    STORE_DRIVER: 'postgres',
    DATABASE_URL: stack.dsn.noverseApp,
    NOVERSE_WORKER_DATABASE_URL: stack.dsn.noverseWorker,
    STORAGE_DRIVER: 'local',
    BLOB_LOCAL_ROOT: blobRoot,
    HTTP_ADDR: `127.0.0.1:${port}`,
    ...(hosted ? {
      PUBLIC_URL: hostedNoverseURL,
      SECURE_COOKIES: '1',
      VERSE_COOKIE_DOMAIN: 'verse.test',
      VERSE_APP_ORIGINS: hostedOrigins,
      TRUSTED_PROXIES: '127.0.0.1/32',
    } : { VERSE_APP_ORIGINS: localOrigins }),
  });
  const viderseEnv = (port: number, hosted: boolean): NodeJS.ProcessEnv => ({
    HOME: path.join(dir, hosted ? 'home-viderse-h' : 'home-viderse'),
    VIDERSE_ADDR: `127.0.0.1:${port}`,
    VIDERSE_DATABASE_URL: stack.dsn.viderseApp,
    VIDERSE_MIGRATION_DATABASE_URL: stack.dsn.viderseMigrate,
    VERSE_IDENTITY_DATABASE_URL: stack.dsn.viderseIdentity,
    ...(hosted ? {
      PUBLIC_URL: hostedViderseURL,
      SECURE_COOKIES: '1',
      VERSE_COOKIE_DOMAIN: 'verse.test',
      VERSE_APP_ORIGINS: hostedOrigins,
      TRUSTED_PROXIES: '127.0.0.1/32',
    } : { VERSE_APP_ORIGINS: localOrigins }),
  });

  stack.noverse = startServer('noverse', stack.binNoverse, ['serve'], noverseEnv(noversePort, false));
  stack.noverse.base = stack.urls.noverse;
  stack.viderse = startServer('viderse', stack.binViderse, ['serve'], viderseEnv(vidersePort, false));
  stack.viderse.base = stack.urls.viderse;
  await waitReady(stack.noverse);
  await waitReady(stack.viderse);

  stack.hostedNoverse = startServer('noverse-hosted', stack.binNoverse, ['serve'], noverseEnv(hostedNoversePort, true));
  stack.hostedNoverse.base = `http://127.0.0.1:${hostedNoversePort}`;
  stack.hostedViderse = startServer('viderse-hosted', stack.binViderse, ['serve'], viderseEnv(hostedVidersePort, true));
  stack.hostedViderse.base = `http://127.0.0.1:${hostedVidersePort}`;
  await waitReady(stack.hostedNoverse);
  await waitReady(stack.hostedViderse);

  stack.proxy = startServer('ssoproxy', stack.binProxy, [
    '-listen', stack.urls.proxyHost,
    '-route', `noverse.verse.test=127.0.0.1:${hostedNoversePort}`,
    '-route', `viderse.verse.test=127.0.0.1:${hostedVidersePort}`,
    '-route', `khac.other.test=127.0.0.1:${hostedNoversePort}`,
  ], {});
  stack.proxy.base = `https://${stack.urls.proxyHost}`;
  const proxyDeadline = Date.now() + 15_000;
  while (!stack.proxy.log().includes('READY') && Date.now() < proxyDeadline) {
    if (procExited(stack.proxy.proc)) throw new Error(`ssoproxy exited:\n${stack.proxy.log()}`);
    await sleep(100);
  }

  // ── first admin through the real setup flow on Noverse ──
  const code = stack.noverse.setupCode();
  const res1 = await fetch(`${stack.urls.noverse}/api/setup/verify`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code }),
  });
  if (!res1.ok) throw new Error(`setup/verify failed: ${res1.status} ${await res1.text()}`);
  const { setupToken } = await res1.json() as { setupToken: string };
  const res2 = await fetch(`${stack.urls.noverse}/api/setup/admin`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ setupToken, name: 'Quản trị', username: stack.admin.username, password: stack.admin.password }),
  });
  if (!res2.ok) throw new Error(`setup/admin failed: ${res2.status} ${await res2.text()}`);

  // ── fixture users via the real admin API + grant endpoints ──
  const api = async (method: string, urlPath: string, body?: unknown, cookie?: string) => {
    const res = await fetch(`${stack.urls.noverse}${urlPath}`, {
      method,
      headers: { 'Content-Type': 'application/json', ...(cookie ? { cookie } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    return res;
  };
  // Admin session for seeding only (a fresh request jar, never persisted).
  const adminLogin = await api('POST', '/api/login', { username: stack.admin.username, password: stack.admin.password });
  if (!adminLogin.ok) throw new Error(`admin seed login failed: ${adminLogin.status} ${await adminLogin.text()}`);
  const adminCookie = adminLogin.headers.getSetCookie().map(c => c.split(';')[0]).join('; ');
  if (!adminCookie.includes('verse_session=')) throw new Error('admin seed login did not set verse_session');

  // one real project + story for membership checks
  const projRes = await api('POST', '/api/projects', { slug: 'e2eproj', name: 'Dự án kiểm thử' }, adminCookie);
  if (projRes.status !== 201) throw new Error(`project seed failed: ${projRes.status} ${await projRes.text()}`);
  const storyRes = await api('POST', '/api/projects/e2eproj/stories', { name: 'Truyện kiểm thử' }, adminCookie);
  if (storyRes.status !== 201) throw new Error(`story seed failed: ${storyRes.status} ${await storyRes.text()}`);

  const newPass = () => `tmp-${randomBytes(9).toString('base64url')}`;
  const mkUser = async (username: string, opts: { projects?: Record<string, string>; settle?: boolean; grants?: string[] } = {}) => {
    const password = newPass();
    const created = await api('POST', '/api/users', {
      username, name: username, role: 'member', password, projects: opts.projects ?? {},
    }, adminCookie);
    if (created.status !== 201) throw new Error(`create ${username}: ${created.status} ${await created.text()}`);
    for (const app of opts.grants ?? []) {
      const g = await api('PUT', `/api/users/${username}/grants/${app}`, { role: 'member' }, adminCookie);
      if (!g.ok) throw new Error(`grant ${app} for ${username}: ${g.status} ${await g.text()}`);
    }
    const rec = { username, password };
    if (opts.settle) {
      // Consume the forced password through the real flow so the account ends
      // as a normal member (flag cleared, temp password revoked server-side).
      const login = await api('POST', '/api/login', { username, password });
      const jar = login.headers.getSetCookie().map(c => c.split(';')[0]).join('; ');
      const next = `${rec.username}-pw-${randomBytes(4).toString('hex')}`;
      const ch = await api('POST', '/api/me/password', { current: password, new: next }, jar);
      if (!ch.ok) throw new Error(`settle ${username}: ${ch.status} ${await ch.text()}`);
      rec.password = next;
    }
    stack.users[username] = rec;
    return rec;
  };

  await mkUser('both', { projects: { e2eproj: 'collab' }, grants: ['viderse'], settle: true });
  await mkUser('novonly', { grants: [], settle: true });                        // default noverse grant only
  const novOnlyRec = stack.users['novonly'];
  await api('PUT', `/api/users/novonly/grants/viderse`, { role: 'member' }, adminCookie); // grant…
  await api('DELETE', `/api/users/novonly/grants/viderse`, undefined, adminCookie);       // …then revoke → session killed; still noverse-only
  stack.users['novonly'] = novOnlyRec;
  await mkUser('vidonly', { grants: ['viderse'], settle: true });
  await api('DELETE', '/api/users/vidonly/grants/noverse', undefined, adminCookie); // revoke noverse → viderse-only
  await mkUser('forced', { grants: ['viderse'] });                              // keep must_change_password
  await mkUser('disabled1', { settle: true });
  await api('POST', '/api/users/disabled1', { disabled: true, reason: 'khoá để kiểm thử' }, adminCookie);
  await mkUser('deleted1', { settle: true });
  await api('POST', '/api/users/deleted1/delete', { reason: 'xoá để kiểm thử' }, adminCookie);
  await mkUser('rateuser', { settle: true });
  await mkUser('muttarget', { settle: true });
  await mkUser('revtarget', { settle: true });
  await mkUser('resetme', { grants: ['viderse'], settle: true });
  // Exercise an admin with zero grant rows; CreateFirstAdmin leaves
  // a default noverse row for legacy compatibility, so revoke it explicitly.
  const rmBoss = await api('DELETE', '/api/users/boss/grants/noverse', undefined, adminCookie);
  if (!rmBoss.ok) throw new Error(`admin grant revoke failed: ${rmBoss.status} ${await rmBoss.text()}`);

  // ── state for workers ──
  // Contains throwaway credentials (role/user passwords, DSNs). The run dir
  // is mode 0700 and is deleted by global-teardown; these secrets authorise
  // only objects created by this run.
  const state = {
    dir, identityDB, viderseDB,
    dsn: stack.dsn,
    passwords: stack.passwords,
    users: { admin: stack.admin, ...stack.users },
    urls: stack.urls,
    extraDBs: stack.extraDBs,
    pgPort: Number(pgPort),
    pids: {
      noverse: stack.noverse.pid,
      viderse: stack.viderse.pid,
      hostedNoverse: stack.hostedNoverse.pid,
      hostedViderse: stack.hostedViderse.pid,
      proxy: stack.proxy.pid,
    },
  };
  writeFileSync(path.join(dir, 'state.json'), JSON.stringify(state, null, 2), { mode: 0o600 });

  // reaper file so a crashed run can still be cleaned up manually
  writeFileSync(path.join(dir, 'cleanup.sh'), [
    '#!/bin/sh',
    `# cleanup for run ${suffix} — only objects this run created`,
    `while read -r _name _pid; do kill -TERM -- "-$_pid" 2>/dev/null; done < '${dir}/pids.txt'`,
    'sleep 1',
    `while read -r _name _pid; do kill -KILL -- "-$_pid" 2>/dev/null; done < '${dir}/pids.txt'`,
    ...stack.extraDBs.map(db => `dropdb --if-exists -h 127.0.0.1 -p ${pgPort} -U dat '${db}'`),
    ...ROLES.map(r => `psql -h 127.0.0.1 -p ${pgPort} -U dat -d postgres -c "DROP ROLE IF EXISTS ${r}"`),
    ...(useOwnCluster ? [`pg_ctl -D '${dir}/pgdata' -m immediate -w stop`] : []),
    `rm -rf '${dir}'`,
  ].join('\n') + '\n', { mode: 0o700 });

  return stack;
}

export function loadState(): Stack {
  const dir = process.env[STATE_ENV];
  if (!dir) throw new Error(`${STATE_ENV} is not set — the suite must run through playwright.config.ts`);
  const raw = JSON.parse(readFileSync(path.join(dir, 'state.json'), 'utf8'));
  pgPort = String(raw.pgPort);
  for (const line of readFileSync(path.join(dir, 'pids.txt'), 'utf8').split('\n')) {
    const [name, pid] = line.trim().split(/\s+/);
    if (name === 'noverse-rst') raw.pids.noverse = Number(pid);
  }
  return {
    ...raw,
    binNoverse: path.join(dir, 'ainovel-cli'),
    binViderse: path.join(dir, 'viderse'),
    binProxy: path.join(dir, 'ssoproxy'),
    dsn: { ...raw.dsn },
    extraServers: [], extraDBs: raw.extraDBs ?? [], pgPort: Number(raw.pgPort ?? pgPort),
  } as Stack;
}

// moduleStack keeps the live Stack for global-teardown (same process): real
// process handles, real DB list. pids.txt/cleanup.sh stay the crash fallback.
let moduleStack: Stack | null = null;
export function setModuleStack(s: Stack): void { moduleStack = s; }
export function getModuleStack(): Stack | null { return moduleStack; }

// serverEnv rebuilds a server environment for test-driven restarts/extra
// instances. kind: 'local' | 'hosted'. The caller may override individual
// DSNs (e.g. point the identity DSN at a fresh race DB).
export function serverEnv(stack: Stack, app: 'noverse' | 'viderse', port: number, opts: {
  hosted?: boolean;
  /** app data DSN (noverse DATABASE_URL / viderse VIDERSE_DATABASE_URL) */
  dataDsn?: string;
  /** shared-account DSN (viderse only) */
  identityDsn?: string;
  workerDsn?: string;
  homeName?: string;
} = {}): NodeJS.ProcessEnv {
  const hosted = !!opts.hosted;
  const home = path.join(stack.dir, opts.homeName ?? `home-${app}-${port}`);
  const origins = hosted
    ? `${stack.urls.hostedNoverse},${stack.urls.hostedViderse}`
    : `http://127.0.0.1:${portOf(stack.urls.noverse)},http://127.0.0.1:${portOf(stack.urls.viderse)},http://127.0.0.1:${port}`;
  const proxyVars = hosted ? {
    PUBLIC_URL: app === 'noverse' ? stack.urls.hostedNoverse : stack.urls.hostedViderse,
    SECURE_COOKIES: '1',
    VERSE_COOKIE_DOMAIN: 'verse.test',
    VERSE_APP_ORIGINS: origins,
    TRUSTED_PROXIES: '127.0.0.1/32',
  } : { VERSE_APP_ORIGINS: origins };
  if (app === 'noverse') {
    return {
      HOME: home,
      STORE_DRIVER: 'postgres',
      DATABASE_URL: opts.dataDsn ?? stack.dsn.noverseApp,
      NOVERSE_WORKER_DATABASE_URL: opts.workerDsn ?? stack.dsn.noverseWorker,
      STORAGE_DRIVER: 'local',
      BLOB_LOCAL_ROOT: path.join(stack.dir, 'blobs'),
      HTTP_ADDR: `127.0.0.1:${port}`,
      ...proxyVars,
    };
  }
  return {
    HOME: home,
    VIDERSE_ADDR: `127.0.0.1:${port}`,
    VIDERSE_DATABASE_URL: stack.dsn.viderseApp,
    VIDERSE_MIGRATION_DATABASE_URL: stack.dsn.viderseMigrate,
    VERSE_IDENTITY_DATABASE_URL: opts.identityDsn ?? stack.dsn.viderseIdentity,
    ...proxyVars,
  };
}

export function portOf(url: string): number {
  return Number(new URL(url).port || (url.startsWith('https') ? 443 : 80));
}

export function noverseDataEnv(stack: Stack, migrate = false): NodeJS.ProcessEnv {
  return {
    HOME: stack.dir,
    STORE_DRIVER: 'postgres',
    DATABASE_URL: migrate ? stack.dsn.noverseMigrate : stack.dsn.noverseApp,
    MIGRATION_DATABASE_URL: stack.dsn.noverseMigrate,
  };
}

export function viderseCliEnv(stack: Stack): NodeJS.ProcessEnv {
  return {
    HOME: stack.dir,
    VIDERSE_DATABASE_URL: stack.dsn.viderseApp,
    VIDERSE_MIGRATION_DATABASE_URL: stack.dsn.viderseMigrate,
    VERSE_IDENTITY_DATABASE_URL: stack.dsn.viderseIdentity,
  };
}

// provisionIdentityDB applies the full Noverse role/migration contract to an
// already-created database: schema ownership + runtime roles (01-roles.sh),
// goose migrations as noverse_migrate, then the viderse_identity and
// noverse_engine_worker least-privilege grants. The role scripts converge on
// rerun, so this is safe for any number of run databases.
export async function provisionIdentityDB(stack: Stack, dbName: string): Promise<void> {
  const initDir = path.join(REPO, 'apps/noverse/deploy/postgres-init');
  await run('sh', [path.join(initDir, '01-roles.sh')], {
    env: {
      ...psqlEnv(),
      POSTGRES_USER: 'dat',
      POSTGRES_DB: dbName,
      POSTGRES_MIGRATE_PASSWORD: stack.passwords['noverse_migrate'],
      POSTGRES_APP_PASSWORD: stack.passwords['noverse_app'],
    },
  });
  await run(stack.binNoverse, ['storage', 'migrate', 'up'], {
    env: {
      HOME: stack.dir,
      STORE_DRIVER: 'postgres',
      DATABASE_URL: dsn('noverse_migrate', stack.passwords['noverse_migrate'], dbName),
      MIGRATION_DATABASE_URL: dsn('noverse_migrate', stack.passwords['noverse_migrate'], dbName),
    },
    timeoutMs: 300_000,
  });
  await psqlFile(dbName, path.join(initDir, '02-identity-role.sql'), {
    IDENTITY_ROLE_PASSWORD: stack.passwords['viderse_identity'],
  });
  await psqlFile(dbName, path.join(initDir, '03-engine-worker-role.sql'), {
    WORKER_ROLE_PASSWORD: stack.passwords['noverse_engine_worker'],
  });
}

// provisionRaceDB builds an additional throwaway identity DB for the
// setup-race and DB-down scenarios; caller records it in extraDBs.
export async function provisionRaceDB(stack: Stack, name: string): Promise<string> {
  await psql('postgres', `CREATE DATABASE ${name}`);
  await provisionIdentityDB(stack, name);
  return name;
}
// teardown stops every process this run started (extra instances included),
// drops the databases and roles it created, then removes the tmp root. The
// DB drop list is the run's verse_sso_* names discovered from pg_database —
// test-created race DBs land there too even though only the worker saw them.
export async function teardown(stack: Stack): Promise<void> {
  const servers = [stack.noverse, stack.viderse, stack.hostedNoverse, stack.hostedViderse, stack.proxy, ...stack.extraServers].filter(Boolean);
  for (const srv of servers) {
    try { await srv.stop(); } catch { /* already gone */ }
  }
  // Worker-started instances are only known through pids.txt.
  try {
    for (const line of readFileSync(path.join(stack.dir, 'pids.txt'), 'utf8').split('\n')) {
      const pid = Number(line.trim().split(/\s+/).pop());
      if (pid) await killGroup(pid);
    }
  } catch { /* no pids file */ }
  let dbs = stack.extraDBs.slice();
  // Only databases carrying THIS run's random suffix — never another run's.
  const suffix = stack.identityDB?.replace(/^verse_sso_/, '') ?? '';
  try {
    if (suffix) {
      const listed = await psql('postgres',
        `SELECT datname FROM pg_database WHERE datname LIKE '%${suffix}%'`);
      for (const line of listed.split('\n')) {
        const name = line.trim().split(' ')[0];
        if (name && name.startsWith('verse_sso_') && name.includes(suffix) && !dbs.includes(name)) dbs.push(name);
      }
    }
  } catch { /* keep the recorded list */ }
  for (const db of dbs) {
    try {
      await psql('postgres', `DROP DATABASE IF EXISTS ${db} WITH (FORCE)`);
    } catch { /* best effort */ }
  }
  if (stack.rolesCreated) {
    for (const role of ROLES) {
      try { await psql('postgres', `DROP ROLE IF EXISTS ${role}`); } catch { /* best effort */ }
    }
  }
  try {
    if (existsSync(path.join(stack.dir, 'pgdata'))) {
      await run('pg_ctl', ['-D', path.join(stack.dir, 'pgdata'), '-m', 'immediate', '-w', 'stop'], { timeoutMs: 30_000 });
    }
  } catch { /* cluster already stopped */ }
  try { rmSync(stack.dir, { recursive: true, force: true }); } catch { /* best effort */ }
}

export { existsSync };

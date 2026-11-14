import type { Bindings } from './index.ts';

// Checked by `npm run worker:types` only: fails when cloudflare.config.ts stops providing what the handler reads.
type Assert<T extends true> = T;

export type EnvCheck = Assert<Env extends Bindings ? true : false>;

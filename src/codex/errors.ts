import { TaggedError } from 'better-result';

// Expected reasons a Codex-key request is refused. Each carries the HTTP status it maps to.
export class InvalidJson extends TaggedError('InvalidJson')<{ message: string; status: 400 }> {}

export class ModelNotAllowed extends TaggedError('ModelNotAllowed')<{ message: string; model: unknown; status: 403 }> {}

export type CodexDenied = InvalidJson | ModelNotAllowed;

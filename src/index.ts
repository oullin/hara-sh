import { app } from '@/app';

// The Durable Object class must be exported from the Worker's main module.
export { CliProxy } from '@/container/cli-proxy';

export default app;

import { describe, expect, it, vi } from 'vitest';
import { app } from '@/app';

vi.mock('@cloudflare/containers', () => ({ Container: class {} }));

describe('worker entry', () => {
	it('exports the Hono app as the default handler and the Durable Object class', async () => {
		const mod = await import('@/index');

		expect(mod.default).toBe(app);
		expect(mod.CliProxy).toBeTypeOf('function');
	});
});

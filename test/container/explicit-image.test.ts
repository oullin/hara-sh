import { describe, expect, it, vi } from 'vitest';
import { withExplicitImage } from '@/container/explicit-image';

function runtime() {
	return {
		images: { custom: 'registry/custom:2', default: 'registry/default:1' },
		inspect: vi.fn(function (this: unknown) {
			return this;
		}),
		running: false,
		start: vi.fn(),
	};
}

describe('withExplicitImage', () => {
	it('starts with the default image and internet enabled', () => {
		const target = runtime();

		withExplicitImage(target as unknown as Container).start({ env: { A: '1' } } as unknown as ContainerStartupOptions);

		expect(target.start).toHaveBeenCalledWith({ enableInternet: true, env: { A: '1' }, image: 'registry/default:1' });
	});

	it('lets callers override options but never the image', () => {
		const target = runtime();

		withExplicitImage(target as unknown as Container, 'custom').start({ enableInternet: false, image: 'ignored' } as unknown as ContainerStartupOptions);

		expect(target.start).toHaveBeenCalledWith({ enableInternet: false, image: 'registry/custom:2' });
	});

	it('starts without options', () => {
		const target = runtime();

		(withExplicitImage(target as unknown as Container).start as () => void)();

		expect(target.start).toHaveBeenCalledWith({ enableInternet: true, image: 'registry/default:1' });
	});

	it('binds other methods to the runtime and passes properties through', () => {
		const target = runtime();
		const wrapped = withExplicitImage(target as unknown as Container) as unknown as ReturnType<typeof runtime>;

		expect(
			wrapped.inspect(),
		).toBe(target);
		expect(wrapped.running).toBe(false);
		expect(wrapped.images).toBe(target.images);
	});
});

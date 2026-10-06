import { mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, nextTick } from 'vue';
import { useCopy } from '@/composables/useCopy';

function host() {
	let api!: ReturnType<typeof useCopy>;

	const wrapper = mount(
		defineComponent(
			{
				setup() {
					api = useCopy(1000);

					return () => null;
				},
			},
		),
	);

	return { api, wrapper };
}

function stubClipboard(writeText: (text: string) => Promise<void>) {
	Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
}

describe('useCopy', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('copies the text and resets after the delay', async () => {
		const writeText = vi.fn(() => Promise.resolve());

		stubClipboard(writeText);

		const { api } = host();

		await api.copy('hello');

		expect(writeText).toHaveBeenCalledWith('hello');
		expect(api.copied.value).toBe(true);
		expect(api.failed.value).toBe(false);

		vi.advanceTimersByTime(1000);

		await nextTick();

		expect(api.copied.value).toBe(false);
	});

	it('reports a clipboard failure instead of throwing', async () => {
		stubClipboard(() => Promise.reject(new Error('denied')));

		const { api } = host();

		await api.copy('hello');

		expect(api.copied.value).toBe(false);
		expect(api.failed.value).toBe(true);

		vi.advanceTimersByTime(1000);

		expect(api.failed.value).toBe(false);
	});

	it('clears the pending reset when the component unmounts', async () => {
		stubClipboard(() => Promise.resolve());

		const clear = vi.spyOn(globalThis, 'clearTimeout');
		const { api, wrapper } = host();

		await api.copy('hello');

		wrapper.unmount();

		expect(clear).toHaveBeenCalled();
	});
});

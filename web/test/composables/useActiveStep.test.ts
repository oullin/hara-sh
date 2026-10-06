import { mount } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, ref } from 'vue';
import { useActiveStep } from '@/composables/useActiveStep';

type Callback = (entries: Array<Partial<IntersectionObserverEntry>>) => void;

function install() {
	const observed: Array<Element> = [];
	const disconnect = vi.fn();

	let callback: Callback | undefined;

	class FakeObserver {
		disconnect = disconnect;

		constructor(cb: Callback) {
			callback = cb;
		}

		observe(element: Element) {
			observed.push(element);
		}
	}

	vi.stubGlobal('IntersectionObserver', FakeObserver);

	return { disconnect, fire: (entries: Array<Partial<IntersectionObserverEntry>>) => callback?.(entries), observed };
}

function host(withContainer = true) {
	let api!: ReturnType<typeof useActiveStep>;

	const wrapper = mount(
		defineComponent(
			{
				setup() {
					const list = ref<HTMLElement | null>(null);

					api = useActiveStep(withContainer ? list : ref(null), 'a');

					return () => h('ol', { ref: list }, [h('li', { 'data-step': 'a' }), h('li', { 'data-step': 'b' }), h('li')]);
				},
			},
		),
	);

	return { api, wrapper };
}

describe('useActiveStep', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('observes every step and activates the one crossing the middle', () => {
		const { fire, observed } = install();
		const { api } = host();

		expect(observed).toHaveLength(2);
		expect(api.active.value).toBe('a');

		fire(
			[
				{ isIntersecting: false, target: observed[0] },
				{ isIntersecting: true, target: observed[1] },
			],
		);

		expect(api.active.value).toBe('b');
	});

	it('ignores intersecting elements without a step id', () => {
		const { fire } = install();
		const { api } = host();

		fire(
			[{ isIntersecting: true, target: document.createElement('li') }],
		);

		expect(api.active.value).toBe('a');
	});

	it('disconnects on unmount', () => {
		const { disconnect } = install();
		const { wrapper } = host();

		wrapper.unmount();

		expect(disconnect).toHaveBeenCalled();
	});

	it('keeps the first step without IntersectionObserver or a container', () => {
		vi.stubGlobal('IntersectionObserver', undefined);

		const { api, wrapper } = host();

		expect(api.active.value).toBe('a');
		wrapper.unmount();

		install();
		expect(host(false).api.active.value).toBe('a');
	});
});

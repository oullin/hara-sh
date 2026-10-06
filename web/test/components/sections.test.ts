import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import App from '@/App.vue';
import CallToAction from '@/components/sections/CallToAction.vue';
import HeroSection from '@/components/sections/HeroSection.vue';
import ProvidersSection from '@/components/sections/ProvidersSection.vue';
import SiteFooter from '@/components/sections/SiteFooter.vue';
import SiteHeader from '@/components/sections/SiteHeader.vue';
import StepsSection from '@/components/sections/StepsSection.vue';
import { clients, DOCS_URL, providers, steps } from '@/lib/content';

// A detached element carrying a step id, for driving the steps panel.
function stepElement(id: string): HTMLElement {
	const element = document.createElement('li');

	element.dataset.step = id;

	return element;
}

function stubClipboard(writeText: (text: string) => Promise<void>) {
	Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
}

describe('SiteHeader', () => {
	it('links home, to the sections, the docs and getting started', () => {
		const wrapper = mount(SiteHeader);

		expect(
			wrapper.find('a[aria-label="hara, home"]').exists(),
		).toBe(true);
		expect(
			wrapper.findAll('nav a').map((a) => a.attributes('href')),
		).toEqual(['#meaning', '#how', '#providers']);
		expect(
			wrapper.find(`a[href="${DOCS_URL}"]`).text(),
		).toBe('Docs');
		expect(
			wrapper.findAll('a').some((a) => a.text() === 'Get started'),
		).toBe(true);
	});
});

describe('HeroSection', () => {
	it('shows the Claude Code command first and copies it', async () => {
		const writeText = vi.fn(() => Promise.resolve());

		stubClipboard(writeText);

		const wrapper = mount(HeroSection);

		expect(
			wrapper.find('h1').text(),
		).toBe('One center for every model');
		expect(
			wrapper.find('code').text(),
		).toBe(clients[0].command);

		await wrapper.find('button[aria-label="Copy command"]').trigger('click');

		await flushPromises();

		expect(writeText).toHaveBeenCalledWith(clients[0].command);
		expect(
			wrapper.find('button[aria-label="Copied"]').exists(),
		).toBe(true);
		expect(
			wrapper.find('[role="status"]').text(),
		).toBe('Command copied');
	});

	it('switches the command per client', async () => {
		const wrapper = mount(
			HeroSection,
			{ attachTo: document.body },
		);

		const codex = wrapper.findAll('[role="tab"]')[1];

		await codex.trigger('mousedown', { button: 0 });

		expect(
			wrapper.find('code').text(),
		).toBe(clients[1].command);
		wrapper.unmount();
	});

	it('reports when the clipboard is unavailable', async () => {
		stubClipboard(() => Promise.reject(new Error('denied')));

		const wrapper = mount(HeroSection);

		await wrapper.find('button[aria-label="Copy command"]').trigger('click');

		await flushPromises();

		expect(
			wrapper.find('button[aria-label="Copy failed"]').exists(),
		).toBe(true);
		expect(
			wrapper.find('[role="status"]').text(),
		).toBe('Copy failed');
	});

	it('explains 腹 (hara) beside the headline', () => {
		const meaning = mount(HeroSection).find('aside#meaning');

		expect(
			meaning.text(),
		).toContain('腹');
		expect(
			meaning.text(),
		).toContain("the body's center of gravity");
	});

	it('falls back to the first command for an unknown selection', async () => {
		const wrapper = mount(HeroSection);
		const tabs = wrapper.findComponent({ name: 'Tabs' });

		tabs.vm.$emit('update:modelValue', 'unknown');

		await flushPromises();

		expect(
			wrapper.find('code').text(),
		).toBe(clients[0].command);
	});
});

describe('StepsSection', () => {
	it('lists the numbered steps and shows the first step in the panel', () => {
		const wrapper = mount(StepsSection);

		expect(
			wrapper.findAll('[data-step]'),
		).toHaveLength(steps.length);
		expect(
			wrapper.text(),
		).toContain('01');
		expect(
			wrapper.text(),
		).toContain('04');
		expect(
			wrapper.find('figcaption').text(),
		).toBe(steps[0].file);
	});

	it('follows the step in view and falls back to the first for an unknown id', async () => {
		let report: ((entries: Array<Partial<IntersectionObserverEntry>>) => void) | undefined;

		vi.stubGlobal(
			'IntersectionObserver',
			class {
				constructor(cb: NonNullable<typeof report>) {
					report = cb;
				}

				disconnect() {}

				observe() {}
			},
		);

		const wrapper = mount(StepsSection);

		report?.(
			[{ isIntersecting: true, target: wrapper.find('[data-step="scope"]').element }],
		);

		await flushPromises();

		expect(
			wrapper.find('figcaption').text(),
		).toBe(steps[3].file);

		report?.(
			[{ isIntersecting: true, target: stepElement('unknown') }],
		);

		await flushPromises();

		expect(
			wrapper.find('figcaption').text(),
		).toBe(steps[0].file);
		vi.unstubAllGlobals();
	});
});

describe('ProvidersSection', () => {
	it('lists every provider with its glyph, without cards', () => {
		const wrapper = mount(ProvidersSection);

		expect(
			wrapper.findAll('li'),
		).toHaveLength(providers.length);
		expect(
			wrapper.text(),
		).toContain('OpenAI-compatible');
		expect(
			wrapper.findAll('.dot'),
		).toHaveLength(providers.length * 25);
	});
});

describe('CallToAction and SiteFooter', () => {
	it('render the closing call to action and the credit', () => {
		const cta = mount(CallToAction);

		expect(
			cta.text(),
		).toContain('Find your center');
		expect(
			cta.find(`a[href="${DOCS_URL}"]`).exists(),
		).toBe(true);
		expect(
			mount(SiteFooter)
				.find('a[href="https://gocanto.sh"]')
				.text(),
		).toBe('gocanto.sh');
	});
});

describe('App', () => {
	it('assembles the page', () => {
		const wrapper = mount(App);

		expect(
			wrapper.find('main#top').exists(),
		).toBe(true);
		expect(
			wrapper.findAll('section'),
		).toHaveLength(4);
		expect(
			wrapper.find('footer').exists(),
		).toBe(true);
	});
});

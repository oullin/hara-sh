import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import App from '@/App.vue';
import CallToAction from '@/components/sections/CallToAction.vue';
import HeroSection from '@/components/sections/HeroSection.vue';
import MeaningSection from '@/components/sections/MeaningSection.vue';
import ProvidersSection from '@/components/sections/ProvidersSection.vue';
import RoutingSection from '@/components/sections/RoutingSection.vue';
import SetupSection from '@/components/sections/SetupSection.vue';
import SiteFooter from '@/components/sections/SiteFooter.vue';
import SiteHeader from '@/components/sections/SiteHeader.vue';
import { providers, SETUP } from '@/lib/content';

describe('SiteHeader', () => {
	it('links home and to every section', () => {
		const wrapper = mount(SiteHeader);

		expect(
			wrapper.find('a[aria-label="hara, home"]').exists(),
		).toBe(true);
		expect(
			wrapper.findAll('nav a').map((a) => a.attributes('href')),
		).toEqual(['#meaning', '#providers', '#routing', '#setup']);
	});
});

describe('HeroSection', () => {
	it('copies the setup command and announces it', async () => {
		const writeText = vi.fn(() => Promise.resolve());

		Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });

		const wrapper = mount(HeroSection);

		expect(
			wrapper.find('h1').text(),
		).toBe('One center for every model.');
		expect(
			wrapper.find('button').attributes('aria-label'),
		).toBe('Copy command');

		await wrapper.find('button').trigger('click');

		await flushPromises();

		expect(writeText).toHaveBeenCalledWith(SETUP.hero);
		expect(
			wrapper.find('button').attributes('aria-label'),
		).toBe('Copied');
		expect(
			wrapper.find('[role="status"]').text(),
		).toBe('Command copied');
	});

	it('reports when the clipboard is unavailable', async () => {
		Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: () => Promise.reject(new Error('denied')) } });

		const wrapper = mount(HeroSection);

		await wrapper.find('button').trigger('click');

		await flushPromises();

		expect(
			wrapper.find('button').attributes('aria-label'),
		).toBe('Copy failed');
		expect(
			wrapper.find('[role="status"]').text(),
		).toBe('Copy failed');
	});
});

describe('MeaningSection', () => {
	it('explains 腹 (hara)', () => {
		const text = mount(MeaningSection).text();

		expect(text).toContain('腹');
		expect(text).toContain("the body's center of gravity");
	});
});

describe('ProvidersSection', () => {
	it('lists every provider with its glyph', () => {
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

describe('RoutingSection', () => {
	it('shows clients, the hara center and the guarantees', () => {
		const text = mount(RoutingSection).text();

		expect(text).toContain('Claude Code');
		expect(text).toContain('hara');
		expect(text).toContain('Failover at the limit');
		expect(text).toContain('Caches stay warm');
	});
});

describe('SetupSection', () => {
	it('shows the Claude Code setup by default', () => {
		const wrapper = mount(SetupSection);

		expect(
			wrapper.findAll('[role="tab"]').map((tab) => tab.text()),
		).toEqual(['Claude Code', 'Codex']);
		expect(
			wrapper.find('pre').text(),
		).toBe(SETUP.claude);
	});

	it('switches to the Codex config', async () => {
		const wrapper = mount(
			SetupSection,
			{ attachTo: document.body },
		);

		const codexTab = wrapper.findAll('[role="tab"]')[1];

		await codexTab.trigger('mousedown', { button: 0 });

		expect(
			codexTab.attributes('aria-selected'),
		).toBe('true');
		expect(
			wrapper.text(),
		).toContain('~/.codex/config.toml');
		expect(
			wrapper.findAll('pre').map((pre) => pre.text()),
		).toContain(SETUP.codex);
		wrapper.unmount();
	});
});

describe('CallToAction and SiteFooter', () => {
	it('render the closing call to action and the credit', () => {
		expect(
			mount(CallToAction).text(),
		).toContain('Find your center.');
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
		expect(wrapper.findAll('section').length).toBeGreaterThanOrEqual(6);
		expect(
			wrapper.find('footer').exists(),
		).toBe(true);
	});
});

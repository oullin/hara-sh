import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import DotGrid from '@/components/dots/DotGrid.vue';
import HaraMark from '@/components/dots/HaraMark.vue';
import { glyphDots } from '@/lib/dots';

describe('DotGrid', () => {
	it('renders one sized, staggered dot per entry and hides itself from assistive tech', () => {
		const wrapper = mount(
			DotGrid,
			{ props: { dots: glyphDots('#####|.....|.....|.....|.....'), size: 8 } },
		);

		const dots = wrapper.findAll('span > span');

		expect(
			wrapper.attributes('aria-hidden'),
		).toBe('true');
		expect(dots).toHaveLength(25);
		expect(
			dots[1].attributes('style'),
		).toContain('animation-delay: 160ms');
		expect(
			dots[1].attributes('style'),
		).toContain('width: 8px');
	});

	it('defaults to 6px dots', () => {
		const wrapper = mount(
			DotGrid,
			{ props: { dots: glyphDots('#....|.....|.....|.....|.....') } },
		);

		expect(
			wrapper.find('span > span').attributes('style'),
		).toContain('height: 6px');
	});
});

describe('HaraMark', () => {
	it('renders the 5×5 mark with one glowing core', () => {
		const wrapper = mount(HaraMark);

		expect(
			wrapper.findAll('.dot'),
		).toHaveLength(25);
		expect(
			wrapper.findAll('.dot-core'),
		).toHaveLength(1);
		expect(
			wrapper.find('.dot').attributes('style'),
		).toContain('width: 4px');
	});
});

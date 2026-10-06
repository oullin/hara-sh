import { describe, expect, it } from 'vitest';
import { glyphDots, ringField } from '@/lib/dots';

describe('glyphDots', () => {
	it('maps a 5×5 glyph to 25 dots, lit or dim', () => {
		const dots = glyphDots('#....|.....|.....|.....|....#');

		expect(dots).toHaveLength(25);
		expect(dots[0]).toEqual({ class: 'dot dot-on', delay: 0 });
		expect(dots[1]).toEqual({ class: 'dot', delay: 0 });
		expect(dots[24]).toEqual({ class: 'dot dot-on', delay: (8 % 7) * 160 });
	});

	it('staggers lit dots along the diagonal', () => {
		const dots = glyphDots('.#...|#....|.....|.....|.....');

		expect(dots[1].delay).toBe(160);
		expect(dots[5].delay).toBe(160);
	});
});

describe('ringField', () => {
	it('puts a glowing core in the center', () => {
		const field = ringField(5);

		expect(field).toHaveLength(25);
		expect(field[12]).toEqual({ class: 'dot dot-core', delay: 0 });
		expect(
			field.filter((dot) => dot.class === 'dot dot-core'),
		).toHaveLength(1);
	});

	it('lights a checkerboard that pulses inward, outer rings first', () => {
		const field = ringField(5);

		expect(field[0]).toEqual({ class: 'dot dot-on', delay: 0 });
		expect(field[6]).toEqual({ class: 'dot dot-on', delay: 160 });
		expect(field[1]).toEqual({ class: 'dot', delay: 0 });
	});
});

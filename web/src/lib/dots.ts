// `class` uses the dot classes from style.css; `delay` staggers the pulse (ms).
export type Dot = { class: 'dot' | 'dot dot-on' | 'dot dot-core'; delay: number };

const SWEEP_STEPS = 7;
const STEP_MS = 160;

// A 5×5 glyph written as rows of '#' (lit) and '.' (dim), separated by '|'.
// Lit dots pulse in a diagonal sweep; dim dots stay faint.
export function glyphDots(glyph: string): Array<Dot> {
	return [...glyph.replaceAll('|', '')].map((cell, index) => {
		if (cell !== '#') {
			return { class: 'dot', delay: 0 };
		}

		const step = ((index % 5) + Math.floor(index / 5)) % SWEEP_STEPS;

		return { class: 'dot dot-on', delay: step * STEP_MS };
	});
}

// A square field whose rings pulse inward to a glowing center: many sources, one center.
export function ringField(size: number): Array<Dot> {
	const mid = (size - 1) / 2;
	const cells: Array<Dot> = [];

	for (let y = 0; y < size; y++) {
		for (let x = 0; x < size; x++) {
			const ring = Math.max(Math.abs(x - mid), Math.abs(y - mid));

			if (ring === 0) {
				cells.push({ class: 'dot dot-core', delay: 0 });
			} else if ((x + y) % 2 === 0) {
				cells.push({ class: 'dot dot-on', delay: (mid - ring) * STEP_MS });
			} else {
				cells.push({ class: 'dot', delay: 0 });
			}
		}
	}

	return cells;
}

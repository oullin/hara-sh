import { timingSafeEqual } from 'node:crypto';

// Workers expose crypto.subtle.timingSafeEqual; Node only has it on node:crypto.
if (!('timingSafeEqual' in crypto.subtle)) {
	Object.defineProperty(crypto.subtle, 'timingSafeEqual', {
		value: (a: Uint8Array, b: Uint8Array) => timingSafeEqual(a, b),
	});
}

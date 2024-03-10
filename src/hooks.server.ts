import { getCachedPageByPath, setCachedPageByPath } from '$lib/server/cache/dragonfly';
import type { Handle } from '@sveltejs/kit';

const CACHEABLE_PATH = /boss-kills\/[0-9]+/;
export const handle: Handle = async ({ event, resolve }) => {
	// cache mw start
	// console.log(event.route);
	const path = event.url.pathname + event.url.search;
	if (CACHEABLE_PATH.test(path)) {
		try {
			const cached = await getCachedPageByPath(path);
			if (cached) {
				return new Response(cached, {
					headers: {
						'Content-Type': 'text/html',
						'X-Cache-Hit': '1'
					}
				});
			} else {
				const response = await resolve(event);
				response.headers.append('X-Cache-Miss', '1');
				setCachedPageByPath(path, await response.clone().text()).catch(() => {});
				return response;
			}
		} catch (e) {
			console.error(e);
		}
	}
	// cache mw end

	const response = await resolve(event);
	return response;
};

import { DRAGONFLY_HOST, DRAGONFLY_PORT } from '$env/static/private';
import Redis from 'ioredis';

let globalDf: Redis | null = null;
export const createDragonflyClient = () => {
	if (globalDf === null) {
		globalDf = new Redis({
			host: DRAGONFLY_HOST,
			port: Number(DRAGONFLY_PORT),
			connectTimeout: 1_000
		});
	}
	return globalDf!;
};

const byPathKey = (path: string) => `page-by-path-${path}`;
export const getCachedPageByPath = async (path: string) => {
	const client = createDragonflyClient();
	const key = byPathKey(path);
	return client.get(key);
};

export const setCachedPageByPath = (path: string, content: string) => {
	const client = createDragonflyClient();
	const key = byPathKey(path);
	return client.setex(key, 10 * 60, content);
};

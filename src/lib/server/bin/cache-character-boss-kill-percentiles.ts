import { difficultiesByExpansion, difficultyToString, isRaidDifficulty } from '$lib/model';
import { realmToExpansion } from '$lib/realm';
import { findBossKills } from '../db/boss-kill';

import { findBossKillPlayers } from '../db/boss-kill-player';
import { createConnection } from '../db/index';
import { realmTable } from '../db/schema/realm.schema';
import { findBosses, setBossPercentilesPerPlayer } from '../model/boss.model';

try {
	console.log('Start');
	const db = await createConnection();
	const realms = await db.select().from(realmTable);
	for (const realm of realms) {
		const realmStart = performance.now();
		const expansion = realmToExpansion(realm.name);
		const diffs = Object.values<number>(difficultiesByExpansion(expansion) ?? {}).filter((diff) =>
			isRaidDifficulty(expansion, diff)
		);

		console.log(`Realm ${realm.name} started`);

		for (const boss of await findBosses({ realm: realm.name })) {
			const bossStart = performance.now();
			console.log(`Boss ${boss.name} - started`);

			for (const difficulty of diffs) {
				const diffStr = difficultyToString(expansion, difficulty);
				const diffStart = performance.now();
				console.log(`  difficulty: ${diffStr} started`);

				const bosskills = await findBossKills({
					realm: realm.name,
					bossId: boss.id,
					difficulty
				});

				console.log(`    found: ${bosskills.length} bosskills`);
				let bkSum = 0;
				let bkCount = 0;
				for (const bk of bosskills) {
					const players = await findBossKillPlayers({ bossKillId: bk.id });
					if (players.length > 0) {
						const bkStart = performance.now();
						await setBossPercentilesPerPlayer({
							bossKillRemoteId: bk.remoteId,
							realm: realm.name,
							bossId: boss.id,
							difficulty,
							players
						});
						const bkEnd = performance.now() - bkStart;
						bkSum += bkEnd;
						bkCount++;
					}
				}

				const bkAvg = bkCount > 0 ? bkSum / bkCount : 0;
				console.log(`    bosskill avg: ${bkAvg.toLocaleString()}ms`);
				console.log(`    bosskill total: ${bkSum.toLocaleString()}ms`);

				const diffEnd = performance.now() - diffStart;
				console.log(`  difficulty: ${diffStr} done, took ${diffEnd.toLocaleString()}ms`);
			}
			const bossEnd = performance.now() - bossStart;
			console.log(`Boss ${boss.name} - done, took ${bossEnd.toLocaleString()}ms`);
		}

		console.log(`Realm ${realm.name} caching`);

		const realmEnd = performance.now() - realmStart;
		console.log(`Realm ${realm.name} done, took: ${realmEnd.toLocaleString()}ms`);
	}

	console.log('Done');
	process.exit(0);
} catch (e) {
	console.error(e);
	process.exit(1);
}

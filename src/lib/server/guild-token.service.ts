import { SECRET_TOKEN_GUILD } from '$env/static/private';
import { links } from '$lib/links';
import { realmIsPublic } from '$lib/realm';
import type { Cookies } from '@sveltejs/kit';
import { error } from '@sveltejs/kit';
import { createHash } from 'node:crypto';

export const createGuildToken = (guild: string): string => {
	const hash = createHash('sha256');
	hash.update(SECRET_TOKEN_GUILD + guild);
	return hash.digest('hex');
};

type VerifyGuildTokenArgs = {
	guild: string;
	realm: string;
	token: string;
};
export const verifyGuildToken = ({ realm, token, guild }: VerifyGuildTokenArgs): boolean => {
	if (realmIsPublic(realm)) {
		return true;
	}
	return token === createGuildToken(guild);
};

type VerifyGuildTokenFromCookiesArgs = {
	guild: string;
	realm: string;
	cookies: Cookies;
};
export const verifyGuildTokenFromCookie = ({
	realm,
	guild,
	cookies
}: VerifyGuildTokenFromCookiesArgs): boolean => {
	return verifyGuildToken({ realm, guild, token: cookies.get('guild-token') ?? '' });
};

export const assertGuildTokenFromCookie = (args: VerifyGuildTokenFromCookiesArgs) => {
	if (verifyGuildTokenFromCookie(args) === false) {
		throw error(403, {
			message: `Fobidden`,
			// @ts-ignore
			details: `Change your guild token if you want to see guild ${args.guild} data`,
			link: links.guildToken(args.realm),
			linkText: 'Set your guild token'
		});
	}
};

import { browser } from '$app/environment';

// import { inject } from '@vercel/analytics';
// inject({ mode: dev ? 'development' : 'production' });

if (browser) {
	const recordDimensions = () => {
		const w = screen?.availWidth ?? window.innerWidth;
		const h = screen?.availHeight ?? window.innerHeight;
		document.cookie = `wiw=${w}; Path=/; SameSite=Lax; Secure`;
		document.cookie = `wih=${h}; Path=/; SameSite=Lax; Secure`;
	};

	recordDimensions();
	window.addEventListener('resize', recordDimensions);
}

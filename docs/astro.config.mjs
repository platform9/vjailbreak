// @ts-check
import starlight from '@astrojs/starlight';
import { defineConfig } from "astro/config";

// https://astro.build/config
export default defineConfig({
	site: process.env.SITE_URL || 'https://platform9.github.io',
	base: '/' + (process.env.BASE || ''),
	trailingSlash: "always",
	integrations: [
		starlight({
			title: 'vJailbreak',
			editLink: {
				baseUrl: 'https://platform9.github.io/vjailbreak/',
			},
			social: {
				github: 'https://github.com/platform9/vjailbreak',
			},
			logo: {
				src: './src/assets/logo.jpg',
				replacesTitle: true,
			},
			sidebar: [
				{
					label: 'Introduction',
					items: [
						// Each item here is one entry in the navigation menu.
						// manually done so to we can keep the order
						{ label: 'What is vJailbreak', slug: 'introduction/what_is_vjailbreak' },
						{ label: 'Components', slug: 'introduction/components' },
						{ label: 'Pre-requisites', slug: 'introduction/prerequisites' },
						{ label: 'Getting Started', slug: 'introduction/getting_started' },
					],
				},
				{
					label: 'Guide',
					autogenerate: { directory: 'guides' },
				},
				{
					label: 'Release Notes',
					autogenerate: { directory: 'release_docs' },
				},
			],
		}),
	],
});

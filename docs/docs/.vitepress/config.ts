import { defineConfig } from 'vitepress'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const sidebarFile = path.resolve(__dirname, 'sidebar.generated.json')

let generatedConfig = { sidebar: {}, nav: [], latestVersion: '0.2.0' }
if (fs.existsSync(sidebarFile)) {
  try {
    generatedConfig = JSON.parse(fs.readFileSync(sidebarFile, 'utf-8'))
  } catch (e) {
    console.error('Failed to parse sidebar.generated.json', e)
  }
}

// https://vitepress.vuejs.org/config/app-configs
export default defineConfig({
  title: 'Lone',
  description: 'A statically typed systems language compiling directly to WebAssembly',
  base: '/',
  lastUpdated: true,
  cleanUrls: true,

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Lone',

    nav: [
      { text: 'Home', link: '/' },
      ...(generatedConfig.nav || []),
      {
        text: 'Tools',
        items: [
          { text: 'Lone Compiler CLI', link: '/content/latest/toolchain/lone-cli' },
          { text: 'Lone Debugger', link: '/content/latest/toolchain/lone-debug' },
          { text: 'Runtime & WASI', link: '/content/latest/toolchain/wasm-and-wasi' },
        ],
      },
      { text: 'GitHub', link: 'https://github.com/lone-lang/lone' },
    ],

    sidebar: generatedConfig.sidebar || {},

    socialLinks: [
      { icon: 'github', link: 'https://github.com/lone-lang/lone' },
    ],

    footer: {
      message: 'Released under the Apache License, Version 2.0.',
      copyright: 'Copyright © 2026 Lone Contributors',
    },

    search: {
      provider: 'local',
    },
  },
})

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { applicationObfuscation } from './build/obfuscation'
export default defineConfig({ plugins:[vue(),applicationObfuscation()],base:'/assets/',build:{outDir:'../assets',emptyOutDir:true,manifest:true,sourcemap:false,minify:'esbuild',rollupOptions:{input:{public:'src/public.ts',studio:'src/studio.ts',setup:'src/setup.ts'},output:{entryFileNames:'[name]-[hash].js',chunkFileNames:'chunks/[name]-[hash].js',assetFileNames:'[name]-[hash][extname]'}}}})

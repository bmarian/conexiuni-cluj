import {mergeConfig} from 'vite'
import base from './vite.config.ts'

export default mergeConfig(base, {
  cacheDir: 'node_modules/.vite-areas',
  server: {port: 5175, strictPort: true, proxy: {'/api': {target: 'http://127.0.0.1:6700', changeOrigin: true}}},
})

import 'vite/modulepreload-polyfill'
import './public'
import './studio.css'
import './editor-layout.css'
import {createApp} from 'vue'
import App from './App.vue'
createApp(App).mount('#workbench')

import './management.css'
import './studio-shadcn.css'

import './mobileWorkbench.css'

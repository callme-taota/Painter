import 'vite/modulepreload-polyfill'
import './style.css'
import './controls.css'
import './header-scroll.css'
import {initScrollHeader} from './scrollHeader'
import {initTheme} from './theme'
import {createApp} from 'vue'
import InstallWizard from './components/InstallWizard.vue'
initTheme()
const host=document.querySelector('[data-install-wizard]')
if(host)createApp(InstallWizard).mount(host)

initScrollHeader()

import './shadcn-theme.css'

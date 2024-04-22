<script setup lang="ts">
import Layout from './layout/layout.vue'
import {useThemeStore} from './stores/theme'
import { storeToRefs } from 'pinia'
import { NConfigProvider, type GlobalThemeOverrides, darkTheme } from 'naive-ui'

const themeStore = useThemeStore()

const { nDark } = storeToRefs(themeStore)

const lightThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#67a7ec',
    primaryColorHover: '#378CE7',
    primaryColorPressed: '#67a7ec',
    primaryColorSuppl: '#378CE7',
    borderRadius: '12px',
    borderRadiusSmall: '6px',
    textColorBase: '#000',
  },
}

const darkThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#67a7ec',
    primaryColorHover: '#378CE7',
    primaryColorPressed: '#67a7ec',
    primaryColorSuppl: '#378CE7',
    borderRadius: '12px',
    borderRadiusSmall: '6px',
    textColorBase: '#fff',
  },
}

</script>

<template>
  <n-config-provider  :theme="nDark" :theme-overrides="nDark === null ? lightThemeOverrides : darkThemeOverrides">
    <layout></layout>
  </n-config-provider>
</template>

<style scoped></style>

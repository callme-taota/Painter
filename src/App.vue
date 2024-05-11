<script setup lang="ts">
import Layout from './layout/layout.vue'
import { useThemeStore } from './stores/theme'
import { storeToRefs } from 'pinia'
import { NConfigProvider, NMessageProvider, type GlobalThemeOverrides, darkTheme } from 'naive-ui'

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
  Steps: {
    indicatorColorProcess: '#378CE7FF',
    indicatorBorderColorFinish: '#378CE7FF',
  },
  Result: {
    iconColorSuccess: '#378CE7FF',
  }
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
  Steps: {
    indicatorColorProcess: '#378CE7FF',
    indicatorBorderColorFinish: '#378CE7FF',
  },
  Result: {
    iconColorSuccess: '#378CE7FF',
  }
}

</script>

<template>
  <n-config-provider :theme="nDark" :theme-overrides="nDark === null ? lightThemeOverrides : darkThemeOverrides">
    <n-message-provider>
      <layout></layout>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped></style>

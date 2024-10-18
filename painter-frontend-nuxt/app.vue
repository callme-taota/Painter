<script setup lang="ts">
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { NConfigProvider, NMessageProvider, type GlobalThemeOverrides, darkTheme } from 'naive-ui'
import { useThemeStore } from './stores/theme'
import { useInfoStore } from './stores/info'
import { useTitleStore } from '@/stores/title';
import { useUserStore } from './stores/user'

const themeStore = useThemeStore()
const InfoStore = useInfoStore()
const TitleStore = useTitleStore()
const UserStore = useUserStore()
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
  Steps: {
    indicatorColorProcess: '#378CE7FF',
    indicatorBorderColorFinish: '#378CE7FF',
  },
  Result: {
    iconColorSuccess: '#378CE7FF',
  },
  DataTable: {
    thColor: "#00000020",
    tdColor: "#00000010",
    tdColorHover: "#00000020",
  }
}

onMounted(() => {
  InfoStore.getInfo()
  TitleStore.setPrimaryTitle("Painter")
  UserStore.loadSelfData()
})

</script>

<template>
  <n-config-provider :theme="nDark" :theme-overrides="nDark === null ? lightThemeOverrides : darkThemeOverrides">
    <n-message-provider>
      <NuxtLayout>
        <NuxtPage />
      </NuxtLayout>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped></style>

import { defineStore } from 'pinia'
import { darkTheme } from 'naive-ui'
import type { BuiltInGlobalTheme } from 'naive-ui/es/themes/interface'

const localThemeName = 'painter-theme'

export enum Theme {
    Light = 'light',
    Dark = 'dark',
    Auto = 'auto',
}

export const useThemeStore = defineStore('themeStore', {
    state: () => ({
        isDeviceDarkTheme:
            typeof window !== 'undefined'
                ? window.matchMedia('(prefers-color-scheme: dark)').matches
                : null,
        themeSetting: themeFromLocalStorage() as Theme,
        nDark: null as BuiltInGlobalTheme | null,
        headerDisplay: true,
    }),
    actions: {
        Mounted() {
            this.themeSetting = themeFromLocalStorage() as Theme
            switch (this.themeSetting) {
                case Theme.Light:
                    this.SetThemeLight()
                    break
                case Theme.Dark:
                    this.SetThemeDark()
                    break
                case Theme.Auto:
                    this.AutoChangeTheme()
                    break
            }
        },
        SetThemeLight() {
            this.themeSetting = Theme.Light
            setLocalStorageTheme(Theme.Light)
            this.nDark = null
            if (typeof window !== 'undefined') {
                document.documentElement.removeAttribute('theme')
            }
            this.CleanAutoChangeTheme()
        },
        SetThemeDark() {
            this.themeSetting = Theme.Dark
            setLocalStorageTheme(Theme.Dark)
            this.nDark = darkTheme
            if (typeof window !== 'undefined') {
                document.documentElement.setAttribute('theme', 'dark')
            }
            this.CleanAutoChangeTheme()
        },
        SetThemeAuto() {
            this.themeSetting = Theme.Auto
            setLocalStorageTheme(Theme.Auto)
            if (this.isDeviceDarkTheme == true) {
                this.nDark = darkTheme
                if (typeof window !== 'undefined') {
                    document.documentElement.setAttribute('theme', 'dark')
                }
            } else {
                this.nDark = null
                if (typeof window !== 'undefined') {
                    document.documentElement.removeAttribute('theme')
                }
            }
            this.AutoChangeTheme()
        },
        AutoChangeTheme() {
            window
                .matchMedia('(prefers-color-scheme: dark)')
                .addEventListener('change', this.ChangeThemeHandler)
        },
        CleanAutoChangeTheme() {
            window
                .matchMedia('(prefers-color-scheme: dark)')
                .removeEventListener('change', this.ChangeThemeHandler)
        },
        ChangeThemeHandler() {
            if (typeof window == 'undefined') {
                return
            }
            if (
                window.matchMedia('(prefers-color-scheme: dark)').matches ==
                true
            ) {
                document.documentElement.setAttribute('theme', 'dark')
                this.nDark = darkTheme
            } else {
                document.documentElement.removeAttribute('theme')
                this.nDark = null
            }
        },
        hideHeader() {
            this.headerDisplay = false
        },
        showHeader() {
            this.headerDisplay = true
        },
    },
    getters: {
        isDarkTheme(): any {
            if (this.themeSetting == Theme.Auto) return this.isDeviceDarkTheme
            return this.themeSetting == Theme.Dark ? true : false
        },
        currentTheme(): number {
            if (this.themeSetting == Theme.Auto) {
                return 1
            } else if (this.themeSetting == Theme.Light) {
                return 2
            }
            return 3
        },
    },
})

const themeFromLocalStorage = () => {
    if (typeof window !== 'undefined' && window.localStorage) {
        const theme = localStorage.getItem(localThemeName)
        if (theme) {
            return theme as Theme
        }
    }
    return Theme.Auto
}

const setLocalStorageTheme = (theme: Theme) => {
    if (typeof window !== 'undefined' && window.localStorage) {
        localStorage.setItem(localThemeName, theme)
    }
}

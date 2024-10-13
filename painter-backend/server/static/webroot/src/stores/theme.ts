import { defineStore } from "pinia";
import { darkTheme } from 'naive-ui'
import type { BuiltInGlobalTheme } from "naive-ui/es/themes/interface";

export const useThemeStore = defineStore("themeStore", {
    state: () => ({
        isDeviceDarkTheme: window.matchMedia('(prefers-color-scheme: dark)').matches,
        themeSetting: "auto",
        nDark: null as BuiltInGlobalTheme | null,
        headerDisplay: true
    }),
    actions: {
        SetThemeLight() {
            this.themeSetting = "light"
            this.nDark = null
            document.documentElement.removeAttribute('theme')
            this.CleanAutoChangeTheme()
        },
        SetThemeDark() {
            this.themeSetting = "dark"
            this.nDark = darkTheme
            document.documentElement.setAttribute('theme', 'dark');
            this.CleanAutoChangeTheme()
        },
        SetThemeAuto() {
            this.themeSetting = "auto"
            if (this.isDeviceDarkTheme == true) {
                this.nDark = darkTheme
                document.documentElement.setAttribute('theme', 'dark');
            } else {
                this.nDark = null
                document.documentElement.removeAttribute('theme')
            }
            this.AutoChangeTheme()
        },
        AutoChangeTheme() {
            window.matchMedia('(prefers-color-scheme: dark)').addEventListener("change", this.ChangeThemeHandler)
        },
        CleanAutoChangeTheme() {
            window.matchMedia('(prefers-color-scheme: dark)').removeEventListener("change", this.ChangeThemeHandler)
        },
        ChangeThemeHandler() {
            if (window.matchMedia('(prefers-color-scheme: dark)').matches == true) {
                document.documentElement.setAttribute('theme', 'dark');
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
        }
    },
    getters: {
        isDarkTheme(): any {
            if (this.themeSetting == "auto") return this.isDeviceDarkTheme
            return this.themeSetting == "dark" ? true : false
        },
        currentTheme(): number {
            if (this.themeSetting == "auto") {
                return 1
            } else if (this.themeSetting == "light") {
                return 2
            }
            return 3
        }
    }
})
